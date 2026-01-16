package util

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/continuity/fs"
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

func ReadFile(ctx context.Context, root string, req ReadRequest) ([]byte, error) {
	fp, err := fs.RootPath(root, req.Filename)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	f, err := os.Open(fp)
	if err != nil {
		// The filename here is internal to the mount, so we can restore
		// the request base path for error reporting.
		// See os.DirFS.Open for details.
		pe := &os.PathError{}
		if errors.As(err, &pe) {
			pe.Path = req.Filename
		}
		return nil, errors.WithStack(err)
	}
	defer f.Close()

	var rdr io.Reader = f
	if req.Range != nil {
		rdr = io.NewSectionReader(f, int64(req.Range.Offset), int64(req.Range.Length))
	}
	dt, err := io.ReadAll(rdr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return dt, nil
}

type ReadDirRequest struct {
	Path           string
	IncludePattern string
}

func ReadDir(ctx context.Context, root string, req ReadDirRequest) ([]*fstypes.Stat, error) {
	var (
		rd []*fstypes.Stat
		fo fsutil.FilterOpt
	)
	if req.IncludePattern != "" {
		fo.IncludePatterns = append(fo.IncludePatterns, req.IncludePattern)
	}
	fp, err := fs.RootPath(root, req.Path)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	err = fsutil.Walk(ctx, fp, &fo, func(path string, info os.FileInfo, err error) error {
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
	if err != nil {
		return nil, err
	}
	return rd, nil
}

func StatFile(ctx context.Context, root string, path string) (*fstypes.Stat, error) {
	fp, err := fs.RootPath(root, path)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	st, err := fsutil.Stat(fp)
	if err != nil {
		// The filename here is internal to the mount, so we can restore
		// the request base path for error reporting.
		// See os.DirFS.Open for details.
		replaceErrorPath(err, path)
		return nil, errors.WithStack(err)
	}
	return st, nil
}

// replaceErrorPath will override the path in an os.PathError in the error chain.
// This works with the fsutil library, but it isn't necessarily the correct
// way to do this because the error message of wrapped errors doesn't necessarily
// update or change when a wrapped error is changed.
//
// Still, this method of updating the path works with the way this specific
// library returns errors.
func replaceErrorPath(err error, path string) {
	var pe *os.PathError
	if errors.As(err, &pe) {
		pe.Path = path
	}
}
