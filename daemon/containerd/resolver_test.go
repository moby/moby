package containerd

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/distribution/reference"
	registrytypes "github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
)

func TestHostsWrapperReusesAuthorizers(t *testing.T) {
	ref, err := reference.ParseNormalizedNamed("example.com/library/test:latest")
	assert.NilError(t, err)

	hostsCalls := 0
	tokenRequests := 0
	hostsFn := func(host string) ([]docker.RegistryHost, error) {
		assert.Equal(t, host, "example.com")
		hostsCalls++
		call := hostsCalls
		return []docker.RegistryHost{
			{
				Host: "example.com",
				Client: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, call, 1, "token request used a client from a later hosts call")
					assert.Equal(t, req.URL.Host, "auth.example.com")
					tokenRequests++
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": {"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"access_token":"cached-token","expires_in":3600}`)),
						Request:    req,
					}, nil
				})},
			},
			{Host: "mirror.example.com", Client: &http.Client{}},
		}, nil
	}
	wrapper := hostsWrapper(hostsFn, &registrytypes.AuthConfig{
		ServerAddress: "example.com",
		Username:      "user",
		Password:      "password",
	}, ref)

	first, err := wrapper("example.com")
	assert.NilError(t, err)
	second, err := wrapper("example.com")
	assert.NilError(t, err)

	assert.Assert(t, first[0].Client != second[0].Client, "test must return fresh clients on each hosts call")
	assert.Assert(t, first[0].Authorizer == second[0].Authorizer, "authorizer was not reused for the same registry host")
	assert.Assert(t, first[1].Authorizer == second[1].Authorizer, "authorizer was not reused for the same mirror host")
	assert.Assert(t, first[0].Authorizer != first[1].Authorizer, "different registry hosts must not share an authorizer")

	request, err := http.NewRequestWithContext(t.Context(), http.MethodHead, "https://example.com/v2/library/test/manifests/latest", nil)
	assert.NilError(t, err)
	challenge := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header: http.Header{
			"Www-Authenticate": {`Bearer realm="https://auth.example.com/token",service="example.com",scope="repository:library/test:pull"`},
		},
		Body:    io.NopCloser(strings.NewReader("")),
		Request: request,
	}
	assert.NilError(t, first[0].Authorizer.AddResponses(t.Context(), []*http.Response{challenge}))

	assert.NilError(t, first[0].Authorizer.Authorize(t.Context(), request))
	assert.Equal(t, request.Header.Get("Authorization"), "Bearer cached-token")
	assert.Equal(t, tokenRequests, 1)

	reusedRequest, err := http.NewRequestWithContext(t.Context(), http.MethodHead, request.URL.String(), nil)
	assert.NilError(t, err)
	assert.NilError(t, second[0].Authorizer.Authorize(t.Context(), reusedRequest))
	assert.Equal(t, reusedRequest.Header.Get("Authorization"), "Bearer cached-token")
	assert.Equal(t, tokenRequests, 1, "cached token was fetched again")
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
