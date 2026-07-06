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

// TestShutdownSkipsUninitialized locks down that Shutdown never runs on an
// extension whose Init did not run. A host that fails to start (a launch or a
// later Register errors before Init) shuts the broker down for cleanup; calling
// Shutdown on an extension that was only registered would tear down state its
// Init never set up.
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

	// Init was never called, so nothing is initialized.
	assert.NilError(t, b.Shutdown(context.Background()))
	assert.Check(t, len(shutdown) == 0, "Shutdown ran on an uninitialized extension: %v", shutdown)
}

// TestShutdownUnwindsPartialInit locks down that when Init fails part-way, a
// following Shutdown tears down exactly the extensions that were initialized,
// in reverse order, and leaves the rest alone.
func TestShutdownUnwindsPartialInit(t *testing.T) {
	b := New()
	var order []extensions.ExtensionID
	shutdownRecorder := func(id extensions.ExtensionID) func(context.Context) error {
		return func(context.Context) error {
			order = append(order, id)
			return nil
		}
	}
	// No dependencies, so resolved order is registration order: first, then
	// boom, then last. boom's Init fails; last is never reached.
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
	// Only "org.test.first.v1" initialized; "org.test.boom.v1" failed its Init, "org.test.last.v1" never ran.
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
	_, err = b.SingleProvider("point")
	assert.ErrorContains(t, err, "multiple providers")
}

func TestSingleProvider(t *testing.T) {
	b := New()
	provider := pingProvider{}
	assert.NilError(t, b.Register(extensions.New(extensions.Declaration{ID: "org.test.only.v1", Providers: []extensions.Provider{{Point: "point", Impl: provider}}})))

	got, err := b.SingleProvider("point")
	assert.NilError(t, err)
	assert.Equal(t, got, provider)
}

// TestConcurrentAccess exercises the broker under concurrent reads and a
// registration, so `go test -race` proves the lock protects the registry. It
// stands in for a future Load that mutates the broker while handlers read it.
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
			_, _ = b.SingleProvider("a.point.v1")
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
