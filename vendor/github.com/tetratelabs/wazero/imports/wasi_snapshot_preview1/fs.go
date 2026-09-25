package wasi_snapshot_preview1

import (
	"context"
	"io"
	"io/fs"
	"math"
	"path"
	"strings"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	socketapi "github.com/tetratelabs/wazero/internal/sock"
	"github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/internal/wasip1"
	"github.com/tetratelabs/wazero/internal/wasm"
	sysapi "github.com/tetratelabs/wazero/sys"
)

// fdAdvise is the WASI function named FdAdviseName which provides file
// advisory information on a file descriptor.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_advisefd-fd-offset-filesize-len-filesize-advice-advice---errno
var fdAdvise = newHostFunc(
	wasip1.FdAdviseName, fdAdviseFn,
	[]wasm.ValueType{i32, i64, i64, i32},
	"fd", "offset", "len", "advice",
)

func fdAdviseFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fd := int32(params[0])
	_ = params[1]
	_ = params[2]
	advice := byte(params[3])
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	_, ok := fsc.LookupFile(fd)
	if !ok {
		return experimentalsys.EBADF
	}

	switch advice {
	case wasip1.FdAdviceNormal,
		wasip1.FdAdviceSequential,
		wasip1.FdAdviceRandom,
		wasip1.FdAdviceWillNeed,
		wasip1.FdAdviceDontNeed,
		wasip1.FdAdviceNoReuse:
	default:
		return experimentalsys.EINVAL
	}

	// FdAdvice corresponds to posix_fadvise, but it can only be supported on linux.
	// However, the purpose of the call is just to do best-effort optimization on OS kernels,
	// so just making this noop rather than returning NoSup error makes sense and doesn't affect
	// the semantics of Wasm applications.
	// TODO: invoke posix_fadvise on linux, and partially on darwin.
	// - https://gitlab.com/cznic/fileutil/-/blob/v1.1.2/fileutil_linux.go#L87-95
	// - https://github.com/bytecodealliance/system-interface/blob/62b97f9776b86235f318c3a6e308395a1187439b/src/fs/file_io_ext.rs#L430-L442
	return 0
}

// fdAllocate is the WASI function named FdAllocateName which forces the
// allocation of space in a file.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_allocatefd-fd-offset-filesize-len-filesize---errno
var fdAllocate = newHostFunc(
	wasip1.FdAllocateName, fdAllocateFn,
	[]wasm.ValueType{i32, i64, i64},
	"fd", "offset", "len",
)

func fdAllocateFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fd := int32(params[0])
	offset := params[1]
	length := params[2]

	fsc := mod.(*wasm.ModuleInstance).Sys.FS()
	f, ok := fsc.LookupFile(fd)
	if !ok {
		return experimentalsys.EBADF
	}

	tail := int64(offset + length)
	if tail < 0 {
		return experimentalsys.EINVAL
	}

	st, errno := f.File.Stat()
	if errno != 0 {
		return errno
	}

	if st.Size >= tail {
		return 0 // We already have enough space.
	}

	return f.File.Truncate(tail)
}

// fdClose is the WASI function named FdCloseName which closes a file
// descriptor.
//
// # Parameters
//
//   - fd: file descriptor to close
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: the fd was not open.
//   - sys.ENOTSUP: the fs was a pre-open
//
// Note: This is similar to `close` in POSIX.
// See https://github.com/WebAssembly/WASI/blob/main/phases/snapshot/docs.md#fd_close
// and https://linux.die.net/man/3/close
var fdClose = newHostFunc(wasip1.FdCloseName, fdCloseFn, []api.ValueType{i32}, "fd")

func fdCloseFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()
	fd := int32(params[0])

	return fsc.CloseFile(fd)
}

// fdDatasync is the WASI function named FdDatasyncName which synchronizes
// the data of a file to disk.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_datasyncfd-fd---errno
var fdDatasync = newHostFunc(wasip1.FdDatasyncName, fdDatasyncFn, []api.ValueType{i32}, "fd")

func fdDatasyncFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()
	fd := int32(params[0])

	// Check to see if the file descriptor is available
	if f, ok := fsc.LookupFile(fd); !ok {
		return experimentalsys.EBADF
	} else {
		return f.File.Datasync()
	}
}

// fdFdstatGet is the WASI function named FdFdstatGetName which returns the
// attributes of a file descriptor.
//
// # Parameters
//
//   - fd: file descriptor to get the fdstat attributes data
//   - resultFdstat: offset to write the result fdstat data
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.EFAULT: `resultFdstat` points to an offset out of memory
//
// fdstat byte layout is 24-byte size, with the following fields:
//   - fs_filetype 1 byte: the file type
//   - fs_flags 2 bytes: the file descriptor flag
//   - 5 pad bytes
//   - fs_right_base 8 bytes: ignored as rights were removed from WASI.
//   - fs_right_inheriting 8 bytes: ignored as rights were removed from WASI.
//
// For example, with a file corresponding with `fd` was a directory (=3) opened
// with `fd_read` right (=1) and no fs_flags (=0), parameter resultFdstat=1,
// this function writes the below to api.Memory:
//
//	                uint16le   padding            uint64le                uint64le
//	       uint8 --+  +--+  +-----------+  +--------------------+  +--------------------+
//	               |  |  |  |           |  |                    |  |                    |
//	     []byte{?, 3, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0}
//	resultFdstat --^  ^-- fs_flags         ^-- fs_right_base       ^-- fs_right_inheriting
//	               |
//	               +-- fs_filetype
//
// Note: fdFdstatGet returns similar flags to `fsync(fd, F_GETFL)` in POSIX, as
// well as additional fields.
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fdstat
// and https://linux.die.net/man/3/fsync
var fdFdstatGet = newHostFunc(wasip1.FdFdstatGetName, fdFdstatGetFn, []api.ValueType{i32, i32}, "fd", "result.stat")

// fdFdstatGetFn cannot currently use proxyResultParams because fdstat is larger
// than api.ValueTypeI64 (i64 == 8 bytes, but fdstat is 24).
func fdFdstatGetFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd, resultFdstat := int32(params[0]), uint32(params[1])

	// Ensure we can write the fdstat
	buf, ok := mod.Memory().Read(resultFdstat, 24)
	if !ok {
		return experimentalsys.EFAULT
	}

	var fdflags uint16
	var st sysapi.Stat_t
	var errno experimentalsys.Errno
	f, ok := fsc.LookupFile(fd)
	if !ok {
		return experimentalsys.EBADF
	} else if st, errno = f.File.Stat(); errno != 0 {
		return errno
	} else if f.File.IsAppend() {
		fdflags |= wasip1.FD_APPEND
	}

	if f.File.IsNonblock() {
		fdflags |= wasip1.FD_NONBLOCK
	}

	var fsRightsBase uint32
	var fsRightsInheriting uint32
	fileType := getExtendedWasiFiletype(f.File, st.Mode)

	switch fileType {
	case wasip1.FILETYPE_DIRECTORY:
		// To satisfy wasi-testsuite, we must advertise that directories cannot
		// be given seek permission (RIGHT_FD_SEEK).
		fsRightsBase = dirRightsBase
		fsRightsInheriting = fileRightsBase | dirRightsBase
	case wasip1.FILETYPE_CHARACTER_DEVICE:
		// According to wasi-libc,
		// > A tty is a character device that we can't seek or tell on.
		// See https://github.com/WebAssembly/wasi-libc/blob/a6f871343313220b76009827ed0153586361c0d5/libc-bottom-half/sources/isatty.c#L13-L18
		fsRightsBase = fileRightsBase &^ wasip1.RIGHT_FD_SEEK &^ wasip1.RIGHT_FD_TELL
	default:
		fsRightsBase = fileRightsBase
	}

	writeFdstat(buf, fileType, fdflags, fsRightsBase, fsRightsInheriting)
	return 0
}

// isPreopenedStdio returns true if the FD is sys.FdStdin, sys.FdStdout or
// sys.FdStderr and pre-opened. This double check is needed in case the guest
// closes stdin and re-opens it with a random alternative file.
//
// Currently, we only support non-blocking mode for standard I/O streams.
// Non-blocking mode is rarely supported for regular files, and we don't
// yet have support for sockets, so we make a special case.
//
// Note: this to get or set FD_NONBLOCK, but skip FD_APPEND. Our current
// implementation can't set FD_APPEND, without re-opening files. As stdio are
// pre-opened, we don't know how to re-open them, neither should we close the
// underlying file. Later, we could add support for setting FD_APPEND, similar
// to SetNonblock.
func isPreopenedStdio(fd int32, f *sys.FileEntry) bool {
	return fd <= sys.FdStderr && f.IsPreopen
}

