package distribution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/registry"
	registrypkg "github.com/moby/moby/v2/daemon/pkg/registry"
	"gotest.tools/v3/assert"
)

const secretRegistryToken = "mysecrettoken"

type tokenPassThruHandler struct {
	reached       bool
	gotToken      bool
	shouldSend401 func(url string) bool
}

func (h *tokenPassThruHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.reached = true
	if strings.Contains(r.Header.Get("Authorization"), secretRegistryToken) {
		// Detected registry token in auth header
		h.gotToken = true
	}
	if h.shouldSend401 == nil || h.shouldSend401(r.RequestURI) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="foorealm"`)
		w.WriteHeader(http.StatusUnauthorized)
	}
}

func testTokenPassThru(t *testing.T, ts *httptest.Server) {
	uri, err := url.Parse(ts.URL)
	assert.NilError(t, err, "could not parse url from test server")

	repoName, err := reference.ParseNormalizedNamed("testremotename")
	assert.NilError(t, err)

	imagePullConfig := &ImagePullConfig{
		Config: Config{
			MetaHeaders: http.Header{},
			AuthConfig: &registry.AuthConfig{
				RegistryToken: secretRegistryToken,
			},
		},
	}
	p := newPuller(registrypkg.APIEndpoint{URL: uri}, repoName, imagePullConfig, nil)
	ctx := context.Background()
	p.repo, err = newRepository(ctx, repoName, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig, "pull")
	if err != nil {
		t.Fatal(err)
	}

	tag, err := reference.WithTag(repoName, "tag_goes_here")
	assert.NilError(t, err)

	// We expect it to fail, since we haven't mock'd the full registry exchange in our handler above
	_ = p.pullRepository(ctx, tag)
}

func TestTokenPassThru(t *testing.T) {
	handler := &tokenPassThruHandler{shouldSend401: func(url string) bool { return url == "/v2/" }}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	testTokenPassThru(t, ts)

	assert.Check(t, handler.reached, "Handler not reached")
	assert.Check(t, handler.gotToken, "Failed to receive registry token")
}

func TestTokenPassThruDifferentHost(t *testing.T) {
	handler := new(tokenPassThruHandler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	tsredirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/v2/" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="foorealm"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, ts.URL+r.URL.Path, http.StatusMovedPermanently)
	}))
	defer tsredirect.Close()

	testTokenPassThru(t, tsredirect)

	assert.Check(t, handler.reached, "Handler not reached")
	assert.Check(t, !handler.gotToken, "Redirect should not forward Authorization header to another host")
}
