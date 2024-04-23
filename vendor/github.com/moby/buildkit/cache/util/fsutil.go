package util

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/snapshot"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
)

type ReadRequest struct {
	Filename string
	Range    *FileRange
}

type FileRange struct {
	Offset int
	Length int
}

func withMount(mount snapshot.Mountable, cb func(string) error) error {
	lm := snapshot.LocalMounter(mount)

	root, err := lm.Mount()
	if err != nil {
		return err
	}

	defer func() {
		if lm != nil {
			lm.Unmount()
		}
	}()

	if err := cb(root); err != nil {
		return err
	}

	if err := lm.Unmount(); err != nil {
		return err
	}
	lm = nil
	return nil
}

func ReadFile(ctx context.Context, mount snapshot.Mountable, req ReadRequest) ([]byte, error) {
	var dt []byte

	err := withMount(mount, func(root string) error {
		fp, err := fs.RootPath(root, req.Filename)
		if err != nil {
			return errors.WithStack(err)
		}

		f, err := os.Open(fp)
		if err != nil {
			// The filename here is internal to the mount, so we can restore
			// the request base path for error reporting.
			// See os.DirFS.Open for details.
			if pe, ok := err.(*os.PathError); ok {
				pe.Path = req.Filename
			}
			return errors.WithStack(err)
		}
		defer f.Close()

		var rdr io.Reader = f
		if req.Range != nil {
			rdr = io.NewSectionReader(f, int64(req.Range.Offset), int64(req.Range.Length))
		}
		dt, err = io.ReadAll(rdr)
		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	})
	return dt, err
}

type ReadDirRequest struct {
	Path           string
	IncludePattern string
}

func ReadDir(ctx context.Context, mount snapshot.Mountable, req ReadDirRequest) ([]*fstypes.Stat, error) {
	var (
		rd []*fstypes.Stat
		fo fsutil.FilterOpt
	)
	if req.IncludePattern != "" {
		fo.IncludePatterns = append(fo.IncludePatterns, req.IncludePattern)
	}
	err := withMount(mount, func(root string) error {
		fp, err := fs.RootPath(root, req.Path)
		if err != nil {
			return errors.WithStack(err)
		}
		return fsutil.Walk(ctx, fp, &fo, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return errors.Wrapf(err, "walking %q", root)
			}
			stat, ok := info.Sys().(*fstypes.Stat)
			if !ok {
				// This "can't happen(tm)".
				return errors.Errorf("expected a *fsutil.Stat but got %T", info.Sys())
			}
			rd = append(rd, stat)

			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		})
	})
	return rd, err
}

func StatFile(ctx context.Context, mount snapshot.Mountable, path string) (*fstypes.Stat, error) {
	var st *fstypes.Stat
	err := withMount(mount, func(root string) error {
		fp, err := fs.RootPath(root, path)
		if err != nil {
			return errors.WithStack(err)
		}
		if st, err = fsutil.Stat(fp); err != nil {
			// The filename here is internal to the mount, so we can restore
			// the request base path for error reporting.
			// See os.DirFS.Open for details.
			err1 := err
			if err := errors.Cause(err); err != nil {
				err1 = err
			}
			if pe, ok := err1.(*os.PathError); ok {
				pe.Path = path
			}
			return errors.WithStack(err)
		}
		return nil
	})
	return st, err
}
