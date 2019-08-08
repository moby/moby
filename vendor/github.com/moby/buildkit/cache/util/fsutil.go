package util

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/cache"
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

func withMount(ctx context.Context, ref cache.ImmutableRef, cb func(string) error) error {
	mount, err := ref.Mount(ctx, true)
	if err != nil {
		return err
	}

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

func ReadFile(ctx context.Context, ref cache.ImmutableRef, req ReadRequest) ([]byte, error) {
	var dt []byte

	err := withMount(ctx, ref, func(root string) error {
		fp, err := fs.RootPath(root, req.Filename)
		if err != nil {
			return err
		}

		if req.Range == nil {
			dt, err = ioutil.ReadFile(fp)
			if err != nil {
				return err
			}
		} else {
			f, err := os.Open(fp)
			if err != nil {
				return err
			}
			dt, err = ioutil.ReadAll(io.NewSectionReader(f, int64(req.Range.Offset), int64(req.Range.Length)))
			f.Close()
			if err != nil {
				return err
			}
		}
		return nil
	})
	return dt, err
}

type ReadDirRequest struct {
	Path           string
	IncludePattern string
}

func ReadDir(ctx context.Context, ref cache.ImmutableRef, req ReadDirRequest) ([]*fstypes.Stat, error) {
	var (
		rd []*fstypes.Stat
		wo fsutil.WalkOpt
	)
	if req.IncludePattern != "" {
		wo.IncludePatterns = append(wo.IncludePatterns, req.IncludePattern)
	}
	err := withMount(ctx, ref, func(root string) error {
		fp, err := fs.RootPath(root, req.Path)
		if err != nil {
			return err
		}
		return fsutil.Walk(ctx, fp, &wo, func(path string, info os.FileInfo, err error) error {
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

func StatFile(ctx context.Context, ref cache.ImmutableRef, path string) (*fstypes.Stat, error) {
	var st *fstypes.Stat
	err := withMount(ctx, ref, func(root string) error {
		fp, err := fs.RootPath(root, path)
		if err != nil {
			return err
		}
		if st, err = fsutil.Stat(fp); err != nil {
			return err
		}
		return nil
	})
	return st, err
}
