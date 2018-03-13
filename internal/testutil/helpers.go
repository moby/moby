package testutil // import "github.com/docker/docker/internal/testutil"

import (
	"io"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

// ErrorContains checks that the error is not nil, and contains the expected
// substring.
func ErrorContains(t assert.TestingT, err error, expectedError string, msgAndArgs ...interface{}) {
	assert.Assert(t, is.ErrorContains(err, ""), msgAndArgs)
	assert.Check(t, is.Contains(err.Error(), expectedError), msgAndArgs)
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
