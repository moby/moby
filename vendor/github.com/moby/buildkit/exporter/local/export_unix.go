//go:build !windows

package local

import (
	"context"
	gofs "io/fs"

	"github.com/tonistiigi/fsutil"
)

func fsWalk(ctx context.Context, fs fsutil.FS, s string, walkFn gofs.WalkDirFunc) error {
	return fs.Walk(ctx, s, walkFn)
}
