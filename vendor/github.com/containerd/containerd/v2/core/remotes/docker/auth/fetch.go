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

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	remoteserrors "github.com/containerd/containerd/v2/core/remotes/errors"
	"github.com/containerd/containerd/v2/pkg/tracing"
	"github.com/containerd/containerd/v2/version"
	"github.com/containerd/log"
)

var (
	// ErrNoToken is returned if a request is successful but the body does not
	// contain an authorization token.
	ErrNoToken = errors.New("authorization server did not include a token in the response")
)

// GenerateTokenOptions generates options for fetching a token based on a challenge
func GenerateTokenOptions(ctx context.Context, host, username, secret string, c Challenge) (TokenOptions, error) {
	realm, ok := c.Parameters["realm"]
	if !ok {
		return TokenOptions{}, errors.New("no realm specified for token auth challenge")
	}

	realmURL, err := url.Parse(realm)
	if err != nil {
		return TokenOptions{}, fmt.Errorf("invalid token auth challenge realm: %w", err)
	}

	to := TokenOptions{
		Realm:    realmURL.String(),
		Service:  c.Parameters["service"],
		Username: username,
		Secret:   secret,
	}

	scope, ok := c.Parameters["scope"]
	if ok {
		to.Scopes = append(to.Scopes, strings.Split(scope, " ")...)
	} else {
		log.G(ctx).WithField("host", host).Debug("no scope specified for token auth challenge")
	}

	return to, nil
}

// TokenOptions are options for requesting a token
type TokenOptions struct {
	Realm    string
	Service  string
	Scopes   []string
	Username string
	Secret   string

	// FetchRefreshToken enables fetching a refresh token (aka "identity token", "offline token") along with the bearer token.
	//
	// For HTTP GET mode (FetchToken), FetchRefreshToken sets `offline_token=true` in the request.
	// https://docs.docker.com/registry/spec/auth/token/#requesting-a-token
	//
	// For HTTP POST mode (FetchTokenWithOAuth), FetchRefreshToken sets `access_type=offline` in the request.
	// https://docs.docker.com/registry/spec/auth/oauth/#getting-a-token
	FetchRefreshToken bool
}

// OAuthTokenResponse is response from fetching token with a OAuth POST request
type OAuthTokenResponse struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	ExpiresInSeconds int       `json:"expires_in"`
	IssuedAt         time.Time `json:"issued_at"`
	Scope            string    `json:"scope"`
}

// FetchTokenWithOAuth fetches a token using a POST request
func FetchTokenWithOAuth(ctx context.Context, client *http.Client, headers http.Header, clientID string, to TokenOptions) (*OAuthTokenResponse, error) {
	c := *client
	client = &c
	tracing.UpdateHTTPClient(client, tracing.Name("remotes.docker.resolver", "FetchTokenWithOAuth"))

	form := url.Values{}
	if len(to.Scopes) > 0 {
		form.Set("scope", strings.Join(to.Scopes, " "))
	}
	form.Set("service", to.Service)
	form.Set("client_id", clientID)

	if to.Username == "" {
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", to.Secret)
	} else {
		form.Set("grant_type", "password")
		form.Set("username", to.Username)
		form.Set("password", to.Secret)
	}
	if to.FetchRefreshToken {
		form.Set("access_type", "offline")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, to.Realm, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	for k, v := range headers {
		req.Header[k] = append(req.Header[k], v...)
	}
	if len(req.Header.Get("User-Agent")) == 0 {
		req.Header.Set("User-Agent", "containerd/"+version.Version)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, remoteserrors.NewUnexpectedStatusErr(resp)
	}

	decoder := json.NewDecoder(resp.Body)

	var tr OAuthTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return nil, fmt.Errorf("unable to decode token response: %w", err)
	}

	if tr.AccessToken == "" {
		return nil, ErrNoToken
	}

	return &tr, nil
}

// FetchTokenResponse is response from fetching token with GET request
type FetchTokenResponse struct {
	Token            string    `json:"token"`
	AccessToken      string    `json:"access_token"`
	ExpiresInSeconds int       `json:"expires_in"`
	IssuedAt         time.Time `json:"issued_at"`
	RefreshToken     string    `json:"refresh_token"`
}

// FetchToken fetches a token using a GET request
func FetchToken(ctx context.Context, client *http.Client, headers http.Header, to TokenOptions) (*FetchTokenResponse, error) {
	c := *client
	client = &c
	tracing.UpdateHTTPClient(client, tracing.Name("remotes.docker.resolver", "FetchToken"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, to.Realm, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header[k] = append(req.Header[k], v...)
	}
	if len(req.Header.Get("User-Agent")) == 0 {
		req.Header.Set("User-Agent", "containerd/"+version.Version)
	}

	reqParams := req.URL.Query()

	if to.Service != "" {
		reqParams.Add("service", to.Service)
	}

	for _, scope := range to.Scopes {
		reqParams.Add("scope", scope)
	}

	if to.Secret != "" {
		req.SetBasicAuth(to.Username, to.Secret)
	}

	if to.FetchRefreshToken {
		reqParams.Add("offline_token", "true")
	}

	req.URL.RawQuery = reqParams.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, remoteserrors.NewUnexpectedStatusErr(resp)
	}

	decoder := json.NewDecoder(resp.Body)

	var tr FetchTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return nil, fmt.Errorf("unable to decode token response: %w", err)
	}

	// `access_token` is equivalent to `token` and if both are specified
	// the choice is undefined.  Canonicalize `access_token` by sticking
	// things in `token`.
	if tr.AccessToken != "" {
		tr.Token = tr.AccessToken
	}

	if tr.Token == "" {
		return nil, ErrNoToken
	}

	return &tr, nil
}
