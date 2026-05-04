package containerd

import (
	"context"
	"net/http"
	"testing"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/distribution/reference"
	registrytypes "github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestResolveAuthHost(t *testing.T) {
	for _, tc := range []struct {
		name     string
		auth     registrytypes.AuthConfig
		ref      string
		expected string
	}{
		{
			name:     "docker hub via index URL",
			auth:     registrytypes.AuthConfig{ServerAddress: "https://index.docker.io/v1/"},
			ref:      "docker.io/library/nginx",
			expected: "registry-1.docker.io",
		},
		{
			name:     "docker hub via index hostname",
			auth:     registrytypes.AuthConfig{ServerAddress: "index.docker.io"},
			ref:      "docker.io/library/nginx",
			expected: "registry-1.docker.io",
		},
		{
			name:     "docker hub via docker.io",
			auth:     registrytypes.AuthConfig{ServerAddress: "docker.io"},
			ref:      "docker.io/library/nginx",
			expected: "registry-1.docker.io",
		},
		{
			name:     "empty server address defaults to ref domain",
			auth:     registrytypes.AuthConfig{},
			ref:      "docker.io/library/nginx",
			expected: "registry-1.docker.io",
		},
		{
			name:     "private registry",
			auth:     registrytypes.AuthConfig{ServerAddress: "ghcr.io"},
			ref:      "ghcr.io/org/image",
			expected: "ghcr.io",
		},
		{
			name:     "private registry with port",
			auth:     registrytypes.AuthConfig{ServerAddress: "registry.example.com:5000"},
			ref:      "registry.example.com:5000/myimage",
			expected: "registry.example.com:5000",
		},
		{
			name:     "private registry via https URL",
			auth:     registrytypes.AuthConfig{ServerAddress: "https://registry.example.com:5000"},
			ref:      "registry.example.com:5000/myimage",
			expected: "registry.example.com:5000",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := reference.ParseNormalizedNamed(tc.ref)
			assert.NilError(t, err)
			got := resolveAuthHost(tc.auth, ref)
			assert.Check(t, is.Equal(got, tc.expected))
		})
	}
}

// stubAuthorizer is a test Authorizer that records whether it was used.
type stubAuthorizer struct {
	name string
}

func (a *stubAuthorizer) Authorize(_ context.Context, _ *http.Request) error { return nil }
func (a *stubAuthorizer) AddResponses(_ context.Context, _ []*http.Response) error {
	return nil
}

func TestHostsWrapper_OnlyAppliesToTargetHost(t *testing.T) {
	mirrorAuth := &stubAuthorizer{name: "mirror-auth"}
	primaryAuth := &stubAuthorizer{name: "primary-auth"}

	hostsFn := func(n string) ([]docker.RegistryHost, error) {
		return []docker.RegistryHost{
			{Host: "mirror.example.com", Authorizer: mirrorAuth},
			{Host: "registry-1.docker.io", Authorizer: primaryAuth},
		}, nil
	}

	ref, err := reference.ParseNormalizedNamed("docker.io/library/nginx")
	assert.NilError(t, err)

	auth := &registrytypes.AuthConfig{
		Username:      "user",
		Password:      "pass",
		ServerAddress: "https://index.docker.io/v1/",
	}

	wrapped := hostsWrapper(hostsFn, auth, ref)
	hosts, err := wrapped("docker.io")
	assert.NilError(t, err)
	assert.Assert(t, is.Len(hosts, 2))

	// Mirror host should keep its original authorizer.
	assert.Check(t, hosts[0].Host == "mirror.example.com")
	assert.Check(t, hosts[0].Authorizer == mirrorAuth,
		"mirror authorizer was replaced; expected it to be preserved")

	// Primary host should get the new authorizer from authConfig.
	assert.Check(t, hosts[1].Host == "registry-1.docker.io")
	assert.Check(t, hosts[1].Authorizer != primaryAuth,
		"primary authorizer was not replaced; expected it to be updated with authConfig credentials")
}

func TestHostsWrapper_NilAuthConfig(t *testing.T) {
	called := false
	hostsFn := func(n string) ([]docker.RegistryHost, error) {
		called = true
		return []docker.RegistryHost{{Host: "registry-1.docker.io"}}, nil
	}

	ref, err := reference.ParseNormalizedNamed("docker.io/library/nginx")
	assert.NilError(t, err)

	wrapped := hostsWrapper(hostsFn, nil, ref)

	// When authConfig is nil, hostsWrapper should return hostsFn unchanged.
	hosts, err := wrapped("docker.io")
	assert.NilError(t, err)
	assert.Assert(t, called)
	assert.Assert(t, is.Len(hosts, 1))
}

func TestHostsWrapper_PreservesMirrorAuthorizer(t *testing.T) {
	// Simulates a hosts.toml-configured mirror that has its own Authorizer
	// with static Authorization header (e.g., [header] Authorization = "Basic ...").
	mirrorAuth := &stubAuthorizer{name: "hosts-toml-mirror"}

	hostsFn := func(n string) ([]docker.RegistryHost, error) {
		return []docker.RegistryHost{
			{Host: "nexus.corp.example.com", Authorizer: mirrorAuth},
			{Host: "registry-1.docker.io", Authorizer: nil},
		}, nil
	}

	ref, err := reference.ParseNormalizedNamed("docker.io/library/alpine")
	assert.NilError(t, err)

	auth := &registrytypes.AuthConfig{
		Username:      "hubuser",
		Password:      "hubpass",
		ServerAddress: "https://index.docker.io/v1/",
	}

	wrapped := hostsWrapper(hostsFn, auth, ref)
	hosts, err := wrapped("docker.io")
	assert.NilError(t, err)

	// Mirror's authorizer from hosts.toml must be preserved.
	assert.Check(t, hosts[0].Host == "nexus.corp.example.com")
	assert.Check(t, hosts[0].Authorizer == mirrorAuth,
		"hosts.toml mirror authorizer was replaced; credentials would be lost")

	// Primary gets the authConfig-based authorizer.
	assert.Check(t, hosts[1].Host == "registry-1.docker.io")
	assert.Check(t, hosts[1].Authorizer != nil,
		"primary host should have an authorizer set from authConfig")
}

func TestHostsWrapper_PrivateRegistry(t *testing.T) {
	// For non-docker.io registries, there are no mirrors. The single host
	// should get the authConfig authorizer applied.
	hostsFn := func(n string) ([]docker.RegistryHost, error) {
		return []docker.RegistryHost{
			{Host: "ghcr.io"},
		}, nil
	}

	ref, err := reference.ParseNormalizedNamed("ghcr.io/org/myimage")
	assert.NilError(t, err)

	auth := &registrytypes.AuthConfig{
		Username:      "ghuser",
		Password:      "ghtoken",
		ServerAddress: "ghcr.io",
	}

	wrapped := hostsWrapper(hostsFn, auth, ref)
	hosts, err := wrapped("ghcr.io")
	assert.NilError(t, err)
	assert.Assert(t, is.Len(hosts, 1))
	assert.Check(t, hosts[0].Authorizer != nil,
		"private registry host should get authConfig authorizer")
}
