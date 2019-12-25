package fsutil

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

// constructs a Stat object. path is where the path can be found right
// now, relpath is the desired path to be recorded in the stat (so
// relative to whatever base dir is relevant). fi is the os.Stat
// info. inodemap is used to calculate hardlinks over a series of
// mkstat calls and maps inode to the canonical (aka "first") path for
// a set of hardlinks to that inode.
func mkstat(path, relpath string, fi os.FileInfo, inodemap map[uint64]string) (*types.Stat, error) {
	relpath = filepath.ToSlash(relpath)

	stat := &types.Stat{
		Path:    relpath,
		Mode:    uint32(fi.Mode()),
		ModTime: fi.ModTime().UnixNano(),
	}

	setUnixOpt(fi, stat, relpath, inodemap)

	if !fi.IsDir() {
		stat.Size_ = fi.Size()
		if fi.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to readlink %s", path)
			}
			stat.Linkname = link
		}
	}
	if err := loadXattr(path, stat); err != nil {
		return nil, errors.Wrapf(err, "failed to xattr %s", relpath)
	}

	if runtime.GOOS == "windows" {
		permPart := stat.Mode & uint32(os.ModePerm)
		noPermPart := stat.Mode &^ uint32(os.ModePerm)
		// Add the x bit: make everything +x from windows
		permPart |= 0111
		permPart &= 0755
		stat.Mode = noPermPart | permPart
	}

	return stat, nil
}

func Stat(path string) (*types.Stat, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, errors.Wrap(err, "os stat")
	}
	return mkstat(path, filepath.Base(path), fi, nil)
}
