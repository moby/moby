package containerd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/distribution/reference"
	registrytypes "github.com/moby/moby/api/types/registry"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

func TestResolverReusesRegistryHostsAndBearerTokensAcrossPhases(t *testing.T) {
	t.Parallel()

	manifest := []byte(`{"schemaVersion":2}`)
	manifestDigest := digest.FromBytes(manifest)
	type testRegistry struct {
		server               *httptest.Server
		token                string
		service              string
		rejectManifest       bool
		tokenRequests        atomic.Int32
		unauthorizedRequests atomic.Int32
	}
	registries := make([]*testRegistry, 2)
	for i, config := range []struct {
		token          string
		service        string
		rejectManifest bool
	}{
		{token: "first-token", service: "first-registry", rejectManifest: true},
		{token: "second-token", service: "second-registry"},
	} {
		registry := &testRegistry{
			token:          config.token,
			service:        config.service,
			rejectManifest: config.rejectManifest,
		}
		registry.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/token":
				registry.tokenRequests.Add(1)
				if r.Method != http.MethodGet || r.URL.Query().Get("service") != registry.service || r.URL.Query().Get("scope") != "repository:repo:pull" {
					http.Error(w, "unexpected token request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"token":%q,"expires_in":300}`, registry.token)
			case strings.HasPrefix(r.URL.Path, "/v2/repo/manifests/"):
				if r.Header.Get("Authorization") != "Bearer "+registry.token {
					registry.unauthorizedRequests.Add(1)
					w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q,service=%q,scope="repository:repo:pull"`, registry.server.URL+"/token", registry.service))
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if registry.rejectManifest {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
				w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
				w.Header().Set("Docker-Content-Digest", manifestDigest.String())
				if r.Method == http.MethodGet {
					_, _ = w.Write(manifest)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(registry.server.Close)
		registries[i] = registry
	}

	host := strings.TrimPrefix(registries[0].server.URL, "https://")
	ref, err := reference.ParseNormalizedNamed(host + "/repo:latest")
	assert.NilError(t, err)

	newClient := func(server *httptest.Server) *http.Client {
		transport := server.Client().Transport.(*http.Transport).Clone()
		return &http.Client{Transport: transport}
	}
	var hostCalls atomic.Int32
	hostsFn := func(string) ([]docker.RegistryHost, error) {
		hostCalls.Add(1)
		return []docker.RegistryHost{
			{
				Client:       newClient(registries[0].server),
				Host:         strings.TrimPrefix(registries[0].server.URL, "https://"),
				Scheme:       "https",
				Path:         "v2",
				Capabilities: docker.HostCapabilityPull,
			},
			{
				Client:       newClient(registries[1].server),
				Host:         strings.TrimPrefix(registries[1].server.URL, "https://"),
				Scheme:       "https",
				Path:         "v2",
				Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
			},
		}, nil
	}
	authConfig := registrytypes.AuthConfig{ServerAddress: registries[0].server.URL}
	imageService := ImageService{registryHosts: hostsFn}
	resolver, _ := imageService.newResolverFromAuthConfig(t.Context(), &authConfig, ref, nil)

	_, desc, err := resolver.Resolve(t.Context(), ref.String())
	assert.NilError(t, err)
	assert.Equal(t, desc.Digest, manifestDigest)

	fetcher, err := resolver.Fetcher(t.Context(), ref.String())
	assert.NilError(t, err)
	reader, err := fetcher.Fetch(t.Context(), desc)
	assert.NilError(t, err)
	content, err := io.ReadAll(reader)
	assert.NilError(t, err)
	assert.NilError(t, reader.Close())
	assert.Equal(t, string(content), string(manifest))

	assert.Equal(t, hostCalls.Load(), int32(1))
	for _, registry := range registries {
		assert.Equal(t, registry.tokenRequests.Load(), int32(1))
		assert.Equal(t, registry.unauthorizedRequests.Load(), int32(1))
	}
}

func TestNormalizeRegistryHost(t *testing.T) {
	t.Parallel()

	for tc, expected := range map[string]string{
		"example.com":          "example.com",
		"EXAMPLE.Com":          "example.com",
		"example.com:443":      "example.com",
		"example.com:80":       "example.com",
		"example.com:5000":     "example.com:5000",
		"REG.example.com:5000": "reg.example.com:5000",
		"[::1]:443":            "[::1]",
		"[fd00::1]:5000":       "[fd00::1]:5000",
		"::1":                  "::1",
	} {
		assert.Equal(t, normalizeRegistryHost(tc), expected, tc)
	}
}

// TestResolverPrimesAuthChallengeBeforeRegistryRequests verifies the
// resolver probes for a challenge up front, so the first content request
// carries credentials even when the registry never sends a 401.
func TestResolverPrimesAuthChallengeBeforeRegistryRequests(t *testing.T) {
	t.Parallel()

	manifest := []byte(`{"schemaVersion":2}`)
	manifestDigest := digest.FromBytes(manifest)
	const token = "primed-token"

	type registry struct {
		server *httptest.Server
	}
	reg := &registry{}

	var (
		probeRequests       atomic.Int32
		tokenRequests       atomic.Int32
		unauthorizedContent atomic.Int32
	)
	reg.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		challenge := func() {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q,service="test"`, reg.server.URL+"/token"))
			w.WriteHeader(http.StatusUnauthorized)
		}
		switch {
		case r.URL.Path == "/v2/":
			probeRequests.Add(1)
			challenge()
		case r.URL.Path == "/token":
			tokenRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"access_token":%q,"token":%q,"expires_in":300}`, token, token)
		case strings.HasPrefix(r.URL.Path, "/v2/repo/manifests/"):
			if r.Header.Get("Authorization") != "Bearer "+token {
				unauthorizedContent.Add(1)
				challenge()
				return
			}
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
			w.Header().Set("Docker-Content-Digest", manifestDigest.String())
			if r.Method == http.MethodGet {
				_, _ = w.Write(manifest)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(reg.server.Close)

	host := strings.TrimPrefix(reg.server.URL, "https://")
	ref, err := reference.ParseNormalizedNamed(host + "/repo:latest")
	assert.NilError(t, err)

	hostsFn := func(string) ([]docker.RegistryHost, error) {
		transport := reg.server.Client().Transport.(*http.Transport).Clone()
		return []docker.RegistryHost{
			{
				Client:       &http.Client{Transport: transport},
				Host:         host,
				Scheme:       "https",
				Path:         "v2",
				Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
			},
		}, nil
	}
	authConfig := registrytypes.AuthConfig{ServerAddress: reg.server.URL, Username: "user", Password: "pass"}
	imageService := ImageService{registryHosts: hostsFn}
	resolver, _ := imageService.newResolverFromAuthConfig(t.Context(), &authConfig, ref, nil)

	_, desc, err := resolver.Resolve(t.Context(), ref.String())
	assert.NilError(t, err)
	assert.Equal(t, desc.Digest, manifestDigest)

	assert.Equal(t, probeRequests.Load(), int32(1), "registry should have been probed for a challenge")
	assert.Equal(t, tokenRequests.Load(), int32(1))
	assert.Equal(t, unauthorizedContent.Load(), int32(0), "content requests must not be sent unauthenticated first")
}
