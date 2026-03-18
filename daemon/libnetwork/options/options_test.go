package options

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGenerate(t *testing.T) {
	gen := Generic{
		"Int":     1,
		"Rune":    'b',
		"Float64": 2.0,
	}

	type Model struct {
		Int     int
		Rune    rune
		Float64 float64
	}

	expected := Model{
		Int:     1,
		Rune:    'b',
		Float64: 2.0,
	}
	result, err := GenerateFromModel[Model](gen)
	assert.Check(t, err)
	assert.Check(t, is.DeepEqual(result, expected))
}

func TestGeneratePtr(t *testing.T) {
	gen := Generic{
		"Int":     1,
		"Rune":    'b',
		"Float64": 2.0,
	}

	type Model struct {
		Int     int
		Rune    rune
		Float64 float64
	}

	expected := &Model{
		Int:     1,
		Rune:    'b',
		Float64: 2.0,
	}

	result, err := GenerateFromModel[*Model](gen)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(result, expected))
}

func TestGenerateMissingField(t *testing.T) {
	type Model struct{}
	gen := Generic{"foo": "bar"}
	_, err := GenerateFromModel[Model](gen)
	const expected = `no field "foo" in type "options.Model"`
	assert.Check(t, is.Error(err, expected))
	assert.Check(t, is.ErrorType(err, NoSuchFieldError{}))
}

func TestFieldCannotBeSet(t *testing.T) {
	type Model struct {
		foo int //nolint:nolintlint,unused // un-exported field is used to test error-handling
	}
	gen := Generic{"foo": "bar"}
	_, err := GenerateFromModel[Model](gen)
	const expected = `cannot set field "foo" of type "options.Model"`
	assert.Check(t, is.Error(err, expected))
	assert.Check(t, is.ErrorType(err, CannotSetFieldError{}))
}

func TestTypeMismatchError(t *testing.T) {
	type Model struct {
		Foo int
	}
	gen := Generic{"Foo": "bar"}
	_, err := GenerateFromModel[Model](gen)
	const expected = `type mismatch, field Foo require type int, actual type string`
	assert.Check(t, is.Error(err, expected))
	assert.Check(t, is.ErrorType(err, TypeMismatchError{}))
}
