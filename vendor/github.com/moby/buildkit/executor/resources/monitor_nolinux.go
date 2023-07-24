//go:build !linux
// +build !linux

package resources

func isCgroup2() bool {
	return false
}
