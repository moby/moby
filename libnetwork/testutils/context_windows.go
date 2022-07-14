package testutils

import "testing"

// SetupTestOSContext joins a new network namespace, and returns its associated
// teardown function.
//
// Example usage:
//
//	defer SetupTestOSContext(t)()
func SetupTestOSContext(t *testing.T) func() {
	return func() {}
}
