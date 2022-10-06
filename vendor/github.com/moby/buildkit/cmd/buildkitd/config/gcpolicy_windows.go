//go:build windows
// +build windows

package config

func DetectDefaultGCCap(root string) int64 {
	return defaultCap
}
