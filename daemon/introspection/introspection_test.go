package introspection

import (
	"reflect"
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestJoinScope(t *testing.T) {
	assert.Equal(t, joinScope(".", "foo", "bar"), ".foo.bar")
}

func TestInScope(t *testing.T) {
	assert.Equal(t, InScope(".foo", []string{"."}), true)
	assert.Equal(t, InScope(".foo", []string{".foo"}), true)
	assert.Equal(t, InScope(".foo.bar", []string{".foo"}), true)
	assert.Equal(t, InScope(".bar", []string{".foo"}), false)
	assert.Equal(t, InScope(".bar", []string{".foo", ".bar"}), true)

	// FIXME (does not hurt when scope set is verified)
	// assert.Equal(t, InScope(".foo", []string{".fo"}), false)
}

type testStruct0_0 struct {
	X int
}

type testStruct0 struct {
	Foo struct {
		Str string
		Map map[string]string
	}
	Bar int
	Baz testStruct0_0
}

func TestScopes(t *testing.T) {
	s := &testStruct0{}
	scopes, err := Scopes(reflect.ValueOf(s))
	assert.NilError(t, err)
	assert.EqualStringSlice(t, scopes,
		[]string{".", ".foo", ".foo.str", ".foo.map", ".bar", ".baz", ".baz.x"})
}

func TestVerifyScopes(t *testing.T) {
	s := &testStruct0{}
	ref := reflect.ValueOf(s)

	assert.NilError(t, VerifyScopes([]string{"."}, ref))
	assert.NilError(t, VerifyScopes([]string{".", ".foo", ".foo.str", ".foo.map", ".bar", ".baz", ".baz.x"}, ref))
	assert.Error(t, VerifyScopes([]string{}, ref), "empty scope set")
	assert.Error(t, VerifyScopes([]string{""}, ref), "invalid scope")
	assert.Error(t, VerifyScopes([]string{"baz"}, ref), "invalid scope")
}
