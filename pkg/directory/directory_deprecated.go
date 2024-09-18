package directory

import (
	"context"

	"github.com/docker/docker/internal/directory"
)

// Size walks a directory tree and returns its total size in bytes.
//
// Deprecated: this function is only used internally, and will be removed in the next release.
func Size(ctx context.Context, dir string) (int64, error) {
	return directory.Size(ctx, dir)
}
