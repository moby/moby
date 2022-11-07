package testutils

import "testing"

// Logger is used to log non-fatal messages during tests.
type Logger interface {
	Logf(format string, args ...any)
}

var _ Logger = (*testing.T)(nil)

// SetupTestOSContext joins the current goroutine to a new network namespace,
// and returns its associated teardown function.
//
// Example usage:
//
//	defer SetupTestOSContext(t)()
func SetupTestOSContext(t *testing.T) func() {
	c := SetupTestOSContextEx(t)
	return func() { c.Cleanup(t) }
}

// Go starts running fn in a new goroutine inside the test OS context.
func (c *OSContext) Go(t *testing.T, fn func()) {
	t.Helper()
	errCh := make(chan error, 1)
	go func() {
		teardown, err := c.Set()
		if err != nil {
			errCh <- err
			return
		}
		defer teardown(t)
		close(errCh)
		fn()
	}()

	if err := <-errCh; err != nil {
		t.Fatalf("%+v", err)
	}
}
