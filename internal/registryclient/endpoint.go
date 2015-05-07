package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/api/v2"
)

// Authorizer is used to apply Authorization to an HTTP request
type Authorizer interface {
	// Authorizer updates an HTTP request with the needed authorization
	Authorize(req *http.Request) error
}

// CredentialStore is an interface for getting credentials for
// a given URL
type CredentialStore interface {
	// Basic returns basic auth for the given URL
	Basic(*url.URL) (string, string)
}

// RepositoryEndpoint represents a single host endpoint serving up
// the distribution API.
type RepositoryEndpoint struct {
	Endpoint string
	Mirror   bool

	Header      http.Header
	Credentials CredentialStore

	ub *v2.URLBuilder
}

type nullAuthorizer struct{}

func (na nullAuthorizer) Authorize(req *http.Request) error {
	return nil
}

type repositoryTransport struct {
	Transport  http.RoundTripper
	Header     http.Header
	Authorizer Authorizer
}

func (rt *repositoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqCopy := new(http.Request)
	*reqCopy = *req

	// Copy existing headers then static headers
	reqCopy.Header = make(http.Header, len(req.Header)+len(rt.Header))
	for k, s := range req.Header {
		reqCopy.Header[k] = append([]string(nil), s...)
	}
	for k, s := range rt.Header {
		reqCopy.Header[k] = append(reqCopy.Header[k], s...)
	}

	if rt.Authorizer != nil {
		if err := rt.Authorizer.Authorize(reqCopy); err != nil {
			return nil, err
		}
	}

	logrus.Debugf("HTTP: %s %s", req.Method, req.URL)

	if rt.Transport != nil {
		return rt.Transport.RoundTrip(reqCopy)
	}
	return http.DefaultTransport.RoundTrip(reqCopy)
}

type authTransport struct {
	Transport http.RoundTripper
	Header    http.Header
}

func (rt *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqCopy := new(http.Request)
	*reqCopy = *req

	// Copy existing headers then static headers
	reqCopy.Header = make(http.Header, len(req.Header)+len(rt.Header))
	for k, s := range req.Header {
		reqCopy.Header[k] = append([]string(nil), s...)
	}
	for k, s := range rt.Header {
		reqCopy.Header[k] = append(reqCopy.Header[k], s...)
	}

	logrus.Debugf("HTTP: %s %s", req.Method, req.URL)

	if rt.Transport != nil {
		return rt.Transport.RoundTrip(reqCopy)
	}
	return http.DefaultTransport.RoundTrip(reqCopy)
}

// URLBuilder returns a new URL builder
func (e *RepositoryEndpoint) URLBuilder() (*v2.URLBuilder, error) {
	if e.ub == nil {
		var err error
		e.ub, err = v2.NewURLBuilderFromString(e.Endpoint)
		if err != nil {
			return nil, err
		}
	}

	return e.ub, nil
}

// HTTPClient returns a new HTTP client configured for this endpoint
func (e *RepositoryEndpoint) HTTPClient(name string) (*http.Client, error) {
	// TODO(dmcgowan): create http.Transport

	transport := &repositoryTransport{
		Header: e.Header,
	}
	client := &http.Client{
		Transport: transport,
	}

	challenges, err := e.ping(client)
	if err != nil {
		return nil, err
	}
	actions := []string{"pull"}
	if !e.Mirror {
		actions = append(actions, "push")
	}

	transport.Authorizer = &endpointAuthorizer{
		client:     &http.Client{Transport: &authTransport{Header: e.Header}},
		challenges: challenges,
		creds:      e.Credentials,
		resource:   "repository",
		scope:      name,
		actions:    actions,
	}

	return client, nil
}

func (e *RepositoryEndpoint) ping(client *http.Client) ([]AuthorizationChallenge, error) {
	ub, err := e.URLBuilder()
	if err != nil {
		return nil, err
	}
	u, err := ub.BuildBaseURL()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header = make(http.Header, len(e.Header))
	for k, s := range e.Header {
		req.Header[k] = append([]string(nil), s...)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
		return nil, fmt.Errorf("%s does not appear to be a v2 registry endpoint", e.Endpoint)
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

type endpointAuthorizer struct {
	client     *http.Client
	challenges []AuthorizationChallenge
	creds      CredentialStore

	resource string
	scope    string
	actions  []string

	tokenLock       sync.Mutex
	tokenCache      string
	tokenExpiration time.Time
}

func (ta *endpointAuthorizer) Authorize(req *http.Request) error {
	token, err := ta.getToken()
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	} else if ta.creds != nil {
		username, password := ta.creds.Basic(req.URL)
		if username != "" && password != "" {
			req.SetBasicAuth(username, password)
		}
	}
	return nil
}

func (ta *endpointAuthorizer) getToken() (string, error) {
	ta.tokenLock.Lock()
	defer ta.tokenLock.Unlock()
	now := time.Now()
	if now.Before(ta.tokenExpiration) {
		//log.Debugf("Using cached token for %q", ta.auth.Username)
		return ta.tokenCache, nil
	}

	for _, challenge := range ta.challenges {
		switch strings.ToLower(challenge.Scheme) {
		case "basic":
			// no token necessary
		case "bearer":
			//log.Debugf("Getting bearer token with %s for %s", challenge.Parameters, ta.auth.Username)
			params := map[string]string{}
			for k, v := range challenge.Parameters {
				params[k] = v
			}
			params["scope"] = fmt.Sprintf("%s:%s:%s", ta.resource, ta.scope, strings.Join(ta.actions, ","))
			token, err := getToken(ta.creds, params, ta.client)
			if err != nil {
				return "", err
			}
			ta.tokenCache = token
			ta.tokenExpiration = now.Add(time.Minute)

			return token, nil
		default:
			//log.Infof("Unsupported auth scheme: %q", challenge.Scheme)
		}
	}

	// Do not expire cache since there are no challenges which use a token
	ta.tokenExpiration = time.Now().Add(time.Hour * 24)

	return "", nil
}