const fileRightsBase = wasip1.RIGHT_FD_DATASYNC |
	wasip1.RIGHT_FD_READ |
	wasip1.RIGHT_FD_SEEK |
	wasip1.RIGHT_FDSTAT_SET_FLAGS |
	wasip1.RIGHT_FD_SYNC |
	wasip1.RIGHT_FD_TELL |
	wasip1.RIGHT_FD_WRITE |
	wasip1.RIGHT_FD_ADVISE |
	wasip1.RIGHT_FD_ALLOCATE |
	wasip1.RIGHT_FD_FILESTAT_GET |
	wasip1.RIGHT_FD_FILESTAT_SET_SIZE |
	wasip1.RIGHT_FD_FILESTAT_SET_TIMES |
	wasip1.RIGHT_POLL_FD_READWRITE

const dirRightsBase = wasip1.RIGHT_FD_DATASYNC |
	wasip1.RIGHT_FDSTAT_SET_FLAGS |
	wasip1.RIGHT_FD_SYNC |
	wasip1.RIGHT_PATH_CREATE_DIRECTORY |
	wasip1.RIGHT_PATH_CREATE_FILE |
	wasip1.RIGHT_PATH_LINK_SOURCE |
	wasip1.RIGHT_PATH_LINK_TARGET |
	wasip1.RIGHT_PATH_OPEN |
	wasip1.RIGHT_FD_READDIR |
	wasip1.RIGHT_PATH_READLINK |
	wasip1.RIGHT_PATH_RENAME_SOURCE |
	wasip1.RIGHT_PATH_RENAME_TARGET |
	wasip1.RIGHT_PATH_FILESTAT_GET |
	wasip1.RIGHT_PATH_FILESTAT_SET_SIZE |
	wasip1.RIGHT_PATH_FILESTAT_SET_TIMES |
	wasip1.RIGHT_FD_FILESTAT_GET |
	wasip1.RIGHT_FD_FILESTAT_SET_TIMES |
	wasip1.RIGHT_PATH_SYMLINK |
	wasip1.RIGHT_PATH_REMOVE_DIRECTORY |
	wasip1.RIGHT_PATH_UNLINK_FILE

func writeFdstat(buf []byte, fileType uint8, fdflags uint16, fsRightsBase, fsRightsInheriting uint32) {
	b := (*[24]byte)(buf)
	le.PutUint16(b[0:], uint16(fileType))
	le.PutUint16(b[2:], fdflags)
	le.PutUint32(b[4:], 0)
	le.PutUint64(b[8:], uint64(fsRightsBase))
	le.PutUint64(b[16:], uint64(fsRightsInheriting))
}

// fdFdstatSetFlags is the WASI function named FdFdstatSetFlagsName which
// adjusts the flags associated with a file descriptor.
var fdFdstatSetFlags = newHostFunc(wasip1.FdFdstatSetFlagsName, fdFdstatSetFlagsFn, []wasm.ValueType{i32, i32}, "fd", "flags")

func fdFdstatSetFlagsFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fd, wasiFlag := int32(params[0]), uint16(params[1])
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	// Currently we only support APPEND and NONBLOCK.
	if wasip1.FD_DSYNC&wasiFlag != 0 || wasip1.FD_RSYNC&wasiFlag != 0 || wasip1.FD_SYNC&wasiFlag != 0 {
		return experimentalsys.EINVAL
	}

	if f, ok := fsc.LookupFile(fd); !ok {
		return experimentalsys.EBADF
	} else {
		nonblock := wasip1.FD_NONBLOCK&wasiFlag != 0
		errno := f.File.SetNonblock(nonblock)
		if errno != 0 {
			return errno
		}
		if stat, err := f.File.Stat(); err == 0 && stat.Mode.IsRegular() {
			// For normal files, proceed to apply an append flag.
			append := wasip1.FD_APPEND&wasiFlag != 0
			return f.File.SetAppend(append)
		}
	}

	return 0
}

// fdFdstatSetRights will not be implemented as rights were removed from WASI.
//
// See https://github.com/bytecodealliance/wasmtime/pull/4666
var fdFdstatSetRights = stubFunction(
	wasip1.FdFdstatSetRightsName,
	[]wasm.ValueType{i32, i64, i64},
	"fd", "fs_rights_base", "fs_rights_inheriting",
)

// fdFilestatGet is the WASI function named FdFilestatGetName which returns
// the stat attributes of an open file.
//
// # Parameters
//
//   - fd: file descriptor to get the filestat attributes data for
//   - resultFilestat: offset to write the result filestat data
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.EIO: could not stat `fd` on filesystem
//   - sys.EFAULT: `resultFilestat` points to an offset out of memory
//
// filestat byte layout is 64-byte size, with the following fields:
//   - dev 8 bytes: the device ID of device containing the file
//   - ino 8 bytes: the file serial number
//   - filetype 1 byte: the type of the file
//   - 7 pad bytes
//   - nlink 8 bytes: number of hard links to the file
//   - size 8 bytes: for regular files, the file size in bytes. For symbolic links, the length in bytes of the pathname contained in the symbolic link
//   - atim 8 bytes: ast data access timestamp
//   - mtim 8 bytes: last data modification timestamp
//   - ctim 8 bytes: ast file status change timestamp
//
// For example, with a regular file this function writes the below to api.Memory:
//
//	                                                             uint8 --+
//		                         uint64le                uint64le        |        padding               uint64le                uint64le                         uint64le                               uint64le                             uint64le
//		                 +--------------------+  +--------------------+  |  +-----------------+  +--------------------+  +-----------------------+  +----------------------------------+  +----------------------------------+  +----------------------------------+
//		                 |                    |  |                    |  |  |                 |  |                    |  |                       |  |                                  |  |                                  |  |                                  |
//		          []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 117, 80, 0, 0, 0, 0, 0, 0, 160, 153, 212, 128, 110, 221, 35, 23, 160, 153, 212, 128, 110, 221, 35, 23, 160, 153, 212, 128, 110, 221, 35, 23}
//		resultFilestat   ^-- dev                 ^-- ino                 ^                       ^-- nlink               ^-- size                   ^-- atim                              ^-- mtim                              ^-- ctim
//		                                                                 |
//		                                                                 +-- filetype
//
// The following properties of filestat are not implemented:
//   - dev: not supported by Golang FS
//   - ino: not supported by Golang FS
//   - nlink: not supported by Golang FS, we use 1
//   - atime: not supported by Golang FS, we use mtim for this
//   - ctim: not supported by Golang FS, we use mtim for this
//
// Note: This is similar to `fstat` in POSIX.
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_filestat_getfd-fd---errno-filestat
// and https://linux.die.net/man/3/fstat
var fdFilestatGet = newHostFunc(wasip1.FdFilestatGetName, fdFilestatGetFn, []api.ValueType{i32, i32}, "fd", "result.filestat")

// fdFilestatGetFn cannot currently use proxyResultParams because filestat is
// larger than api.ValueTypeI64 (i64 == 8 bytes, but filestat is 64).
func fdFilestatGetFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	return fdFilestatGetFunc(mod, int32(params[0]), uint32(params[1]))
}

func fdFilestatGetFunc(mod api.Module, fd int32, resultBuf uint32) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	// Ensure we can write the filestat
	buf, ok := mod.Memory().Read(resultBuf, 64)
	if !ok {
		return experimentalsys.EFAULT
	}

	f, ok := fsc.LookupFile(fd)
	if !ok {
		return experimentalsys.EBADF
	}

	st, errno := f.File.Stat()
	if errno != 0 {
		return errno
	}

	filetype := getExtendedWasiFiletype(f.File, st.Mode)
	return writeFilestat(buf, &st, filetype)
}

func getExtendedWasiFiletype(file experimentalsys.File, fm fs.FileMode) (ftype uint8) {
	ftype = getWasiFiletype(fm)
	if ftype == wasip1.FILETYPE_UNKNOWN {
		if _, ok := file.(socketapi.TCPSock); ok {
			ftype = wasip1.FILETYPE_SOCKET_STREAM
		} else if _, ok = file.(socketapi.TCPConn); ok {
			ftype = wasip1.FILETYPE_SOCKET_STREAM
		}
	}
	return
}

func getWasiFiletype(fm fs.FileMode) uint8 {
	switch {
	case fm.IsRegular():
		return wasip1.FILETYPE_REGULAR_FILE
	case fm.IsDir():
		return wasip1.FILETYPE_DIRECTORY
	case fm&fs.ModeSymlink != 0:
		return wasip1.FILETYPE_SYMBOLIC_LINK
	case fm&fs.ModeDevice != 0:
		// Unlike ModeDevice and ModeCharDevice, FILETYPE_CHARACTER_DEVICE and
		// FILETYPE_BLOCK_DEVICE are set mutually exclusively.
		if fm&fs.ModeCharDevice != 0 {
			return wasip1.FILETYPE_CHARACTER_DEVICE
		}
		return wasip1.FILETYPE_BLOCK_DEVICE
	default: // unknown
		return wasip1.FILETYPE_UNKNOWN
	}
}

