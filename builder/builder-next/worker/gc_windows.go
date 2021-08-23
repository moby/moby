//go:build windows
// +build windows

package worker

func detectDefaultGCCap(root string) int64 {
	return defaultCap
}
