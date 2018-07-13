// +build windows

package fsutil

import (
	"github.com/pkg/errors"
)

func rewriteMetadata(p string, stat *Stat) error {
	return chtimes(p, stat.ModTime)
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo
func handleTarTypeBlockCharFifo(path string, stat *Stat) error {
	return errors.New("Not implemented on windows")
}