func writeFilestat(buf []byte, st *sysapi.Stat_t, ftype uint8) (errno experimentalsys.Errno) {
	le.PutUint64(buf, st.Dev)
	le.PutUint64(buf[8:], st.Ino)
	le.PutUint64(buf[16:], uint64(ftype))
	le.PutUint64(buf[24:], st.Nlink)
	le.PutUint64(buf[32:], uint64(st.Size))
	le.PutUint64(buf[40:], uint64(st.Atim))
	le.PutUint64(buf[48:], uint64(st.Mtim))
	le.PutUint64(buf[56:], uint64(st.Ctim))
	return
}

// fdFilestatSetSize is the WASI function named FdFilestatSetSizeName which
// adjusts the size of an open file.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_filestat_set_sizefd-fd-size-filesize---errno
var fdFilestatSetSize = newHostFunc(wasip1.FdFilestatSetSizeName, fdFilestatSetSizeFn, []wasm.ValueType{i32, i64}, "fd", "size")

func fdFilestatSetSizeFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fd := int32(params[0])
	size := int64(params[1])

	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	// Check to see if the file descriptor is available
	if f, ok := fsc.LookupFile(fd); !ok {
		return experimentalsys.EBADF
	} else {
		return f.File.Truncate(size)
	}
}

// fdFilestatSetTimes is the WASI function named functionFdFilestatSetTimes
// which adjusts the times of an open file.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_filestat_set_timesfd-fd-atim-timestamp-mtim-timestamp-fst_flags-fstflags---errno
var fdFilestatSetTimes = newHostFunc(
	wasip1.FdFilestatSetTimesName, fdFilestatSetTimesFn,
	[]wasm.ValueType{i32, i64, i64, i32},
	"fd", "atim", "mtim", "fst_flags",
)

func fdFilestatSetTimesFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fd := int32(params[0])
	atim := int64(params[1])
	mtim := int64(params[2])
	fstFlags := uint16(params[3])

	sys := mod.(*wasm.ModuleInstance).Sys
	fsc := sys.FS()

	f, ok := fsc.LookupFile(fd)
	if !ok {
		return experimentalsys.EBADF
	}

	atim, mtim, errno := toTimes(sys.WalltimeNanos, atim, mtim, fstFlags)
	if errno != 0 {
		return errno
	}

	// Try to update the file timestamps by file-descriptor.
	errno = f.File.Utimens(atim, mtim)

	// Fall back to path based, despite it being less precise.
	switch errno {
	case experimentalsys.EPERM, experimentalsys.ENOSYS:
		errno = f.FS.Utimens(f.Name, atim, mtim)
	}

	return errno
}

func toTimes(walltime func() int64, atim, mtim int64, fstFlags uint16) (int64, int64, experimentalsys.Errno) {
	// times[0] == atim, times[1] == mtim

	var nowTim int64

	// coerce atim into a timespec
	if set, now := fstFlags&wasip1.FstflagsAtim != 0, fstFlags&wasip1.FstflagsAtimNow != 0; set && now {
		return 0, 0, experimentalsys.EINVAL
	} else if set {
		// atim is already correct
	} else if now {
		nowTim = walltime()
		atim = nowTim
	} else {
		atim = experimentalsys.UTIME_OMIT
	}

	// coerce mtim into a timespec
	if set, now := fstFlags&wasip1.FstflagsMtim != 0, fstFlags&wasip1.FstflagsMtimNow != 0; set && now {
		return 0, 0, experimentalsys.EINVAL
	} else if set {
		// mtim is already correct
	} else if now {
		if nowTim != 0 {
			mtim = nowTim
		} else {
			mtim = walltime()
		}
	} else {
		mtim = experimentalsys.UTIME_OMIT
	}
	return atim, mtim, 0
}

// fdPread is the WASI function named FdPreadName which reads from a file
// descriptor, without using and updating the file descriptor's offset.
//
// Except for handling offset, this implementation is identical to fdRead.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_preadfd-fd-iovs-iovec_array-offset-filesize---errno-size
var fdPread = newHostFunc(
	wasip1.FdPreadName, fdPreadFn,
	[]api.ValueType{i32, i32, i32, i64, i32},
	"fd", "iovs", "iovs_len", "offset", "result.nread",
)

func fdPreadFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	return fdReadOrPread(mod, params, true)
}

// fdPrestatGet is the WASI function named FdPrestatGetName which returns
// the prestat data of a file descriptor.
//
// # Parameters
//
//   - fd: file descriptor to get the prestat
//   - resultPrestat: offset to write the result prestat data
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid or the `fd` is not a pre-opened directory
//   - sys.EFAULT: `resultPrestat` points to an offset out of memory
//
// prestat byte layout is 8 bytes, beginning with an 8-bit tag and 3 pad bytes.
// The only valid tag is `prestat_dir`, which is tag zero. This simplifies the
// byte layout to 4 empty bytes followed by the uint32le encoded path length.
//
// For example, the directory name corresponding with `fd` was "/tmp" and
// parameter resultPrestat=1, this function writes the below to api.Memory:
//
//	                   padding   uint32le
//	        uint8 --+  +-----+  +--------+
//	                |  |     |  |        |
//	      []byte{?, 0, 0, 0, 0, 4, 0, 0, 0, ?}
//	resultPrestat --^           ^
//	          tag --+           |
//	                            +-- size in bytes of the string "/tmp"
//
// See fdPrestatDirName and
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#prestat
var fdPrestatGet = newHostFunc(wasip1.FdPrestatGetName, fdPrestatGetFn, []api.ValueType{i32, i32}, "fd", "result.prestat")

func fdPrestatGetFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()
	fd, resultPrestat := int32(params[0]), uint32(params[1])

	name, errno := preopenPath(fsc, fd)
	if errno != 0 {
		return errno
	}

	// Upper 32-bits are zero because...
	// * Zero-value 8-bit tag, and 3-byte zero-value padding
	prestat := uint64(len(name) << 32)
	if !mod.Memory().WriteUint64Le(resultPrestat, prestat) {
		return experimentalsys.EFAULT
	}
	return 0
}

// fdPrestatDirName is the WASI function named FdPrestatDirNameName which
// returns the path of the pre-opened directory of a file descriptor.
//
// # Parameters
//
//   - fd: file descriptor to get the path of the pre-opened directory
//   - path: offset in api.Memory to write the result path
//   - pathLen: count of bytes to write to `path`
//   - This should match the uint32le fdPrestatGet writes to offset
//     `resultPrestat`+4
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.EFAULT: `path` points to an offset out of memory
//   - sys.ENAMETOOLONG: `pathLen` is longer than the actual length of the result
//
// For example, the directory name corresponding with `fd` was "/tmp" and
// # Parameters path=1 pathLen=4 (correct), this function will write the below to
// api.Memory:
//
//	               pathLen
//	           +--------------+
//	           |              |
//	[]byte{?, '/', 't', 'm', 'p', ?}
//	    path --^
//
// See fdPrestatGet
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_prestat_dir_name
var fdPrestatDirName = newHostFunc(
	wasip1.FdPrestatDirNameName, fdPrestatDirNameFn,
	[]api.ValueType{i32, i32, i32},
	"fd", "result.path", "result.path_len",
)

func fdPrestatDirNameFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()
	fd, path, pathLen := int32(params[0]), uint32(params[1]), uint32(params[2])

	name, errno := preopenPath(fsc, fd)
	if errno != 0 {
		return errno
	}

	// Some runtimes may have another semantics. See /RATIONALE.md
	if uint32(len(name)) < pathLen {
		return experimentalsys.ENAMETOOLONG
	}

	if !mod.Memory().Write(path, []byte(name)[:pathLen]) {
		return experimentalsys.EFAULT
	}
	return 0
}

// fdPwrite is the WASI function named FdPwriteName which writes to a file
// descriptor, without using and updating the file descriptor's offset.
//
// Except for handling offset, this implementation is identical to fdWrite.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_pwritefd-fd-iovs-ciovec_array-offset-filesize---errno-size
var fdPwrite = newHostFunc(
	wasip1.FdPwriteName, fdPwriteFn,
	[]api.ValueType{i32, i32, i32, i64, i32},
	"fd", "iovs", "iovs_len", "offset", "result.nwritten",
)

func fdPwriteFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	return fdWriteOrPwrite(mod, params, true)
}

// fdRead is the WASI function named FdReadName which reads from a file
// descriptor.
//
// # Parameters
//
//   - fd: an opened file descriptor to read data from
//   - iovs: offset in api.Memory to read offset, size pairs representing where
//     to write file data
//   - Both offset and length are encoded as uint32le
//   - iovsCount: count of memory offset, size pairs to read sequentially
//     starting at iovs
//   - resultNread: offset in api.Memory to write the number of bytes read
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.EFAULT: `iovs` or `resultNread` point to an offset out of memory
//   - sys.EIO: a file system error
//
// For example, this function needs to first read `iovs` to determine where
// to write contents. If parameters iovs=1 iovsCount=2, this function reads two
// offset/length pairs from api.Memory:
//
//	                  iovs[0]                  iovs[1]
//	          +---------------------+   +--------------------+
//	          | uint32le    uint32le|   |uint32le    uint32le|
//	          +---------+  +--------+   +--------+  +--------+
//	          |         |  |        |   |        |  |        |
//	[]byte{?, 18, 0, 0, 0, 4, 0, 0, 0, 23, 0, 0, 0, 2, 0, 0, 0, ?... }
//	   iovs --^            ^            ^           ^
//	          |            |            |           |
//	 offset --+   length --+   offset --+  length --+
//
// If the contents of the `fd` parameter was "wazero" (6 bytes) and parameter
// resultNread=26, this function writes the below to api.Memory:
//
//	                    iovs[0].length        iovs[1].length
//	                   +--------------+       +----+       uint32le
//	                   |              |       |    |      +--------+
//	[]byte{ 0..16, ?, 'w', 'a', 'z', 'e', ?, 'r', 'o', ?, 6, 0, 0, 0 }
//	  iovs[0].offset --^                      ^           ^
//	                         iovs[1].offset --+           |
//	                                        resultNread --+
//
// Note: This is similar to `readv` in POSIX. https://linux.die.net/man/3/readv
//
// See fdWrite
// and https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_read
var fdRead = newHostFunc(
	wasip1.FdReadName, fdReadFn,
	[]api.ValueType{i32, i32, i32, i32},
	"fd", "iovs", "iovs_len", "result.nread",
)

// preader tracks an offset across multiple reads.
type preader struct {
	f      experimentalsys.File
	offset int64
}

// Read implements the same function as documented on sys.File.
func (w *preader) Read(buf []byte) (n int, errno experimentalsys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // less overhead on zero-length reads.
	}

	n, err := w.f.Pread(buf, w.offset)
	w.offset += int64(n)
	return n, err
}

func fdReadFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	return fdReadOrPread(mod, params, false)
}

func fdReadOrPread(mod api.Module, params []uint64, isPread bool) experimentalsys.Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	iovs := uint32(params[1])
	iovsCount := uint32(params[2])

	var resultNread uint32
	var reader func(buf []byte) (n int, errno experimentalsys.Errno)
	if f, ok := fsc.LookupFile(fd); !ok {
		return experimentalsys.EBADF
	} else if isPread {
		offset := int64(params[3])
		reader = (&preader{f: f.File, offset: offset}).Read
		resultNread = uint32(params[4])
	} else {
		reader = f.File.Read
		resultNread = uint32(params[3])
	}

	nread, errno := readv(mem, iovs, iovsCount, reader)
	if errno != 0 {
		return errno
	}
	if !mem.WriteUint32Le(resultNread, nread) {
		return experimentalsys.EFAULT
	} else {
		return 0
	}
}

func readv(mem api.Memory, iovs uint32, iovsCount uint32, reader func(buf []byte) (nread int, errno experimentalsys.Errno)) (uint32, experimentalsys.Errno) {
	var nread uint32
	iovsStop := iovsCount << 3 // iovsCount * 8
	iovsBuf, ok := mem.Read(iovs, iovsStop)
	if !ok {
		return 0, experimentalsys.EFAULT
	}

	for iovsPos := uint32(0); iovsPos < iovsStop; iovsPos += 8 {
		offset := le.Uint32(iovsBuf[iovsPos:])
		l := le.Uint32(iovsBuf[iovsPos+4:])

		if l == 0 { // A zero length iovec could be ahead of another.
			continue
		}

		b, ok := mem.Read(offset, l)
		if !ok {
			return 0, experimentalsys.EFAULT
		}

		n, errno := reader(b)
		nread += uint32(n)

		if errno == experimentalsys.ENOSYS {
			return 0, experimentalsys.EBADF // e.g. unimplemented for read
		} else if errno != 0 {
			return 0, errno
		} else if n < int(l) {
			break // stop when we read less than capacity.
		}
	}
	return nread, 0
}

// fdReaddir is the WASI function named wasip1.FdReaddirName which reads
// directory entries from a directory.  Special behaviors required by this
// function are implemented in sys.DirentCache.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_readdirfd-fd-buf-pointeru8-buf_len-size-cookie-dircookie---errno-size
//
// # Result (Errno)
//
// The return value is 0 except the following known error conditions:
//   - sys.ENOSYS: the implementation does not support this function.
//   - sys.EBADF: the file was closed or not a directory.
//   - sys.EFAULT: `buf` or `buf_len` point to an offset out of memory.
//   - sys.ENOENT: `cookie` was invalid.
//   - sys.EINVAL: `buf_len` was not large enough to write a dirent header.
//
// # End of Directory (EOF)
//
// More entries are available when `result.bufused` == `buf_len`. See
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_readdir
// https://github.com/WebAssembly/wasi-libc/blob/659ff414560721b1660a19685110e484a081c3d4/libc-bottom-half/cloudlibc/src/libc/dirent/readdir.c#L44
var fdReaddir = newHostFunc(
	wasip1.FdReaddirName, fdReaddirFn,
	[]wasm.ValueType{i32, i32, i32, i64, i32},
	"fd", "buf", "buf_len", "cookie", "result.bufused",
)

func fdReaddirFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	buf := uint32(params[1])
	bufLen := uint32(params[2])
	cookie := params[3]
	resultBufused := uint32(params[4])

	// The bufLen must be enough to write a dirent header.
	if bufLen < wasip1.DirentSize {
		// This is a bug in the caller, as unless `buf_len` is large enough to
		// write a dirent, it can't read the `d_namlen` from it.
		return experimentalsys.EINVAL
	}

	// Get or open a dirent cache for this file descriptor.
	dir, errno := direntCache(fsc, fd)
	if errno != 0 {
		return errno
	}

	// First, determine the maximum directory entries that can be encoded as
	// dirents. The total size is DirentSize(24) + nameSize, for each file.
	// Since a zero-length file name is invalid, the minimum size entry is
	// 25 (DirentSize + 1 character).
	maxDirEntries := bufLen/wasip1.DirentSize + 1

	// While unlikely maxDirEntries will fit into bufLen, add one more just in
	// case, as we need to know if we hit the end of the directory or not to
	// write the correct bufused (e.g. == bufLen unless EOF).
	//	>> If less than the size of the read buffer, the end of the
	//	>> directory has been reached.
	maxDirEntries += 1

	// Read up to max entries. The underlying implementation will cache these,
	// starting at the current location, so that they can be re-read. This is
	// important because even the first could end up larger than bufLen due to
	// the size of its name.
	dirents, errno := dir.Read(cookie, maxDirEntries)
	if errno != 0 {
		return errno
	}

	// Determine how many dirents we can write, including a potentially
	// truncated last entry.
	bufToWrite, direntCount, truncatedLen := maxDirents(dirents, bufLen)

	// Now, write entries to the underlying buffer.
	if bufToWrite > 0 {

		// d_next is the index of the next file in the list, so it should
		// always be one higher than the requested cookie.
		d_next := cookie + 1
		// ^^ yes this can overflow to negative, which means our implementation
		// doesn't support writing greater than max int64 entries.

		buf, ok := mem.Read(buf, bufToWrite)
		if !ok {
			return experimentalsys.EFAULT
		}

		writeDirents(buf, dirents, d_next, direntCount, truncatedLen)
	}

	// bufused == bufLen means more dirents exist, which is the case when one
	// is truncated.
	bufused := bufToWrite
	if truncatedLen > 0 {
		bufused = bufLen
	}

	if !mem.WriteUint32Le(resultBufused, bufused) {
		return experimentalsys.EFAULT
	}
	return 0
}

const largestDirent = int64(math.MaxUint32 - wasip1.DirentSize)

