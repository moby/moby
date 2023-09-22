//go:build windows
// +build windows

package config

func DetectDefaultGCCap() DiskSpace {
	return DiskSpace{Bytes: defaultCap}
}

func (d DiskSpace) AsBytes(root string) int64 {
	return d.Bytes
}
