package testutils

import "testing"

// Logger is used to log non-fatal messages during tests.
type Logger interface {
	Logf(format string, args ...any)
}

var _ Logger = (*testing.T)(nil)
