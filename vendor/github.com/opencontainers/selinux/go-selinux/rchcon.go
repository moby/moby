//go:build linux && go1.16
// +build linux,go1.16

package selinux

import (
	"errors"
	"io/fs"
	"os"

	"github.com/opencontainers/selinux/pkg/pwalkdir"
)

func rchcon(fpath, label string) error {
	fastMode := false
	// If the current label matches the new label, assume
	// other labels are correct.
	if cLabel, err := lFileLabel(fpath); err == nil && cLabel == label {
		fastMode = true
	}
	return pwalkdir.Walk(fpath, func(p string, _ fs.DirEntry, _ error) error {
		if fastMode {
			if cLabel, err := lFileLabel(fpath); err == nil && cLabel == label {
				return nil
			}
		}
		e := lSetFileLabel(p, label)
		// Walk a file tree can race with removal, so ignore ENOENT.
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		return e
	})
}
