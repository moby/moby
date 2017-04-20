package authsession

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client/session"
	"github.com/pkg/errors"
)

type AuthConfigProvider interface {
	GetAuthConfig(registry string) types.AuthConfig
}
type tokenResponseWithRefreshToken struct {
	GetTokenResponse
	RefreshToken string
}
type authConfigHandler struct {
	name            string
	provider        AuthConfigProvider
	registryAuthLog func(string)
	tokenCache      map[string]tokenResponseWithRefreshToken
}

func NewAuthconfigHandler(name string, provider AuthConfigProvider, registryAuthLog func(string)) session.Attachment {
	h := &authConfigHandler{
		name:            name,
		provider:        provider,
		registryAuthLog: registryAuthLog,
		tokenCache:      make(map[string]tokenResponseWithRefreshToken),
	}
	return h
}

func (h *authConfigHandler) RegisterHandlers(fn func(id, method string) error) error {
	if err := fn(h.name, "GetAuth"); err != nil {
		return err
	}
	if err := fn(h.name, "GetAccessToken"); err != nil {
		return err
	}
	return nil
}
func (h *authConfigHandler) HandleGetAuth(opts map[string][]string, stream session.Stream) error {
	registries := opts["registries"]
	auths := new(AuthConfigs)
	auths.Auths = make(map[string]*AuthConfig)
	for _, registry := range registries {
		if h.registryAuthLog != nil {
			h.registryAuthLog(registry + " by sending credentials to daemon")
		}
		c := h.provider.GetAuthConfig(registry)
		auths.Auths[registry] = &AuthConfig{
			Auth:          c.Auth,
			Email:         c.Email,
			IdentityToken: c.IdentityToken,
			Password:      c.Password,
			RegistryToken: c.RegistryToken,
			ServerAddress: c.ServerAddress,
			Username:      c.Username,
		}
	}
	return stream.SendMsg(auths)
}

func getCacheKey(opts GetTokenRequest) string {
	return strings.Join(append([]string{opts.Registry, opts.Realm, opts.Service, opts.ClientID}, opts.Scopes...), "|")
}
func (h *authConfigHandler) HandleGetAccessToken(opts map[string][]string, stream session.Stream) error {
	req := new(GetTokenRequest)
	if err := stream.RecvMsg(req); err != nil {
		return err
	}

	h.registryAuthLog(req.Registry + " by sending access token to daemon")
	cacheKey := getCacheKey(*req)
	var resp *GetTokenResponse
	var err error
	if h.tokenCache[cacheKey].RefreshToken != "" || req.ForceOAuth {
		resp, err = h.getAccessTokenOAuth(cacheKey, req.Registry, req.Realm, req.Service, req.ClientID, req.Scopes, req.SkipCache, req.OfflineAccess)
	} else {
		resp, err = h.getAccessTokenBasicAuth(cacheKey, req.Registry, req.Realm, req.Service, req.ClientID, req.Scopes, req.SkipCache, req.OfflineAccess)
	}
	if err != nil {
		return err
	}
	return stream.SendMsg(resp)
}

const defaultClientID = "registry-client"
const minimumTokenLifetimeSeconds = 60

type postTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
	Scope        string    `json:"scope"`
}

