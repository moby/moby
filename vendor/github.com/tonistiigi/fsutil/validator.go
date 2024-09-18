package fsutil

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/pkg/errors"
)

type parent struct {
	dir  string
	last string
}

type Validator struct {
	parentDirs []parent
}

func (v *Validator) HandleChange(kind ChangeKind, p string, fi os.FileInfo, err error) (retErr error) {
	if err != nil {
		return err
	}
	// test that all paths are in order and all parent dirs were present
	if v.parentDirs == nil {
		v.parentDirs = make([]parent, 1, 10)
	}
	if p != filepath.Clean(p) {
		return errors.WithStack(&os.PathError{Path: p, Err: syscall.EINVAL, Op: "unclean path"})
	}
	if filepath.IsAbs(p) {
		return errors.WithStack(&os.PathError{Path: p, Err: syscall.EINVAL, Op: "absolute path"})
	}
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	if dir == "." {
		dir = ""
	}
	if dir == ".." || strings.HasPrefix(p, filepath.FromSlash("../")) {
		return errors.WithStack(&os.PathError{Path: p, Err: syscall.EINVAL, Op: "escape check"})
	}

	// find a parent dir from saved records
	i := sort.Search(len(v.parentDirs), func(i int) bool {
		return ComparePath(v.parentDirs[len(v.parentDirs)-1-i].dir, dir) <= 0
	})
	i = len(v.parentDirs) - 1 - i
	if i != len(v.parentDirs)-1 { // skipping back to grandparent
		v.parentDirs = v.parentDirs[:i+1]
	}

	if dir != v.parentDirs[len(v.parentDirs)-1].dir || v.parentDirs[i].last >= base {
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

func ComparePath(p1, p2 string) int {
	// byte-by-byte comparison to be compatible with str<>str
	min := min(len(p1), len(p2))
	for i := 0; i < min; i++ {
		switch {
		case p1[i] == p2[i]:
			continue
		case p2[i] != filepath.Separator && p1[i] < p2[i] || p1[i] == filepath.Separator:
			return -1
		default:
			return 1
		}
	}
	return len(p1) - len(p2)
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
