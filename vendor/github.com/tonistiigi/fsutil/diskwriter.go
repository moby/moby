package fsutil

import (
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type writeToFunc func(context.Context, string, io.WriteCloser) error

type DiskWriter struct {
	asyncDataFunc writeToFunc
	syncDataFunc  writeToFunc
	dest          string

	wg           sync.WaitGroup
	mu           sync.RWMutex
	err          error
	ctx          context.Context
	cancel       func()
	notifyHashed func(ChangeKind, string, os.FileInfo, error) error
}

func (dw *DiskWriter) Wait() error {
	dw.wg.Wait()
	dw.mu.RLock()
	defer dw.mu.RUnlock()
	return dw.err
}

func (dw *DiskWriter) HandleChange(kind ChangeKind, p string, fi os.FileInfo, err error) (retErr error) {
	if err != nil {
		return err
	}

	if dw.ctx == nil {
		ctx, cancel := context.WithCancel(context.Background())
		dw.ctx = ctx
		dw.cancel = cancel
	}

	defer func() {
		if retErr != nil {
			dw.mu.Lock()
			if dw.err == nil {
				dw.err = err
			}
			dw.mu.Unlock()
			dw.cancel()
		}
	}()

	destPath := filepath.Join(dw.dest, p)

	if kind == ChangeKindDelete {
		// todo: no need to validate if diff is trusted but is it always?
		if err := os.RemoveAll(destPath); err != nil {
			return errors.Wrapf(err, "failed to remove: %s", destPath)
		}
		if dw.notifyHashed != nil {
			if err := dw.notifyHashed(kind, p, nil, nil); err != nil {
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

	// todo: combine with hardlink validation

	asyncRequestFileData := false
	var hw *hashedWriter

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
		file, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY, fi.Mode()) //todo: windows
		if err != nil {
			return errors.Wrapf(err, "failed to create %s", newPath)
		}
		if dw.syncDataFunc != nil {
			var h io.WriteCloser = file
			if dw.notifyHashed != nil {
				if hw, err = newHashWriter(p, fi, file); err != nil {
					return err
				}
				h = hw
			}
			if err := dw.syncDataFunc(dw.ctx, p, h); err != nil {
				return errors.Wrapf(err, "failed to write %s", newPath)
			}
			break
		} else if dw.asyncDataFunc != nil {
			asyncRequestFileData = true
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

	if asyncRequestFileData {
		dw.requestAsyncFileData(p, destPath, stat)
	} else if dw.notifyHashed != nil {
		if hw == nil {
			if hw, err = newHashWriter(p, fi, nil); err != nil {
				return err
			}
			hw.Close()
		}
		if err := dw.notifyHashed(kind, p, hw, nil); err != nil {
			return err
		}
	}

	return nil
}

func (dw *DiskWriter) requestAsyncFileData(p, dest string, stat *Stat) {
	dw.wg.Add(1)
	// todo: limit worker threads
	go func() (retErr error) {
		defer func() {
			if retErr != nil {
				dw.mu.Lock()
				if dw.err == nil {
					dw.err = retErr
					dw.cancel()
				}
				dw.mu.Unlock()
			}
		}()
		var hw *hashedWriter
		var h io.WriteCloser = &lazyFileWriter{
			dest: dest,
		}
		if dw.notifyHashed != nil {
			var err error
			if hw, err = newHashWriter(p, &StatInfo{stat}, h); err != nil {
				return err
			}
			h = hw
		}
		if err := dw.asyncDataFunc(dw.ctx, p, h); err != nil {
			return err
		}
		if hw != nil {
			if err := dw.notifyHashed(ChangeKindAdd, p, hw, nil); err != nil {
				return err
			}
		}
		if err := chtimes(dest, stat.ModTime); err != nil { // TODO: check parent dirs
			return err
		}
		dw.wg.Done()
		return nil
	}()
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
		logrus.Error(err)
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

func (hw *hashedWriter) SetHash(s string) {
}

// Hashed defines an extra method intended for implementations of os.FileInfo.
type Hashed interface {
	// Hash returns the hash of a file.
	Hash() string
	SetHash(string)
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
