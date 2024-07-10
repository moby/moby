package local

import (
	"context"
	"io"

	"github.com/Microsoft/go-winio"
	"github.com/tonistiigi/fsutil"
)

func writeTar(ctx context.Context, fs fsutil.FS, w io.WriteCloser) error {
	// Windows rootfs has a few special metadata files that
	// require extra privileges to be accessed.
	privileges := []string{winio.SeBackupPrivilege}
	return winio.RunWithPrivileges(privileges, func() error {
		return fsutil.WriteTar(ctx, fs, w)
	})
}
