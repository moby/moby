package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Authorizer is used to apply Authorization to an HTTP request
type Authorizer interface {
	// Authorizer updates an HTTP request with the needed authorization
	Authorize(req *http.Request) error
}

// AuthenticationHandler is an interface for authorizing a request from
// params from a "WWW-Authenicate" header for a single scheme.
type AuthenticationHandler interface {
	Scheme() string
	AuthorizeRequest(req *http.Request, params map[string]string) error
}

// CredentialStore is an interface for getting credentials for
// a given URL
type CredentialStore interface {
	// Basic returns basic auth for the given URL
	Basic(*url.URL) (string, string)
}

// RepositoryConfig holds the base configuration needed to communicate
// with a registry including a method of authorization and HTTP headers.
type RepositoryConfig struct {
	Header       http.Header
	AuthSource   Authorizer
	AllowMirrors bool
}

// HTTPClient returns a new HTTP client configured for this configuration
func (rc *RepositoryConfig) HTTPClient() (*http.Client, error) {
	// TODO(dmcgowan): create base http.Transport with proper TLS configuration

	transport := &Transport{
		ExtraHeader: rc.Header,
		AuthSource:  rc.AuthSource,
	}

	client := &http.Client{
		Transport: transport,
	}

	return client, nil
}

// NewTokenAuthorizer returns an authorizer which is capable of getting a token
// from a token server. The expected authorization method will be discovered
// by the authorizer, getting the token server endpoint from the URL being
// requested. Basic authentication may either be done to the token source or
// directly with the requested endpoint depending on the endpoint's
// WWW-Authenticate header.
func NewTokenAuthorizer(creds CredentialStore, header http.Header, scope TokenScope) Authorizer {
	return &tokenAuthorizer{
		header:     header,
		challenges: map[string]map[string]authorizationChallenge{},
		handlers: []AuthenticationHandler{
			NewTokenHandler(creds, scope, header),
			NewBasicHandler(creds),
		},
	}
}

// NewAuthorizer creates an authorizer which can handle multiple authentication
// schemes. The handlers are tried in order, the higher priority authentication
// methods should be first.
func NewAuthorizer(header http.Header, handlers ...AuthenticationHandler) Authorizer {
	return &tokenAuthorizer{
		header:     header,
		challenges: map[string]map[string]authorizationChallenge{},
		handlers:   handlers,
	}
}

type tokenAuthorizer struct {
	header     http.Header
	challenges map[string]map[string]authorizationChallenge
	handlers   []AuthenticationHandler
}

func (ta *tokenAuthorizer) client() *http.Client {
	// TODO(dmcgowan): Use same transport which has properly configured TLS
	return &http.Client{Transport: &Transport{ExtraHeader: ta.header}}
}

func (ta *tokenAuthorizer) ping(endpoint string) (map[string]authorizationChallenge, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := ta.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// TODO(dmcgowan): Add version string which would allow skipping this section
	var supportsV2 bool
HeaderLoop:
	for _, supportedVersions := range resp.Header[http.CanonicalHeaderKey("Docker-Distribution-API-Version")] {
		for _, versionName := range strings.Fields(supportedVersions) {
			if versionName == "registry/2.0" {
				supportsV2 = true
				break HeaderLoop
			}
		}
	}

	if !supportsV2 {
		return nil, fmt.Errorf("%s does not appear to be a v2 registry endpoint", endpoint)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		// Parse the WWW-Authenticate Header and store the challenges
		// on this endpoint object.
		return parseAuthHeader(resp.Header), nil
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to get valid ping response: %d", resp.StatusCode)
	}

	return nil, nil
}

func (ta *tokenAuthorizer) Authorize(req *http.Request) error {
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

	challenges, ok := ta.challenges[pingEndpoint]
	if !ok {
		var err error
		challenges, err = ta.ping(pingEndpoint)
		if err != nil {
			return err
		}
		ta.challenges[pingEndpoint] = challenges
	}

	for _, handler := range ta.handlers {
		challenge, ok := challenges[handler.Scheme()]
		if ok {
			if err := handler.AuthorizeRequest(req, challenge.Parameters); err != nil {
				return err
			}
		}
	}

	return nil
}

type tokenHandler struct {
	header http.Header
	creds  CredentialStore
	scope  TokenScope

	tokenLock       sync.Mutex
	tokenCache      string
	tokenExpiration time.Time
}

// TokenScope represents the scope at which a token will be requested.
// This represents a specific action on a registry resource.
type TokenScope struct {
	Resource string
	Scope    string
	Actions  []string
}

// NewTokenHandler creates a new AuthenicationHandler which supports
// fetching tokens from a remote token server.
func NewTokenHandler(creds CredentialStore, scope TokenScope, header http.Header) AuthenticationHandler {
	return &tokenHandler{
		header: header,
		creds:  creds,
		scope:  scope,
	}
}

func (ts TokenScope) String() string {
	return fmt.Sprintf("%s:%s:%s", ts.Resource, ts.Scope, strings.Join(ts.Actions, ","))
}

func (ts *tokenHandler) client() *http.Client {
	// TODO(dmcgowan): Use same transport which has properly configured TLS
	return &http.Client{Transport: &Transport{ExtraHeader: ts.header}}
}

func (ts *tokenHandler) Scheme() string {
	return "bearer"
}

func (ts *tokenHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	if err := ts.refreshToken(params); err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ts.tokenCache))

	return nil
}

func (ts *tokenHandler) refreshToken(params map[string]string) error {
	ts.tokenLock.Lock()
	defer ts.tokenLock.Unlock()
	now := time.Now()
	if now.After(ts.tokenExpiration) {
		token, err := ts.fetchToken(params)
		if err != nil {
			return err
		}
		ts.tokenCache = token
		ts.tokenExpiration = now.Add(time.Minute)
	}

	return nil
}

type tokenResponse struct {
	Token string `json:"token"`
}

func (ts *tokenHandler) fetchToken(params map[string]string) (token string, err error) {
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
	scope := ts.scope.String()

	if service != "" {
		reqParams.Add("service", service)
	}

	for _, scopeField := range strings.Fields(scope) {
		reqParams.Add("scope", scopeField)
	}

	if ts.creds != nil {
		username, password := ts.creds.Basic(realmURL)
		if username != "" && password != "" {
			reqParams.Add("account", username)
			req.SetBasicAuth(username, password)
		}
	}

	req.URL.RawQuery = reqParams.Encode()

	resp, err := ts.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
