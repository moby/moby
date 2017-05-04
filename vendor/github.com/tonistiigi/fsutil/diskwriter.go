// +build linux windows

package fsutil

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

type WriteToFunc func(context.Context, string, io.WriteCloser) error

type DiskWriterOpt struct {
	AsyncDataCb WriteToFunc
	SyncDataCb  WriteToFunc
	NotifyCb    func(ChangeKind, string, os.FileInfo, error) error
}

type DiskWriter struct {
	opt  DiskWriterOpt
	dest string

	wg     sync.WaitGroup
	ctx    context.Context
	cancel func()
	eg     *errgroup.Group
}

func NewDiskWriter(ctx context.Context, dest string, opt DiskWriterOpt) (*DiskWriter, error) {
	if opt.SyncDataCb == nil && opt.AsyncDataCb == nil {
		return nil, errors.New("no data callback specified")
	}
	if opt.SyncDataCb != nil && opt.AsyncDataCb != nil {
		return nil, errors.New("can't specify both sync and async data callbacks")
	}

	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)

	return &DiskWriter{
		opt:    opt,
		dest:   dest,
		eg:     eg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (dw *DiskWriter) Wait(ctx context.Context) error {
	return dw.eg.Wait()
}

func (dw *DiskWriter) HandleChange(kind ChangeKind, p string, fi os.FileInfo, err error) (retErr error) {
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

	p = filepath.FromSlash(p)

	destPath := filepath.Join(dw.dest, p)

	if kind == ChangeKindDelete {
		// todo: no need to validate if diff is trusted but is it always?
		if err := os.RemoveAll(destPath); err != nil {
			return errors.Wrapf(err, "failed to remove: %s", destPath)
		}
		if dw.opt.NotifyCb != nil {
			if err := dw.opt.NotifyCb(kind, p, nil, nil); err != nil {
				return err
			}
		}
		return nil
	}

	stat, ok := fi.Sys().(*Stat)
	if !ok {
		return errors.Errorf("%s invalid change without stat information", p)
	}

	rename := true
	oldFi, err := os.Lstat(destPath)
	if err != nil {
		if os.IsNotExist(err) {
			if kind != ChangeKindAdd {
				return errors.Wrapf(err, "invalid addition: %s", destPath)
			}
			rename = false
		} else {
			return errors.Wrapf(err, "failed to stat %s", destPath)
		}
	}

	if oldFi != nil && fi.IsDir() && oldFi.IsDir() {
		if err := rewriteMetadata(destPath, stat); err != nil {
			return errors.Wrapf(err, "error setting dir metadata for %s", destPath)
		}
		return nil
	}

	newPath := destPath
	if rename {
		newPath = filepath.Join(filepath.Dir(destPath), ".tmp."+nextSuffix())
	}

	isRegularFile := false

	switch {
	case fi.IsDir():
		if err := os.Mkdir(newPath, fi.Mode()); err != nil {
			return errors.Wrapf(err, "failed to create dir %s", newPath)
		}
	case fi.Mode()&os.ModeDevice != 0 || fi.Mode()&os.ModeNamedPipe != 0:
		if err := handleTarTypeBlockCharFifo(newPath, stat); err != nil {
			return errors.Wrapf(err, "failed to create device %s", newPath)
		}
	case fi.Mode()&os.ModeSymlink != 0:
		if err := os.Symlink(stat.Linkname, newPath); err != nil {
			return errors.Wrapf(err, "failed to symlink %s", newPath)
		}
	case stat.Linkname != "":
		if err := os.Link(filepath.Join(dw.dest, stat.Linkname), newPath); err != nil {
			return errors.Wrapf(err, "failed to link %s to %s", newPath, stat.Linkname)
		}
	default:
		isRegularFile = true
		file, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY, fi.Mode()) //todo: windows
		if err != nil {
			return errors.Wrapf(err, "failed to create %s", newPath)
		}
		if dw.opt.SyncDataCb != nil {
			if err := dw.processChange(ChangeKindAdd, p, fi, file); err != nil {
				file.Close()
				return err
			}
			break
		}
		if err := file.Close(); err != nil {
			return errors.Wrapf(err, "failed to close %s", newPath)
		}
	}

	if err := rewriteMetadata(newPath, stat); err != nil {
		return errors.Wrapf(err, "error setting metadata for %s", newPath)
	}

	if rename {
		if err := os.Rename(newPath, destPath); err != nil {
			return errors.Wrapf(err, "failed to rename %s to %s", newPath, destPath)
		}
	}

	if isRegularFile {
		if dw.opt.AsyncDataCb != nil {
			dw.requestAsyncFileData(p, destPath, fi)
		}
	} else {
		return dw.processChange(kind, p, fi, nil)
	}

	return nil
}

