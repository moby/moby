//go:build !windows
// +build !windows

package fsutil

import (
	"os"

	"github.com/pkg/errors"
)

func isNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
