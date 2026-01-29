package sys

import (
	"io"
	"io/fs"
	"net"

	"github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/descriptor"
	"github.com/tetratelabs/wazero/internal/fsapi"
	socketapi "github.com/tetratelabs/wazero/internal/sock"
	"github.com/tetratelabs/wazero/internal/sysfs"
)

const (
	FdStdin int32 = iota
	FdStdout
	FdStderr
	// FdPreopen is the file descriptor of the first pre-opened directory.
	//
	// # Why file descriptor 3?
	//
	// While not specified, the most common WASI implementation, wasi-libc,
	// expects POSIX style file descriptor allocation, where the lowest
	// available number is used to open the next file. Since 1 and 2 are taken
	// by stdout and stderr, the next is 3.
	//   - https://github.com/WebAssembly/WASI/issues/122
	//   - https://pubs.opengroup.org/onlinepubs/9699919799/functions/V2_chap02.html#tag_15_14
	//   - https://github.com/WebAssembly/wasi-libc/blob/wasi-sdk-16/libc-bottom-half/sources/preopens.c#L215
	FdPreopen
)

const modeDevice = fs.ModeDevice | 0o640

// FileEntry maps a path to an open file in a file system.
type FileEntry struct {
	// Name is the name of the directory up to its pre-open, or the pre-open
	// name itself when IsPreopen.
	//
	// # Notes
	//
	//   - This can drift on rename.
	//   - This relates to the guest path, which is not the real file path
	//     except if the entire host filesystem was made available.
	Name string

	// IsPreopen is a directory that is lazily opened.
	IsPreopen bool

	// FS is the filesystem associated with the pre-open.
	FS sys.FS

	// File is always non-nil.
	File fsapi.File

	// direntCache is nil until DirentCache was called.
	direntCache *DirentCache
}

// DirentCache gets or creates a DirentCache for this file or returns an error.
//
// # Errors
//
// A zero sys.Errno is success. The below are expected otherwise:
//   - sys.ENOSYS: the implementation does not support this function.
//   - sys.EBADF: the dir was closed or not readable.
//   - sys.ENOTDIR: the file was not a directory.
//
// # Notes
//
//   - See /RATIONALE.md for design notes.
func (f *FileEntry) DirentCache() (*DirentCache, sys.Errno) {
	if dir := f.direntCache; dir != nil {
		return dir, 0
	}

	// Require the file to be a directory vs a late error on the same.
	if isDir, errno := f.File.IsDir(); errno != 0 {
		return nil, errno
	} else if !isDir {
		return nil, sys.ENOTDIR
	}

	// Generate the dotEntries only once.
	if dotEntries, errno := synthesizeDotEntries(f); errno != 0 {
		return nil, errno
	} else {
		f.direntCache = &DirentCache{f: f.File, dotEntries: dotEntries}
	}

	return f.direntCache, 0
}

// DirentCache is a caching abstraction of sys.File Readdir.
//
// This is special-cased for "wasi_snapshot_preview1.fd_readdir", and may be
// unneeded, or require changes, to support preview1 or preview2.
//   - The position of the dirents are serialized as `d_next`. For reasons
//     described below, any may need to be re-read. This accepts any positions
//     in the cache, rather than track the position of the last dirent.
//   - dot entries ("." and "..") must be returned. See /RATIONALE.md for why.
//   - An sys.Dirent Name is variable length, it could exceed memory size and
//     need to be re-read.
//   - Multiple dirents may be returned. It is more efficient to read from the
//     underlying file in bulk vs one-at-a-time.
//
// The last results returned by Read are cached, but entries before that
// position are not. This support re-reading entries that couldn't fit into
// memory without accidentally caching all entries in a large directory. This
// approach is sometimes called a sliding window.
type DirentCache struct {
	// f is the underlying file
	f sys.File

	// dotEntries are the "." and ".." entries added when the directory is
	// initialized.
	dotEntries []sys.Dirent

	// dirents are the potentially unread directory entries.
	//
	// Internal detail: nil is different from zero length. Zero length is an
	// exhausted directory (eof). nil means the re-read.
	dirents []sys.Dirent

	// countRead is the total count of dirents read since last rewind.
	countRead uint64

	// eof is true when the underlying file is at EOF. This avoids re-reading
	// the directory when it is exhausted. Entires in an exhausted directory
	// are not visible until it is rewound via calling Read with `pos==0`.
	eof bool
}

