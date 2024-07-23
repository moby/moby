package errdefs

import (
	"errors"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
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

func TestImplements(t *testing.T) {
	var errorNotFound errNotFound
	var errorInvalidParameter errInvalidParameter
	errOther := errors.New("other")
	tests := map[string]struct {
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

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, ok := getImplementer(tc.err).(ErrNotFound)
			assert.Equal(t, ok, tc.expected)
		})
	}
}
