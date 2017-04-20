package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
)

// RepositoryData tracks the image list, list of endpoints for a repository
type RepositoryData struct {
	// ImgList is a list of images in the repository
	ImgList map[string]*ImgData
	// Endpoints is a list of endpoints returned in X-Docker-Endpoints
	Endpoints []string
}

// ImgData is used to transfer image checksums to and from the registry
type ImgData struct {
	// ID is an opaque string that identifies the image
	ID              string `json:"id"`
	Checksum        string `json:"checksum,omitempty"`
	ChecksumPayload string `json:"-"`
	Tag             string `json:",omitempty"`
}

// PingResult contains the information returned when pinging a registry. It
// indicates the registry's version and whether the registry claims to be a
// standalone registry.
type PingResult struct {
	// Version is the registry version supplied by the registry in an HTTP
	// header
	Version string `json:"version"`
	// Standalone is set to true if the registry indicates it is a
	// standalone registry in the X-Docker-Registry-Standalone
	// header
	Standalone bool `json:"standalone"`
}

// APIVersion is an integral representation of an API version (presently
// either 1 or 2)
type APIVersion int

func (av APIVersion) String() string {
	return apiVersions[av]
}

// API Version identifiers.
const (
	_                      = iota
	APIVersion1 APIVersion = iota
	APIVersion2
)

var apiVersions = map[APIVersion]string{
	APIVersion1: "v1",
	APIVersion2: "v2",
}

// RepositoryInfo describes a repository
type RepositoryInfo struct {
	Name reference.Named
	// Index points to registry information
	Index *registrytypes.IndexInfo
	// Official indicates whether the repository is considered official.
	// If the registry is official, and the normalized name does not
	// contain a '/' (e.g. "foo"), then it is considered an official repo.
	Official bool
	// Class represents the class of the repository, such as "plugin"
	// or "image".
	Class string
}

type authConfigAuthenticator struct {
	config                *types.AuthConfig
	refreshToken          string
	accessToken           string
	accessTokenExpiration time.Time
}

func NewAuthConfigAuthenticator(c *types.AuthConfig) auth.Authenticator {
	return &authConfigAuthenticator{config: c}
}

func (a *authConfigAuthenticator) GetBasicAuthInfo() (username, password string) {
	if a.config == nil {
		return "", ""
	}
	return a.config.Username, a.config.Password
}

func (a *authConfigAuthenticator) HasBasicAuthInfo() bool {
	return a.config != nil && a.config.Username != "" && a.config.Password != ""
}
func (a *authConfigAuthenticator) HasUsername() bool {
	return a.config != nil && a.config.Username != ""
}
func (a *authConfigAuthenticator) GetUsername() string {
	if a.config == nil {
		return ""
	}
	return a.config.Username
}
func (a *authConfigAuthenticator) HasIdentityTokenInfo() bool {
	return a.config != nil && a.config.Username != "" && a.config.IdentityToken != ""
}
func (a *authConfigAuthenticator) GetIdentityTokenInfo() (username, identityToken string) {
	if a.config == nil {
		return "", ""
	}
	return a.config.Username, a.config.IdentityToken
}

func (a *authConfigAuthenticator) GetPassthroughToken() string {
	if a.config == nil {
		return ""
	}
	return a.config.RegistryToken
}

func (a *authConfigAuthenticator) HasPassthroughToken() bool {
	return a.config != nil && a.config.RegistryToken != ""
}

func (a *authConfigAuthenticator) GetAccessToken(realm, service, clientID string, scopes []string, skipCache, offlineAccess, forceOAuth bool) (token string, err error) {
	if a.accessToken != "" && a.accessTokenExpiration.After(time.Now()) {
		return a.accessToken, nil
	}
	if a.refreshToken != "" || forceOAuth {
		return a.getAccessTokenOAuth(realm, service, clientID, scopes, skipCache, offlineAccess)
	}
	return a.getAccessTokenBasicAuth(realm, service, clientID, scopes, skipCache, offlineAccess)

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

func (a *authConfigAuthenticator) getAccessTokenOAuth(realm, service, clientID string, scopes []string, skipCache, offlineAccess bool) (token string, err error) {
	form := url.Values{}
	form.Set("scope", strings.Join(scopes, " "))
	form.Set("service", service)

	if clientID == "" {
		// Use default client, this is a required field
		clientID = defaultClientID
	}
	form.Set("client_id", clientID)

	if a.refreshToken != "" {
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", a.refreshToken)
	} else if a.HasBasicAuthInfo() {
		form.Set("grant_type", "password")
		username, password := a.GetBasicAuthInfo()
		form.Set("username", username)
		form.Set("password", password)

		// attempt to get a refresh token
		form.Set("access_type", "offline")
	} else {
		// refuse to do oauth without a grant type
		return "", fmt.Errorf("no supported grant type")
	}

	// todo: proper client config
	httpCli := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpCli.PostForm(realm, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if !client.SuccessStatus(resp.StatusCode) {
		err := client.HandleErrorResponse(resp)
		return "", err
	}

	decoder := json.NewDecoder(resp.Body)

	var tr postTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return "", fmt.Errorf("unable to decode token response: %s", err)
	}

	if !skipCache && tr.RefreshToken != "" && tr.RefreshToken != a.refreshToken {
		a.refreshToken = tr.RefreshToken
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
	if !skipCache {
		a.accessToken = tr.AccessToken
		a.accessTokenExpiration = tr.IssuedAt.Add(time.Duration(tr.ExpiresIn) * time.Second)
	}

	return tr.AccessToken, nil
}
func (a *authConfigAuthenticator) getAccessTokenBasicAuth(realm, service, clientID string, scopes []string, skipCache, offlineAccess bool) (token string, err error) {
	req, err := http.NewRequest("GET", realm, nil)
	if err != nil {
		return "", err
	}

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

	if a.HasBasicAuthInfo() {
		username, password := a.GetBasicAuthInfo()
		if username != "" && password != "" {
			reqParams.Add("account", username)
			req.SetBasicAuth(username, password)
		}
	}

	req.URL.RawQuery = reqParams.Encode()

	// todo: proper client config
	httpCli := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpCli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if !client.SuccessStatus(resp.StatusCode) {
		err := client.HandleErrorResponse(resp)
		return "", err
	}

	decoder := json.NewDecoder(resp.Body)

	var tr getTokenResponse
	if err = decoder.Decode(&tr); err != nil {
		return "", fmt.Errorf("unable to decode token response: %s", err)
	}

	if tr.RefreshToken != "" && !skipCache {
		a.refreshToken = tr.RefreshToken
	}

	// `access_token` is equivalent to `token` and if both are specified
	// the choice is undefined.  Canonicalize `access_token` by sticking
	// things in `token`.
	if tr.AccessToken != "" {
		tr.Token = tr.AccessToken
	}

	if tr.Token == "" {
		return "", auth.ErrNoToken
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
	if !skipCache {
		a.accessToken = tr.Token
		a.accessTokenExpiration = tr.IssuedAt.Add(time.Duration(tr.ExpiresIn) * time.Second)
	}

	return tr.Token, nil

}
