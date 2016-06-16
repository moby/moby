// Package assert contains functions for making assertions in unit tests
package assert

import (
	"strings"
)

// TestingT is an interface which defines the methods of testing.T that are
// required by this package
type TestingT interface {
	Fatalf(string, ...interface{})
}

// Equal compare the actual value to the expected value and fails the test if
// they are not equal.
func Equal(t TestingT, actual, expected interface{}) {
	if expected != actual {
		t.Fatalf("Expected '%v' (%T) got '%v' (%T)", expected, expected, actual, actual)
	}
}

// NilError asserts that the error is nil, otherwise it fails the test.
func NilError(t TestingT, err error) {
	if err != nil {
		t.Fatalf("Expected no error, got: %s", err.Error())
	}
}

// Error asserts that error is not nil, and contains the expected text,
// otherwise it fails the test.
func Error(t TestingT, err error, contains string) {
	if err == nil {
		t.Fatalf("Expected an error, but error was nil")
	}

	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("Expected error to contain '%s', got '%s'", contains, err.Error())
	}
}

// Contains asserts that the string contains a substring, otherwise it fails the
// test.
func Contains(t TestingT, actual, contains string) {
	if !strings.Contains(actual, contains) {
		t.Fatalf("Expected '%s' to contain '%s'", actual, contains)
	}
}
