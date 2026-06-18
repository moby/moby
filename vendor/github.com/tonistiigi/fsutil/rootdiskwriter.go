package fsutil

import (
	"context"
	"io"
	gofs "io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
	"golang.org/x/sync/errgroup"
)

type RootDiskWriter struct {
	opt       DiskWriterOpt
	dest      Root
	rootStack *rootStack
	rootCache *rootCache

	ctx         context.Context
	cancel      func()
	eg          *errgroup.Group
	egCtx       context.Context
	filter      FilterFunc
	dirModTimes map[string]int64
}

func NewRootDiskWriter(ctx context.Context, dest Root, opt DiskWriterOpt) (*RootDiskWriter, error) {
	if opt.SyncDataCb == nil && opt.AsyncDataCb == nil {
		return nil, errors.New("no data callback specified")
	}
	if opt.SyncDataCb != nil && opt.AsyncDataCb != nil {
		return nil, errors.New("can't specify both sync and async data callbacks")
	}

	ctx, cancel := context.WithCancel(ctx)
	eg, egCtx := errgroup.WithContext(ctx)

	return &RootDiskWriter{
		opt:         opt,
		dest:        dest,
		rootStack:   newRootStack(dest),
		rootCache:   newRootCache(dest, rootCacheDefaultSize),
		eg:          eg,
		ctx:         ctx,
		egCtx:       egCtx,
		cancel:      cancel,
		filter:      opt.Filter,
		dirModTimes: map[string]int64{},
	}, nil
}

func (dw *RootDiskWriter) Wait(ctx context.Context) error {
	err := dw.eg.Wait()
	if closeErr := dw.rootCache.Close(); err == nil {
		err = closeErr
	}
	if closeErr := dw.rootStack.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return gofs.WalkDir(dw.dest.FS(), ".", func(path string, d gofs.DirEntry, prevErr error) error {
		if prevErr != nil {
			return prevErr
		}
		if !d.IsDir() {
			return nil
		}
		if mtime, ok := dw.dirModTimes[path]; ok {
			return rootChtimes(dw.dest, filepath.FromSlash(path), mtime)
		}
		return nil
	})
}

func (dw *RootDiskWriter) HandleChange(kind ChangeKind, p string, fi os.FileInfo, err error) (retErr error) {
	if err != nil {
		return err
	}

	select {
	case <-dw.ctx.Done():
		return dw.ctx.Err()
	default:
	}

	defer func() {
		if retErr != nil {
			dw.cancel()
		}
	}()

	destPath := cleanRootPath(p)
	destRoot, base, err := dw.rootStack.get(destPath)
	if err != nil {
		return err
	}

	if kind == ChangeKindDelete {
		if dw.filter != nil {
			var empty types.Stat
			if ok := dw.filter(p, &empty); !ok {
				return nil
			}
		}
		// todo: no need to validate if diff is trusted but is it always?
		if err := destRoot.RemoveAll(base); err != nil {
			return errors.Wrapf(err, "failed to remove: %s", destPath)
		}
		if dw.opt.NotifyCb != nil {
			if err := dw.opt.NotifyCb(kind, p, nil, nil); err != nil {
				return err
			}
		}
		return nil
	}

	stat, ok := fi.Sys().(*types.Stat)
	if !ok {
		return errors.WithStack(&os.PathError{Path: p, Err: syscall.EBADMSG, Op: "change without stat info"})
	}

	statCopy := stat.Clone()

	if dw.filter != nil {
		if ok := dw.filter(p, statCopy); !ok {
			return nil
		}
	}

	rename := true
	oldFi, err := destRoot.Lstat(base)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if kind != ChangeKindAdd {
				return errors.Wrap(err, "modify/rm")
			}
			rename = false
		} else {
			return errors.WithStack(err)
		}
	}

	if oldFi != nil && fi.IsDir() && oldFi.IsDir() {
		if err := rewriteRootMetadata(destRoot, base, statCopy); err != nil {
			return errors.Wrapf(err, "error setting dir metadata for %s", destPath)
		}
		return nil
	}

	newPath := base
	if rename {
		newPath = ".tmp." + nextSuffix()
	}

	isRegularFile := false

	switch {
	case fi.IsDir():
		if err := destRoot.Mkdir(newPath, fi.Mode().Perm()); err != nil {
			if errors.Is(err, syscall.EEXIST) {
				// we saw a race to create this directory, so try again
				return dw.HandleChange(kind, p, fi, nil)
			}
			return errors.Wrapf(err, "failed to create dir %s", newPath)
		}
		dw.dirModTimes[filepath.ToSlash(destPath)] = statCopy.ModTime
	case fi.Mode()&os.ModeDevice != 0 || fi.Mode()&os.ModeNamedPipe != 0:
		if err := handleRootTarTypeBlockCharFifo(destRoot, newPath, statCopy); err != nil {
			return errors.Wrapf(err, "failed to create device %s", newPath)
		}
	case fi.Mode()&os.ModeSymlink != 0:
		if err := destRoot.Symlink(statCopy.Linkname, newPath); err != nil {
			return errors.Wrapf(err, "failed to symlink %s", newPath)
		}
	case statCopy.Linkname != "":
		linkNewName := destPath
		if rename {
			linkNewName = filepath.Join(filepath.Dir(destPath), newPath)
		}
		if err := dw.dest.Link(statCopy.Linkname, linkNewName); err != nil {
			return errors.Wrapf(err, "failed to link %s to %s", newPath, statCopy.Linkname)
		}
	default:
		isRegularFile = true
		file, err := destRoot.OpenFile(newPath, os.O_CREATE|os.O_WRONLY, fi.Mode().Perm())
		if err != nil {
			return errors.Wrapf(err, "failed to create %s", newPath)
		}
		if dw.opt.SyncDataCb != nil {
			if err := dw.processChange(dw.ctx, ChangeKindAdd, p, fi, file); err != nil {
				file.Close()
				return err
			}
		}
		if err := file.Close(); err != nil {
			return errors.Wrapf(err, "failed to close %s", newPath)
		}
	}

	if err := rewriteRootMetadata(destRoot, newPath, statCopy); err != nil {
		return errors.Wrapf(err, "error setting metadata for %s", newPath)
	}

	if rename {
		if oldFi.IsDir() != fi.IsDir() {
			if err := destRoot.RemoveAll(base); err != nil {
				return errors.Wrapf(err, "failed to remove %s", destPath)
			}
		}

		if err := destRoot.Rename(newPath, base); err != nil {
			return errors.Wrapf(err, "failed to rename %s to %s", newPath, destPath)
		}
	}

	if isRegularFile {
		if dw.opt.AsyncDataCb != nil {
			dw.requestAsyncFileData(p, destPath, fi, statCopy)
		}
	} else {
		return dw.processChange(dw.ctx, kind, p, fi, nil)
	}

	return nil
}

