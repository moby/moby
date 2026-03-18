package netnsutils

import "testing"

// SetupTestOSContext is a no-op on Windows.
func SetupTestOSContext(*testing.T) func() {
	return func() {}
}
