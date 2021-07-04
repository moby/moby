package internal

import (
	"errors"
	"fmt"
	"os"

	"github.com/cilium/ebpf/internal/unix"
)

func Pin(currentPath, newPath string, fd *FD) error {
	if newPath == "" {
		return errors.New("given pinning path cannot be empty")
	}
	if currentPath == newPath {
		return nil
	}
	if currentPath == "" {
		return BPFObjPin(newPath, fd)
	}
	var err error
	// Renameat2 is used instead of os.Rename to disallow the new path replacing
	// an existing path.
	if err = unix.Renameat2(unix.AT_FDCWD, currentPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE); err == nil {
		// Object is now moved to the new pinning path.
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("unable to move pinned object to new path %v: %w", newPath, err)
	}
	// Internal state not in sync with the file system so let's fix it.
	return BPFObjPin(newPath, fd)
}

func Unpin(pinnedPath string) error {
	if pinnedPath == "" {
		return nil
	}
	err := os.Remove(pinnedPath)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}
