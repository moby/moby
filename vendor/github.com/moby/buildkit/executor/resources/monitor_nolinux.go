//go:build !linux

package resources

func isCgroup2() bool {
	return false
}
