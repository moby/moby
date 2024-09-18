package fileutils // import "github.com/docker/docker/pkg/fileutils"

import "context"

// GetTotalUsedFds Returns the number of used File Descriptors. Not supported
// on Windows.
func GetTotalUsedFds(ctx context.Context) int {
	return -1
}
