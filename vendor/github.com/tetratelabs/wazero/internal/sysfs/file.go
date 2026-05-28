package sysfs

import (
	"io"
	"io/fs"
	"os"
	"time"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/fsapi"
	"github.com/tetratelabs/wazero/sys"
)

func NewStdioFile(stdin bool, f fs.File) (fsapi.File, error) {
	// Return constant stat, which has fake times, but keep the underlying
	// file mode. Fake times are needed to pass wasi-testsuite.
	// https://github.com/WebAssembly/wasi-testsuite/blob/af57727/tests/rust/src/bin/fd_filestat_get.rs#L1-L19
	var mode fs.FileMode
	if st, err := f.Stat(); err != nil {
		return nil, err
	} else {
		mode = st.Mode()
	}
	var flag experimentalsys.Oflag
	if stdin {
		flag = experimentalsys.O_RDONLY
	} else {
		flag = experimentalsys.O_WRONLY
	}
	var file fsapi.File
	if of, ok := f.(*os.File); ok {
		// This is ok because functions that need path aren't used by stdioFile
		file = newOsFile("", flag, 0, of)
	} else {
		file = &fsFile{file: f}
	}
	return &stdioFile{File: file, st: sys.Stat_t{Mode: mode, Nlink: 1}}, nil
}

func OpenFile(path string, flag experimentalsys.Oflag, perm fs.FileMode) (*os.File, experimentalsys.Errno) {
	return openFile(path, flag, perm)
}

func OpenOSFile(path string, flag experimentalsys.Oflag, perm fs.FileMode) (experimentalsys.File, experimentalsys.Errno) {
	f, errno := OpenFile(path, flag, perm)
	if errno != 0 {
		return nil, errno
	}
	return newOsFile(path, flag, perm, f), 0
}

func OpenFSFile(fs fs.FS, path string, flag experimentalsys.Oflag, perm fs.FileMode) (experimentalsys.File, experimentalsys.Errno) {
	if flag&experimentalsys.O_DIRECTORY != 0 && flag&(experimentalsys.O_WRONLY|experimentalsys.O_RDWR) != 0 {
		return nil, experimentalsys.EISDIR // invalid to open a directory writeable
	}
	f, err := fs.Open(path)
	if errno := experimentalsys.UnwrapOSError(err); errno != 0 {
		return nil, errno
	}
	// Don't return an os.File because the path is not absolute. osFile needs
	// the path to be real and certain FS.File impls are subrooted.
	return &fsFile{fs: fs, name: path, file: f}, 0
}

type stdioFile struct {
	fsapi.File
	st sys.Stat_t
}

// SetAppend implements File.SetAppend
func (f *stdioFile) SetAppend(bool) experimentalsys.Errno {
	// Ignore for stdio.
	return 0
}

// IsAppend implements File.SetAppend
func (f *stdioFile) IsAppend() bool {
	return true
}

// Stat implements File.Stat
func (f *stdioFile) Stat() (sys.Stat_t, experimentalsys.Errno) {
	return f.st, 0
}

// Close implements File.Close
func (f *stdioFile) Close() experimentalsys.Errno {
	return 0
}

// fsFile is used for wrapped fs.File, like os.Stdin or any fs.File
// implementation. Notably, this does not have access to the full file path.
// so certain operations can't be supported, such as inode lookups on Windows.
type fsFile struct {
	experimentalsys.UnimplementedFile

	// fs is the file-system that opened the file, or nil when wrapped for
	// pre-opens like stdio.
	fs fs.FS

	// name is what was used in fs for Open, so it may not be the actual path.
	name string

	// file is always set, possibly an os.File like os.Stdin.
	file fs.File

	// reopenDir is true if reopen should be called before Readdir. This flag
	// is deferred until Readdir to prevent redundant rewinds. This could
	// happen if Seek(0) was called twice, or if in Windows, Seek(0) was called
	// before Readdir.
	reopenDir bool

	// closed is true when closed was called. This ensures proper sys.EBADF
	closed bool

	// cachedStat includes fields that won't change while a file is open.
	cachedSt *cachedStat
}

type cachedStat struct {
	// dev is the same as sys.Stat_t Dev.
	dev uint64

	// dev is the same as sys.Stat_t Ino.
	ino sys.Inode

	// isDir is sys.Stat_t Mode masked with fs.ModeDir
	isDir bool
}

