package registry

import (
	"net/http"
	"testing"

	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types/registry"
	"gotest.tools/v3/assert"
)

func TestSearchRepositories(t *testing.T) {
	r := spawnTestRegistrySession(t)
	results, err := r.searchRepositories("fakequery", 25)
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("Expected non-nil SearchResults object")
	}
	assert.Equal(t, results.NumResults, 1, "Expected 1 search results")
	assert.Equal(t, results.Query, "fakequery", "Expected 'fakequery' as query")
	assert.Equal(t, results.Results[0].StarCount, 42, "Expected 'fakeimage' to have 42 stars")
}

func spawnTestRegistrySession(t *testing.T) *session {
	authConfig := &registry.AuthConfig{}
	endpoint, err := newV1Endpoint(makeIndex("/v1/"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	userAgent := "docker test client"
	var tr http.RoundTripper = debugTransport{newTransport(nil), t.Log}
	tr = transport.NewTransport(newAuthTransport(tr, authConfig, false), Headers(userAgent, nil)...)
	client := httpClient(tr)

	if err := authorizeClient(client, authConfig, endpoint); err != nil {
		t.Fatal(err)
	}
	r := newSession(client, endpoint)

	// In a normal scenario for the v1 registry, the client should send a `X-Docker-Token: true`
	// header while authenticating, in order to retrieve a token that can be later used to
	// perform authenticated actions.
	//
	// The mock v1 registry does not support that, (TODO(tiborvass): support it), instead,
	// it will consider authenticated any request with the header `X-Docker-Token: fake-token`.
	//
	// Because we know that the client's transport is an `*authTransport` we simply cast it,
	// in order to set the internal cached token to the fake token, and thus send that fake token
	// upon every subsequent requests.
	r.client.Transport.(*authTransport).token = []string{"fake-token"}
	return r
}
