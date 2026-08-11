package broker

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/moby/moby/v2/internal/extensions"
	"gotest.tools/v3/assert"
)

type pingProvider struct{}

func (pingProvider) Ping(context.Context) error { return nil }

func TestInitOrdersDependencies(t *testing.T) {
	ctx := context.Background()
	b := New()
	var order []extensions.ExtensionID

	err := b.Register(extensions.New(extensions.Declaration{
		ID:        "org.test.dependent.v1",
		Providers: []extensions.Provider{{Point: "dependent.point", Impl: pingProvider{}}},
		Dependencies: []extensions.Dependency{
			{Point: "dependency.point"},
			{Extension: "org.test.named-dependency.v1"},
		},
		Init: func(context.Context, extensions.Config, extensions.Resolver) error {
			order = append(order, "org.test.dependent.v1")
			return nil
		},
	}))
	assert.NilError(t, err)
	err = b.Register(extensions.New(extensions.Declaration{
		ID:        "org.test.point-dependency.v1",
		Providers: []extensions.Provider{{Point: "dependency.point", Impl: pingProvider{}}},
		Init: func(context.Context, extensions.Config, extensions.Resolver) error {
			order = append(order, "org.test.point-dependency.v1")
			return nil
		},
	}))
	assert.NilError(t, err)
	err = b.Register(extensions.New(extensions.Declaration{
		ID: "org.test.named-dependency.v1",
		Init: func(context.Context, extensions.Config, extensions.Resolver) error {
			order = append(order, "org.test.named-dependency.v1")
			return nil
		},
	}))
	assert.NilError(t, err)

	assert.NilError(t, b.Init(ctx, nil))

	dependentIndex := slices.Index(order, extensions.ExtensionID("org.test.dependent.v1"))
	pointDependencyIndex := slices.Index(order, extensions.ExtensionID("org.test.point-dependency.v1"))
	namedDependencyIndex := slices.Index(order, extensions.ExtensionID("org.test.named-dependency.v1"))
	assert.Check(t, dependentIndex >= 0)
	assert.Check(t, pointDependencyIndex >= 0)
	assert.Check(t, namedDependencyIndex >= 0)
	assert.Check(t, pointDependencyIndex < dependentIndex)
	assert.Check(t, namedDependencyIndex < dependentIndex)
}

func TestShutdownOrdersDependenciesInReverse(t *testing.T) {
	b := New()
	var order []extensions.ExtensionID
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{
		ID:           "org.test.dependent.v1",
		Dependencies: []extensions.Dependency{{Extension: "org.test.dependency.v1"}},
		Shutdown: func(context.Context) error {
			order = append(order, "org.test.dependent.v1")
			return nil
		},
	})))
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{
		ID: "org.test.dependency.v1",
		Shutdown: func(context.Context) error {
			order = append(order, "org.test.dependency.v1")
			return nil
		},
	})))
	assert.NilError(t, b.Init(context.Background(), nil))

	err := b.Shutdown(context.Background())
	assert.NilError(t, err)
	assert.DeepEqual(t, order, []extensions.ExtensionID{"org.test.dependent.v1", "org.test.dependency.v1"})
}

func TestShutdownSkipsUninitialized(t *testing.T) {
	b := New()
	var shutdown []extensions.ExtensionID
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{
		ID: "org.test.registered-not-initialized.v1",
		Shutdown: func(context.Context) error {
			shutdown = append(shutdown, "org.test.registered-not-initialized.v1")
			return nil
		},
	})))

	assert.NilError(t, b.Shutdown(context.Background()))
	assert.Check(t, len(shutdown) == 0, "Shutdown ran on an uninitialized extension: %v", shutdown)
}

func TestShutdownUnwindsPartialInit(t *testing.T) {
	b := New()
	var order []extensions.ExtensionID
	shutdownRecorder := func(id extensions.ExtensionID) func(context.Context) error {
		return func(context.Context) error {
			order = append(order, id)
			return nil
		}
	}
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{
		ID:       "org.test.first.v1",
		Init:     func(context.Context, extensions.Config, extensions.Resolver) error { return nil },
		Shutdown: shutdownRecorder("org.test.first.v1"),
	})))
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{
		ID: "org.test.boom.v1",
		Init: func(context.Context, extensions.Config, extensions.Resolver) error {
			return errors.New("init failed")
		},
		Shutdown: shutdownRecorder("org.test.boom.v1"),
	})))
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{
		ID:       "org.test.last.v1",
		Init:     func(context.Context, extensions.Config, extensions.Resolver) error { return nil },
		Shutdown: shutdownRecorder("org.test.last.v1"),
	})))

	err := b.Init(context.Background(), nil)
	assert.ErrorContains(t, err, "init failed")

	assert.NilError(t, b.Shutdown(context.Background()))
	assert.DeepEqual(t, order, []extensions.ExtensionID{"org.test.first.v1"})
}

