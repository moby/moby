package fsutil

import (
	"context"
	"hash"
	gofs "io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

type walkerFn func(ctx context.Context, pathC chan<- *currentPath) error

type HandleChangeFn func(ChangeKind, string, os.FileInfo, error) error

type ContentHasher func(*types.Stat) (hash.Hash, error)

func getWalkerFn(root string) walkerFn {
	return getFSWalkerFn(func() (FS, error) {
		return NewFS(root)
	})
}

func getRootWalkerFn(root Root) walkerFn {
	return getFSWalkerFn(func() (FS, error) {
		return NewRootFS(root), nil
	})
}

func getFSWalkerFn(newFS func() (FS, error)) walkerFn {
	return func(ctx context.Context, pathC chan<- *currentPath) error {
		fs, err := newFS()
		if err != nil {
			return errors.Wrap(err, "failed to walk")
		}
		return errors.Wrap(fs.Walk(ctx, "/", func(path string, entry gofs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			fi, err := entry.Info()
			if err != nil {
				return err
			}
			stat, ok := fi.Sys().(*types.Stat)
			if !ok {
				return errors.Errorf("%T invalid file without stat information", fi.Sys())
			}
			p := &currentPath{
				path: path,
				stat: stat,
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case pathC <- p:
				return nil
			}
		}), "failed to walk")
	}
}

func mkrootstat(root Root, relpath string, fi os.FileInfo, inodemap map[uint64]string) (*types.Stat, error) {
	stat := &types.Stat{
		Path:    filepath.FromSlash(filepath.ToSlash(relpath)),
		Mode:    uint32(fi.Mode()),
		ModTime: fi.ModTime().UnixNano(),
	}

	setUnixOpt(fi, stat, relpath, inodemap)

	if !fi.IsDir() {
		stat.Size = fi.Size()
		if fi.Mode()&os.ModeSymlink != 0 {
			link, err := root.Readlink(relpath)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			stat.Linkname = link
		}
	}
	if fi.IsDir() || fi.Mode().IsRegular() {
		if err := loadRootXattr(root, relpath, stat); err != nil {
			return nil, err
		}
	}

	if runtime.GOOS == "windows" {
		permPart := stat.Mode & uint32(os.ModePerm)
		noPermPart := stat.Mode &^ uint32(os.ModePerm)
		// Add the x bit: make everything +x from windows
		permPart |= 0111
		permPart &= 0755
		stat.Mode = noPermPart | permPart
	}

	// Clear the socket bit since archive/tar.FileInfoHeader does not handle it
	stat.Mode &^= uint32(os.ModeSocket)

	return stat, nil
}

func emptyWalker(ctx context.Context, pathC chan<- *currentPath) error {
	return nil
}
