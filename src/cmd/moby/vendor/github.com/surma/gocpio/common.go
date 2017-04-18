// Package cpio implements access to cpio archives.
// Implementation of the new ASCII formate (SVR4) defined here:
// http://people.freebsd.org/~kientzle/libarchive/man/cpio.5.txt
package cpio

const (
	VERSION = "1.1.0"
)

// Header represents file meta data in an archive.
// Some fields may not be populated.
type Header struct {
	Mode     int64 // permission and mode bits.
	Uid      int   // user id of owner.
	Gid      int   // group id of owner.
	Mtime    int64 // modified time; seconds since epoch.
	Size     int64 // length in bytes.
	Devmajor int64 // major number of character or block device.
	Devminor int64 // minor number of character or block device.
	Type     int64
	Name     string // name of header file entry.
}

func (h *Header) IsTrailer() bool {
	return h.Name == trailer.Name &&
		h.Uid == trailer.Uid &&
		h.Gid == trailer.Gid &&
		h.Mtime == trailer.Mtime
}

// File types
const (
	TYPE_SOCK    = 014
	TYPE_SYMLINK = 012
	TYPE_REG     = 010
	TYPE_BLK     = 006
	TYPE_DIR     = 004
	TYPE_CHAR    = 002
	TYPE_FIFO    = 001
)

var (
	trailer = Header{
		Name: "TRAILER!!!",
	}
)
