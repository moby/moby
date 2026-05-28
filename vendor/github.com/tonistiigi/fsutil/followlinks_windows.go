package fsutil

import (
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

func isNotFound(err error) bool {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, windows.ERROR_INVALID_NAME) {
		return true
	}
	return false
}
