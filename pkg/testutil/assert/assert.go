// Package assert contains functions for making assertions in unit tests
package assert

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"unicode"

	"github.com/davecgh/go-spew/spew"
)

// TestingT is an interface which defines the methods of testing.T that are
// required by this package
type TestingT interface {
	Fatalf(string, ...interface{})
}

// Equal compare the actual value to the expected value and fails the test if
// they are not equal.
func Equal(t TestingT, actual, expected interface{}, extra ...string) {
	if expected != actual {
		fatalWithExtra(t, extra, "Expected '%v' (%T) got '%v' (%T)", expected, expected, actual, actual)
	}
}

// EqualNormalizedString compare the actual value to the expected value after applying the specified
// transform function. It fails the test if these two transformed string are not equal.
// For example `EqualNormalizedString(t, RemoveSpace, "foo\n", "foo")` wouldn't fail the test as
// spaces (and thus '\n') are removed before comparing the string.
func EqualNormalizedString(t TestingT, transformFun func(rune) rune, actual, expected string) {
	if strings.Map(transformFun, actual) != strings.Map(transformFun, expected) {
		fatal(t, "Expected '%v' got '%v'", expected, expected, actual, actual)
	}
}

// RemoveSpace returns -1 if the specified runes is considered as a space (unicode)
// and the rune itself otherwise.
func RemoveSpace(r rune) rune {
	if unicode.IsSpace(r) {
		return -1
	}
	return r
}

//EqualStringSlice compares two slices and fails the test if they do not contain
// the same items.
func EqualStringSlice(t TestingT, actual, expected []string) {
	if len(actual) != len(expected) {
		fatal(t, "Expected (length %d): %q\nActual (length %d): %q",
			len(expected), expected, len(actual), actual)
	}
	for i, item := range actual {
		if item != expected[i] {
			fatal(t, "Slices differ at element %d, expected %q got %q",
				i, expected[i], item)
		}
	}
}

// NilError asserts that the error is nil, otherwise it fails the test.
func NilError(t TestingT, err error) {
	if err != nil {
		fatal(t, "Expected no error, got: %s", err.Error())
	}
}

// DeepEqual compare the actual value to the expected value and fails the test if
// they are not "deeply equal".
func DeepEqual(t TestingT, actual, expected interface{}) {
	if !reflect.DeepEqual(actual, expected) {
		fatal(t, "Expected (%T):\n%v\n\ngot (%T):\n%s\n",
			expected, spew.Sdump(expected), actual, spew.Sdump(actual))
	}
}

// Error asserts that error is not nil, and contains the expected text,
// otherwise it fails the test.
func Error(t TestingT, err error, contains string) {
	if err == nil {
		fatal(t, "Expected an error, but error was nil")
	}

	if !strings.Contains(err.Error(), contains) {
		fatal(t, "Expected error to contain '%s', got '%s'", contains, err.Error())
	}
}

// Contains asserts that the string contains a substring, otherwise it fails the
// test.
func Contains(t TestingT, actual, contains string) {
	if !strings.Contains(actual, contains) {
		fatal(t, "Expected '%s' to contain '%s'", actual, contains)
	}
}

// NotNil fails the test if the object is nil
func NotNil(t TestingT, obj interface{}) {
	if obj == nil {
		fatal(t, "Expected non-nil value.")
	}
}

// Nil fails the test if the object is not nil
func Nil(t TestingT, obj interface{}) {
	if obj != nil {
		fatal(t, "Expected nil value, got (%T) %s", obj, obj)
	}
}

func fatal(t TestingT, format string, args ...interface{}) {
	t.Fatalf(errorSource()+format, args...)
}

func fatalWithExtra(t TestingT, extra []string, format string, args ...interface{}) {
	msg := fmt.Sprintf(errorSource()+format, args...)
	if len(extra) > 0 {
		msg += ": " + strings.Join(extra, ", ")
	}
	t.Fatalf(msg)
}

// See testing.decorate()
func errorSource() string {
	_, filename, line, ok := runtime.Caller(3)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s:%d: ", filepath.Base(filename), line)
}
