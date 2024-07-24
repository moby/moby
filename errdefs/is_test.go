package errdefs

import (
	"errors"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

var (
	errorNotFound         errNotFound
	errorInvalidParameter errInvalidParameter
	errOther              = errors.New("other")
)

type errCause struct {
	err error
}

func newErrCause(err error) errCause {
	return errCause{err: err}
}

func (e errCause) Error() string {
	return e.err.Error()
}

func (e errCause) Cause() error {
	return e.err
}

var tests = map[string]struct {
	err      error
	expected bool
}{

	"nil": {
		err: nil,
	},
	"direct-not-found": {
		err:      errorNotFound,
		expected: true,
	},
	"direct-other": {
		err: errOther,
	},
	"wrapped-not-found": {
		err:      fmt.Errorf("wrap: %w", errorNotFound),
		expected: true,
	},
	"wrapped-other": {
		err: fmt.Errorf("wrap: %w", errOther),
	},
	"multi-wrapped-not-found": {
		err:      fmt.Errorf("wrap: %w", fmt.Errorf("wrap: %w", errorNotFound)),
		expected: true,
	},
	"multi-wrapped-other": {
		err: fmt.Errorf("wrap: %w", fmt.Errorf("wrap: %w", errOther)),
	},
	"join-not-found": {
		err:      errors.Join(errOther, errorNotFound),
		expected: true,
	},
	"join-other": {
		err: errors.Join(errOther, errOther),
	},
	"join-invalid-param": {
		err: errors.Join(errOther, errorInvalidParameter, errorNotFound),
	},
	"cause-not-found": {
		err:      newErrCause(errorNotFound),
		expected: true,
	},
	"join-cause-not-found": {
		err:      errors.Join(errOther, newErrCause(errorNotFound)),
		expected: true,
	},
	"join-cause-invalid-param": {
		err: errors.Join(errOther, newErrCause(errorInvalidParameter), newErrCause(errorNotFound)),
	},
	"join-cause-other": {
		err: errors.Join(errOther, newErrCause(errOther)),
	},
}

func TestIsNotFound(t *testing.T) {
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, IsNotFound(tc.err), tc.expected)
		})
	}
}

func BenchmarkIsNotFound(b *testing.B) {
	for name, tc := range tests {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				IsNotFound(tc.err)
			}
		})
	}
}
