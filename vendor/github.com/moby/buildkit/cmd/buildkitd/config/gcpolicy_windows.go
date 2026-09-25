//go:build windows

package config

const (
	// Windows images are generally larger.
	// set as double that for Linux since
	DiskSpaceReservePercentage int64 = 20
	DiskSpaceReserveBytes      int64 = 10 * 1e9 // 10GB
	DiskSpaceFreePercentage    int64 = 20
	DiskSpaceMaxPercentage     int64 = 80
	DiskSpaceMaxBytes          int64 = 100 * 1e9 // 100GB
)
