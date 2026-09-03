package container_test

import (
	"testing"
)

func checkEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func checkNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func checkError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected error %q, got nil", want)
	} else if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}
