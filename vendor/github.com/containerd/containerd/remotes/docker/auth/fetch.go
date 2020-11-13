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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/containerd/containerd/log"
	remoteserrors "github.com/containerd/containerd/remotes/errors"
	"github.com/pkg/errors"
	"golang.org/x/net/context/ctxhttp"
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
		return TokenOptions{}, errors.Wrap(err, "invalid token auth challenge realm")
	}

	to := TokenOptions{
		Realm:    realmURL.String(),
		Service:  c.Parameters["service"],
		Username: username,
		Secret:   secret,
	}

	scope, ok := c.Parameters["scope"]
	if ok {
		to.Scopes = append(to.Scopes, scope)
	} else {
		log.G(ctx).WithField("host", host).Debug("no scope specified for token auth challenge")
	}

	return to, nil
}

// TokenOptions are optios for requesting a token
type TokenOptions struct {
	Realm    string
	Service  string
	Scopes   []string
	Username string
	Secret   string
}

// OAuthTokenResponse is response from fetching token with a OAuth POST request
type OAuthTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
	Scope        string    `json:"scope"`
}

// FetchTokenWithOAuth fetches a token using a POST request
func FetchTokenWithOAuth(ctx context.Context, client *http.Client, headers http.Header, clientID string, to TokenOptions) (*OAuthTokenResponse, error) {
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

	req, err := http.NewRequest("POST", to.Realm, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	if headers != nil {
		for k, v := range headers {
			req.Header[k] = append(req.Header[k], v...)
		}
	}

	resp, err := ctxhttp.Do(ctx, client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, errors.WithStack(remoteserrors.NewUnexpectedStatusErr(resp))
	}

	decoder := json.NewDecoder(resp.Body)

	var tr OAuthTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return nil, errors.Wrap(err, "unable to decode token response")
	}

	if tr.AccessToken == "" {
		return nil, errors.WithStack(ErrNoToken)
	}

	return &tr, nil
}

// FetchTokenResponse is response from fetching token with GET request
type FetchTokenResponse struct {
	Token        string    `json:"token"`
	AccessToken  string    `json:"access_token"`
	ExpiresIn    int       `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
	RefreshToken string    `json:"refresh_token"`
}

// FetchToken fetches a token using a GET request
func FetchToken(ctx context.Context, client *http.Client, headers http.Header, to TokenOptions) (*FetchTokenResponse, error) {
	req, err := http.NewRequest("GET", to.Realm, nil)
	if err != nil {
		return nil, err
	}

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = append(req.Header[k], v...)
		}
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

	req.URL.RawQuery = reqParams.Encode()

	resp, err := ctxhttp.Do(ctx, client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, errors.WithStack(remoteserrors.NewUnexpectedStatusErr(resp))
	}

	decoder := json.NewDecoder(resp.Body)

	var tr FetchTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return nil, errors.Wrap(err, "unable to decode token response")
	}

	// `access_token` is equivalent to `token` and if both are specified
	// the choice is undefined.  Canonicalize `access_token` by sticking
	// things in `token`.
	if tr.AccessToken != "" {
		tr.Token = tr.AccessToken
	}

	if tr.Token == "" {
		return nil, errors.WithStack(ErrNoToken)
	}

	return &tr, nil
}