// synthesizeDotEntries generates a slice of the two elements "." and "..".
func synthesizeDotEntries(f *FileEntry) ([]sys.Dirent, sys.Errno) {
	dotIno, errno := f.File.Ino()
	if errno != 0 {
		return nil, errno
	}
	result := [2]sys.Dirent{}
	result[0] = sys.Dirent{Name: ".", Ino: dotIno, Type: fs.ModeDir}
	// See /RATIONALE.md for why we don't attempt to get an inode for ".." and
	// why in wasi-libc this won't fan-out either.
	result[1] = sys.Dirent{Name: "..", Ino: 0, Type: fs.ModeDir}
	return result[:], 0
}

// exhaustedDirents avoids allocating empty slices.
var exhaustedDirents = [0]sys.Dirent{}

// Read is similar to and returns the same errors as `Readdir` on sys.File.
// The main difference is this caches entries returned, resulting in multiple
// valid positions to read from.
//
// When zero, `pos` means rewind to the beginning of this directory. This
// implies a rewind (Seek to zero on the underlying sys.File), unless the
// initial entries are still cached.
//
// When non-zero, `pos` is the zero based index of all dirents returned since
// last rewind. Only entries beginning at `pos` are cached for subsequent
// calls. A non-zero `pos` before the cache returns sys.ENOENT for reasons
// described on DirentCache documentation.
//
// Up to `n` entries are cached and returned. When `n` exceeds the cache, the
// difference are read from the underlying sys.File via `Readdir`. EOF is
// when `len(dirents)` returned are less than `n`.
func (d *DirentCache) Read(pos uint64, n uint32) (dirents []sys.Dirent, errno sys.Errno) {
	switch {
	case pos > d.countRead: // farther than read or negative coerced to uint64.
		return nil, sys.ENOENT
	case pos == 0 && d.dirents != nil:
		// Rewind if we have already read entries. This allows us to see new
		// entries added after the directory was opened.
		if _, errno = d.f.Seek(0, io.SeekStart); errno != 0 {
			return
		}
		d.dirents = nil // dump cache
		d.countRead = 0
	}

	if n == 0 {
		return // special case no entries.
	}

	if d.dirents == nil {
		// Always populate dot entries, which makes min len(dirents) == 2.
		d.dirents = d.dotEntries
		d.countRead = 2
		d.eof = false

		if countToRead := int(n - 2); countToRead <= 0 {
			return
		} else if dirents, errno = d.f.Readdir(countToRead); errno != 0 {
			return
		} else if countRead := len(dirents); countRead > 0 {
			d.eof = countRead < countToRead
			d.dirents = append(d.dotEntries, dirents...)
			d.countRead += uint64(countRead)
		}

		return d.cachedDirents(n), 0
	}

	// Reset our cache to the first entry being read.
	cacheStart := d.countRead - uint64(len(d.dirents))
	if pos < cacheStart {
		// We don't currently allow reads before our cache because Seek(0) is
		// the only portable way. Doing otherwise requires skipping, which we
		// won't do unless wasi-testsuite starts requiring it. Implementing
		// this would allow re-reading a large directory, so care would be
		// needed to not buffer the entire directory in memory while skipping.
		errno = sys.ENOENT
		return
	} else if posInCache := pos - cacheStart; posInCache != 0 {
		if uint64(len(d.dirents)) == posInCache {
			// Avoid allocation re-slicing to zero length.
			d.dirents = exhaustedDirents[:]
		} else {
			d.dirents = d.dirents[posInCache:]
		}
	}

	// See if we need more entries.
	if countToRead := int(n) - len(d.dirents); countToRead > 0 && !d.eof {
		// Try to read more, which could fail.
		if dirents, errno = d.f.Readdir(countToRead); errno != 0 {
			return
		}

		// Append the next read entries if we weren't at EOF.
		if countRead := len(dirents); countRead > 0 {
			d.eof = countRead < countToRead
			d.dirents = append(d.dirents, dirents...)
			d.countRead += uint64(countRead)
		}
	}

	return d.cachedDirents(n), 0
}

