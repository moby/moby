package host

import (
	"context"
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

func TestExtensionFromLaunchedRejectsUnsupportedPoints(t *testing.T) {
	const supported = extensions.PointID("org.mobyproject.extension.supported.v1")
	const unsupported = extensions.PointID("org.example.own.api.v1")

	providers := map[extensions.PointID]clientpoint.Provider{
		supported: func(grpc.ClientConnInterface) extensions.Provider {
			return extensions.Provider{Point: supported, Impl: "impl"}
		},
	}

	ext, err := extensionFromLaunched(&launcher.Launched{
		ID:     "org.example.ext.v1",
		Points: []launcher.LaunchedPoint{{ID: supported}},
	}, providers, nil)
	assert.NilError(t, err)
	assert.Equal(t, len(ext.Declaration().Providers), 1)

	_, err = extensionFromLaunched(&launcher.Launched{
		ID:     "org.example.ext.v1",
		Points: []launcher.LaunchedPoint{{ID: supported}, {ID: unsupported}},
	}, providers, nil)
	assert.ErrorContains(t, err, "unsupported point")
	assert.ErrorContains(t, err, string(unsupported))

	ext, err = extensionFromLaunched(&launcher.Launched{
		ID:     "org.example.ext.v1",
		Points: []launcher.LaunchedPoint{{ID: supported}, {ID: unsupported}},
	}, providers, map[extensions.PointID]bool{unsupported: true})
	assert.NilError(t, err)
	assert.Equal(t, len(ext.Declaration().Providers), 1)
}

func TestClientProviderMap(t *testing.T) {
	const pointA = extensions.PointID("org.example.a.v1")
	const pointB = extensions.PointID("org.example.b.v1")
	build := func(grpc.ClientConnInterface) extensions.Provider {
		return extensions.Provider{}
	}

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

	_, err = clientProviderMap([]clientpoint.Registration{
		{Point: pointA, Provider: build},
		{Point: pointA, Provider: build},
	})
	assert.ErrorContains(t, err, "duplicate client provider")
	assert.ErrorContains(t, err, string(pointA))
}

func newProviderExtension(id extensions.ExtensionID, point extensions.PointID) extensions.Extension {
	return extensions.New(extensions.Declaration{
		ID:        id,
		Providers: []extensions.Provider{{Point: point, Impl: "impl"}},
	})
}

func TestServeCallback(t *testing.T) {
	const dep = extensions.PointID("org.mobyproject.extension.dep.v1")

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

func TestSinglePointRejectsTwoProviders(t *testing.T) {
	const point = extensions.PointID("org.example.decider.v1")
	ext := func(id extensions.ExtensionID) extensions.Extension {
		return extensions.New(extensions.Declaration{
			ID:        id,
			Providers: []extensions.Provider{{Point: point, Impl: struct{}{}}},
		})
	}
	singleReg := clientpoint.Registration{
		Point:    point,
		Provider: func(grpc.ClientConnInterface) extensions.Provider { return extensions.Provider{} },
		Single:   true,
	}

	_, err := New(context.Background(), Options{
		RuntimeDir:      t.TempDir(),
		Extensions:      []extensions.Extension{ext("org.example.one.v1"), ext("org.example.two.v1")},
		ClientProviders: []clientpoint.Registration{singleReg},
	})
	assert.ErrorContains(t, err, `point "org.example.decider.v1" admits a single provider`)
	assert.ErrorContains(t, err, "org.example.one.v1")
	assert.ErrorContains(t, err, "org.example.two.v1")

	h, err := New(context.Background(), Options{
		RuntimeDir:      t.TempDir(),
		Extensions:      []extensions.Extension{ext("org.example.one.v1")},
		ClientProviders: []clientpoint.Registration{singleReg},
	})
	assert.NilError(t, err)
	assert.NilError(t, h.Shutdown(context.Background()))
}

// TestLaunchedExtensionCarriesShutdown verifies launched extensions participate
// in broker shutdown ordering.
func TestLaunchedExtensionCarriesShutdown(t *testing.T) {
	const point = extensions.PointID("org.mobyproject.extension.supported.v1")
	providers := map[extensions.PointID]clientpoint.Provider{
		point: func(grpc.ClientConnInterface) extensions.Provider {
			return extensions.Provider{Point: point, Impl: "impl"}
		},
	}

	ext, err := extensionFromLaunched(&launcher.Launched{
		ID:     "org.example.ext.v1",
		Points: []launcher.LaunchedPoint{{ID: point}},
	}, providers, nil)
	assert.NilError(t, err)
	assert.Assert(t, ext.Declaration().Shutdown != nil,
		"a launched extension must declare a Shutdown so the broker stops it in dependency order")
}
