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

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/transport"
)

// ErrNoBasicAuthCredentials is returned if a request can't be authorized with
// basic auth due to lack of credentials.
var ErrNoBasicAuthCredentials = errors.New("no basic auth credentials")

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

// This is the minimum duration a token can last (in seconds).
// A token must not live less than 60 seconds because older versions
// of the Docker client didn't read their expiration from the token
// response and assumed 60 seconds.  So to remain compatible with
// those implementations, a token must live at least this long.
const minimumTokenLifetimeSeconds = 60

// Private interface for time used by this package to enable tests to provide their own implementation.
type clock interface {
	Now() time.Time
}

type tokenHandler struct {
	header    http.Header
	creds     CredentialStore
	scope     tokenScope
	transport http.RoundTripper
	clock     clock

	tokenLock       sync.Mutex
	tokenCache      string
	tokenExpiration time.Time

	additionalScopes map[string]struct{}
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

// An implementation of clock for providing real time data.
type realClock struct{}

// Now implements clock
func (realClock) Now() time.Time { return time.Now() }

// NewTokenHandler creates a new AuthenicationHandler which supports
// fetching tokens from a remote token server.
func NewTokenHandler(transport http.RoundTripper, creds CredentialStore, scope string, actions ...string) AuthenticationHandler {
	return newTokenHandler(transport, creds, realClock{}, scope, actions...)
}

// newTokenHandler exposes the option to provide a clock to manipulate time in unit testing.
func newTokenHandler(transport http.RoundTripper, creds CredentialStore, c clock, scope string, actions ...string) AuthenticationHandler {
	return &tokenHandler{
		transport: transport,
		creds:     creds,
		clock:     c,
		scope: tokenScope{
			Resource: "repository",
			Scope:    scope,
			Actions:  actions,
		},
		additionalScopes: map[string]struct{}{},
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
	var additionalScopes []string
	if fromParam := req.URL.Query().Get("from"); fromParam != "" {
		additionalScopes = append(additionalScopes, tokenScope{
			Resource: "repository",
			Scope:    fromParam,
			Actions:  []string{"pull"},
		}.String())
	}
	if err := th.refreshToken(params, additionalScopes...); err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", th.tokenCache))

	return nil
}

func (th *tokenHandler) refreshToken(params map[string]string, additionalScopes ...string) error {
	th.tokenLock.Lock()
	defer th.tokenLock.Unlock()
	var addedScopes bool
	for _, scope := range additionalScopes {
		if _, ok := th.additionalScopes[scope]; !ok {
			th.additionalScopes[scope] = struct{}{}
			addedScopes = true
		}
	}
	now := th.clock.Now()
	if now.After(th.tokenExpiration) || addedScopes {
		tr, err := th.fetchToken(params)
		if err != nil {
			return err
		}
		th.tokenCache = tr.Token
		th.tokenExpiration = tr.IssuedAt.Add(time.Duration(tr.ExpiresIn) * time.Second)
	}

	return nil
}

type tokenResponse struct {
	Token       string    `json:"token"`
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"`
	IssuedAt    time.Time `json:"issued_at"`
}

func (th *tokenHandler) fetchToken(params map[string]string) (token *tokenResponse, err error) {
	realm, ok := params["realm"]
	if !ok {
		return nil, errors.New("no realm specified for token auth challenge")
	}

	// TODO(dmcgowan): Handle empty scheme

	realmURL, err := url.Parse(realm)
	if err != nil {
		return nil, fmt.Errorf("invalid token auth challenge realm: %s", err)
	}

	req, err := http.NewRequest("GET", realmURL.String(), nil)
	if err != nil {
		return nil, err
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

	for scope := range th.additionalScopes {
		reqParams.Add("scope", scope)
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
		return nil, err
	}
	defer resp.Body.Close()

	if !client.SuccessStatus(resp.StatusCode) {
		err := client.HandleErrorResponse(resp)
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)

	tr := new(tokenResponse)
	if err = decoder.Decode(tr); err != nil {
		return nil, fmt.Errorf("unable to decode token response: %s", err)
	}

	// `access_token` is equivalent to `token` and if both are specified
	// the choice is undefined.  Canonicalize `access_token` by sticking
	// things in `token`.
	if tr.AccessToken != "" {
		tr.Token = tr.AccessToken
	}

	if tr.Token == "" {
		return nil, errors.New("authorization server did not include a token in the response")
	}

	if tr.ExpiresIn < minimumTokenLifetimeSeconds {
		// The default/minimum lifetime.
		tr.ExpiresIn = minimumTokenLifetimeSeconds
		logrus.Debugf("Increasing token expiration to: %d seconds", tr.ExpiresIn)
	}

	if tr.IssuedAt.IsZero() {
		// issued_at is optional in the token response.
		tr.IssuedAt = th.clock.Now()
	}

	return tr, nil
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
	return ErrNoBasicAuthCredentials
}
