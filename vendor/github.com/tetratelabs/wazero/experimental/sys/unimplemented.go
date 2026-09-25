package sys

import (
	"io/fs"

	"github.com/tetratelabs/wazero/sys"
)

// UnimplementedFS is an FS that returns ENOSYS for all functions,
// This should be embedded to have forward compatible implementations.
type UnimplementedFS struct{}

// OpenFile implements FS.OpenFile
func (UnimplementedFS) OpenFile(path string, flag Oflag, perm fs.FileMode) (File, Errno) {
	return nil, ENOSYS
}

// Lstat implements FS.Lstat
func (UnimplementedFS) Lstat(path string) (sys.Stat_t, Errno) {
	return sys.Stat_t{}, ENOSYS
}

// Stat implements FS.Stat
func (UnimplementedFS) Stat(path string) (sys.Stat_t, Errno) {
	return sys.Stat_t{}, ENOSYS
}

// Readlink implements FS.Readlink
func (UnimplementedFS) Readlink(path string) (string, Errno) {
	return "", ENOSYS
}

// Mkdir implements FS.Mkdir
func (UnimplementedFS) Mkdir(path string, perm fs.FileMode) Errno {
	return ENOSYS
}

// Chmod implements FS.Chmod
func (UnimplementedFS) Chmod(path string, perm fs.FileMode) Errno {
	return ENOSYS
}

// Rename implements FS.Rename
func (UnimplementedFS) Rename(from, to string) Errno {
	return ENOSYS
}

// Rmdir implements FS.Rmdir
func (UnimplementedFS) Rmdir(path string) Errno {
	return ENOSYS
}

// Link implements FS.Link
func (UnimplementedFS) Link(_, _ string) Errno {
	return ENOSYS
}

// Symlink implements FS.Symlink
func (UnimplementedFS) Symlink(_, _ string) Errno {
	return ENOSYS
}

// Unlink implements FS.Unlink
func (UnimplementedFS) Unlink(path string) Errno {
	return ENOSYS
}

// Utimens implements FS.Utimens
func (UnimplementedFS) Utimens(path string, atim, mtim int64) Errno {
	return ENOSYS
}

// UnimplementedFile is a File that returns ENOSYS for all functions,
// except where no-op are otherwise documented.
//
// This should be embedded to have forward compatible implementations.
type UnimplementedFile struct{}

// Dev implements File.Dev
func (UnimplementedFile) Dev() (uint64, Errno) {
	return 0, 0
}

// Ino implements File.Ino
func (UnimplementedFile) Ino() (sys.Inode, Errno) {
	return 0, 0
}

// IsDir implements File.IsDir
func (UnimplementedFile) IsDir() (bool, Errno) {
	return false, 0
}

// IsAppend implements File.IsAppend
func (UnimplementedFile) IsAppend() bool {
	return false
}

// SetAppend implements File.SetAppend
func (UnimplementedFile) SetAppend(bool) Errno {
	return ENOSYS
}

// Stat implements File.Stat
func (UnimplementedFile) Stat() (sys.Stat_t, Errno) {
	return sys.Stat_t{}, ENOSYS
}

// Read implements File.Read
func (UnimplementedFile) Read([]byte) (int, Errno) {
	return 0, ENOSYS
}

// Pread implements File.Pread
func (UnimplementedFile) Pread([]byte, int64) (int, Errno) {
	return 0, ENOSYS
}

// Seek implements File.Seek
func (UnimplementedFile) Seek(int64, int) (int64, Errno) {
	return 0, ENOSYS
}

// Readdir implements File.Readdir
func (UnimplementedFile) Readdir(int) (dirents []Dirent, errno Errno) {
	return nil, ENOSYS
}

// Write implements File.Write
func (UnimplementedFile) Write([]byte) (int, Errno) {
	return 0, ENOSYS
}

// Pwrite implements File.Pwrite
func (UnimplementedFile) Pwrite([]byte, int64) (int, Errno) {
	return 0, ENOSYS
}

// Truncate implements File.Truncate
func (UnimplementedFile) Truncate(int64) Errno {
	return ENOSYS
}

// Sync implements File.Sync
func (UnimplementedFile) Sync() Errno {
	return 0 // not ENOSYS
}

// Datasync implements File.Datasync
func (UnimplementedFile) Datasync() Errno {
	return 0 // not ENOSYS
}

// Utimens implements File.Utimens
func (UnimplementedFile) Utimens(int64, int64) Errno {
	return ENOSYS
}

// Close implements File.Close
func (UnimplementedFile) Close() (errno Errno) { return }
