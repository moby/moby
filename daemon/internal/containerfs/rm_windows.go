package containerfs

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func prepareRemoveAll(string) {
}

func isNotEmptyDirError(err error) bool {
	return false
}

func retryRemoveAllError(_ string, pe *os.PathError) (bool, error) {
	return errors.Is(pe.Err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(pe.Err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(pe.Err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(pe.Err, windows.ERROR_DIR_NOT_EMPTY), nil
}
