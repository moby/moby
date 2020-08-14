/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context/ctxhttp"
)

type dockerAuthorizer struct {
	credentials func(string) (string, string, error)

	client *http.Client
	header http.Header
	mu     sync.Mutex

	// indexed by host name
	handlers map[string]*authHandler
}

// NewAuthorizer creates a Docker authorizer using the provided function to
// get credentials for the token server or basic auth.
// Deprecated: Use NewDockerAuthorizer
func NewAuthorizer(client *http.Client, f func(string) (string, string, error)) Authorizer {
	return NewDockerAuthorizer(WithAuthClient(client), WithAuthCreds(f))
}

type authorizerConfig struct {
	credentials func(string) (string, string, error)
	client      *http.Client
	header      http.Header
}

// AuthorizerOpt configures an authorizer
type AuthorizerOpt func(*authorizerConfig)

// WithAuthClient provides the HTTP client for the authorizer
func WithAuthClient(client *http.Client) AuthorizerOpt {
	return func(opt *authorizerConfig) {
		opt.client = client
	}
}

// WithAuthCreds provides a credential function to the authorizer
func WithAuthCreds(creds func(string) (string, string, error)) AuthorizerOpt {
	return func(opt *authorizerConfig) {
		opt.credentials = creds
	}
}

// WithAuthHeader provides HTTP headers for authorization
func WithAuthHeader(hdr http.Header) AuthorizerOpt {
	return func(opt *authorizerConfig) {
		opt.header = hdr
	}
}

// NewDockerAuthorizer creates an authorizer using Docker's registry
// authentication spec.
// See https://docs.docker.com/registry/spec/auth/
func NewDockerAuthorizer(opts ...AuthorizerOpt) Authorizer {
	var ao authorizerConfig
	for _, opt := range opts {
		opt(&ao)
	}

	if ao.client == nil {
		ao.client = http.DefaultClient
	}

	return &dockerAuthorizer{
		credentials: ao.credentials,
		client:      ao.client,
		header:      ao.header,
		handlers:    make(map[string]*authHandler),
	}
}

// Authorize handles auth request.
func (a *dockerAuthorizer) Authorize(ctx context.Context, req *http.Request) error {
	// skip if there is no auth handler
	ah := a.getAuthHandler(req.URL.Host)
	if ah == nil {
		return nil
	}

	auth, err := ah.authorize(ctx)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", auth)
	return nil
}

func (a *dockerAuthorizer) getAuthHandler(host string) *authHandler {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.handlers[host]
}

func (a *dockerAuthorizer) AddResponses(ctx context.Context, responses []*http.Response) error {
	last := responses[len(responses)-1]
	host := last.Request.URL.Host

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range parseAuthHeader(last.Header) {
		if c.scheme == bearerAuth {
			if err := invalidAuthorization(c, responses); err != nil {
				delete(a.handlers, host)
				return err
			}

			// reuse existing handler
			//
			// assume that one registry will return the common
			// challenge information, including realm and service.
			// and the resource scope is only different part
			// which can be provided by each request.
			if _, ok := a.handlers[host]; ok {
				return nil
			}

			common, err := a.generateTokenOptions(ctx, host, c)
			if err != nil {
				return err
			}

			a.handlers[host] = newAuthHandler(a.client, a.header, c.scheme, common)
			return nil
		} else if c.scheme == basicAuth && a.credentials != nil {
			username, secret, err := a.credentials(host)
			if err != nil {
				return err
			}

			if username != "" && secret != "" {
				common := tokenOptions{
					username: username,
					secret:   secret,
				}

				a.handlers[host] = newAuthHandler(a.client, a.header, c.scheme, common)
				return nil
			}
		}
	}
	return errors.Wrap(errdefs.ErrNotImplemented, "failed to find supported auth scheme")
}

func (a *dockerAuthorizer) generateTokenOptions(ctx context.Context, host string, c challenge) (tokenOptions, error) {
	realm, ok := c.parameters["realm"]
	if !ok {
		return tokenOptions{}, errors.New("no realm specified for token auth challenge")
	}

	realmURL, err := url.Parse(realm)
	if err != nil {
		return tokenOptions{}, errors.Wrap(err, "invalid token auth challenge realm")
	}

	to := tokenOptions{
		realm:   realmURL.String(),
		service: c.parameters["service"],
	}

	scope, ok := c.parameters["scope"]
	if ok {
		to.scopes = append(to.scopes, scope)
	} else {
		log.G(ctx).WithField("host", host).Debug("no scope specified for token auth challenge")
	}

	if a.credentials != nil {
		to.username, to.secret, err = a.credentials(host)
		if err != nil {
			return tokenOptions{}, err
		}
	}
	return to, nil
}

// authResult is used to control limit rate.
type authResult struct {
	sync.WaitGroup
	token string
	err   error
}

// authHandler is used to handle auth request per registry server.
type authHandler struct {
	sync.Mutex

	header http.Header

	client *http.Client

	// only support basic and bearer schemes
	scheme authenticationScheme

	// common contains common challenge answer
	common tokenOptions

	// scopedTokens caches token indexed by scopes, which used in
	// bearer auth case
	scopedTokens map[string]*authResult
}

func newAuthHandler(client *http.Client, hdr http.Header, scheme authenticationScheme, opts tokenOptions) *authHandler {
	return &authHandler{
		header:       hdr,
		client:       client,
		scheme:       scheme,
		common:       opts,
		scopedTokens: map[string]*authResult{},
	}
}

func (ah *authHandler) authorize(ctx context.Context) (string, error) {
	switch ah.scheme {
	case basicAuth:
		return ah.doBasicAuth(ctx)
	case bearerAuth:
		return ah.doBearerAuth(ctx)
	default:
		return "", errors.Wrap(errdefs.ErrNotImplemented, "failed to find supported auth scheme")
	}
}

