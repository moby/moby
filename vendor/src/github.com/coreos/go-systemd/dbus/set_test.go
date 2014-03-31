package dbus

import (
	"testing"
)

// TestBasicSetActions asserts that Add & Remove behavior is correct
func TestBasicSetActions(t *testing.T) {
	s := newSet()

	if s.Contains("foo") {
		t.Fatal("set should not contain 'foo'")
	}

	s.Add("foo")

	if !s.Contains("foo") {
		t.Fatal("set should contain 'foo'")
	}

	s.Remove("foo")

	if s.Contains("foo") {
		t.Fatal("set should not contain 'foo'")
	}
}