// maxDirents returns the dirents to write.
//
// `bufToWrite` is the amount of memory needed to write direntCount, which
// includes up to wasip1.DirentSize of a last truncated entry.
func maxDirents(dirents []experimentalsys.Dirent, bufLen uint32) (bufToWrite uint32, direntCount int, truncatedLen uint32) {
	lenRemaining := bufLen
	for i := range dirents {
		if lenRemaining == 0 {
			break
		}
		d := dirents[i]
		direntCount++

		// use int64 to guard against huge filenames
		nameLen := int64(len(d.Name))
		var entryLen uint32

		// Check to see if DirentSize + nameLen overflows, or if it would be
		// larger than possible to encode.
		if el := int64(wasip1.DirentSize) + nameLen; el < 0 || el > largestDirent {
			// panic, as testing is difficult. ex we would have to extract a
			// function to get size of a string or allocate a 2^32 size one!
			panic("invalid filename: too large")
		} else { // we know this can fit into a uint32
			entryLen = uint32(el)
		}

		if entryLen > lenRemaining {
			// We haven't room to write the entry, and docs say to write the
			// header. This helps especially when there is an entry with a very
			// long filename. Ex if bufLen is 4096 and the filename is 4096,
			// we need to write DirentSize(24) + 4096 bytes to write the entry.
			// In this case, we only write up to DirentSize(24) to allow the
			// caller to resize.
			if lenRemaining >= wasip1.DirentSize {
				truncatedLen = wasip1.DirentSize
			} else {
				truncatedLen = lenRemaining
			}
			bufToWrite += truncatedLen
			break
		}

		// This won't go negative because we checked entryLen <= lenRemaining.
		lenRemaining -= entryLen
		bufToWrite += entryLen
	}
	return
}

// writeDirents writes the directory entries to the buffer, which is pre-sized
// based on maxDirents.	truncatedEntryLen means the last is written without its
// name.
func writeDirents(buf []byte, dirents []experimentalsys.Dirent, d_next uint64, direntCount int, truncatedLen uint32) {
	pos := uint32(0)
	skipNameI := -1

	// If the last entry was truncated, we either skip it or write it without
	// its name, depending on the length.
	if truncatedLen > 0 {
		if truncatedLen < wasip1.DirentSize {
			direntCount-- // skip as too small to write the header.
		} else {
			skipNameI = direntCount - 1 // write the header, but not the name.
		}
	}

	for i := 0; i < direntCount; i++ {
		e := dirents[i]
		nameLen := uint32(len(e.Name))
		writeDirent(buf[pos:], d_next, e.Ino, nameLen, e.Type)
		d_next++
		pos += wasip1.DirentSize

		if i != skipNameI {
			copy(buf[pos:], e.Name)
			pos += nameLen
		}
	}
}

// writeDirent writes DirentSize bytes
func writeDirent(buf []byte, dNext uint64, ino sysapi.Inode, dNamlen uint32, dType fs.FileMode) {
	le.PutUint64(buf, dNext)        // d_next
	le.PutUint64(buf[8:], ino)      // d_ino
	le.PutUint32(buf[16:], dNamlen) // d_namlen
	filetype := getWasiFiletype(dType)
	le.PutUint32(buf[20:], uint32(filetype)) //  d_type
}

// direntCache lazy opens a sys.DirentCache for this directory or returns an
// error.
func direntCache(fsc *sys.FSContext, fd int32) (*sys.DirentCache, experimentalsys.Errno) {
	if f, ok := fsc.LookupFile(fd); !ok {
		return nil, experimentalsys.EBADF
	} else if dir, errno := f.DirentCache(); errno == 0 {
		return dir, 0
	} else if errno == experimentalsys.ENOTDIR {
		// fd_readdir docs don't indicate whether to return sys.ENOTDIR or
		// sys.EBADF. It has been noticed that rust will crash on sys.ENOTDIR,
		// and POSIX C ref seems to not return this, so we don't either.
		//
		// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_readdir
		// and https://en.wikibooks.org/wiki/C_Programming/POSIX_Reference/dirent.h
		return nil, experimentalsys.EBADF
	} else {
		return nil, errno
	}
}

// fdRenumber is the WASI function named FdRenumberName which atomically
// replaces a file descriptor by renumbering another file descriptor.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_renumberfd-fd-to-fd---errno
var fdRenumber = newHostFunc(wasip1.FdRenumberName, fdRenumberFn, []wasm.ValueType{i32, i32}, "fd", "to")

func fdRenumberFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	from := int32(params[0])
	to := int32(params[1])

	if errno := fsc.Renumber(from, to); errno != 0 {
		return errno
	}
	return 0
}

// fdSeek is the WASI function named FdSeekName which moves the offset of a
// file descriptor.
//
// # Parameters
//
//   - fd: file descriptor to move the offset of
//   - offset: signed int64, which is encoded as uint64, input argument to
//     `whence`, which results in a new offset
//   - whence: operator that creates the new offset, given `offset` bytes
//   - If io.SeekStart, new offset == `offset`.
//   - If io.SeekCurrent, new offset == existing offset + `offset`.
//   - If io.SeekEnd, new offset == file size of `fd` + `offset`.
//   - resultNewoffset: offset in api.Memory to write the new offset to,
//     relative to start of the file
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.EFAULT: `resultNewoffset` points to an offset out of memory
//   - sys.EINVAL: `whence` is an invalid value
//   - sys.EIO: a file system error
//   - sys.EISDIR: the file was a directory.
//
// For example, if fd 3 is a file with offset 0, and parameters fd=3, offset=4,
// whence=0 (=io.SeekStart), resultNewOffset=1, this function writes the below
// to api.Memory:
//
//	                         uint64le
//	                  +--------------------+
//	                  |                    |
//	        []byte{?, 4, 0, 0, 0, 0, 0, 0, 0, ? }
//	resultNewoffset --^
//
// Note: This is similar to `lseek` in POSIX. https://linux.die.net/man/3/lseek
//
// See io.Seeker
// and https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_seek
var fdSeek = newHostFunc(
	wasip1.FdSeekName, fdSeekFn,
	[]api.ValueType{i32, i64, i32, i32},
	"fd", "offset", "whence", "result.newoffset",
)

func fdSeekFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()
	fd := int32(params[0])
	offset := params[1]
	whence := uint32(params[2])
	resultNewoffset := uint32(params[3])

	if f, ok := fsc.LookupFile(fd); !ok {
		return experimentalsys.EBADF
	} else if isDir, _ := f.File.IsDir(); isDir {
		return experimentalsys.EISDIR // POSIX doesn't forbid seeking a directory, but wasi-testsuite does.
	} else if newOffset, errno := f.File.Seek(int64(offset), int(whence)); errno != 0 {
		return errno
	} else if !mod.Memory().WriteUint64Le(resultNewoffset, uint64(newOffset)) {
		return experimentalsys.EFAULT
	}
	return 0
}

// fdSync is the WASI function named FdSyncName which synchronizes the data
// and metadata of a file to disk.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_syncfd-fd---errno
var fdSync = newHostFunc(wasip1.FdSyncName, fdSyncFn, []api.ValueType{i32}, "fd")

func fdSyncFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()
	fd := int32(params[0])

	// Check to see if the file descriptor is available
	if f, ok := fsc.LookupFile(fd); !ok {
		return experimentalsys.EBADF
	} else {
		return f.File.Sync()
	}
}

// fdTell is the WASI function named FdTellName which returns the current
// offset of a file descriptor.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_tellfd-fd---errno-filesize
var fdTell = newHostFunc(wasip1.FdTellName, fdTellFn, []api.ValueType{i32, i32}, "fd", "result.offset")

func fdTellFn(ctx context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fd := params[0]
	offset := uint64(0)
	whence := uint64(io.SeekCurrent)
	resultNewoffset := params[1]

	fdSeekParams := []uint64{fd, offset, whence, resultNewoffset}
	return fdSeekFn(ctx, mod, fdSeekParams)
}

