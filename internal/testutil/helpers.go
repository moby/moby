package testutil // import "github.com/docker/docker/internal/testutil"

import (
	"io"

	"github.com/gotestyourself/gotestyourself/assert"
)

type helperT interface {
	Helper()
}

// ErrorContains checks that the error is not nil, and contains the expected
// substring.
// Deprecated: use assert.Assert(t, cmp.ErrorContains(err, expected))
func ErrorContains(t assert.TestingT, err error, expectedError string, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert.ErrorContains(t, err, expectedError, msgAndArgs...)
}

// DevZero acts like /dev/zero but in an OS-independent fashion.
var DevZero io.Reader = devZero{}

type devZero struct{}

func (d devZero) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