func (ah *authHandler) doBasicAuth(ctx context.Context) (string, error) {
	username, secret := ah.common.username, ah.common.secret

	if username == "" || secret == "" {
		return "", fmt.Errorf("failed to handle basic auth because missing username or secret")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + secret))
	return fmt.Sprintf("Basic %s", auth), nil
}

func (ah *authHandler) doBearerAuth(ctx context.Context) (string, error) {
	// copy common tokenOptions
	to := ah.common

	to.scopes = GetTokenScopes(ctx, to.scopes)

	// Docs: https://docs.docker.com/registry/spec/auth/scope
	scoped := strings.Join(to.scopes, " ")

	ah.Lock()
	if r, exist := ah.scopedTokens[scoped]; exist {
		ah.Unlock()
		r.Wait()
		return r.token, r.err
	}

	// only one fetch token job
	r := new(authResult)
	r.Add(1)
	ah.scopedTokens[scoped] = r
	ah.Unlock()

	// fetch token for the resource scope
	var (
		token string
		err   error
	)
	if to.secret != "" {
		// credential information is provided, use oauth POST endpoint
		token, err = ah.fetchTokenWithOAuth(ctx, to)
		err = errors.Wrap(err, "failed to fetch oauth token")
	} else {
		// do request anonymously
		token, err = ah.fetchToken(ctx, to)
		err = errors.Wrap(err, "failed to fetch anonymous token")
	}
	token = fmt.Sprintf("Bearer %s", token)

	r.token, r.err = token, err
	r.Done()
	return r.token, r.err
}

type tokenOptions struct {
	realm    string
	service  string
	scopes   []string
	username string
	secret   string
}

type postTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
	Scope        string    `json:"scope"`
}

func (ah *authHandler) fetchTokenWithOAuth(ctx context.Context, to tokenOptions) (string, error) {
	form := url.Values{}
	if len(to.scopes) > 0 {
		form.Set("scope", strings.Join(to.scopes, " "))
	}
	form.Set("service", to.service)
	// TODO: Allow setting client_id
	form.Set("client_id", "containerd-client")

	if to.username == "" {
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", to.secret)
	} else {
		form.Set("grant_type", "password")
		form.Set("username", to.username)
		form.Set("password", to.secret)
	}

	req, err := http.NewRequest("POST", to.realm, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	if ah.header != nil {
		for k, v := range ah.header {
			req.Header[k] = append(req.Header[k], v...)
		}
	}

	resp, err := ctxhttp.Do(ctx, ah.client, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Registries without support for POST may return 404 for POST /v2/token.
	// As of September 2017, GCR is known to return 404.
	// As of February 2018, JFrog Artifactory is known to return 401.
	if (resp.StatusCode == 405 && to.username != "") || resp.StatusCode == 404 || resp.StatusCode == 401 {
		return ah.fetchToken(ctx, to)
	} else if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		b, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 64000)) // 64KB
		log.G(ctx).WithFields(logrus.Fields{
			"status": resp.Status,
			"body":   string(b),
		}).Debugf("token request failed")
		// TODO: handle error body and write debug output
		return "", errors.Errorf("unexpected status: %s", resp.Status)
	}

	decoder := json.NewDecoder(resp.Body)

	var tr postTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return "", fmt.Errorf("unable to decode token response: %s", err)
	}

	return tr.AccessToken, nil
}

type getTokenResponse struct {
	Token        string    `json:"token"`
	AccessToken  string    `json:"access_token"`
	ExpiresIn    int       `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
	RefreshToken string    `json:"refresh_token"`
}

// fetchToken fetches a token using a GET request
func (ah *authHandler) fetchToken(ctx context.Context, to tokenOptions) (string, error) {
	req, err := http.NewRequest("GET", to.realm, nil)
	if err != nil {
		return "", err
	}

	if ah.header != nil {
		for k, v := range ah.header {
			req.Header[k] = append(req.Header[k], v...)
		}
	}

	reqParams := req.URL.Query()

	if to.service != "" {
		reqParams.Add("service", to.service)
	}

	for _, scope := range to.scopes {
		reqParams.Add("scope", scope)
	}

	if to.secret != "" {
		req.SetBasicAuth(to.username, to.secret)
	}

	req.URL.RawQuery = reqParams.Encode()

	resp, err := ctxhttp.Do(ctx, ah.client, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		// TODO: handle error body and write debug output
		return "", errors.Errorf("unexpected status: %s", resp.Status)
	}

	decoder := json.NewDecoder(resp.Body)

	var tr getTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return "", fmt.Errorf("unable to decode token response: %s", err)
	}

	// `access_token` is equivalent to `token` and if both are specified
	// the choice is undefined.  Canonicalize `access_token` by sticking
	// things in `token`.
	if tr.AccessToken != "" {
		tr.Token = tr.AccessToken
	}

	if tr.Token == "" {
		return "", ErrNoToken
	}

	return tr.Token, nil
}

func invalidAuthorization(c challenge, responses []*http.Response) error {
	errStr := c.parameters["error"]
	if errStr == "" {
		return nil
	}

	n := len(responses)
	if n == 1 || (n > 1 && !sameRequest(responses[n-2].Request, responses[n-1].Request)) {
		return nil
	}

	return errors.Wrapf(ErrInvalidAuthorization, "server message: %s", errStr)
}

func sameRequest(r1, r2 *http.Request) bool {
	if r1.Method != r2.Method {
		return false
	}
	if *r1.URL != *r2.URL {
		return false
	}
	return true
}