// cachedStat returns the cacheable parts of sys.Stat_t or an error if they
// couldn't be retrieved.
func (f *fsFile) cachedStat() (dev uint64, ino sys.Inode, isDir bool, errno experimentalsys.Errno) {
	if f.cachedSt == nil {
		if _, errno = f.Stat(); errno != 0 {
			return
		}
	}
	return f.cachedSt.dev, f.cachedSt.ino, f.cachedSt.isDir, 0
}

// Dev implements the same method as documented on sys.File
func (f *fsFile) Dev() (uint64, experimentalsys.Errno) {
	dev, _, _, errno := f.cachedStat()
	return dev, errno
}

// Ino implements the same method as documented on sys.File
func (f *fsFile) Ino() (sys.Inode, experimentalsys.Errno) {
	_, ino, _, errno := f.cachedStat()
	return ino, errno
}

// IsDir implements the same method as documented on sys.File
func (f *fsFile) IsDir() (bool, experimentalsys.Errno) {
	_, _, isDir, errno := f.cachedStat()
	return isDir, errno
}

// IsAppend implements the same method as documented on sys.File
func (f *fsFile) IsAppend() bool {
	return false
}

// SetAppend implements the same method as documented on sys.File
func (f *fsFile) SetAppend(bool) (errno experimentalsys.Errno) {
	return fileError(f, f.closed, experimentalsys.ENOSYS)
}

// Stat implements the same method as documented on sys.File
func (f *fsFile) Stat() (sys.Stat_t, experimentalsys.Errno) {
	if f.closed {
		return sys.Stat_t{}, experimentalsys.EBADF
	}

	st, errno := statFile(f.file)
	switch errno {
	case 0:
		f.cachedSt = &cachedStat{dev: st.Dev, ino: st.Ino, isDir: st.Mode&fs.ModeDir == fs.ModeDir}
	case experimentalsys.EIO:
		errno = experimentalsys.EBADF
	}
	return st, errno
}

// Read implements the same method as documented on sys.File
func (f *fsFile) Read(buf []byte) (n int, errno experimentalsys.Errno) {
	if n, errno = read(f.file, buf); errno != 0 {
		// Defer validation overhead until we've already had an error.
		errno = fileError(f, f.closed, errno)
	}
	return
}

// Pread implements the same method as documented on sys.File
func (f *fsFile) Pread(buf []byte, off int64) (n int, errno experimentalsys.Errno) {
	if ra, ok := f.file.(io.ReaderAt); ok {
		if n, errno = pread(ra, buf, off); errno != 0 {
			// Defer validation overhead until we've already had an error.
			errno = fileError(f, f.closed, errno)
		}
		return
	}

	// See /RATIONALE.md "fd_pread: io.Seeker fallback when io.ReaderAt is not supported"
	if rs, ok := f.file.(io.ReadSeeker); ok {
		// Determine the current position in the file, as we need to revert it.
		currentOffset, err := rs.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, fileError(f, f.closed, experimentalsys.UnwrapOSError(err))
		}

		// Put the read position back when complete.
		defer func() { _, _ = rs.Seek(currentOffset, io.SeekStart) }()

		// If the current offset isn't in sync with this reader, move it.
		if off != currentOffset {
			if _, err = rs.Seek(off, io.SeekStart); err != nil {
				return 0, fileError(f, f.closed, experimentalsys.UnwrapOSError(err))
			}
		}

		n, err = rs.Read(buf)
		if errno = experimentalsys.UnwrapOSError(err); errno != 0 {
			// Defer validation overhead until we've already had an error.
			errno = fileError(f, f.closed, errno)
		}
	} else {
		errno = experimentalsys.ENOSYS // unsupported
	}
	return
}

// Seek implements the same method as documented on sys.File
func (f *fsFile) Seek(offset int64, whence int) (newOffset int64, errno experimentalsys.Errno) {
	// If this is a directory, and we're attempting to seek to position zero,
	// we have to re-open the file to ensure the directory state is reset.
	var isDir bool
	if offset == 0 && whence == io.SeekStart {
		if isDir, errno = f.IsDir(); errno == 0 && isDir {
			f.reopenDir = true
			return
		}
	}

	if s, ok := f.file.(io.Seeker); ok {
		if newOffset, errno = seek(s, offset, whence); errno != 0 {
			// Defer validation overhead until we've already had an error.
			errno = fileError(f, f.closed, errno)
		}
	} else {
		errno = experimentalsys.ENOSYS // unsupported
	}
	return
}

