package containerfs

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// EnsureRemoveAll wraps [os.RemoveAll] to retry transient Windows errors.
//
// Windows can report sharing violations, lock violations, access denied, or a
// non-empty directory while another process is releasing handles below the path.
// Only use EnsureRemoveAll when the caller really wants to make every effort to
// remove the path.
func EnsureRemoveAll(path string) error {
	const maxRetry = 50

	for retry := range maxRetry + 1 {
		err := os.RemoveAll(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		if retry == maxRetry || !isTransientRemoveError(err) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func isTransientRemoveError(err error) bool {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	return errors.Is(pathErr.Err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(pathErr.Err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(pathErr.Err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(pathErr.Err, windows.ERROR_DIR_NOT_EMPTY)
}
