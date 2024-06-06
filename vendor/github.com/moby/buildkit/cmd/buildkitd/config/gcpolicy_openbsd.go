//go:build openbsd
// +build openbsd

package config

import (
	"syscall"
)

var DiskSpacePercentage int64 = 10

func getDiskSize(root string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err != nil {
		return 0, err
	}
	diskSize := int64(st.F_bsize) * int64(st.F_blocks)
	return diskSize, nil
}
