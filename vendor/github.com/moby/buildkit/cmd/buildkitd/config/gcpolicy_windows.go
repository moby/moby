//go:build windows
// +build windows

package config

import (
	"golang.org/x/sys/windows"
)

// set as double that for Linux since
// Windows images are generally larger.
var DiskSpacePercentage int64 = 20

func getDiskSize(root string) (int64, error) {
	rootUTF16, err := windows.UTF16FromString(root)
	if err != nil {
		return 0, err
	}
	var freeAvailableBytes uint64
	var totalBytes uint64
	var totalFreeBytes uint64

	if err := windows.GetDiskFreeSpaceEx(
		&rootUTF16[0],
		&freeAvailableBytes,
		&totalBytes,
		&totalFreeBytes); err != nil {
		return 0, err
	}
	return int64(totalBytes), nil
}
