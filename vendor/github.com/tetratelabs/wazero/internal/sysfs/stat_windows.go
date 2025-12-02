//go:build (amd64 || arm64) && windows

package sysfs

import (
	"io/fs"
	"syscall"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/sys"
)

// dirNlinkIncludesDot is false because Windows does not return dot entries.
//
// Note: this is only used in tests
const dirNlinkIncludesDot = false

func lstat(path string) (sys.Stat_t, experimentalsys.Errno) {
	attrs := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	// Use FILE_FLAG_OPEN_REPARSE_POINT, otherwise CreateFile will follow symlink.
	// See https://docs.microsoft.com/en-us/windows/desktop/FileIO/symbolic-link-effects-on-file-systems-functions#createfile-and-createfiletransacted
	attrs |= syscall.FILE_FLAG_OPEN_REPARSE_POINT
	return statPath(attrs, path)
}

func stat(path string) (sys.Stat_t, experimentalsys.Errno) {
	attrs := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	return statPath(attrs, path)
}

func statPath(createFileAttrs uint32, path string) (sys.Stat_t, experimentalsys.Errno) {
	if len(path) == 0 {
		return sys.Stat_t{}, experimentalsys.ENOENT
	}
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return sys.Stat_t{}, experimentalsys.EINVAL
	}

	// open the file handle
	h, err := syscall.CreateFile(pathp, 0, 0, nil,
		syscall.OPEN_EXISTING, createFileAttrs, 0)
	if err != nil {
		errno := experimentalsys.UnwrapOSError(err)
		// To match expectations of WASI, e.g. TinyGo TestStatBadDir, return
		// ENOENT, not ENOTDIR.
		if errno == experimentalsys.ENOTDIR {
			errno = experimentalsys.ENOENT
		}
		return sys.Stat_t{}, errno
	}
	defer syscall.CloseHandle(h)

	return statHandle(h)
}

// fdFile allows masking the `Fd` function on os.File.
type fdFile interface {
	Fd() uintptr
}

func statFile(f fs.File) (sys.Stat_t, experimentalsys.Errno) {
	if osF, ok := f.(fdFile); ok {
		// Attempt to get the stat by handle, which works for normal files
		st, err := statHandle(syscall.Handle(osF.Fd()))

		// ERROR_INVALID_HANDLE happens before Go 1.20. Don't fail as we only
		// use that approach to fill in inode data, which is not critical.
		//
		// Note: statHandle uses UnwrapOSError which coerces
		// ERROR_INVALID_HANDLE to EBADF.
		if err != experimentalsys.EBADF {
			return st, err
		}
	}
	return defaultStatFile(f)
}

func statHandle(h syscall.Handle) (sys.Stat_t, experimentalsys.Errno) {
	winFt, err := syscall.GetFileType(h)
	if err != nil {
		return sys.Stat_t{}, experimentalsys.UnwrapOSError(err)
	}

	var fi syscall.ByHandleFileInformation
	if err = syscall.GetFileInformationByHandle(h, &fi); err != nil {
		return sys.Stat_t{}, experimentalsys.UnwrapOSError(err)
	}

	var m fs.FileMode
	if fi.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		m |= 0o444
	} else {
		m |= 0o666
	}

	switch { // check whether this is a symlink first
	case fi.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0:
		m |= fs.ModeSymlink
	case winFt == syscall.FILE_TYPE_PIPE:
		m |= fs.ModeNamedPipe
	case winFt == syscall.FILE_TYPE_CHAR:
		m |= fs.ModeDevice | fs.ModeCharDevice
	case fi.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0:
		m |= fs.ModeDir | 0o111 // e.g. 0o444 -> 0o555
	}

	st := sys.Stat_t{}
	// FileIndex{High,Low} can be combined and used as a unique identifier like inode.
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/ns-fileapi-by_handle_file_information
	st.Dev = uint64(fi.VolumeSerialNumber)
	st.Ino = (uint64(fi.FileIndexHigh) << 32) | uint64(fi.FileIndexLow)
	st.Mode = m
	st.Nlink = uint64(fi.NumberOfLinks)
	st.Size = int64(fi.FileSizeHigh)<<32 + int64(fi.FileSizeLow)
	st.Atim = fi.LastAccessTime.Nanoseconds()
	st.Mtim = fi.LastWriteTime.Nanoseconds()
	st.Ctim = fi.CreationTime.Nanoseconds()
	return st, 0
}
