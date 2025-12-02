package sysfs

import (
	"io/fs"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func openFile(path string, oflag sys.Oflag, perm fs.FileMode) (*os.File, sys.Errno) {
	isDir := oflag&sys.O_DIRECTORY > 0
	flag := toOsOpenFlag(oflag)

	// TODO: document why we are opening twice
	fd, err := open(path, flag|syscall.O_CLOEXEC, uint32(perm))
	if err == nil {
		return os.NewFile(uintptr(fd), path), 0
	}

	// TODO: Set FILE_SHARE_DELETE for directory as well.
	f, err := os.OpenFile(path, flag, perm)
	errno := sys.UnwrapOSError(err)
	if errno == 0 {
		return f, 0
	}

	switch errno {
	case sys.EINVAL:
		// WASI expects ENOTDIR for a file path with a trailing slash.
		if strings.HasSuffix(path, "/") {
			errno = sys.ENOTDIR
		}
	// To match expectations of WASI, e.g. TinyGo TestStatBadDir, return
	// ENOENT, not ENOTDIR.
	case sys.ENOTDIR:
		errno = sys.ENOENT
	case sys.ENOENT:
		if isSymlink(path) {
			// Either symlink or hard link not found. We change the returned
			// errno depending on if it is symlink or not to have consistent
			// behavior across OSes.
			if isDir {
				// Dangling symlink dir must raise ENOTDIR.
				errno = sys.ENOTDIR
			} else {
				errno = sys.ELOOP
			}
		}
	}
	return f, errno
}

const supportedSyscallOflag = sys.O_NONBLOCK

// Map to synthetic values here https://github.com/golang/go/blob/go1.20/src/syscall/types_windows.go#L34-L48
func withSyscallOflag(oflag sys.Oflag, flag int) int {
	// O_DIRECTORY not defined in windows
	// O_DSYNC not defined in windows
	// O_NOFOLLOW not defined in windows
	if oflag&sys.O_NONBLOCK != 0 {
		flag |= syscall.O_NONBLOCK
	}
	// O_RSYNC not defined in windows
	return flag
}

func isSymlink(path string) bool {
	if st, e := os.Lstat(path); e == nil && st.Mode()&os.ModeSymlink != 0 {
		return true
	}
	return false
}

// # Differences from syscall.Open
//
// This code is based on syscall.Open from the below link with some differences
// https://github.com/golang/go/blame/go1.20/src/syscall/syscall_windows.go#L308-L379
//
//   - syscall.O_CREAT doesn't imply syscall.GENERIC_WRITE as that breaks
//     flag expectations in wasi.
//   - add support for setting FILE_SHARE_DELETE.
func open(path string, mode int, perm uint32) (fd syscall.Handle, err error) {
	if len(path) == 0 {
		return syscall.InvalidHandle, syscall.ERROR_FILE_NOT_FOUND
	}
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	var access uint32
	switch mode & (syscall.O_RDONLY | syscall.O_WRONLY | syscall.O_RDWR) {
	case syscall.O_RDONLY:
		access = syscall.GENERIC_READ
	case syscall.O_WRONLY:
		access = syscall.GENERIC_WRITE
	case syscall.O_RDWR:
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	if mode&syscall.O_APPEND != 0 {
		access &^= syscall.GENERIC_WRITE
		access |= syscall.FILE_APPEND_DATA
	}
	sharemode := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	var sa *syscall.SecurityAttributes
	if mode&syscall.O_CLOEXEC == 0 {
		var _sa syscall.SecurityAttributes
		_sa.Length = uint32(unsafe.Sizeof(sa))
		_sa.InheritHandle = 1
		sa = &_sa
	}
	var createmode uint32
	switch {
	case mode&(syscall.O_CREAT|syscall.O_EXCL) == (syscall.O_CREAT | syscall.O_EXCL):
		createmode = syscall.CREATE_NEW
	case mode&(syscall.O_CREAT|syscall.O_TRUNC) == (syscall.O_CREAT | syscall.O_TRUNC):
		createmode = syscall.CREATE_ALWAYS
	case mode&syscall.O_CREAT == syscall.O_CREAT:
		createmode = syscall.OPEN_ALWAYS
	case mode&syscall.O_TRUNC == syscall.O_TRUNC:
		createmode = syscall.TRUNCATE_EXISTING
	default:
		createmode = syscall.OPEN_EXISTING
	}
	var attrs uint32 = syscall.FILE_ATTRIBUTE_NORMAL
	if perm&syscall.S_IWRITE == 0 {
		attrs = syscall.FILE_ATTRIBUTE_READONLY
		if createmode == syscall.CREATE_ALWAYS {
			// We have been asked to create a read-only file.
			// If the file already exists, the semantics of
			// the Unix open system call is to preserve the
			// existing permissions. If we pass CREATE_ALWAYS
			// and FILE_ATTRIBUTE_READONLY to CreateFile,
			// and the file already exists, CreateFile will
			// change the file permissions.
			// Avoid that to preserve the Unix semantics.
			h, e := syscall.CreateFile(pathp, access, sharemode, sa, syscall.TRUNCATE_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
			switch e {
			case syscall.ERROR_FILE_NOT_FOUND, syscall.ERROR_PATH_NOT_FOUND:
				// File does not exist. These are the same
				// errors as Errno.Is checks for ErrNotExist.
				// Carry on to create the file.
			default:
				// Success or some different error.
				return h, e
			}
		}
	}

	// This shouldn't be included before 1.20 to have consistent behavior.
	// https://github.com/golang/go/commit/0f0aa5d8a6a0253627d58b3aa083b24a1091933f
	if createmode == syscall.OPEN_EXISTING && access == syscall.GENERIC_READ {
		// Necessary for opening directory handles.
		attrs |= syscall.FILE_FLAG_BACKUP_SEMANTICS
	}

	h, e := syscall.CreateFile(pathp, access, sharemode, sa, createmode, attrs, 0)
	return h, e
}