type getTokenResponse struct {
	Token        string    `json:"token"`
	AccessToken  string    `json:"access_token"`
	ExpiresIn    int       `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
	RefreshToken string    `json:"refresh_token"`
}

func (a *authConfigHandler) getAccessTokenOAuth(cacheKey, registry, realm, service, clientID string, scopes []string, skipCache, offlineAccess bool) (token *GetTokenResponse, err error) {
	form := url.Values{}
	form.Set("scope", strings.Join(scopes, " "))
	form.Set("service", service)

	if clientID == "" {
		// Use default client, this is a required field
		clientID = defaultClientID
	}
	form.Set("client_id", clientID)

	cache, _ := a.tokenCache[cacheKey]

	if cache.RefreshToken != "" {
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", cache.RefreshToken)
	} else if auth := a.provider.GetAuthConfig(registry); auth.Username != "" && auth.Password != "" {
		form.Set("grant_type", "password")
		form.Set("username", auth.Username)
		form.Set("password", auth.Password)

		// attempt to get a refresh token
		form.Set("access_type", "offline")
	} else {
		// refuse to do oauth without a grant type
		return nil, fmt.Errorf("no supported grant type")
	}

	// todo: proper client config
	httpCli := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpCli.PostForm(realm, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !client.SuccessStatus(resp.StatusCode) {
		err := client.HandleErrorResponse(resp)
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)

	var tr postTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return nil, fmt.Errorf("unable to decode token response: %s", err)
	}

	if !skipCache && tr.RefreshToken != "" && tr.RefreshToken != cache.RefreshToken {
		cache.RefreshToken = tr.RefreshToken
	}

	if tr.ExpiresIn < minimumTokenLifetimeSeconds {
		// The default/minimum lifetime.
		tr.ExpiresIn = minimumTokenLifetimeSeconds
		logrus.Debugf("Increasing token expiration to: %d seconds", tr.ExpiresIn)
	}

	if tr.IssuedAt.IsZero() {
		// issued_at is optional in the token response.
		tr.IssuedAt = time.Now().UTC()
	}
	cache.AccessToken = tr.AccessToken
	cache.IssuedAt = tr.IssuedAt.Unix()
	cache.ExpiresIn = int32(tr.ExpiresIn)

	if !skipCache {
		a.tokenCache[cacheKey] = cache
	}

	return &cache.GetTokenResponse, nil
}
func (a *authConfigHandler) getAccessTokenBasicAuth(cacheKey, registry, realm, service, clientID string, scopes []string, skipCache, offlineAccess bool) (token *GetTokenResponse, err error) {
	req, err := http.NewRequest("GET", realm, nil)
	if err != nil {
		return nil, err
	}

	cache, _ := a.tokenCache[cacheKey]
	reqParams := req.URL.Query()

	if service != "" {
		reqParams.Add("service", service)
	}

	for _, scope := range scopes {
		reqParams.Add("scope", scope)
	}

	if offlineAccess {
		reqParams.Add("offline_token", "true")
		if clientID == "" {
			clientID = defaultClientID
		}
		reqParams.Add("client_id", clientID)
	}

	if auth := a.provider.GetAuthConfig(registry); auth.Username != "" && auth.Password != "" {
		reqParams.Add("account", auth.Username)
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	req.URL.RawQuery = reqParams.Encode()

	// todo: proper client config
	httpCli := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !client.SuccessStatus(resp.StatusCode) {
		err := client.HandleErrorResponse(resp)
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)

	var tr getTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return nil, fmt.Errorf("unable to decode token response: %s", err)
	}

	if tr.RefreshToken != "" && !skipCache {
		cache.RefreshToken = tr.RefreshToken
	}

	// `access_token` is equivalent to `token` and if both are specified
	// the choice is undefined.  Canonicalize `access_token` by sticking
	// things in `token`.
	if tr.AccessToken != "" {
		tr.Token = tr.AccessToken
	}

	if tr.Token == "" {
		return nil, auth.ErrNoToken
	}

	if tr.ExpiresIn < minimumTokenLifetimeSeconds {
		// The default/minimum lifetime.
		tr.ExpiresIn = minimumTokenLifetimeSeconds
		logrus.Debugf("Increasing token expiration to: %d seconds", tr.ExpiresIn)
	}

	if tr.IssuedAt.IsZero() {
		// issued_at is optional in the token response.
		tr.IssuedAt = time.Now().UTC()
	}
	cache.AccessToken = tr.Token
	cache.IssuedAt = tr.IssuedAt.Unix()
	cache.ExpiresIn = int32(tr.ExpiresIn)

	if !skipCache {
		a.tokenCache[cacheKey] = cache
	}

	return &cache.GetTokenResponse, nil
}

func (h *authConfigHandler) Handle(ctx context.Context, id, method string, opts map[string][]string, stream session.Stream) error {
	if id != h.name {
		return errors.Errorf("invalid id %s", id)
	}
	if method == "GetAuth" {
		return h.HandleGetAuth(opts, stream)
	} else if method == "GetAccessToken" {
		return h.HandleGetAccessToken(opts, stream)
	}

	return errors.Errorf("unknown method %s", method)
}
