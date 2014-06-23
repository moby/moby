package testutils

import (
	"testing"
	"time"
)

// Timeout calls f and waits for 100ms for it to complete.
// If it doesn't, it causes the tests to fail.
// t must be a valid testing context.
func Timeout(t *testing.T, f func()) {
	onTimeout := time.After(100 * time.Millisecond)
	onDone := make(chan bool)
	go func() {
		f()
		close(onDone)
	}()
	select {
	case <-onTimeout:
		t.Fatalf("timeout")
	case <-onDone:
	}
}
