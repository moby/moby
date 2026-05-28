package sysfs

import (
	"io/fs"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
)

type ReadFS struct {
	experimentalsys.FS
}

// OpenFile implements the same method as documented on sys.FS
func (r *ReadFS) OpenFile(path string, flag experimentalsys.Oflag, perm fs.FileMode) (experimentalsys.File, experimentalsys.Errno) {
	// Mask the mutually exclusive bits as they determine write mode.
	switch flag & (experimentalsys.O_RDONLY | experimentalsys.O_WRONLY | experimentalsys.O_RDWR) {
	case experimentalsys.O_WRONLY, experimentalsys.O_RDWR:
		// Return the correct error if a directory was opened for write.
		if flag&experimentalsys.O_DIRECTORY != 0 {
			return nil, experimentalsys.EISDIR
		}
		return nil, experimentalsys.ENOSYS
	default: // sys.O_RDONLY (integer zero) so we are ok!
	}

	f, errno := r.FS.OpenFile(path, flag, perm)
	if errno != 0 {
		return nil, errno
	}
	return &readFile{f}, 0
}

// Mkdir implements the same method as documented on sys.FS
func (r *ReadFS) Mkdir(path string, perm fs.FileMode) experimentalsys.Errno {
	return experimentalsys.EROFS
}

// Chmod implements the same method as documented on sys.FS
func (r *ReadFS) Chmod(path string, perm fs.FileMode) experimentalsys.Errno {
	return experimentalsys.EROFS
}

// Rename implements the same method as documented on sys.FS
func (r *ReadFS) Rename(from, to string) experimentalsys.Errno {
	return experimentalsys.EROFS
}

// Rmdir implements the same method as documented on sys.FS
func (r *ReadFS) Rmdir(path string) experimentalsys.Errno {
	return experimentalsys.EROFS
}

// Link implements the same method as documented on sys.FS
func (r *ReadFS) Link(_, _ string) experimentalsys.Errno {
	return experimentalsys.EROFS
}

// Symlink implements the same method as documented on sys.FS
func (r *ReadFS) Symlink(_, _ string) experimentalsys.Errno {
	return experimentalsys.EROFS
}

// Unlink implements the same method as documented on sys.FS
func (r *ReadFS) Unlink(path string) experimentalsys.Errno {
	return experimentalsys.EROFS
}

// Utimens implements the same method as documented on sys.FS
func (r *ReadFS) Utimens(path string, atim, mtim int64) experimentalsys.Errno {
	return experimentalsys.EROFS
}

// compile-time check to ensure readFile implements api.File.
var _ experimentalsys.File = (*readFile)(nil)

type readFile struct {
	experimentalsys.File
}

// Write implements the same method as documented on sys.File.
func (r *readFile) Write([]byte) (int, experimentalsys.Errno) {
	return 0, r.writeErr()
}

// Pwrite implements the same method as documented on sys.File.
func (r *readFile) Pwrite([]byte, int64) (n int, errno experimentalsys.Errno) {
	return 0, r.writeErr()
}

// Truncate implements the same method as documented on sys.File.
func (r *readFile) Truncate(int64) experimentalsys.Errno {
	return r.writeErr()
}

// Sync implements the same method as documented on sys.File.
func (r *readFile) Sync() experimentalsys.Errno {
	return experimentalsys.EBADF
}

// Datasync implements the same method as documented on sys.File.
func (r *readFile) Datasync() experimentalsys.Errno {
	return experimentalsys.EBADF
}

// Utimens implements the same method as documented on sys.File.
func (r *readFile) Utimens(int64, int64) experimentalsys.Errno {
	return experimentalsys.EBADF
}

func (r *readFile) writeErr() experimentalsys.Errno {
	if isDir, errno := r.IsDir(); errno != 0 {
		return errno
	} else if isDir {
		return experimentalsys.EISDIR
	}
	return experimentalsys.EBADF
}