// cachedDirents returns up to `n` dirents from the cache.
func (d *DirentCache) cachedDirents(n uint32) []sys.Dirent {
	direntCount := uint32(len(d.dirents))
	switch {
	case direntCount == 0:
		return nil
	case direntCount > n:
		return d.dirents[:n]
	}
	return d.dirents
}

type FSContext struct {
	// openedFiles is a map of file descriptor numbers (>=FdPreopen) to open files
	// (or directories) and defaults to empty.
	// TODO: This is unguarded, so not goroutine-safe!
	openedFiles FileTable
}

// FileTable is a specialization of the descriptor.Table type used to map file
// descriptors to file entries.
type FileTable = descriptor.Table[int32, *FileEntry]

// LookupFile returns a file if it is in the table.
func (c *FSContext) LookupFile(fd int32) (*FileEntry, bool) {
	return c.openedFiles.Lookup(fd)
}

// OpenFile opens the file into the table and returns its file descriptor.
// The result must be closed by CloseFile or Close.
func (c *FSContext) OpenFile(fs sys.FS, path string, flag sys.Oflag, perm fs.FileMode) (int32, sys.Errno) {
	if f, errno := fs.OpenFile(path, flag, perm); errno != 0 {
		return 0, errno
	} else {
		fe := &FileEntry{FS: fs, File: fsapi.Adapt(f)}
		if path == "/" || path == "." {
			fe.Name = ""
		} else {
			fe.Name = path
		}
		if newFD, ok := c.openedFiles.Insert(fe); !ok {
			return 0, sys.EBADF
		} else {
			return newFD, 0
		}
	}
}

// Renumber assigns the file pointed by the descriptor `from` to `to`.
func (c *FSContext) Renumber(from, to int32) sys.Errno {
	fromFile, ok := c.openedFiles.Lookup(from)
	if !ok || to < 0 {
		return sys.EBADF
	} else if fromFile.IsPreopen {
		return sys.ENOTSUP
	}

	// If toFile is already open, we close it to prevent windows lock issues.
	//
	// The doc is unclear and other implementations do nothing for already-opened To FDs.
	// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_renumberfd-fd-to-fd---errno
	// https://github.com/bytecodealliance/wasmtime/blob/main/crates/wasi-common/src/snapshots/preview_1.rs#L531-L546
	if toFile, ok := c.openedFiles.Lookup(to); ok {
		if toFile.IsPreopen {
			return sys.ENOTSUP
		}
		_ = toFile.File.Close()
	}

	c.openedFiles.Delete(from)
	if !c.openedFiles.InsertAt(fromFile, to) {
		return sys.EBADF
	}
	return 0
}

// SockAccept accepts a sock.TCPConn into the file table and returns its file
// descriptor.
func (c *FSContext) SockAccept(sockFD int32, nonblock bool) (int32, sys.Errno) {
	var sock socketapi.TCPSock
	if e, ok := c.LookupFile(sockFD); !ok || !e.IsPreopen {
		return 0, sys.EBADF // Not a preopen
	} else if sock, ok = e.File.(socketapi.TCPSock); !ok {
		return 0, sys.EBADF // Not a sock
	}

	conn, errno := sock.Accept()
	if errno != 0 {
		return 0, errno
	}

	fe := &FileEntry{File: fsapi.Adapt(conn)}

	if nonblock {
		if errno = fe.File.SetNonblock(true); errno != 0 {
			_ = conn.Close()
			return 0, errno
		}
	}

	if newFD, ok := c.openedFiles.Insert(fe); !ok {
		return 0, sys.EBADF
	} else {
		return newFD, 0
	}
}

