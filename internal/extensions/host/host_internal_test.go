package host

import (
	"path/filepath"
	"testing"

	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/clientpoint"
	"github.com/moby/moby/v2/internal/extensions/internal/broker"
	"github.com/moby/moby/v2/internal/extensions/internal/launcher"
	"github.com/moby/moby/v2/internal/extensions/serverpoint"
	"google.golang.org/grpc"
	"gotest.tools/v3/assert"
)

// TestExtensionFromLaunchedRejectsUnsupportedPoints locks down that a launched
// extension declaring a point the host has no ClientProvider for is rejected --
// the host cannot call it, so accepting it would be meaningless. A gRPC service
// an extension only publishes on the socket is not a point (it is served raw and
// named through service.grpc), so it does not reach here.
func TestExtensionFromLaunchedRejectsUnsupportedPoints(t *testing.T) {
	const supported = extensions.PointID("org.mobyproject.extension.supported.v1")
	const unsupported = extensions.PointID("org.example.own.api.v1")

	providers := map[extensions.PointID]clientpoint.Provider{
		supported: func(grpc.ClientConnInterface) extensions.Provider {
			return extensions.Provider{Point: supported, Impl: "impl"}
		},
	}

	// A declaration of only the supported point is accepted and wired.
	ext, err := extensionFromLaunched(&launcher.Launched{
		ID:     "org.example.ext.v1",
		Points: []launcher.LaunchedPoint{{ID: supported}},
	}, providers, nil)
	assert.NilError(t, err)
	assert.Equal(t, len(ext.Declaration().Providers), 1)

	// Declaring a point with no ClientProvider is rejected.
	_, err = extensionFromLaunched(&launcher.Launched{
		ID:     "org.example.ext.v1",
		Points: []launcher.LaunchedPoint{{ID: supported}, {ID: unsupported}},
	}, providers, nil)
	assert.ErrorContains(t, err, "unsupported point")
	assert.ErrorContains(t, err, string(unsupported))

	// An expose-only point has no ClientProvider but is exempt from the check --
	// it is published, not called -- so it is accepted and not wired.
	ext, err = extensionFromLaunched(&launcher.Launched{
		ID:     "org.example.ext.v1",
		Points: []launcher.LaunchedPoint{{ID: supported}, {ID: unsupported}},
	}, providers, map[extensions.PointID]bool{unsupported: true})
	assert.NilError(t, err)
	assert.Equal(t, len(ext.Declaration().Providers), 1)
}

// TestClientProviderMap locks down that client provider registrations are
// indexed by point id and that two registrations for the same point are
// rejected: the host resolves at most one client caller per point, so a
// duplicate is a misconfiguration rather than a silent last-wins overwrite.
func TestClientProviderMap(t *testing.T) {
	const pointA = extensions.PointID("org.example.a.v1")
	const pointB = extensions.PointID("org.example.b.v1")
	build := func(grpc.ClientConnInterface) extensions.Provider {
		return extensions.Provider{}
	}

	// Distinct point ids are indexed, one entry each.
	m, err := clientProviderMap([]clientpoint.Registration{
		{Point: pointA, Provider: build},
		{Point: pointB, Provider: build},
	})
	assert.NilError(t, err)
	assert.Equal(t, len(m), 2)
	_, okA := m[pointA]
	_, okB := m[pointB]
	assert.Assert(t, okA)
	assert.Assert(t, okB)

	// Two registrations for the same point are rejected.
	_, err = clientProviderMap([]clientpoint.Registration{
		{Point: pointA, Provider: build},
		{Point: pointA, Provider: build},
	})
	assert.ErrorContains(t, err, "duplicate client provider")
	assert.ErrorContains(t, err, string(pointA))
}

// newProviderExtension builds a trivial in-process extension identified by id
// that provides point, so a broker can be seeded with a known number of
// providers for the callback tests.
func newProviderExtension(id extensions.ExtensionID, point extensions.PointID) extensions.Extension {
	return extensions.New(extensions.Declaration{
		ID:        id,
		Providers: []extensions.Provider{{Point: point, Impl: "impl"}},
	})
}

// TestServeCallback exercises serveCallback's provider-count branch for a
// dependency point offered on the callback: zero providers is skipped (the
// point is simply not served), exactly one is registered, and more than one is
// a fatal ambiguity -- the callback serves a single provider per point, so an
// ambiguous point is failed loudly at startup rather than yielding Unimplemented
// at call time.
func TestServeCallback(t *testing.T) {
	const dep = extensions.PointID("org.mobyproject.extension.dep.v1")

	// A stub server registration for the dependency point that records the
	// implementations it was asked to serve; it needs only to be invokable.
	newDep := func(served *[]any) serverpoint.Registration {
		return serverpoint.Registration{
			Point: dep,
			Register: func(_ grpc.ServiceRegistrar, impl any) {
				*served = append(*served, impl)
			},
		}
	}

	t.Run("zero providers is skipped", func(t *testing.T) {
		b := broker.New()
		var served []any
		endpoint := filepath.Join(t.TempDir(), "callback.sock")
		srv, err := serveCallback(endpoint, []serverpoint.Registration{newDep(&served)}, b)
		assert.NilError(t, err)
		if srv != nil {
			defer srv.Stop()
		}
		assert.Equal(t, len(served), 0)
	})

	t.Run("one provider is registered", func(t *testing.T) {
		b := broker.New()
		assert.NilError(t, b.Register(newProviderExtension("org.example.a.v1", dep)))
		var served []any
		endpoint := filepath.Join(t.TempDir(), "callback.sock")
		srv, err := serveCallback(endpoint, []serverpoint.Registration{newDep(&served)}, b)
		assert.NilError(t, err)
		assert.Assert(t, srv != nil)
		defer srv.Stop()
		assert.Equal(t, len(served), 1)
	})

	t.Run("multiple providers is an error", func(t *testing.T) {
		b := broker.New()
		assert.NilError(t, b.Register(newProviderExtension("org.example.a.v1", dep)))
		assert.NilError(t, b.Register(newProviderExtension("org.example.b.v1", dep)))
		var served []any
		endpoint := filepath.Join(t.TempDir(), "callback.sock")
		srv, err := serveCallback(endpoint, []serverpoint.Registration{newDep(&served)}, b)
		if srv != nil {
			srv.Stop()
		}
		assert.ErrorContains(t, err, string(dep))
		assert.Equal(t, len(served), 0)
	})
}
