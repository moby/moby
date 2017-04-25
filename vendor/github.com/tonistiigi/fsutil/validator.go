package fsutil

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

type parent struct {
	dir  string
	last string
}

type Validator struct {
	parentDirs []parent
}

func (v *Validator) HandleChange(kind ChangeKind, p string, fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if v.parentDirs == nil {
		v.parentDirs = make([]parent, 1, 10)
	}
	if p != filepath.Clean(p) {
		return errors.Errorf("invalid unclean path %s", p)
	}
	if filepath.IsAbs(p) {
		return errors.Errorf("abolute path %s not allowed", p)
	}
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	if dir == "." {
		dir = ""
	}
	if dir == ".." || strings.HasPrefix(p, "../") {
		return errors.Errorf("invalid path: %s", p)
	}
	i := sort.Search(len(v.parentDirs), func(i int) bool {
		return v.parentDirs[len(v.parentDirs)-1-i].dir <= dir
	})
	i = len(v.parentDirs) - 1 - i
	if i != len(v.parentDirs)-1 {
		v.parentDirs = v.parentDirs[:i+1]
	}

	if i == 0 && dir != "" || v.parentDirs[i].last >= base {
		return errors.Errorf("changes out of order: %q %q", p, filepath.Join(v.parentDirs[i].dir, v.parentDirs[i].last))
	}
	v.parentDirs[i].last = base
	if kind != ChangeKindDelete && fi.IsDir() {
		v.parentDirs = append(v.parentDirs, parent{
			dir:  filepath.Join(dir, base),
			last: "",
		})
	}
	// todo: validate invalid mode combinations
	return err
}