// CloseFile returns any error closing the existing file.
func (c *FSContext) CloseFile(fd int32) (errno sys.Errno) {
	f, ok := c.openedFiles.Lookup(fd)
	if !ok {
		return sys.EBADF
	}
	if errno = f.File.Close(); errno != 0 {
		return errno
	}
	c.openedFiles.Delete(fd)
	return errno
}

// Close implements io.Closer
func (c *FSContext) Close() (err error) {
	// Close any files opened in this context
	c.openedFiles.Range(func(fd int32, entry *FileEntry) bool {
		if errno := entry.File.Close(); errno != 0 {
			err = errno // This means err returned == the last non-nil error.
		}
		return true
	})
	// A closed FSContext cannot be reused so clear the state.
	c.openedFiles = FileTable{}
	return
}

// InitFSContext initializes a FSContext with stdio streams and optional
// pre-opened filesystems and TCP listeners.
func (c *Context) InitFSContext(
	stdin io.Reader,
	stdout, stderr io.Writer,
	fs []sys.FS, guestPaths []string,
	tcpListeners []*net.TCPListener,
) (err error) {
	inFile, err := stdinFileEntry(stdin)
	if err != nil {
		return err
	}
	c.fsc.openedFiles.Insert(inFile)
	outWriter, err := stdioWriterFileEntry("stdout", stdout)
	if err != nil {
		return err
	}
	c.fsc.openedFiles.Insert(outWriter)
	errWriter, err := stdioWriterFileEntry("stderr", stderr)
	if err != nil {
		return err
	}
	c.fsc.openedFiles.Insert(errWriter)

	for i, f := range fs {
		guestPath := guestPaths[i]

		if StripPrefixesAndTrailingSlash(guestPath) == "" {
			// Default to bind to '/' when guestPath is effectively empty.
			guestPath = "/"
		}
		c.fsc.openedFiles.Insert(&FileEntry{
			FS:        f,
			Name:      guestPath,
			IsPreopen: true,
			File:      &lazyDir{fs: f},
		})
	}

	for _, tl := range tcpListeners {
		c.fsc.openedFiles.Insert(&FileEntry{IsPreopen: true, File: fsapi.Adapt(sysfs.NewTCPListenerFile(tl))})
	}
	return nil
}

// StripPrefixesAndTrailingSlash skips any leading "./" or "/" such that the
// result index begins with another string. A result of "." coerces to the
// empty string "" because the current directory is handled by the guest.
//
// Results are the offset/len pair which is an optimization to avoid re-slicing
// overhead, as this function is called for every path operation.
//
// Note: Relative paths should be handled by the guest, as that's what knows
// what the current directory is. However, paths that escape the current
// directory e.g. "../.." have been found in `tinygo test` and this
// implementation takes care to avoid it.
func StripPrefixesAndTrailingSlash(path string) string {
	// strip trailing slashes
	pathLen := len(path)
	for ; pathLen > 0 && path[pathLen-1] == '/'; pathLen-- {
	}

	pathI := 0
loop:
	for pathI < pathLen {
		switch path[pathI] {
		case '/':
			pathI++
		case '.':
			nextI := pathI + 1
			if nextI < pathLen && path[nextI] == '/' {
				pathI = nextI + 1
			} else if nextI == pathLen {
				pathI = nextI
			} else {
				break loop
			}
		default:
			break loop
		}
	}
	return path[pathI:pathLen]
}
