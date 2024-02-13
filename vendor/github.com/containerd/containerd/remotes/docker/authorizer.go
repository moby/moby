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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes/docker/auth"
	remoteerrors "github.com/containerd/containerd/remotes/errors"
)

type dockerAuthorizer struct {
	credentials func(string) (string, string, error)

	client *http.Client
	header http.Header
	mu     sync.RWMutex

	// indexed by host name
	handlers map[string]*authHandler

	onFetchRefreshToken OnFetchRefreshToken
}

type authorizerConfig struct {
	credentials         func(string) (string, string, error)
	client              *http.Client
	header              http.Header
	onFetchRefreshToken OnFetchRefreshToken
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

// OnFetchRefreshToken is called on fetching request token.
type OnFetchRefreshToken func(ctx context.Context, refreshToken string, req *http.Request)

// WithFetchRefreshToken enables fetching "refresh token" (aka "identity token", "offline token").
func WithFetchRefreshToken(f OnFetchRefreshToken) AuthorizerOpt {
	return func(opt *authorizerConfig) {
		opt.onFetchRefreshToken = f
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
		credentials:         ao.credentials,
		client:              ao.client,
		header:              ao.header,
		handlers:            make(map[string]*authHandler),
		onFetchRefreshToken: ao.onFetchRefreshToken,
	}
}

// Authorize handles auth request.
func (a *dockerAuthorizer) Authorize(ctx context.Context, req *http.Request) error {
	// skip if there is no auth handler
	ah := a.getAuthHandler(req.URL.Host)
	if ah == nil {
		return nil
	}

	auth, refreshToken, err := ah.authorize(ctx)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", auth)

	if refreshToken != "" {
		a.mu.RLock()
		onFetchRefreshToken := a.onFetchRefreshToken
		a.mu.RUnlock()
		if onFetchRefreshToken != nil {
			onFetchRefreshToken(ctx, refreshToken, req)
		}
	}
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
	for _, c := range auth.ParseAuthHeader(last.Header) {
		if c.Scheme == auth.BearerAuth {
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

			var username, secret string
			if a.credentials != nil {
				var err error
				username, secret, err = a.credentials(host)
				if err != nil {
					return err
				}
			}

			common, err := auth.GenerateTokenOptions(ctx, host, username, secret, c)
			if err != nil {
				return err
			}
			common.FetchRefreshToken = a.onFetchRefreshToken != nil

			a.handlers[host] = newAuthHandler(a.client, a.header, c.Scheme, common)
			return nil
		} else if c.Scheme == auth.BasicAuth && a.credentials != nil {
			username, secret, err := a.credentials(host)
			if err != nil {
				return err
			}

			if username == "" || secret == "" {
				return fmt.Errorf("%w: no basic auth credentials", ErrInvalidAuthorization)
			}

			a.handlers[host] = newAuthHandler(a.client, a.header, c.Scheme, auth.TokenOptions{
				Username: username,
				Secret:   secret,
			})
			return nil
		}
	}
	return fmt.Errorf("failed to find supported auth scheme: %w", errdefs.ErrNotImplemented)
}

// authResult is used to control limit rate.
type authResult struct {
	sync.WaitGroup
	token        string
	refreshToken string
	err          error
}

// authHandler is used to handle auth request per registry server.
type authHandler struct {
	sync.Mutex

	header http.Header

	client *http.Client

	// only support basic and bearer schemes
	scheme auth.AuthenticationScheme

	// common contains common challenge answer
	common auth.TokenOptions

	// scopedTokens caches token indexed by scopes, which used in
	// bearer auth case
	scopedTokens map[string]*authResult
}

func newAuthHandler(client *http.Client, hdr http.Header, scheme auth.AuthenticationScheme, opts auth.TokenOptions) *authHandler {
	return &authHandler{
		header:       hdr,
		client:       client,
		scheme:       scheme,
		common:       opts,
		scopedTokens: map[string]*authResult{},
	}
}

func (ah *authHandler) authorize(ctx context.Context) (string, string, error) {
	switch ah.scheme {
	case auth.BasicAuth:
		return ah.doBasicAuth(ctx)
	case auth.BearerAuth:
		return ah.doBearerAuth(ctx)
	default:
		return "", "", fmt.Errorf("failed to find supported auth scheme: %s: %w", string(ah.scheme), errdefs.ErrNotImplemented)
	}
}

func (ah *authHandler) doBasicAuth(ctx context.Context) (string, string, error) {
	username, secret := ah.common.Username, ah.common.Secret

	if username == "" || secret == "" {
		return "", "", fmt.Errorf("failed to handle basic auth because missing username or secret")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + secret))
	return fmt.Sprintf("Basic %s", auth), "", nil
}

func (ah *authHandler) doBearerAuth(ctx context.Context) (token, refreshToken string, err error) {
	// copy common tokenOptions
	to := ah.common

	to.Scopes = GetTokenScopes(ctx, to.Scopes)

	// Docs: https://docs.docker.com/registry/spec/auth/scope
	scoped := strings.Join(to.Scopes, " ")

	ah.Lock()
	if r, exist := ah.scopedTokens[scoped]; exist {
		ah.Unlock()
		r.Wait()
		return r.token, r.refreshToken, r.err
	}

	// only one fetch token job
	r := new(authResult)
	r.Add(1)
	ah.scopedTokens[scoped] = r
	ah.Unlock()

	defer func() {
		token = fmt.Sprintf("Bearer %s", token)
		r.token, r.refreshToken, r.err = token, refreshToken, err
		r.Done()
	}()

	// fetch token for the resource scope
	if to.Secret != "" {
		defer func() {
			if err != nil {
				err = fmt.Errorf("failed to fetch oauth token: %w", err)
			}
		}()
		// credential information is provided, use oauth POST endpoint
		// TODO: Allow setting client_id
		resp, err := auth.FetchTokenWithOAuth(ctx, ah.client, ah.header, "containerd-client", to)
		if err != nil {
			var errStatus remoteerrors.ErrUnexpectedStatus
			if errors.As(err, &errStatus) {
				// Registries without support for POST may return 404 for POST /v2/token.
				// As of September 2017, GCR is known to return 404.
				// As of February 2018, JFrog Artifactory is known to return 401.
				// As of January 2022, ACR is known to return 400.
				if (errStatus.StatusCode == 405 && to.Username != "") || errStatus.StatusCode == 404 || errStatus.StatusCode == 401 || errStatus.StatusCode == 400 {
					resp, err := auth.FetchToken(ctx, ah.client, ah.header, to)
					if err != nil {
						return "", "", err
					}
					return resp.Token, resp.RefreshToken, nil
				}
				log.G(ctx).WithFields(log.Fields{
					"status": errStatus.Status,
					"body":   string(errStatus.Body),
				}).Debugf("token request failed")
			}
			return "", "", err
		}
		return resp.AccessToken, resp.RefreshToken, nil
	}
	// do request anonymously
	resp, err := auth.FetchToken(ctx, ah.client, ah.header, to)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch anonymous token: %w", err)
	}
	return resp.Token, resp.RefreshToken, nil
}

func invalidAuthorization(c auth.Challenge, responses []*http.Response) error {
	errStr := c.Parameters["error"]
	if errStr == "" {
		return nil
	}

	n := len(responses)
	if n == 1 || (n > 1 && !sameRequest(responses[n-2].Request, responses[n-1].Request)) {
		return nil
	}

	return fmt.Errorf("server message: %s: %w", errStr, ErrInvalidAuthorization)
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