func (dw *DiskWriter) requestAsyncFileData(p, dest string, fi os.FileInfo) {
	// todo: limit worker threads
	dw.eg.Go(func() error {
		if err := dw.processChange(ChangeKindAdd, p, fi, &lazyFileWriter{
			dest: dest,
		}); err != nil {
			return err
		}
		return chtimes(dest, fi.ModTime().UnixNano()) // TODO: parent dirs
	})
}

func (dw *DiskWriter) processChange(kind ChangeKind, p string, fi os.FileInfo, w io.WriteCloser) error {
	origw := w
	var hw *hashedWriter
	if dw.opt.NotifyCb != nil {
		var err error
		if hw, err = newHashWriter(p, fi, w); err != nil {
			return err
		}
		w = hw
	}
	if origw != nil {
		fn := dw.opt.SyncDataCb
		if fn == nil && dw.opt.AsyncDataCb != nil {
			fn = dw.opt.AsyncDataCb
		}
		if err := fn(dw.ctx, p, w); err != nil {
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

type hashedWriter struct {
	os.FileInfo
	io.Writer
	h   hash.Hash
	w   io.WriteCloser
	sum string
}

func newHashWriter(p string, fi os.FileInfo, w io.WriteCloser) (*hashedWriter, error) {
	h, err := NewTarsumHash(p, fi)
	if err != nil {
		return nil, err
	}
	hw := &hashedWriter{
		FileInfo: fi,
		Writer:   io.MultiWriter(w, h),
		h:        h,
		w:        w,
	}
	return hw, nil
}

func (hw *hashedWriter) Close() error {
	hw.sum = string(hex.EncodeToString(hw.h.Sum(nil)))
	if hw.w != nil {
		return hw.w.Close()
	}
	return nil
}

func (hw *hashedWriter) Hash() string {
	return hw.sum
}

type lazyFileWriter struct {
	dest string
	ctx  context.Context
	f    *os.File
}

func (lfw *lazyFileWriter) Write(dt []byte) (int, error) {
	if lfw.f == nil {
		file, err := os.OpenFile(lfw.dest, os.O_WRONLY, 0) //todo: windows
		if err != nil {
			return 0, errors.Wrapf(err, "failed to open %s", lfw.dest)
		}
		lfw.f = file
	}
	return lfw.f.Write(dt)
}

func (lfw *lazyFileWriter) Close() error {
	if lfw.f != nil {
		return lfw.f.Close()
	}
	return nil
}

func mkdev(major int64, minor int64) uint32 {
	return uint32(((minor & 0xfff00) << 12) | ((major & 0xfff) << 8) | (minor & 0xff))
}

// Random number state.
// We generate random temporary file names so that there's a good
// chance the file doesn't exist yet - keeps the number of tries in
// TempFile to a minimum.
var rand uint32
var randmu sync.Mutex

func reseed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextSuffix() string {
	randmu.Lock()
	r := rand
	if r == 0 {
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	rand = r
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}

func NewTarsumHash(p string, fi os.FileInfo) (hash.Hash, error) {
	stat, ok := fi.Sys().(*Stat)
	link := ""
	if ok {
		link = stat.Linkname
	}
	if fi.IsDir() {
		p += string(os.PathSeparator)
	}
	h, err := archive.FileInfoHeader(p, fi, link)
	if err != nil {
		return nil, err
	}
	h.Name = p
	if ok {
		h.Uid = int(stat.Uid)
		h.Gid = int(stat.Gid)
		h.Linkname = stat.Linkname
		if stat.Xattrs != nil {
			h.Xattrs = make(map[string]string)
			for k, v := range stat.Xattrs {
				h.Xattrs[k] = string(v)
			}
		}
	}
	tsh := &tarsumHash{h: h, Hash: sha256.New()}
	tsh.Reset()
	return tsh, nil
}

// Reset resets the Hash to its initial state.
func (tsh *tarsumHash) Reset() {
	tsh.Hash.Reset()
	tarsum.WriteV1Header(tsh.h, tsh.Hash)
}

type tarsumHash struct {
	hash.Hash
	h *tar.Header
}
