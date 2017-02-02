package fsutil

import (
	"os"

	"github.com/pkg/errors"
)

// Hardlinks validates that all targets for links were

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

	stat, ok := fi.Sys().(*Stat)
	if !ok {
		return errors.Errorf("invalid change without stat info: %s", p)
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
