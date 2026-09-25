//go:build !windows

package containerfs

import (
	"os"
	"syscall"

	"github.com/moby/sys/mount"
	"github.com/pkg/errors"
)

func prepareRemoveAll(dir string) {
	mount.RecursiveUnmount(dir)
}

func isNotEmptyDirError(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY)
}

func retryRemoveAllError(dir string, pe *os.PathError) (bool, error) {
	if !errors.Is(pe.Err, syscall.EBUSY) {
		return false, nil
	}
	if err := mount.Unmount(pe.Path); err != nil {
		return false, errors.Wrapf(err, "error while removing %s", dir)
	}
	return true, nil
}
