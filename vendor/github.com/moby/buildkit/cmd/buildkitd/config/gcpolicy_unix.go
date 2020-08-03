// +build !windows

package config

import (
	"syscall"
)

func DetectDefaultGCCap(root string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err != nil {
		return defaultCap
	}
	diskSize := int64(st.Bsize) * int64(st.Blocks)
	avail := diskSize / 10
	return (avail/(1<<30) + 1) * 1e9 // round up
}
