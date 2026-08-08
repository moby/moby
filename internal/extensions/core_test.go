package extensions

import (
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

// panics reports whether f panics.
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
