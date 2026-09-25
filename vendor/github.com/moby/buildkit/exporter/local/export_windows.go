package local

import (
	"context"
	gofs "io/fs"

	"github.com/Microsoft/go-winio"
	"github.com/tonistiigi/fsutil"
)

func fsWalk(ctx context.Context, fs fsutil.FS, s string, walkFn gofs.WalkDirFunc) error {
	// Windows has some special files that require
	// SeBackupPrivilege to be accessed. Ref #4994
	return winio.RunWithPrivilege(winio.SeBackupPrivilege, func() error {
		return fs.Walk(ctx, s, walkFn)
	})
}
