package registry // import "github.com/docker/docker/registry"

import (
	// this is required for some certificates
	_ "crypto/sha512"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// A Session is used to communicate with a V1 registry
type Session struct {
	indexEndpoint *V1Endpoint
	client        *http.Client
	// TODO(tiborvass): remove authConfig
	authConfig *types.AuthConfig
	id         string
}

type authTransport struct {
	http.RoundTripper
	*types.AuthConfig

	alwaysSetBasicAuth bool
	token              []string

	mu     sync.Mutex                      // guards modReq
	modReq map[*http.Request]*http.Request // original -> modified
}

// AuthTransport handles the auth layer when communicating with a v1 registry (private or official)
//
// For private v1 registries, set alwaysSetBasicAuth to true.
//
// For the official v1 registry, if there isn't already an Authorization header in the request,
// but there is an X-Docker-Token header set to true, then Basic Auth will be used to set the Authorization header.
// After sending the request with the provided base http.RoundTripper, if an X-Docker-Token header, representing
// a token, is present in the response, then it gets cached and sent in the Authorization header of all subsequent
// requests.
//
// If the server sends a token without the client having requested it, it is ignored.
//
// This RoundTripper also has a CancelRequest method important for correct timeout handling.
func AuthTransport(base http.RoundTripper, authConfig *types.AuthConfig, alwaysSetBasicAuth bool) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &authTransport{
		RoundTripper:       base,
		AuthConfig:         authConfig,
		alwaysSetBasicAuth: alwaysSetBasicAuth,
		modReq:             make(map[*http.Request]*http.Request),
	}
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}

	return r2
}

// RoundTrip changes an HTTP request's headers to add the necessary
// authentication-related headers
func (tr *authTransport) RoundTrip(orig *http.Request) (*http.Response, error) {
	// Authorization should not be set on 302 redirect for untrusted locations.
	// This logic mirrors the behavior in addRequiredHeadersToRedirectedRequests.
	// As the authorization logic is currently implemented in RoundTrip,
	// a 302 redirect is detected by looking at the Referrer header as go http package adds said header.
	// This is safe as Docker doesn't set Referrer in other scenarios.
	if orig.Header.Get("Referer") != "" && !trustedLocation(orig) {
		return tr.RoundTripper.RoundTrip(orig)
	}

	req := cloneRequest(orig)
	tr.mu.Lock()
	tr.modReq[orig] = req
	tr.mu.Unlock()

	if tr.alwaysSetBasicAuth {
		if tr.AuthConfig == nil {
			return nil, errors.New("unexpected error: empty auth config")
		}
		req.SetBasicAuth(tr.Username, tr.Password)
		return tr.RoundTripper.RoundTrip(req)
	}

	// Don't override
	if req.Header.Get("Authorization") == "" {
		if req.Header.Get("X-Docker-Token") == "true" && tr.AuthConfig != nil && len(tr.Username) > 0 {
			req.SetBasicAuth(tr.Username, tr.Password)
		} else if len(tr.token) > 0 {
			req.Header.Set("Authorization", "Token "+strings.Join(tr.token, ","))
		}
	}
	resp, err := tr.RoundTripper.RoundTrip(req)
	if err != nil {
		tr.mu.Lock()
		delete(tr.modReq, orig)
		tr.mu.Unlock()
		return nil, err
	}
	if len(resp.Header["X-Docker-Token"]) > 0 {
		tr.token = resp.Header["X-Docker-Token"]
	}
	resp.Body = &ioutils.OnEOFReader{
		Rc: resp.Body,
		Fn: func() {
			tr.mu.Lock()
			delete(tr.modReq, orig)
			tr.mu.Unlock()
		},
	}
	return resp, nil
}

// CancelRequest cancels an in-flight request by closing its connection.
func (tr *authTransport) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := tr.RoundTripper.(canceler); ok {
		tr.mu.Lock()
		modReq := tr.modReq[req]
		delete(tr.modReq, req)
		tr.mu.Unlock()
		cr.CancelRequest(modReq)
	}
}

func authorizeClient(client *http.Client, authConfig *types.AuthConfig, endpoint *V1Endpoint) error {
	var alwaysSetBasicAuth bool

	// If we're working with a standalone private registry over HTTPS, send Basic Auth headers
	// alongside all our requests.
	if endpoint.String() != IndexServer && endpoint.URL.Scheme == "https" {
		info, err := endpoint.Ping()
		if err != nil {
			return err
		}
		if info.Standalone && authConfig != nil {
			logrus.Debugf("Endpoint %s is eligible for private registry. Enabling decorator.", endpoint.String())
			alwaysSetBasicAuth = true
		}
	}

	// Annotate the transport unconditionally so that v2 can
	// properly fallback on v1 when an image is not found.
	client.Transport = AuthTransport(client.Transport, authConfig, alwaysSetBasicAuth)

	jar, err := cookiejar.New(nil)
	if err != nil {
		return errors.New("cookiejar.New is not supposed to return an error")
	}
	client.Jar = jar

	return nil
}

func newSession(client *http.Client, authConfig *types.AuthConfig, endpoint *V1Endpoint) *Session {
	return &Session{
		authConfig:    authConfig,
		client:        client,
		indexEndpoint: endpoint,
		id:            stringid.GenerateRandomID(),
	}
}

// NewSession creates a new session
// TODO(tiborvass): remove authConfig param once registry client v2 is vendored
func NewSession(client *http.Client, authConfig *types.AuthConfig, endpoint *V1Endpoint) (*Session, error) {
	if err := authorizeClient(client, authConfig, endpoint); err != nil {
		return nil, err
	}

	return newSession(client, authConfig, endpoint), nil
}

// SearchRepositories performs a search against the remote repository
func (r *Session) SearchRepositories(term string, limit int) (*registrytypes.SearchResults, error) {
	if limit < 1 || limit > 100 {
		return nil, errdefs.InvalidParameter(errors.Errorf("Limit %d is outside the range of [1, 100]", limit))
	}
	logrus.Debugf("Index server: %s", r.indexEndpoint)
	u := r.indexEndpoint.String() + "search?q=" + url.QueryEscape(term) + "&n=" + url.QueryEscape(fmt.Sprintf("%d", limit))

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, errors.Wrap(errdefs.InvalidParameter(err), "Error building request")
	}
	// Have the AuthTransport send authentication, when logged in.
	req.Header.Set("X-Docker-Token", "true")
	res, err := r.client.Do(req)
	if err != nil {
		return nil, errdefs.System(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, &jsonmessage.JSONError{
			Message: fmt.Sprintf("Unexpected status code %d", res.StatusCode),
			Code:    res.StatusCode,
		}
	}
	result := new(registrytypes.SearchResults)
	return result, errors.Wrap(json.NewDecoder(res.Body).Decode(result), "error decoding registry search results")
}
