package fsutil

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

type FS interface {
	Walk(context.Context, filepath.WalkFunc) error
	Open(string) (io.ReadCloser, error)
}

func NewFS(root string, opt *WalkOpt) FS {
	return &fs{
		root: root,
		opt:  opt,
	}
}

type fs struct {
	root string
	opt  *WalkOpt
}

func (fs *fs) Walk(ctx context.Context, fn filepath.WalkFunc) error {
	return Walk(ctx, fs.root, fs.opt, fn)
}

func (fs *fs) Open(p string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(fs.root, p))
}

func SubDirFS(fs FS, stat types.Stat) FS {
	return &subDirFS{fs: fs, stat: stat}
}

type subDirFS struct {
	fs   FS
	stat types.Stat
}

func (fs *subDirFS) Walk(ctx context.Context, fn filepath.WalkFunc) error {
	main := &StatInfo{Stat: &fs.stat}
	if !main.IsDir() {
		return errors.Errorf("fs subdir not mode directory")
	}
	if main.Name() != fs.stat.Path {
		return errors.Errorf("subdir path must be single file")
	}
	if err := fn(fs.stat.Path, main, nil); err != nil {
		return err
	}
	return fs.fs.Walk(ctx, func(p string, fi os.FileInfo, err error) error {
		stat, ok := fi.Sys().(*types.Stat)
		if !ok {
			return errors.Wrapf(err, "invalid fileinfo without stat info: %s", p)
		}
		stat.Path = path.Join(fs.stat.Path, stat.Path)
		return fn(filepath.Join(fs.stat.Path, p), &StatInfo{stat}, nil)
	})
}

func (fs *subDirFS) Open(p string) (io.ReadCloser, error) {
	return fs.fs.Open(strings.TrimPrefix(p, fs.stat.Path+"/"))
}
