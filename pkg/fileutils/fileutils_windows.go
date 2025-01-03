package fileutils // import "github.com/docker/docker/pkg/fileutils"

import "context"

// GetTotalUsedFds Returns the number of used File Descriptors. Not supported
// on Windows.
//
// Deprecated: this function is only used internally, and will be removed in the next release.
func GetTotalUsedFds(ctx context.Context) int {
	return -1
}
