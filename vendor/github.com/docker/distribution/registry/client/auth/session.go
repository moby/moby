package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
)

var (
	// ErrNoBasicAuthCredentials is returned if a request can't be authorized with
	// basic auth due to lack of credentials.
	ErrNoBasicAuthCredentials = errors.New("no basic auth credentials")

	// ErrNoToken is returned if a request is successful but the body does not
	// contain an authorization token.
	ErrNoToken = errors.New("authorization server did not include a token in the response")
)

const defaultClientID = "registry-client"

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

// Authenticator provides authentication callbacks such as accesstoken provider or user/password
type Authenticator interface {
	GetBasicAuthInfo() (username, password string)
	HasBasicAuthInfo() bool
	HasUsername() bool
	GetUsername() string
	HasIdentityTokenInfo() bool
	GetIdentityTokenInfo() (username, identityToken string)
	HasPassthroughToken() bool
	GetPassthroughToken() string
	GetAccessToken(realm, service, clientID string, scopes []string, skipCache, offlineAccess, forceOAuth bool) (token string, err error)
}

// NewAuthorizer creates an authorizer which can handle multiple authentication
// schemes. The handlers are tried in order, the higher priority authentication
// methods should be first. The challengeMap holds a list of challenges for
// a given root API endpoint (for example "https://registry-1.docker.io/v2/").
func NewAuthorizer(manager challenge.Manager, handlers ...AuthenticationHandler) transport.RequestModifier {
	return &endpointAuthorizer{
		challenges: manager,
		handlers:   handlers,
	}
}

type endpointAuthorizer struct {
	challenges challenge.Manager
	handlers   []AuthenticationHandler
	transport  http.RoundTripper
}

func (ea *endpointAuthorizer) ModifyRequest(req *http.Request) error {
	pingPath := req.URL.Path
	if v2Root := strings.Index(req.URL.Path, "/v2/"); v2Root != -1 {
		pingPath = pingPath[:v2Root+4]
	} else if v1Root := strings.Index(req.URL.Path, "/v1/"); v1Root != -1 {
		pingPath = pingPath[:v1Root] + "/v2/"
	} else {
		return nil
	}

	ping := url.URL{
		Host:   req.URL.Host,
		Scheme: req.URL.Scheme,
		Path:   pingPath,
	}

	challenges, err := ea.challenges.GetChallenges(ping)
	if err != nil {
		return err
	}

	if len(challenges) > 0 {
		for _, handler := range ea.handlers {
			for _, c := range challenges {
				if c.Scheme != handler.Scheme() {
					continue
				}
				if err := handler.AuthorizeRequest(req, c.Parameters); err != nil {
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
	header        http.Header
	authenticator Authenticator
	transport     http.RoundTripper
	clock         clock

	offlineAccess bool
	forceOAuth    bool
	clientID      string
	scopes        []Scope

	tokenLock sync.Mutex
}

// Scope is a type which is serializable to a string
// using the allow scope grammar.
type Scope interface {
	String() string
}

// RepositoryScope represents a token scope for access
// to a repository.
type RepositoryScope struct {
	Repository string
	Class      string
	Actions    []string
}

// String returns the string representation of the repository
// using the scope grammar
func (rs RepositoryScope) String() string {
	repoType := "repository"
	// Keep existing format for image class to maintain backwards compatibility
	// with authorization servers which do not support the expanded grammar.
	if rs.Class != "" && rs.Class != "image" {
		repoType = fmt.Sprintf("%s(%s)", repoType, rs.Class)
	}
	return fmt.Sprintf("%s:%s:%s", repoType, rs.Repository, strings.Join(rs.Actions, ","))
}

// RegistryScope represents a token scope for access
// to resources in the registry.
type RegistryScope struct {
	Name    string
	Actions []string
}

// String returns the string representation of the user
// using the scope grammar
func (rs RegistryScope) String() string {
	return fmt.Sprintf("registry:%s:%s", rs.Name, strings.Join(rs.Actions, ","))
}

// TokenHandlerOptions is used to configure a new token handler
type TokenHandlerOptions struct {
	Transport     http.RoundTripper
	Authenticator Authenticator

	OfflineAccess bool
	ForceOAuth    bool
	ClientID      string
	Scopes        []Scope
}

// An implementation of clock for providing real time data.
type realClock struct{}

// Now implements clock
func (realClock) Now() time.Time { return time.Now() }

// NewTokenHandler creates a new AuthenicationHandler which supports
// fetching tokens from a remote token server.
func NewTokenHandler(transport http.RoundTripper, authenticator Authenticator, scope string, actions ...string) AuthenticationHandler {
	// Create options...
	return NewTokenHandlerWithOptions(TokenHandlerOptions{
		Transport:     transport,
		Authenticator: authenticator,
		Scopes: []Scope{
			RepositoryScope{
				Repository: scope,
				Actions:    actions,
			},
		},
	})
}

// NewTokenHandlerWithOptions creates a new token handler using the provided
// options structure.
func NewTokenHandlerWithOptions(options TokenHandlerOptions) AuthenticationHandler {
	handler := &tokenHandler{
		transport:     options.Transport,
		authenticator: options.Authenticator,
		offlineAccess: options.OfflineAccess,
		forceOAuth:    options.ForceOAuth,
		clientID:      options.ClientID,
		scopes:        options.Scopes,
		clock:         realClock{},
	}

	return handler
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
		additionalScopes = append(additionalScopes, RepositoryScope{
			Repository: fromParam,
			Actions:    []string{"pull"},
		}.String())
	}

	token, err := th.getToken(params, additionalScopes...)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	return nil
}

func (th *tokenHandler) getToken(params map[string]string, additionalScopes ...string) (string, error) {
	th.tokenLock.Lock()
	defer th.tokenLock.Unlock()
	scopes := make([]string, 0, len(th.scopes)+len(additionalScopes))
	for _, scope := range th.scopes {
		scopes = append(scopes, scope.String())
	}
	var addedScopes bool
	for _, scope := range additionalScopes {
		scopes = append(scopes, scope)
		addedScopes = true
	}

	realm, ok := params["realm"]
	if !ok {
		return "", errors.New("no realm specified for token auth challenge")
	}

	// TODO(dmcgowan): Handle empty scheme and relative realm
	_, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("invalid token auth challenge realm: %s", err)
	}

	service := params["service"]

	return th.authenticator.GetAccessToken(realm, service, th.clientID, scopes, addedScopes, th.offlineAccess, th.forceOAuth)

}

type basicHandler struct {
	authenticator Authenticator
}

// NewBasicHandler creaters a new authentiation handler which adds
// basic authentication credentials to a request.
func NewBasicHandler(authenticator Authenticator) AuthenticationHandler {
	return &basicHandler{
		authenticator: authenticator,
	}
}

func (*basicHandler) Scheme() string {
	return "basic"
}

func (bh *basicHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	if bh.authenticator != nil {
		username, password := bh.authenticator.GetBasicAuthInfo()
		if username != "" && password != "" {
			req.SetBasicAuth(username, password)
			return nil
		}
	}
	return ErrNoBasicAuthCredentials
}
