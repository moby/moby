//go:build !linux && !freebsd && !netbsd

package fsutil

import (
	"os"

	"github.com/pkg/errors"
)

func unsupportedRootOp(op, name string, err error) error {
	return errors.WithStack(&os.PathError{Op: op, Path: name, Err: err})
}
