//go:build windows

package filesync

import (
	"github.com/Microsoft/go-winio"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
)

func sendDiffCopy(stream Stream, fs fsutil.FS, progress progressCb) error {
	// adding one SeBackupPrivilege to the process so as to be able
	// to run the subsequent goroutines in fsutil.Send that need
	// to copy over special Windows metadata files.
	// TODO(profnandaa): need to cross-check that this cannot be
	// exploited in any way.
	winio.EnableProcessPrivileges([]string{winio.SeBackupPrivilege})
	defer winio.DisableProcessPrivileges([]string{winio.SeBackupPrivilege})
	return errors.WithStack(fsutil.Send(stream.Context(), stream, fs, progress))
}