func TestLookupProviders(t *testing.T) {
	b := New()
	first := pingProvider{}
	second := pingProvider{}

	for _, ext := range []extensions.Declaration{
		{ID: "org.test.first.v1", Providers: []extensions.Provider{{Point: "point", Impl: first}}},
		{ID: "org.test.second.v1", Providers: []extensions.Provider{{Point: "point", Impl: second}}},
	} {
		assert.NilError(t, b.Register(extensions.New(ext)))
	}

	provider, err := b.Provider("point", "org.test.second.v1")
	assert.NilError(t, err)
	assert.Equal(t, provider, second)
	providers := b.Providers("point")
	assert.Equal(t, len(providers), 2)
	providerIDs := map[extensions.ExtensionID]bool{}
	for _, provider := range providers {
		providerIDs[provider.Extension] = true
	}
	assert.Check(t, providerIDs["org.test.first.v1"])
	assert.Check(t, providerIDs["org.test.second.v1"])
}

// TestConcurrentAccess exercises concurrent reads and registration.
func TestConcurrentAccess(t *testing.T) {
	b := New()
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{
		ID:        "org.test.a.v1",
		Providers: []extensions.Provider{{Point: "a.point.v1", Impl: pingProvider{}}},
	})))
	assert.NilError(t, b.Init(context.Background(), nil))

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Providers("a.point.v1")
			_, _ = b.Provider("a.point.v1", "org.test.a.v1")
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = b.Register(extensions.New(extensions.Declaration{ID: "org.test.b.v1"}))
	}()
	wg.Wait()
}

func TestTypedPointLookup(t *testing.T) {
	point := extensions.DefinePoint[interface{ Ping(context.Context) error }]("test.typed.v1")
	b := New()
	first := pingProvider{}
	second := pingProvider{}
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{ID: "org.test.first.v1", Providers: []extensions.Provider{point.Provide(first)}})))
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{ID: "org.test.second.v1", Providers: []extensions.Provider{point.Provide(second)}})))

	providers, err := point.All(b)
	assert.NilError(t, err)
	assert.Equal(t, len(providers), 2)
	assert.Equal(t, providers[0].Extension, extensions.ExtensionID("org.test.first.v1"))
	assert.Equal(t, providers[0].Impl, first)

	provider, err := point.ByExtension(b, "org.test.second.v1")
	assert.NilError(t, err)
	assert.Equal(t, provider, second)

	assert.Equal(t, point.Dependency(), extensions.Dependency{Point: point.ID()})
}

func TestTypedPointLookupRejectsWrongImplementationType(t *testing.T) {
	point := extensions.DefinePoint[interface{ Ping(context.Context) error }]("test.typed.v1")
	b := New()
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{ID: "org.test.broken.v1", Providers: []extensions.Provider{{Point: point.ID(), Impl: "not a ping provider"}}})))

	_, err := point.All(b)
	assert.ErrorContains(t, err, `extension "org.test.broken.v1" provider for point "test.typed.v1" has type string`)
}