// Readdir implements the same method as documented on sys.File
//
// Notably, this uses readdirFile or fs.ReadDirFile if available. This does not
// return inodes on windows.
func (f *fsFile) Readdir(n int) (dirents []experimentalsys.Dirent, errno experimentalsys.Errno) {
	// Windows lets you Readdir after close, FS.File also may not implement
	// close in a meaningful way. read our closed field to return consistent
	// results.
	if f.closed {
		errno = experimentalsys.EBADF
		return
	}

	if f.reopenDir { // re-open the directory if needed.
		f.reopenDir = false
		if errno = adjustReaddirErr(f, f.closed, f.rewindDir()); errno != 0 {
			return
		}
	}

	if of, ok := f.file.(readdirFile); ok {
		// We can't use f.name here because it is the path up to the sys.FS,
		// not necessarily the real path. For this reason, Windows may not be
		// able to populate inodes. However, Darwin and Linux will.
		if dirents, errno = readdir(of, "", n); errno != 0 {
			errno = adjustReaddirErr(f, f.closed, errno)
		}
		return
	}

	// Try with FS.ReadDirFile which is available on api.FS implementations
	// like embed:FS.
	if rdf, ok := f.file.(fs.ReadDirFile); ok {
		entries, e := rdf.ReadDir(n)
		if errno = adjustReaddirErr(f, f.closed, e); errno != 0 {
			return
		}
		dirents = make([]experimentalsys.Dirent, 0, len(entries))
		for _, e := range entries {
			// By default, we don't attempt to read inode data
			dirents = append(dirents, experimentalsys.Dirent{Name: e.Name(), Type: e.Type()})
		}
	} else {
		errno = experimentalsys.EBADF // not a directory
	}
	return
}

// Write implements the same method as documented on sys.File.
func (f *fsFile) Write(buf []byte) (n int, errno experimentalsys.Errno) {
	if w, ok := f.file.(io.Writer); ok {
		if n, errno = write(w, buf); errno != 0 {
			// Defer validation overhead until we've already had an error.
			errno = fileError(f, f.closed, errno)
		}
	} else {
		errno = experimentalsys.ENOSYS // unsupported
	}
	return
}

// Pwrite implements the same method as documented on sys.File.
func (f *fsFile) Pwrite(buf []byte, off int64) (n int, errno experimentalsys.Errno) {
	if wa, ok := f.file.(io.WriterAt); ok {
		if n, errno = pwrite(wa, buf, off); errno != 0 {
			// Defer validation overhead until we've already had an error.
			errno = fileError(f, f.closed, errno)
		}
	} else {
		errno = experimentalsys.ENOSYS // unsupported
	}
	return
}

// Close implements the same method as documented on sys.File.
func (f *fsFile) Close() experimentalsys.Errno {
	if f.closed {
		return 0
	}
	f.closed = true
	return f.close()
}

func (f *fsFile) close() experimentalsys.Errno {
	return experimentalsys.UnwrapOSError(f.file.Close())
}

// IsNonblock implements the same method as documented on fsapi.File
func (f *fsFile) IsNonblock() bool {
	return false
}

// SetNonblock implements the same method as documented on fsapi.File
func (f *fsFile) SetNonblock(bool) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Poll implements the same method as documented on fsapi.File
func (f *fsFile) Poll(fsapi.Pflag, int32) (ready bool, errno experimentalsys.Errno) {
	return false, experimentalsys.ENOSYS
}

// dirError is used for commands that work against a directory, but not a file.
func dirError(f experimentalsys.File, isClosed bool, errno experimentalsys.Errno) experimentalsys.Errno {
	if vErrno := validate(f, isClosed, false, true); vErrno != 0 {
		return vErrno
	}
	return errno
}

// fileError is used for commands that work against a file, but not a directory.
func fileError(f experimentalsys.File, isClosed bool, errno experimentalsys.Errno) experimentalsys.Errno {
	if vErrno := validate(f, isClosed, true, false); vErrno != 0 {
		return vErrno
	}
	return errno
}

// validate is used to making syscalls which will fail.
func validate(f experimentalsys.File, isClosed, wantFile, wantDir bool) experimentalsys.Errno {
	if isClosed {
		return experimentalsys.EBADF
	}

	isDir, errno := f.IsDir()
	if errno != 0 {
		return errno
	}

	if wantFile && isDir {
		return experimentalsys.EISDIR
	} else if wantDir && !isDir {
		return experimentalsys.ENOTDIR
	}
	return 0
}

