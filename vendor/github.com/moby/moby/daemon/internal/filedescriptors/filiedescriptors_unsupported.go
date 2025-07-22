//go:build !linux

package filedescriptors

import "context"

// GetTotalUsedFds Returns the number of used File Descriptors. Not supported
// on Windows.
func GetTotalUsedFds(context.Context) int {
	return -1
}
