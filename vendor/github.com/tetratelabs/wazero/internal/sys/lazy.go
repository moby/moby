package sys

import (
	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/fsapi"
	"github.com/tetratelabs/wazero/sys"
)

// compile-time check to ensure lazyDir implements sys.File.
var _ experimentalsys.File = (*lazyDir)(nil)

type lazyDir struct {
	experimentalsys.DirFile

	fs experimentalsys.FS
	f  experimentalsys.File
}

// Dev implements the same method as documented on sys.File
func (d *lazyDir) Dev() (uint64, experimentalsys.Errno) {
	if f, ok := d.file(); !ok {
		return 0, experimentalsys.EBADF
	} else {
		return f.Dev()
	}
}

// Ino implements the same method as documented on sys.File
func (d *lazyDir) Ino() (sys.Inode, experimentalsys.Errno) {
	if f, ok := d.file(); !ok {
		return 0, experimentalsys.EBADF
	} else {
		return f.Ino()
	}
}

// IsDir implements the same method as documented on sys.File
func (d *lazyDir) IsDir() (bool, experimentalsys.Errno) {
	// Note: we don't return a constant because we don't know if this is really
	// backed by a dir, until the first call.
	if f, ok := d.file(); !ok {
		return false, experimentalsys.EBADF
	} else {
		return f.IsDir()
	}
}

// IsAppend implements the same method as documented on sys.File
func (d *lazyDir) IsAppend() bool {
	return false
}

// SetAppend implements the same method as documented on sys.File
func (d *lazyDir) SetAppend(bool) experimentalsys.Errno {
	return experimentalsys.EISDIR
}

// Seek implements the same method as documented on sys.File
func (d *lazyDir) Seek(offset int64, whence int) (newOffset int64, errno experimentalsys.Errno) {
	if f, ok := d.file(); !ok {
		return 0, experimentalsys.EBADF
	} else {
		return f.Seek(offset, whence)
	}
}

// Stat implements the same method as documented on sys.File
func (d *lazyDir) Stat() (sys.Stat_t, experimentalsys.Errno) {
	if f, ok := d.file(); !ok {
		return sys.Stat_t{}, experimentalsys.EBADF
	} else {
		return f.Stat()
	}
}

// Readdir implements the same method as documented on sys.File
func (d *lazyDir) Readdir(n int) (dirents []experimentalsys.Dirent, errno experimentalsys.Errno) {
	if f, ok := d.file(); !ok {
		return nil, experimentalsys.EBADF
	} else {
		return f.Readdir(n)
	}
}

// Sync implements the same method as documented on sys.File
func (d *lazyDir) Sync() experimentalsys.Errno {
	if f, ok := d.file(); !ok {
		return experimentalsys.EBADF
	} else {
		return f.Sync()
	}
}

// Datasync implements the same method as documented on sys.File
func (d *lazyDir) Datasync() experimentalsys.Errno {
	if f, ok := d.file(); !ok {
		return experimentalsys.EBADF
	} else {
		return f.Datasync()
	}
}

// Utimens implements the same method as documented on sys.File
func (d *lazyDir) Utimens(atim, mtim int64) experimentalsys.Errno {
	if f, ok := d.file(); !ok {
		return experimentalsys.EBADF
	} else {
		return f.Utimens(atim, mtim)
	}
}

// file returns the underlying file or false if it doesn't exist.
func (d *lazyDir) file() (experimentalsys.File, bool) {
	if f := d.f; d.f != nil {
		return f, true
	}
	var errno experimentalsys.Errno
	d.f, errno = d.fs.OpenFile(".", experimentalsys.O_RDONLY, 0)
	switch errno {
	case 0:
		return d.f, true
	case experimentalsys.ENOENT:
		return nil, false
	default:
		panic(errno) // unexpected
	}
}

// Close implements fs.File
func (d *lazyDir) Close() experimentalsys.Errno {
	f := d.f
	if f == nil {
		return 0 // never opened
	}
	return f.Close()
}

// IsNonblock implements the same method as documented on fsapi.File
func (d *lazyDir) IsNonblock() bool {
	return false
}

// SetNonblock implements the same method as documented on fsapi.File
func (d *lazyDir) SetNonblock(bool) experimentalsys.Errno {
	return experimentalsys.EISDIR
}

// Poll implements the same method as documented on fsapi.File
func (d *lazyDir) Poll(fsapi.Pflag, int32) (ready bool, errno experimentalsys.Errno) {
	return false, experimentalsys.ENOSYS
}
