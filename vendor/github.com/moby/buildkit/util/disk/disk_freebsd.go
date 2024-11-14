//go:build freebsd

package disk

import (
	"syscall"

	"github.com/pkg/errors"
)

func GetDiskStat(root string) (DiskStat, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err != nil {
		return DiskStat{}, errors.Wrapf(err, "could not stat fs at %s", root)
	}

	return DiskStat{
		Total:     int64(st.Bsize) * int64(st.Blocks),
		Free:      int64(st.Bsize) * int64(st.Bfree),
		Available: int64(st.Bsize) * int64(st.Bavail),
	}, nil
}
