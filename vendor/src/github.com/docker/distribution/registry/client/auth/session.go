package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/transport"
)

// AuthenticationHandler is an interface for authorizing a request from
// params from a "WWW-Authenicate" header for a single scheme.
type AuthenticationHandler interface {
	// Scheme returns the scheme as expected from the "WWW-Authenicate" header.
	Scheme() string

	// AuthorizeRequest adds the authorization header to a request (if needed)
	// using the parameters from "WWW-Authenticate" method. The parameters
	// values depend on the scheme.
	AuthorizeRequest(req *http.Request, params map[string]string) error
}

// CredentialStore is an interface for getting credentials for
// a given URL
type CredentialStore interface {
	// Basic returns basic auth for the given URL
	Basic(*url.URL) (string, string)
}

// NewAuthorizer creates an authorizer which can handle multiple authentication
// schemes. The handlers are tried in order, the higher priority authentication
// methods should be first. The challengeMap holds a list of challenges for
// a given root API endpoint (for example "https://registry-1.docker.io/v2/").
func NewAuthorizer(manager ChallengeManager, handlers ...AuthenticationHandler) transport.RequestModifier {
	return &endpointAuthorizer{
		challenges: manager,
		handlers:   handlers,
	}
}

type endpointAuthorizer struct {
	challenges ChallengeManager
	handlers   []AuthenticationHandler
	transport  http.RoundTripper
}

func (ea *endpointAuthorizer) ModifyRequest(req *http.Request) error {
	v2Root := strings.Index(req.URL.Path, "/v2/")
	if v2Root == -1 {
		return nil
	}

	ping := url.URL{
		Host:   req.URL.Host,
		Scheme: req.URL.Scheme,
		Path:   req.URL.Path[:v2Root+4],
	}

	pingEndpoint := ping.String()

	challenges, err := ea.challenges.GetChallenges(pingEndpoint)
	if err != nil {
		return err
	}

	if len(challenges) > 0 {
		for _, handler := range ea.handlers {
			for _, challenge := range challenges {
				if challenge.Scheme != handler.Scheme() {
					continue
				}
				if err := handler.AuthorizeRequest(req, challenge.Parameters); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

type tokenHandler struct {
	header    http.Header
	creds     CredentialStore
	scope     tokenScope
	transport http.RoundTripper

	tokenLock       sync.Mutex
	tokenCache      string
	tokenExpiration time.Time
}

// tokenScope represents the scope at which a token will be requested.
// This represents a specific action on a registry resource.
type tokenScope struct {
	Resource string
	Scope    string
	Actions  []string
}

func (ts tokenScope) String() string {
	return fmt.Sprintf("%s:%s:%s", ts.Resource, ts.Scope, strings.Join(ts.Actions, ","))
}

// NewTokenHandler creates a new AuthenicationHandler which supports
// fetching tokens from a remote token server.
func NewTokenHandler(transport http.RoundTripper, creds CredentialStore, scope string, actions ...string) AuthenticationHandler {
	return &tokenHandler{
		transport: transport,
		creds:     creds,
		scope: tokenScope{
			Resource: "repository",
			Scope:    scope,
			Actions:  actions,
		},
	}
}

func (th *tokenHandler) client() *http.Client {
	return &http.Client{
		Transport: th.transport,
		Timeout:   15 * time.Second,
	}
}

func (th *tokenHandler) Scheme() string {
	return "bearer"
}

func (th *tokenHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	if err := th.refreshToken(params); err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", th.tokenCache))

	return nil
}

func (th *tokenHandler) refreshToken(params map[string]string) error {
	th.tokenLock.Lock()
	defer th.tokenLock.Unlock()
	now := time.Now()
	if now.After(th.tokenExpiration) {
		token, err := th.fetchToken(params)
		if err != nil {
			return err
		}
		th.tokenCache = token
		th.tokenExpiration = now.Add(time.Minute)
	}

	return nil
}

type tokenResponse struct {
	Token string `json:"token"`
}

func (th *tokenHandler) fetchToken(params map[string]string) (token string, err error) {
	//log.Debugf("Getting bearer token with %s for %s", challenge.Parameters, ta.auth.Username)
	realm, ok := params["realm"]
	if !ok {
		return "", errors.New("no realm specified for token auth challenge")
	}

	// TODO(dmcgowan): Handle empty scheme

	realmURL, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("invalid token auth challenge realm: %s", err)
	}

	req, err := http.NewRequest("GET", realmURL.String(), nil)
	if err != nil {
		return "", err
	}

	reqParams := req.URL.Query()
	service := params["service"]
	scope := th.scope.String()

	if service != "" {
		reqParams.Add("service", service)
	}

	for _, scopeField := range strings.Fields(scope) {
		reqParams.Add("scope", scopeField)
	}

	if th.creds != nil {
		username, password := th.creds.Basic(realmURL)
		if username != "" && password != "" {
			reqParams.Add("account", username)
			req.SetBasicAuth(username, password)
		}
	}

	req.URL.RawQuery = reqParams.Encode()

	resp, err := th.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if !client.SuccessStatus(resp.StatusCode) {
		return "", fmt.Errorf("token auth attempt for registry: %s request failed with status: %d %s", req.URL, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	decoder := json.NewDecoder(resp.Body)

	tr := new(tokenResponse)
	if err = decoder.Decode(tr); err != nil {
		return "", fmt.Errorf("unable to decode token response: %s", err)
	}

	if tr.Token == "" {
		return "", errors.New("authorization server did not include a token in the response")
	}

	return tr.Token, nil
}

type basicHandler struct {
	creds CredentialStore
}

// NewBasicHandler creaters a new authentiation handler which adds
// basic authentication credentials to a request.
func NewBasicHandler(creds CredentialStore) AuthenticationHandler {
	return &basicHandler{
		creds: creds,
	}
}

func (*basicHandler) Scheme() string {
	return "basic"
}

func (bh *basicHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	if bh.creds != nil {
		username, password := bh.creds.Basic(req.URL)
		if username != "" && password != "" {
			req.SetBasicAuth(username, password)
			return nil
		}
	}
	return errors.New("no basic auth credentials")
}
