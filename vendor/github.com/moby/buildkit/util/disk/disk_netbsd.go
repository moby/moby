//go:build netbsd

package disk

import (
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func GetDiskStat(root string) (DiskStat, error) {
	var st unix.Statvfs_t
	if err := unix.Statvfs(root, &st); err != nil {
		return DiskStat{}, errors.Wrapf(err, "could not stat fs at %s", root)
	}

	return DiskStat{
		Total:     int64(st.Frsize) * int64(st.Blocks),
		Free:      int64(st.Frsize) * int64(st.Bfree),
		Available: int64(st.Frsize) * int64(st.Bavail),
	}, nil
}