// fdWrite is the WASI function named FdWriteName which writes to a file
// descriptor.
//
// # Parameters
//
//   - fd: an opened file descriptor to write data to
//   - iovs: offset in api.Memory to read offset, size pairs representing the
//     data to write to `fd`
//   - Both offset and length are encoded as uint32le.
//   - iovsCount: count of memory offset, size pairs to read sequentially
//     starting at iovs
//   - resultNwritten: offset in api.Memory to write the number of bytes
//     written
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.EFAULT: `iovs` or `resultNwritten` point to an offset out of memory
//   - sys.EIO: a file system error
//
// For example, this function needs to first read `iovs` to determine what to
// write to `fd`. If parameters iovs=1 iovsCount=2, this function reads two
// offset/length pairs from api.Memory:
//
//	                  iovs[0]                  iovs[1]
//	          +---------------------+   +--------------------+
//	          | uint32le    uint32le|   |uint32le    uint32le|
//	          +---------+  +--------+   +--------+  +--------+
//	          |         |  |        |   |        |  |        |
//	[]byte{?, 18, 0, 0, 0, 4, 0, 0, 0, 23, 0, 0, 0, 2, 0, 0, 0, ?... }
//	   iovs --^            ^            ^           ^
//	          |            |            |           |
//	 offset --+   length --+   offset --+  length --+
//
// This function reads those chunks api.Memory into the `fd` sequentially.
//
//	                    iovs[0].length        iovs[1].length
//	                   +--------------+       +----+
//	                   |              |       |    |
//	[]byte{ 0..16, ?, 'w', 'a', 'z', 'e', ?, 'r', 'o', ? }
//	  iovs[0].offset --^                      ^
//	                         iovs[1].offset --+
//
// Since "wazero" was written, if parameter resultNwritten=26, this function
// writes the below to api.Memory:
//
//	                   uint32le
//	                  +--------+
//	                  |        |
//	[]byte{ 0..24, ?, 6, 0, 0, 0', ? }
//	 resultNwritten --^
//
// Note: This is similar to `writev` in POSIX. https://linux.die.net/man/3/writev
//
// See fdRead
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#ciovec
// and https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_write
var fdWrite = newHostFunc(
	wasip1.FdWriteName, fdWriteFn,
	[]api.ValueType{i32, i32, i32, i32},
	"fd", "iovs", "iovs_len", "result.nwritten",
)

func fdWriteFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	return fdWriteOrPwrite(mod, params, false)
}

// pwriter tracks an offset across multiple writes.
type pwriter struct {
	f      experimentalsys.File
	offset int64
}

// Write implements the same function as documented on sys.File.
func (w *pwriter) Write(buf []byte) (n int, errno experimentalsys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // less overhead on zero-length writes.
	}

	n, err := w.f.Pwrite(buf, w.offset)
	w.offset += int64(n)
	return n, err
}

func fdWriteOrPwrite(mod api.Module, params []uint64, isPwrite bool) experimentalsys.Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	iovs := uint32(params[1])
	iovsCount := uint32(params[2])

	var resultNwritten uint32
	var writer func(buf []byte) (n int, errno experimentalsys.Errno)
	if f, ok := fsc.LookupFile(fd); !ok {
		return experimentalsys.EBADF
	} else if isPwrite {
		offset := int64(params[3])
		writer = (&pwriter{f: f.File, offset: offset}).Write
		resultNwritten = uint32(params[4])
	} else {
		writer = f.File.Write
		resultNwritten = uint32(params[3])
	}

	nwritten, errno := writev(mem, iovs, iovsCount, writer)
	if errno != 0 {
		return errno
	}

	if !mod.Memory().WriteUint32Le(resultNwritten, nwritten) {
		return experimentalsys.EFAULT
	}
	return 0
}

func writev(mem api.Memory, iovs uint32, iovsCount uint32, writer func(buf []byte) (n int, errno experimentalsys.Errno)) (uint32, experimentalsys.Errno) {
	var nwritten uint32
	iovsStop := iovsCount << 3 // iovsCount * 8
	iovsBuf, ok := mem.Read(iovs, iovsStop)
	if !ok {
		return 0, experimentalsys.EFAULT
	}

	for iovsPos := uint32(0); iovsPos < iovsStop; iovsPos += 8 {
		offset := le.Uint32(iovsBuf[iovsPos:])
		l := le.Uint32(iovsBuf[iovsPos+4:])

		b, ok := mem.Read(offset, l)
		if !ok {
			return 0, experimentalsys.EFAULT
		}
		n, errno := writer(b)
		nwritten += uint32(n)
		if errno == experimentalsys.ENOSYS {
			return 0, experimentalsys.EBADF // e.g. unimplemented for write
		} else if errno != 0 {
			return 0, errno
		}
	}
	return nwritten, 0
}

// pathCreateDirectory is the WASI function named PathCreateDirectoryName which
// creates a directory.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `path` is relative to
//   - path: offset in api.Memory to read the path string from
//   - pathLen: length of `path`
//
// # Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.ENOENT: `path` does not exist.
//   - sys.ENOTDIR: `path` is a file
//
// # Notes
//   - This is similar to mkdirat in POSIX.
//     See https://linux.die.net/man/2/mkdirat
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_create_directoryfd-fd-path-string---errno
var pathCreateDirectory = newHostFunc(
	wasip1.PathCreateDirectoryName, pathCreateDirectoryFn,
	[]wasm.ValueType{i32, i32, i32},
	"fd", "path", "path_len",
)

func pathCreateDirectoryFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	path := uint32(params[1])
	pathLen := uint32(params[2])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), fd, path, pathLen)
	if errno != 0 {
		return errno
	}

	if errno = preopen.Mkdir(pathName, 0o700); errno != 0 {
		return errno
	}

	return 0
}

// pathFilestatGet is the WASI function named PathFilestatGetName which
// returns the stat attributes of a file or directory.
//
// # Parameters
//
//   - fd: file descriptor of the folder to look in for the path
//   - flags: flags determining the method of how paths are resolved
//   - path: path under fd to get the filestat attributes data for
//   - path_len: length of the path that was given
//   - resultFilestat: offset to write the result filestat data
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.ENOTDIR: `fd` points to a file not a directory
//   - sys.EIO: could not stat `fd` on filesystem
//   - sys.EINVAL: the path contained "../"
//   - sys.ENAMETOOLONG: `path` + `path_len` is out of memory
//   - sys.EFAULT: `resultFilestat` points to an offset out of memory
//   - sys.ENOENT: could not find the path
//
// The rest of this implementation matches that of fdFilestatGet, so is not
// repeated here.
//
// Note: This is similar to `fstatat` in POSIX.
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_filestat_getfd-fd-flags-lookupflags-path-string---errno-filestat
// and https://linux.die.net/man/2/fstatat
var pathFilestatGet = newHostFunc(
	wasip1.PathFilestatGetName, pathFilestatGetFn,
	[]api.ValueType{i32, i32, i32, i32, i32},
	"fd", "flags", "path", "path_len", "result.filestat",
)

func pathFilestatGetFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	flags := uint16(params[1])
	path := uint32(params[2])
	pathLen := uint32(params[3])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), fd, path, pathLen)
	if errno != 0 {
		return errno
	}

	// Stat the file without allocating a file descriptor.
	var st sysapi.Stat_t

	if (flags & wasip1.LOOKUP_SYMLINK_FOLLOW) == 0 {
		st, errno = preopen.Lstat(pathName)
	} else {
		st, errno = preopen.Stat(pathName)
	}
	if errno != 0 {
		return errno
	}

	// Write the stat result to memory
	resultBuf := uint32(params[4])
	buf, ok := mod.Memory().Read(resultBuf, 64)
	if !ok {
		return experimentalsys.EFAULT
	}

	filetype := getWasiFiletype(st.Mode)
	return writeFilestat(buf, &st, filetype)
}

// pathFilestatSetTimes is the WASI function named PathFilestatSetTimesName
// which adjusts the timestamps of a file or directory.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_filestat_set_timesfd-fd-flags-lookupflags-path-string-atim-timestamp-mtim-timestamp-fst_flags-fstflags---errno
var pathFilestatSetTimes = newHostFunc(
	wasip1.PathFilestatSetTimesName, pathFilestatSetTimesFn,
	[]wasm.ValueType{i32, i32, i32, i32, i64, i64, i32},
	"fd", "flags", "path", "path_len", "atim", "mtim", "fst_flags",
)

func pathFilestatSetTimesFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fd := int32(params[0])
	flags := uint16(params[1])
	path := uint32(params[2])
	pathLen := uint32(params[3])
	atim := int64(params[4])
	mtim := int64(params[5])
	fstFlags := uint16(params[6])

	sys := mod.(*wasm.ModuleInstance).Sys
	fsc := sys.FS()

	atim, mtim, errno := toTimes(sys.WalltimeNanos, atim, mtim, fstFlags)
	if errno != 0 {
		return errno
	}

	preopen, pathName, errno := atPath(fsc, mod.Memory(), fd, path, pathLen)
	if errno != 0 {
		return errno
	}

	symlinkFollow := flags&wasip1.LOOKUP_SYMLINK_FOLLOW != 0
	if symlinkFollow {
		return preopen.Utimens(pathName, atim, mtim)
	}
	// Otherwise, we need to emulate don't follow by opening the file by path.
	if f, errno := preopen.OpenFile(pathName, experimentalsys.O_WRONLY, 0); errno != 0 {
		return errno
	} else {
		defer f.Close()
		return f.Utimens(atim, mtim)
	}
}

