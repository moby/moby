//go:build !darwin && !windows
// +build !darwin,!windows

package mount

import "strconv"

// mountError records an error from mount or unmount operation
type mountError struct {
	op             string
	source, target string
	flags          uintptr
	data           string
	err            error
}

func (e *mountError) Error() string {
	out := e.op + " "

	if e.source != "" {
		out += e.source + ":" + e.target
	} else {
		out += e.target
	}

	if e.flags != uintptr(0) {
		out += ", flags: 0x" + strconv.FormatUint(uint64(e.flags), 16)
	}
	if e.data != "" {
		out += ", data: " + e.data
	}

	out += ": " + e.err.Error()
	return out
}

// Cause returns the underlying cause of the error.
// This is a convention used in github.com/pkg/errors
func (e *mountError) Cause() error {
	return e.err
}

// Unwrap returns the underlying error.
// This is a convention used in golang 1.13+
func (e *mountError) Unwrap() error {
	return e.err
}