func (dw *RootDiskWriter) requestAsyncFileData(p, dest string, fi os.FileInfo, st *types.Stat) {
	// todo: limit worker threads
	dw.eg.Go(func() error {
		lease, err := dw.rootCache.get(dest)
		if err != nil {
			return err
		}
		defer lease.Release()

		w := &rootLazyFileWriter{lease: lease}
		if err := dw.processChange(dw.egCtx, ChangeKindAdd, p, fi, w); err != nil {
			w.Close()
			return err
		}
		return rootChtimes(lease.root, lease.base, st.ModTime) // TODO: parent dirs
	})
}

func (dw *RootDiskWriter) processChange(ctx context.Context, kind ChangeKind, p string, fi os.FileInfo, w io.WriteCloser) error {
	origw := w
	var hw *hashedWriter
	if dw.opt.NotifyCb != nil {
		var err error
		if hw, err = newHashWriter(dw.opt.ContentHasher, fi, w); err != nil {
			return err
		}
		w = hw
	}
	if origw != nil {
		fn := dw.opt.SyncDataCb
		if fn == nil && dw.opt.AsyncDataCb != nil {
			fn = dw.opt.AsyncDataCb
		}
		if err := fn(ctx, p, w); err != nil {
			return err
		}
	} else {
		if hw != nil {
			hw.Close()
		}
	}
	if hw != nil {
		return dw.opt.NotifyCb(kind, p, hw, nil)
	}
	return nil
}

func cleanRootPath(p string) string {
	if p == "" {
		return "."
	}
	return filepath.Clean(p)
}

func rootChtimes(root Root, p string, un int64) error {
	t := time.Unix(0, un)
	if err := root.Chtimes(p, t, t); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

type rootLazyFileWriter struct {
	lease    *rootLease
	f        *os.File
	fileMode *os.FileMode
	closed   bool
}

func (lfw *rootLazyFileWriter) Write(dt []byte) (int, error) {
	if lfw.f == nil {
		file, err := lfw.lease.root.OpenFile(lfw.lease.base, os.O_WRONLY, 0)
		if os.IsPermission(err) {
			// retry after chmod
			fi, er := lfw.lease.root.Stat(lfw.lease.base)
			if er == nil {
				mode := fi.Mode()
				lfw.fileMode = &mode
				er = lfw.lease.root.Chmod(lfw.lease.base, mode|0222)
				if er == nil {
					file, err = lfw.lease.root.OpenFile(lfw.lease.base, os.O_WRONLY, 0)
				}
			}
		}
		if err != nil {
			return 0, errors.Wrapf(err, "failed to open %s", lfw.lease.base)
		}
		lfw.f = file
	}
	return lfw.f.Write(dt)
}

func (lfw *rootLazyFileWriter) Close() error {
	if lfw.closed {
		return nil
	}
	lfw.closed = true

	var err error
	if lfw.f != nil {
		err = lfw.f.Close()
	}
	if err == nil && lfw.fileMode != nil {
		err = lfw.lease.root.Chmod(lfw.lease.base, *lfw.fileMode)
	}
	return err
}
