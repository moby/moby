//go:build windows
// +build windows

package disk

import (
	"golang.org/x/sys/windows"
)

func GetDiskStat(root string) (DiskStat, error) {
	rootUTF16, err := windows.UTF16FromString(root)
	if err != nil {
		return DiskStat{}, err
	}
	var (
		totalBytes         uint64
		totalFreeBytes     uint64
		freeAvailableBytes uint64
	)
	if err := windows.GetDiskFreeSpaceEx(
		&rootUTF16[0],
		&freeAvailableBytes,
		&totalBytes,
		&totalFreeBytes); err != nil {
		return DiskStat{}, err
	}

	return DiskStat{
		Total:     int64(totalBytes),
		Free:      int64(totalFreeBytes),
		Available: int64(freeAvailableBytes),
	}, nil
}
