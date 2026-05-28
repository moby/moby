package fsutil

import (
	"context"
	"io"
	gofs "io/fs"
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

// WithHardlinkReset returns a FS that fixes hardlinks for FS that has been filtered
// so that original hardlink sources might be missing
func WithHardlinkReset(fs FS) FS {
	return &hardlinkFilter{fs: fs}
}

type hardlinkFilter struct {
	fs FS
}

var _ FS = &hardlinkFilter{}

func (r *hardlinkFilter) Walk(ctx context.Context, target string, fn gofs.WalkDirFunc) error {
	seenFiles := make(map[string]string)
	return r.fs.Walk(ctx, target, func(path string, entry gofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		fi, err := entry.Info()
		if err != nil {
			return err
		}

		if fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			return fn(path, entry, nil)
		}

		stat, ok := fi.Sys().(*types.Stat)
		if !ok {
			return errors.WithStack(&os.PathError{Path: path, Err: syscall.EBADMSG, Op: "fileinfo without stat info"})
		}

		if stat.Linkname != "" {
			if v, ok := seenFiles[stat.Linkname]; !ok {
				seenFiles[stat.Linkname] = stat.Path
				stat.Linkname = ""
				entry = &dirEntryWithStat{DirEntry: entry, stat: stat}
			} else {
				if v != stat.Path {
					stat.Linkname = v
					entry = &dirEntryWithStat{DirEntry: entry, stat: stat}
				}
			}
		}

		seenFiles[path] = stat.Path

		return fn(path, entry, nil)
	})
}

func (r *hardlinkFilter) Open(p string) (io.ReadCloser, error) {
	return r.fs.Open(p)
}

type dirEntryWithStat struct {
	gofs.DirEntry
	stat *types.Stat
}

func (d *dirEntryWithStat) Info() (gofs.FileInfo, error) {
	return &StatInfo{d.stat}, nil
}
