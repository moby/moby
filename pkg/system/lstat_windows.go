package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"syscall"
	"time"
)

const reparsePointNameSurrogate = 0x20000000

type fileInfo struct {
	data        os.FileInfo
	maskReparse bool
}

func (fi *fileInfo) Name() string { return fi.data.Name() }
func (fi *fileInfo) IsDir() bool  { return fi.data.Mode().IsDir() }

// Lstat calls os.Lstat to get a fileinfo interface back.
// This is then copied into our own locally defined structure.
func Lstat(path string) (*StatT, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	return fromStatT(&fi)
}

func (fi *fileInfo) Size() int64 {
	return fi.data.Size()
}

func (fi *fileInfo) Mode() (m os.FileMode) {
	m = fi.data.Mode()

	if fi.maskReparse {
		m &^= os.ModeSymlink
	}

	return m
}

func (fi *fileInfo) ModTime() time.Time {
	return fi.data.ModTime()
}

// Sys returns syscall.Win32FileAttributeData for file fs.
func (fi *fileInfo) Sys() interface{} {
	return fi.data.Sys()
}

func shouldMaskReparsePoint(name string, namep *uint16) (bool, error) {
	var fd syscall.Win32finddata

	h, err := syscall.FindFirstFile(namep, &fd)
	if err != nil {
		return false, &os.PathError{"FindFirstFile", name, err}
	}
	syscall.FindClose(h)

	fullpath := name
	if !isAbs(fullpath) {
		fullpath, err = syscall.FullPath(fullpath)
		if err != nil {
			return false, &os.PathError{"FullPath", fullpath, err}
		}
	}

	if fd.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		if fd.Reserved0&reparsePointNameSurrogate == 0 {
			return true, nil
		}
	}

	return false, nil
}

func isAbs(path string) (b bool) {
	v := volumeName(path)
	if v == "" {
		return false
	}
	path = path[len(v):]
	if path == "" {
		return false
	}
	return os.IsPathSeparator(path[0])
}

func volumeName(path string) (v string) {
	if len(path) < 2 {
		return ""
	}
	// with drive letter
	c := path[0]
	if path[1] == ':' &&
		('0' <= c && c <= '9' || 'a' <= c && c <= 'z' ||
			'A' <= c && c <= 'Z') {
		return path[:2]
	}
	// is it UNC
	if l := len(path); l >= 5 && os.IsPathSeparator(path[0]) && os.IsPathSeparator(path[1]) &&
		!os.IsPathSeparator(path[2]) && path[2] != '.' {
		// first, leading `\\` and next shouldn't be `\`. its server name.
		for n := 3; n < l-1; n++ {
			// second, next '\' shouldn't be repeated.
			if os.IsPathSeparator(path[n]) {
				n++
				// third, following something characters. its share name.
				if !os.IsPathSeparator(path[n]) {
					if path[n] == '.' {
						break
					}
					for ; n < l; n++ {
						if os.IsPathSeparator(path[n]) {
							break
						}
					}
					return path[:n]
				}
				break
			}
		}
	}
	return ""
}

// fixLongPath returns the extended-length (\\?\-prefixed) form of
// path when needed, in order to avoid the default 260 character file
// path limit imposed by Windows. If path is not easily converted to
// the extended-length form (for example, if path is a relative path
// or contains .. elements), or is short enough, fixLongPath returns
// path unmodified.
//
// See https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
func fixLongPath(path string) string {
	// Do nothing (and don't allocate) if the path is "short".
	// Empirically (at least on the Windows Server 2013 builder),
	// the kernel is arbitrarily okay with < 248 bytes. That
	// matches what the docs above say:
	// "When using an API to create a directory, the specified
	// path cannot be so long that you cannot append an 8.3 file
	// name (that is, the directory name cannot exceed MAX_PATH
	// minus 12)." Since MAX_PATH is 260, 260 - 12 = 248.
	//
	// The MSDN docs appear to say that a normal path that is 248 bytes long
	// will work; empirically the path must be less then 248 bytes long.
	if len(path) < 248 {
		// Don't fix. (This is how Go 1.7 and earlier worked,
		// not automatically generating the \\?\ form)
		return path
	}

	// The extended form begins with \\?\, as in
	// \\?\c:\windows\foo.txt or \\?\UNC\server\share\foo.txt.
	// The extended form disables evaluation of . and .. path
	// elements and disables the interpretation of / as equivalent
	// to \. The conversion here rewrites / to \ and elides
	// . elements as well as trailing or duplicate separators. For
	// simplicity it avoids the conversion entirely for relative
	// paths or paths containing .. elements. For now,
	// \\server\share paths are not converted to
	// \\?\UNC\server\share paths because the rules for doing so
	// are less well-specified.
	if len(path) >= 2 && path[:2] == `\\` {
		// Don't canonicalize UNC paths.
		return path
	}
	if !isAbs(path) {
		// Relative path
		return path
	}

	const prefix = `\\?`

	pathbuf := make([]byte, len(prefix)+len(path)+len(`\`))
	copy(pathbuf, prefix)
	n := len(path)
	r, w := 0, len(prefix)
	for r < n {
		switch {
		case os.IsPathSeparator(path[r]):
			// empty block
			r++
		case path[r] == '.' && (r+1 == n || os.IsPathSeparator(path[r+1])):
			// /./
			r++
		case r+1 < n && path[r] == '.' && path[r+1] == '.' && (r+2 == n || os.IsPathSeparator(path[r+2])):
			// /../ is currently unhandled
			return path
		default:
			pathbuf[w] = '\\'
			w++
			for ; r < n && !os.IsPathSeparator(path[r]); r++ {
				pathbuf[w] = path[r]
				w++
			}
		}
	}
	// A drive's root directory needs a trailing \
	if w == len(`\\?\c:`) {
		pathbuf[w] = '\\'
		w++
	}
	return string(pathbuf[:w])
}

// GetFileInfo takes a path to a file and returns
// an os.FileInfo interface type pertaining to that file.
//
// This interface masks reparse points that aren't symbolic
// links in order to ensure that such reparse points are
// not cracked open, since we lack the context to deal with
// them.
//
// Throws an error if the file does not exist

func GetFileInfo(path string) (os.FileInfo, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		namep, e := syscall.UTF16PtrFromString(fixLongPath(path))

		if e != nil {
			return nil, e
		}

		maskReparse, e := shouldMaskReparsePoint(path, namep)
		if e != nil {
			return nil, e
		}

		if maskReparse {
			var maskedFi fileInfo

			maskedFi.data = fi
			maskedFi.maskReparse = true

			return &maskedFi, nil
		}
	}

	return fi, nil
}
