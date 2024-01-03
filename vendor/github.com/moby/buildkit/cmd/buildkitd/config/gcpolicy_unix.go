//go:build !windows
// +build !windows

package config

import (
	"syscall"
)

func DetectDefaultGCCap() DiskSpace {
	return DiskSpace{Percentage: 10}
}

func (d DiskSpace) AsBytes(root string) int64 {
	if d.Bytes != 0 {
		return d.Bytes
	}
	if d.Percentage == 0 {
		return 0
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err != nil {
		return defaultCap
	}
	diskSize := int64(st.Bsize) * int64(st.Blocks)
	avail := diskSize * d.Percentage / 100
	return (avail/(1<<30) + 1) * 1e9 // round up
}
