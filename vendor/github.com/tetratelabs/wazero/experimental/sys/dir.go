package sys

import (
	"fmt"
	"io/fs"

	"github.com/tetratelabs/wazero/sys"
)

// FileType is fs.FileMode masked on fs.ModeType. For example, zero is a
// regular file, fs.ModeDir is a directory and fs.ModeIrregular is unknown.
//
// Note: This is defined by Linux, not POSIX.
type FileType = fs.FileMode

// Dirent is an entry read from a directory via File.Readdir.
//
// # Notes
//
//   - This extends `dirent` defined in POSIX with some fields defined by
//     Linux. See https://man7.org/linux/man-pages/man3/readdir.3.html and
//     https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/dirent.h.html
//   - This has a subset of fields defined in sys.Stat_t. Notably, there is no
//     field corresponding to Stat_t.Dev because that value will be constant
//     for all files in a directory. To get the Dev value, call File.Stat on
//     the directory File.Readdir was called on.
type Dirent struct {
	// Ino is the file serial number, or zero if not available. See Ino for
	// more details including impact returning a zero value.
	Ino sys.Inode

	// Name is the base name of the directory entry. Empty is invalid.
	Name string

	// Type is fs.FileMode masked on fs.ModeType. For example, zero is a
	// regular file, fs.ModeDir is a directory and fs.ModeIrregular is unknown.
	//
	// Note: This is defined by Linux, not POSIX.
	Type fs.FileMode
}

func (d *Dirent) String() string {
	return fmt.Sprintf("name=%s, type=%v, ino=%d", d.Name, d.Type, d.Ino)
}

// IsDir returns true if the Type is fs.ModeDir.
func (d *Dirent) IsDir() bool {
	return d.Type == fs.ModeDir
}

// DirFile is embeddable to reduce the amount of functions to implement a file.
type DirFile struct{}

// IsAppend implements File.IsAppend
func (DirFile) IsAppend() bool {
	return false
}

// SetAppend implements File.SetAppend
func (DirFile) SetAppend(bool) Errno {
	return EISDIR
}

// IsDir implements File.IsDir
func (DirFile) IsDir() (bool, Errno) {
	return true, 0
}

// Read implements File.Read
func (DirFile) Read([]byte) (int, Errno) {
	return 0, EISDIR
}

// Pread implements File.Pread
func (DirFile) Pread([]byte, int64) (int, Errno) {
	return 0, EISDIR
}

// Write implements File.Write
func (DirFile) Write([]byte) (int, Errno) {
	return 0, EISDIR
}

// Pwrite implements File.Pwrite
func (DirFile) Pwrite([]byte, int64) (int, Errno) {
	return 0, EISDIR
}

// Truncate implements File.Truncate
func (DirFile) Truncate(int64) Errno {
	return EISDIR
}
