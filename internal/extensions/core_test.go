package extensions

import (
	"context"
	"errors"
	"testing"

	"gotest.tools/v3/assert"
)

type recordingRegistrar struct {
	extensions []Extension
	err        error
}

func (r *recordingRegistrar) Register(ext Extension) error {
	if r.err != nil {
		return r.err
	}
	r.extensions = append(r.extensions, ext)
	return nil
}

func TestRegisterAllRegistersExtensions(t *testing.T) {
	registrar := &recordingRegistrar{}

	err := RegisterAll(registrar, New(Declaration{ID: "first"}), New(Declaration{ID: "second"}))
	assert.NilError(t, err)
	assert.Equal(t, len(registrar.extensions), 2)
	assert.Equal(t, registrar.extensions[0].Declaration().ID, ExtensionID("first"))
	assert.Equal(t, registrar.extensions[1].Declaration().ID, ExtensionID("second"))
}

func TestRegisterAllReturnsRegisterError(t *testing.T) {
	wantErr := errors.New("register failed")
	registrar := &recordingRegistrar{err: wantErr}

	err := RegisterAll(registrar, New(Declaration{ID: "extension"}))
	assert.ErrorIs(t, err, wantErr)
}

func TestDefinePointValidatesID(t *testing.T) {
	valid := []PointID{
		"org.mobyproject.extension.volume.driver.v1",
		"org.mobyproject.extension.container.create_hook.v0",
		"com.docker.compose.api.v12",
		"a.b.v0",
	}
	for _, id := range valid {
		t.Run("valid/"+string(id), func(t *testing.T) {
			assert.Equal(t, DefinePoint[any](id).ID(), id)
		})
	}

	invalid := []PointID{
		"",
		"org.mobyproject.greeter",            // no version
		"greeter.v1",                         // only one segment before the version
		"org.mobyproject.extension.v1.thing", // version not last
		"Org.Mobyproject.Greeter.v1",         // uppercase
		"org.mobyproject.greeter.v",          // no version number
	}
	for _, id := range invalid {
		t.Run("invalid/"+string(id), func(t *testing.T) {
			assert.Assert(t, panics(func() { DefinePoint[any](id) }), "DefinePoint(%q) should panic", id)
		})
	}
}

func panics(f func()) (panicked bool) {
	defer func() { panicked = recover() != nil }()
	f()
	return false
}

func TestValidateExtensionID(t *testing.T) {
	valid := []ExtensionID{
		"org.example.no-privileged.v1",
		"com.docker.compose.v1",
		"com.docker.mobyextension.nri.v1",
		"org.example.s3-volume.v2",
		"org.mobyproject.example.greeter.v0",
	}
	for _, id := range valid {
		if err := ValidateExtensionID(id); err != nil {
			t.Errorf("ValidateExtensionID(%q) = %v, want nil", id, err)
		}
	}

	invalid := []ExtensionID{
		"",
		"single",                     // not reverse-DNS (one segment)
		"org.example.no-privileged",  // missing version segment
		"com.docker.compose",         // missing version segment
		"foo.v1",                     // version but only one name segment
		"Org.Example.Ext.v1",         // uppercase
		"org.example/evil.v1",        // path separator
		"org.example.../etc",         // path traversal shape
		"org.example.-bad.v1",        // segment leads with hyphen
		"org.example.bad-.v1",        // segment trails with hyphen
		"org.example.a b.v1",         // whitespace
		"org..example.v1",            // empty segment
		"org.example.under_score.v1", // underscore not allowed in extension ids
	}
	for _, id := range invalid {
		if err := ValidateExtensionID(id); err == nil {
			t.Errorf("ValidateExtensionID(%q) = nil, want error", id)
		}
	}
}

func TestSingleSelection(t *testing.T) {
	stock := callerFunc(func(context.Context) error { return nil })
	custom := callerFunc(func(context.Context) error { return nil })

	t.Run("builtin alone stands in", func(t *testing.T) {
		got, err := testPoint.Single(resolverOf(
			ResolvedProvider{Extension: "org.mobyproject.stock.v1", Impl: stock, Builtin: true},
		))
		assert.NilError(t, err)
		assert.NilError(t, got.Call(context.Background()), "the default must stand in when nothing is installed")
	})

	t.Run("installed provider masks the builtin", func(t *testing.T) {
		called := ""
		stockC := callerFunc(func(context.Context) error { called = "stock"; return nil })
		customC := callerFunc(func(context.Context) error { called = "custom"; return nil })
		got, err := testPoint.Single(resolverOf(
			ResolvedProvider{Extension: "org.mobyproject.stock.v1", Impl: stockC, Builtin: true},
			ResolvedProvider{Extension: "org.example.custom.v1", Impl: customC},
		))
		assert.NilError(t, err)
		assert.NilError(t, got.Call(context.Background()))
		assert.Equal(t, called, "custom", "the installed provider must replace the builtin, not conflict with it")
	})

	t.Run("two installed providers are rejected", func(t *testing.T) {
		_, err := testPoint.Single(resolverOf(
			ResolvedProvider{Extension: "org.mobyproject.stock.v1", Impl: stock, Builtin: true},
			ResolvedProvider{Extension: "org.example.one.v1", Impl: custom},
			ResolvedProvider{Extension: "org.example.two.v1", Impl: custom},
		))
		assert.ErrorContains(t, err, "multiple providers")
	})

	t.Run("no providers is an error", func(t *testing.T) {
		_, err := testPoint.Single(resolverOf())
		assert.ErrorContains(t, err, "no providers")
	})

	t.Run("wrong provider type names the extension", func(t *testing.T) {
		_, err := testPoint.Single(resolverOf(
			ResolvedProvider{Extension: "org.example.broken.v1", Impl: 42},
		))
		assert.ErrorContains(t, err, `extension "org.example.broken.v1"`)
	})
}

func TestEffectiveProviders(t *testing.T) {
	ids := func(providers []ResolvedProvider) []ExtensionID {
		out := make([]ExtensionID, 0, len(providers))
		for _, p := range providers {
			out = append(out, p.Extension)
		}
		return out
	}
	def := ResolvedProvider{Extension: "d", Builtin: true}
	a, b := ResolvedProvider{Extension: "a"}, ResolvedProvider{Extension: "b"}

	assert.Equal(t, len(EffectiveProviders(nil)), 0)
	assert.DeepEqual(t, ids(EffectiveProviders([]ResolvedProvider{def})), []ExtensionID{"d"})
	assert.DeepEqual(t, ids(EffectiveProviders([]ResolvedProvider{def, a})), []ExtensionID{"a"})
	assert.DeepEqual(t, ids(EffectiveProviders([]ResolvedProvider{def, a, b})), []ExtensionID{"a", "b"})
}

type stubResolver struct{ providers []ResolvedProvider }

func (stubResolver) Provider(PointID, ExtensionID) (any, error) { return nil, nil }
func (r stubResolver) Providers(PointID) []ResolvedProvider     { return r.providers }

type caller interface {
	Call(ctx context.Context) error
}

type callerFunc func(ctx context.Context) error

func (f callerFunc) Call(ctx context.Context) error { return f(ctx) }

var testPoint = DefinePoint[caller]("org.example.fanout.test.v0")

func resolverOf(providers ...ResolvedProvider) Resolver { return stubResolver{providers: providers} }