func read(r io.Reader, buf []byte) (n int, errno experimentalsys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // less overhead on zero-length reads.
	}

	n, err := r.Read(buf)
	return n, experimentalsys.UnwrapOSError(err)
}

func pread(ra io.ReaderAt, buf []byte, off int64) (n int, errno experimentalsys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // less overhead on zero-length reads.
	}

	n, err := ra.ReadAt(buf, off)
	return n, experimentalsys.UnwrapOSError(err)
}

func seek(s io.Seeker, offset int64, whence int) (int64, experimentalsys.Errno) {
	if uint(whence) > io.SeekEnd {
		return 0, experimentalsys.EINVAL // negative or exceeds the largest valid whence
	}

	newOffset, err := s.Seek(offset, whence)
	return newOffset, experimentalsys.UnwrapOSError(err)
}

func (f *fsFile) rewindDir() experimentalsys.Errno {
	// Reopen the directory to rewind it.
	file, err := f.fs.Open(f.name)
	if err != nil {
		return experimentalsys.UnwrapOSError(err)
	}
	fi, err := file.Stat()
	if err != nil {
		return experimentalsys.UnwrapOSError(err)
	}
	// Can't check if it's still the same file,
	// but is it still a directory, at least?
	if !fi.IsDir() {
		return experimentalsys.ENOTDIR
	}
	// Only update f on success.
	_ = f.file.Close()
	f.file = file
	return 0
}

// readdirFile allows masking the `Readdir` function on os.File.
type readdirFile interface {
	Readdir(n int) ([]fs.FileInfo, error)
}

// readdir uses readdirFile.Readdir, special casing windows when path !="".
func readdir(f readdirFile, path string, n int) (dirents []experimentalsys.Dirent, errno experimentalsys.Errno) {
	fis, e := f.Readdir(n)
	if errno = experimentalsys.UnwrapOSError(e); errno != 0 {
		return
	}

	dirents = make([]experimentalsys.Dirent, 0, len(fis))

	// linux/darwin won't have to fan out to lstat, but windows will.
	var ino sys.Inode
	for fi := range fis {
		t := fis[fi]
		// inoFromFileInfo is more efficient than sys.NewStat_t, as it gets the
		// inode without allocating an instance and filling other fields.
		if ino, errno = inoFromFileInfo(path, t); errno != 0 {
			return
		}
		dirents = append(dirents, experimentalsys.Dirent{Name: t.Name(), Ino: ino, Type: t.Mode().Type()})
	}
	return
}

func write(w io.Writer, buf []byte) (n int, errno experimentalsys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // less overhead on zero-length writes.
	}

	n, err := w.Write(buf)
	return n, experimentalsys.UnwrapOSError(err)
}

func pwrite(w io.WriterAt, buf []byte, off int64) (n int, errno experimentalsys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // less overhead on zero-length writes.
	}

	n, err := w.WriteAt(buf, off)
	return n, experimentalsys.UnwrapOSError(err)
}

func chtimes(path string, atim, mtim int64) (errno experimentalsys.Errno) { //nolint:unused
	// When both inputs are omitted, there is nothing to change.
	if atim == experimentalsys.UTIME_OMIT && mtim == experimentalsys.UTIME_OMIT {
		return
	}

	// UTIME_OMIT is expensive until progress is made in Go, as it requires a
	// stat to read-back the value to re-apply.
	// - https://github.com/golang/go/issues/32558.
	// - https://go-review.googlesource.com/c/go/+/219638 (unmerged)
	var st sys.Stat_t
	if atim == experimentalsys.UTIME_OMIT || mtim == experimentalsys.UTIME_OMIT {
		if st, errno = stat(path); errno != 0 {
			return
		}
	}

	var atime, mtime time.Time
	if atim == experimentalsys.UTIME_OMIT {
		atime = epochNanosToTime(st.Atim)
		mtime = epochNanosToTime(mtim)
	} else if mtim == experimentalsys.UTIME_OMIT {
		atime = epochNanosToTime(atim)
		mtime = epochNanosToTime(st.Mtim)
	} else {
		atime = epochNanosToTime(atim)
		mtime = epochNanosToTime(mtim)
	}
	return experimentalsys.UnwrapOSError(os.Chtimes(path, atime, mtime))
}

func epochNanosToTime(epochNanos int64) time.Time { //nolint:unused
	seconds := epochNanos / 1e9
	nanos := epochNanos % 1e9
	return time.Unix(seconds, nanos)
}
