//go:build !windows

package filesync

import (
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
)

func sendDiffCopy(stream Stream, fs fsutil.FS, progress progressCb) error {
	return errors.WithStack(fsutil.Send(stream.Context(), stream, fs, progress))
}
