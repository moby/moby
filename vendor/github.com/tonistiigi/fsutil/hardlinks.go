package fsutil

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

// Hardlinks validates that all targets for links were part of the changes

type Hardlinks struct {
	seenFiles map[string]struct{}
}

func (v *Hardlinks) HandleChange(kind ChangeKind, p string, fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if v.seenFiles == nil {
		v.seenFiles = make(map[string]struct{})
	}

	if kind == ChangeKindDelete {
		return nil
	}

	stat, ok := fi.Sys().(*types.Stat)
	if !ok {
		return errors.WithStack(&os.PathError{Path: p, Err: syscall.EBADMSG, Op: "change without stat info"})
	}

	if fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	if len(stat.Linkname) > 0 {
		if _, ok := v.seenFiles[stat.Linkname]; !ok {
			return errors.Errorf("invalid link %s to unknown path: %q", p, stat.Linkname)
		}
	} else {
		v.seenFiles[p] = struct{}{}
	}

	return nil
}