// pathLink is the WASI function named PathLinkName which adjusts the
// timestamps of a file or directory.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#path_link
var pathLink = newHostFunc(
	wasip1.PathLinkName, pathLinkFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32, i32, i32},
	"old_fd", "old_flags", "old_path", "old_path_len", "new_fd", "new_path", "new_path_len",
)

func pathLinkFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	oldFD := int32(params[0])
	// TODO: use old_flags?
	_ = uint32(params[1])
	oldPath := uint32(params[2])
	oldPathLen := uint32(params[3])

	oldFS, oldName, errno := atPath(fsc, mem, oldFD, oldPath, oldPathLen)
	if errno != 0 {
		return errno
	}

	newFD := int32(params[4])
	newPath := uint32(params[5])
	newPathLen := uint32(params[6])

	newFS, newName, errno := atPath(fsc, mem, newFD, newPath, newPathLen)
	if errno != 0 {
		return errno
	}

	if oldFS != newFS { // TODO: handle link across filesystems
		return experimentalsys.ENOSYS
	}

	return oldFS.Link(oldName, newName)
}

// pathOpen is the WASI function named PathOpenName which opens a file or
// directory. This returns sys.EBADF if the fd is invalid.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `path` is relative to
//   - dirflags: flags to indicate how to resolve `path`
//   - path: offset in api.Memory to read the path string from
//   - pathLen: length of `path`
//   - oFlags: open flags to indicate the method by which to open the file
//   - fsRightsBase: interpret RIGHT_FD_WRITE to set O_RDWR
//   - fsRightsInheriting: ignored as rights were removed from WASI.
//     created file descriptor for `path`
//   - fdFlags: file descriptor flags
//   - resultOpenedFD: offset in api.Memory to write the newly created file
//     descriptor to.
//   - The result FD value is guaranteed to be less than 2**31
//
// Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.EFAULT: `resultOpenedFD` points to an offset out of memory
//   - sys.ENOENT: `path` does not exist.
//   - sys.EEXIST: `path` exists, while `oFlags` requires that it must not.
//   - sys.ENOTDIR: `path` is not a directory, while `oFlags` requires it.
//   - sys.EIO: a file system error
//
// For example, this function needs to first read `path` to determine the file
// to open. If parameters `path` = 1, `pathLen` = 6, and the path is "wazero",
// pathOpen reads the path from api.Memory:
//
//	                pathLen
//	            +------------------------+
//	            |                        |
//	[]byte{ ?, 'w', 'a', 'z', 'e', 'r', 'o', ?... }
//	     path --^
//
// Then, if parameters resultOpenedFD = 8, and this function opened a new file
// descriptor 5 with the given flags, this function writes the below to
// api.Memory:
//
//	                  uint32le
//	                 +--------+
//	                 |        |
//	[]byte{ 0..6, ?, 5, 0, 0, 0, ?}
//	resultOpenedFD --^
//
// # Notes
//   - This is similar to `openat` in POSIX. https://linux.die.net/man/3/openat
//   - The returned file descriptor is not guaranteed to be the lowest-number
//
// See https://github.com/WebAssembly/WASI/blob/main/phases/snapshot/docs.md#path_open
var pathOpen = newHostFunc(
	wasip1.PathOpenName, pathOpenFn,
	[]api.ValueType{i32, i32, i32, i32, i32, i64, i64, i32, i32},
	"fd", "dirflags", "path", "path_len", "oflags", "fs_rights_base", "fs_rights_inheriting", "fdflags", "result.opened_fd",
)

func pathOpenFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	preopenFD := int32(params[0])

	// TODO: dirflags is a lookupflags, and it only has one bit: symlink_follow
	// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#lookupflags
	dirflags := uint16(params[1])

	path := uint32(params[2])
	pathLen := uint32(params[3])

	oflags := uint16(params[4])

	rights := uint32(params[5])
	// inherited rights aren't used
	_ = params[6]

	fdflags := uint16(params[7])
	resultOpenedFD := uint32(params[8])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), preopenFD, path, pathLen)
	if errno != 0 {
		return errno
	}

	if pathLen == 0 {
		return experimentalsys.EINVAL
	}

	fileOpenFlags := openFlags(dirflags, oflags, fdflags, rights)
	isDir := fileOpenFlags&experimentalsys.O_DIRECTORY != 0

	if isDir && oflags&wasip1.O_CREAT != 0 {
		return experimentalsys.EINVAL // use pathCreateDirectory!
	}

	newFD, errno := fsc.OpenFile(preopen, pathName, fileOpenFlags, 0o600)
	if errno != 0 {
		return errno
	}

	// Check any flags that require the file to evaluate.
	if isDir {
		if f, ok := fsc.LookupFile(newFD); !ok {
			return experimentalsys.EBADF // unexpected
		} else if isDir, errno := f.File.IsDir(); errno != 0 {
			_ = fsc.CloseFile(newFD)
			return errno
		} else if !isDir {
			_ = fsc.CloseFile(newFD)
			return experimentalsys.ENOTDIR
		}
	}

	if !mod.Memory().WriteUint32Le(resultOpenedFD, uint32(newFD)) {
		_ = fsc.CloseFile(newFD)
		return experimentalsys.EFAULT
	}
	return 0
}

// atPath returns the pre-open specific path after verifying it is a directory.
//
// # Notes
//
// Languages including Zig and Rust use only pre-opens for the FD because
// wasi-libc `__wasilibc_find_relpath` will only return a preopen. That said,
// our wasi.c example shows other languages act differently and can use a non
// pre-opened file descriptor.
//
// We don't handle `AT_FDCWD`, as that's resolved in the compiler. There's no
// working directory function in WASI, so most assume CWD is "/". Notably, Zig
// has different behavior which assumes it is whatever the first pre-open name
// is.
//
// See https://github.com/WebAssembly/wasi-libc/blob/659ff414560721b1660a19685110e484a081c3d4/libc-bottom-half/sources/at_fdcwd.c
// See https://linux.die.net/man/2/openat
func atPath(fsc *sys.FSContext, mem api.Memory, fd int32, p, pathLen uint32) (experimentalsys.FS, string, experimentalsys.Errno) {
	b, ok := mem.Read(p, pathLen)
	if !ok {
		return nil, "", experimentalsys.EFAULT
	}
	pathName := string(b)

	// interesting_paths wants us to break on trailing slash if the input ends
	// up a file, not a directory!
	hasTrailingSlash := strings.HasSuffix(pathName, "/")

	// interesting_paths includes paths that include relative links but end up
	// not escaping
	pathName = path.Clean(pathName)

	// interesting_paths wants to break on root paths or anything that escapes.
	// This part is the same as fs.FS.Open()
	if !fs.ValidPath(pathName) {
		return nil, "", experimentalsys.EPERM
	}

	// add the trailing slash back
	if hasTrailingSlash {
		pathName = pathName + "/"
	}

	if f, ok := fsc.LookupFile(fd); !ok {
		return nil, "", experimentalsys.EBADF // closed or invalid
	} else if isDir, errno := f.File.IsDir(); errno != 0 {
		return nil, "", errno
	} else if !isDir {
		return nil, "", experimentalsys.ENOTDIR
	} else if f.IsPreopen { // don't append the pre-open name
		return f.FS, pathName, 0
	} else {
		// Join via concat to avoid name conflict on path.Join
		return f.FS, f.Name + "/" + pathName, 0
	}
}

func preopenPath(fsc *sys.FSContext, fd int32) (string, experimentalsys.Errno) {
	if f, ok := fsc.LookupFile(fd); !ok {
		return "", experimentalsys.EBADF // closed
	} else if !f.IsPreopen {
		return "", experimentalsys.EBADF
	} else if isDir, errno := f.File.IsDir(); errno != 0 || !isDir {
		// In wasip1, only directories can be returned by fd_prestat_get as
		// there are no prestat types defined for files or sockets.
		return "", errno
	} else {
		return f.Name, 0
	}
}