func TestRegisterRejectsExtensionConflicts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		first   []extensions.ExtensionID
		second  []extensions.ExtensionID
		wantErr string
	}{
		{
			name:    "first extension declares conflict",
			first:   []extensions.ExtensionID{"org.test.second.v1"},
			wantErr: `extension "org.test.second.v1" conflicts with extension "org.test.first.v1"`,
		},
		{
			name:    "second extension declares conflict",
			second:  []extensions.ExtensionID{"org.test.first.v1"},
			wantErr: `extension "org.test.second.v1" conflicts with extension "org.test.first.v1"`,
		},
		{
			name:    "both extensions declare conflict",
			first:   []extensions.ExtensionID{"org.test.second.v1"},
			second:  []extensions.ExtensionID{"org.test.first.v1"},
			wantErr: `extension "org.test.second.v1" conflicts with extension "org.test.first.v1"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := New()
			assert.NilError(t, b.Register(extensions.New(extensions.Declaration{ID: "org.test.first.v1", Conflicts: tc.first})))

			err := b.Register(extensions.New(extensions.Declaration{ID: "org.test.second.v1", Conflicts: tc.second}))
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestRegisterRejectsInvalidExtensionConflicts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		conflicts []extensions.ExtensionID
		wantErr   string
	}{
		{
			name:      "empty conflict id",
			conflicts: []extensions.ExtensionID{""},
			wantErr:   `extension "org.test.invalid.v1" has empty conflict id`,
		},
		{
			name:      "self conflict",
			conflicts: []extensions.ExtensionID{"org.test.invalid.v1"},
			wantErr:   `extension "org.test.invalid.v1" conflicts with itself`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := New()
			err := b.Register(extensions.New(extensions.Declaration{ID: "org.test.invalid.v1", Conflicts: tc.conflicts}))
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestInitFailsForMissingRequiredDependency(t *testing.T) {
	b := New()
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{ID: "org.test.dependent.v1", Dependencies: []extensions.Dependency{{Point: "missing.point"}}})))

	err := b.Init(context.Background(), nil)
	assert.ErrorContains(t, err, `requires missing point "missing.point"`)
}

func TestInitAllowsMissingOptionalDependency(t *testing.T) {
	b := New()
	initialized := false
	err := b.Register(extensions.New(extensions.Declaration{
		ID:           "org.test.dependent.v1",
		Dependencies: []extensions.Dependency{{Point: "missing.point", Optional: true}},
		Init: func(context.Context, extensions.Config, extensions.Resolver) error {
			initialized = true
			return nil
		},
	}))
	assert.NilError(t, err)

	assert.NilError(t, b.Init(context.Background(), nil))
	assert.Check(t, initialized)
}

func TestInitFailsForDependencyCycle(t *testing.T) {
	b := New()
	for _, ext := range []extensions.Declaration{
		{ID: "org.test.first.v1", Dependencies: []extensions.Dependency{{Extension: "org.test.second.v1"}}},
		{ID: "org.test.second.v1", Dependencies: []extensions.Dependency{{Extension: "org.test.first.v1"}}},
	} {
		assert.NilError(t, b.Register(extensions.New(ext)))
	}

	err := b.Init(context.Background(), nil)
	assert.ErrorContains(t, err, "extension dependency cycle")
}

func TestInitWrapsExtensionError(t *testing.T) {
	b := New()
	initErr := errors.New("org.test.boom.v1")
	err := b.Register(extensions.New(extensions.Declaration{
		ID: "org.test.broken.v1",
		Init: func(context.Context, extensions.Config, extensions.Resolver) error {
			return initErr
		},
	}))
	assert.NilError(t, err)

	err = b.Init(context.Background(), nil)
	assert.ErrorIs(t, err, initErr)
}

func TestBuiltinEnumeration(t *testing.T) {
	const point = extensions.PointID("org.example.decider.v1")
	stock := "stock"
	custom := "custom"

	stockExt := extensions.New(extensions.Declaration{
		ID:        "org.mobyproject.stock.v1",
		Providers: []extensions.Provider{{Point: point, Impl: stock}},
	})
	customExt := extensions.New(extensions.Declaration{
		ID:        "org.example.custom.v1",
		Providers: []extensions.Provider{{Point: point, Impl: custom}},
	})

	t.Run("origin is recorded at registration", func(t *testing.T) {
		b := New()
		assert.NilError(t, b.RegisterBuiltin(stockExt))
		providers := b.Providers(point)
		assert.Equal(t, len(providers), 1)
		assert.Equal(t, providers[0].Impl, stock)
		assert.Check(t, providers[0].Builtin, "a RegisterBuiltin extension's providers must carry the origin")
	})

	t.Run("builtins are enumerated beside installed providers", func(t *testing.T) {
		b := New()
		assert.NilError(t, b.RegisterBuiltin(stockExt))
		assert.NilError(t, b.Register(customExt))
		providers := b.Providers(point)
		assert.Equal(t, len(providers), 2, "the registry enumerates; it does not mask")

		effective := extensions.EffectiveProviders(providers)
		assert.Equal(t, len(effective), 1, "origin precedence is selection, applied over the enumeration")
		assert.Equal(t, effective[0].Extension, extensions.ExtensionID("org.example.custom.v1"))
		assert.Check(t, !effective[0].Builtin)
	})

	t.Run("a masked builtin stays reachable by id", func(t *testing.T) {
		b := New()
		assert.NilError(t, b.RegisterBuiltin(stockExt))
		assert.NilError(t, b.Register(customExt))
		impl, err := b.Provider(point, "org.mobyproject.stock.v1")
		assert.NilError(t, err)
		assert.Equal(t, impl, stock)
	})
}