func openFlags(dirflags, oflags, fdflags uint16, rights uint32) (openFlags experimentalsys.Oflag) {
	if dirflags&wasip1.LOOKUP_SYMLINK_FOLLOW == 0 {
		openFlags |= experimentalsys.O_NOFOLLOW
	}
	if oflags&wasip1.O_DIRECTORY != 0 {
		openFlags |= experimentalsys.O_DIRECTORY
	} else if oflags&wasip1.O_EXCL != 0 {
		openFlags |= experimentalsys.O_EXCL
	}
	// Because we don't implement rights, we partially rely on the open flags
	// to determine the mode in which the file will be opened. This will create
	// divergent behavior compared to WASI runtimes which have a more strict
	// interpretation of the WASI capabilities model; for example, a program
	// which sets O_CREAT but does not give read or write permissions will
	// successfully create a file when running with wazero, but might get a
	// permission denied error on other runtimes.
	defaultMode := experimentalsys.O_RDONLY
	if oflags&wasip1.O_TRUNC != 0 {
		openFlags |= experimentalsys.O_TRUNC
		defaultMode = experimentalsys.O_RDWR
	}
	if oflags&wasip1.O_CREAT != 0 {
		openFlags |= experimentalsys.O_CREAT
		defaultMode = experimentalsys.O_RDWR
	}
	if fdflags&wasip1.FD_NONBLOCK != 0 {
		openFlags |= experimentalsys.O_NONBLOCK
	}
	if fdflags&wasip1.FD_APPEND != 0 {
		openFlags |= experimentalsys.O_APPEND
		defaultMode = experimentalsys.O_RDWR
	}
	if fdflags&wasip1.FD_DSYNC != 0 {
		openFlags |= experimentalsys.O_DSYNC
	}
	if fdflags&wasip1.FD_RSYNC != 0 {
		openFlags |= experimentalsys.O_RSYNC
	}
	if fdflags&wasip1.FD_SYNC != 0 {
		openFlags |= experimentalsys.O_SYNC
	}

	// Since rights were discontinued in wasi, we only interpret RIGHT_FD_WRITE
	// because it is the only way to know that we need to set write permissions
	// on a file if the application did not pass any of O_CREAT, O_APPEND, nor
	// O_TRUNC.
	const r = wasip1.RIGHT_FD_READ
	const w = wasip1.RIGHT_FD_WRITE
	const rw = r | w
	switch {
	case (rights & rw) == rw:
		openFlags |= experimentalsys.O_RDWR
	case (rights & w) == w:
		openFlags |= experimentalsys.O_WRONLY
	case (rights & r) == r:
		openFlags |= experimentalsys.O_RDONLY
	default:
		openFlags |= defaultMode
	}
	return
}

// pathReadlink is the WASI function named PathReadlinkName that reads the
// contents of a symbolic link.
//
// See: https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_readlinkfd-fd-path-string-buf-pointeru8-buf_len-size---errno-size
var pathReadlink = newHostFunc(
	wasip1.PathReadlinkName, pathReadlinkFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32, i32},
	"fd", "path", "path_len", "buf", "buf_len", "result.bufused",
)

func pathReadlinkFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	path := uint32(params[1])
	pathLen := uint32(params[2])
	buf := uint32(params[3])
	bufLen := uint32(params[4])
	resultBufused := uint32(params[5])

	if pathLen == 0 || bufLen == 0 {
		return experimentalsys.EINVAL
	}

	mem := mod.Memory()
	preopen, p, errno := atPath(fsc, mem, fd, path, pathLen)
	if errno != 0 {
		return errno
	}

	dst, errno := preopen.Readlink(p)
	if errno != 0 {
		return errno
	}

	if len(dst) > int(bufLen) {
		return experimentalsys.ERANGE
	}

	if ok := mem.WriteString(buf, dst); !ok {
		return experimentalsys.EFAULT
	}

	if !mem.WriteUint32Le(resultBufused, uint32(len(dst))) {
		return experimentalsys.EFAULT
	}
	return 0
}

// pathRemoveDirectory is the WASI function named PathRemoveDirectoryName which
// removes a directory.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `path` is relative to
//   - path: offset in api.Memory to read the path string from
//   - pathLen: length of `path`
//
// # Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.ENOENT: `path` does not exist.
//   - sys.ENOTEMPTY: `path` is not empty
//   - sys.ENOTDIR: `path` is a file
//
// # Notes
//   - This is similar to unlinkat with AT_REMOVEDIR in POSIX.
//     See https://linux.die.net/man/2/unlinkat
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_remove_directoryfd-fd-path-string---errno
var pathRemoveDirectory = newHostFunc(
	wasip1.PathRemoveDirectoryName, pathRemoveDirectoryFn,
	[]wasm.ValueType{i32, i32, i32},
	"fd", "path", "path_len",
)

func pathRemoveDirectoryFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	path := uint32(params[1])
	pathLen := uint32(params[2])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), fd, path, pathLen)
	if errno != 0 {
		return errno
	}

	return preopen.Rmdir(pathName)
}

// pathRename is the WASI function named PathRenameName which renames a file or
// directory.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `old_path` is relative to
//   - old_path: offset in api.Memory to read the old path string from
//   - old_path_len: length of `old_path`
//   - new_fd: file descriptor of a directory that `new_path` is relative to
//   - new_path: offset in api.Memory to read the new path string from
//   - new_path_len: length of `new_path`
//
// # Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` or `new_fd` are invalid
//   - sys.ENOENT: `old_path` does not exist.
//   - sys.ENOTDIR: `old` is a directory and `new` exists, but is a file.
//   - sys.EISDIR: `old` is a file and `new` exists, but is a directory.
//
// # Notes
//   - This is similar to unlinkat in POSIX.
//     See https://linux.die.net/man/2/renameat
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_renamefd-fd-old_path-string-new_fd-fd-new_path-string---errno
var pathRename = newHostFunc(
	wasip1.PathRenameName, pathRenameFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32, i32},
	"fd", "old_path", "old_path_len", "new_fd", "new_path", "new_path_len",
)

func pathRenameFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	oldPath := uint32(params[1])
	oldPathLen := uint32(params[2])

	newFD := int32(params[3])
	newPath := uint32(params[4])
	newPathLen := uint32(params[5])

	oldFS, oldPathName, errno := atPath(fsc, mod.Memory(), fd, oldPath, oldPathLen)
	if errno != 0 {
		return errno
	}

	newFS, newPathName, errno := atPath(fsc, mod.Memory(), newFD, newPath, newPathLen)
	if errno != 0 {
		return errno
	}

	if oldFS != newFS { // TODO: handle renames across filesystems
		return experimentalsys.ENOSYS
	}

	return oldFS.Rename(oldPathName, newPathName)
}

// pathSymlink is the WASI function named PathSymlinkName which creates a
// symbolic link.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#path_symlink
var pathSymlink = newHostFunc(
	wasip1.PathSymlinkName, pathSymlinkFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32},
	"old_path", "old_path_len", "fd", "new_path", "new_path_len",
)

func pathSymlinkFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	oldPath := uint32(params[0])
	oldPathLen := uint32(params[1])
	fd := int32(params[2])
	newPath := uint32(params[3])
	newPathLen := uint32(params[4])

	mem := mod.Memory()

	dir, ok := fsc.LookupFile(fd)
	if !ok {
		return experimentalsys.EBADF // closed
	} else if isDir, errno := dir.File.IsDir(); errno != 0 {
		return errno
	} else if !isDir {
		return experimentalsys.ENOTDIR
	}

	if oldPathLen == 0 || newPathLen == 0 {
		return experimentalsys.EINVAL
	}

	oldPathBuf, ok := mem.Read(oldPath, oldPathLen)
	if !ok {
		return experimentalsys.EFAULT
	}

	_, newPathName, errno := atPath(fsc, mod.Memory(), fd, newPath, newPathLen)
	if errno != 0 {
		return errno
	}

	return dir.FS.Symlink(
		// Do not join old path since it's only resolved when dereference the link created here.
		// And the dereference result depends on the opening directory's file descriptor at that point.
		unsafe.String(&oldPathBuf[0], int(oldPathLen)),
		newPathName,
	)
}

// pathUnlinkFile is the WASI function named PathUnlinkFileName which unlinks a
// file.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `path` is relative to
//   - path: offset in api.Memory to read the path string from
//   - pathLen: length of `path`
//
// # Result (Errno)
//
// The return value is 0 except the following error conditions:
//   - sys.EBADF: `fd` is invalid
//   - sys.ENOENT: `path` does not exist.
//   - sys.EISDIR: `path` is a directory
//
// # Notes
//   - This is similar to unlinkat without AT_REMOVEDIR in POSIX.
//     See https://linux.die.net/man/2/unlinkat
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_unlink_filefd-fd-path-string---errno
var pathUnlinkFile = newHostFunc(
	wasip1.PathUnlinkFileName, pathUnlinkFileFn,
	[]wasm.ValueType{i32, i32, i32},
	"fd", "path", "path_len",
)

func pathUnlinkFileFn(_ context.Context, mod api.Module, params []uint64) experimentalsys.Errno {
	fsc := mod.(*wasm.ModuleInstance).Sys.FS()

	fd := int32(params[0])
	path := uint32(params[1])
	pathLen := uint32(params[2])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), fd, path, pathLen)
	if errno != 0 {
		return errno
	}

	return preopen.Unlink(pathName)
}
