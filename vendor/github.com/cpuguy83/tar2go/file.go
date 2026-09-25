package tar2go

import (
	"archive/tar"
	"io"
	"io/fs"
	"time"
)

type file struct {
	idx *indexReader
	rdr *io.SectionReader
}

func newFile(idx *indexReader) *file {
	return &file{idx: idx, rdr: io.NewSectionReader(idx.rdr, idx.offset, idx.size)}
}

// fileinfo is an fs.FileInfo backed by a tar header for real entries or
// synthesized for directories that only exist implicitly in the archive.
type fileinfo struct {
	name string      // base name of the entry
	h    *tar.Header // nil for synthesized directories
	dir  bool
}

// newFileInfo builds a fileinfo for a concrete tar entry.
func newFileInfo(name string, h *tar.Header) *fileinfo {
	return &fileinfo{name: name, h: h, dir: h != nil && h.Typeflag == tar.TypeDir}
}

func (f *fileinfo) Name() string {
	return f.name
}

func (f *fileinfo) Size() int64 {
	if f.h == nil {
		return 0
	}
	return f.h.Size
}

// Mode reports the entry's type and permission bits. It delegates to the
// stdlib mapping (tar.Header.FileInfo().Mode()), which derives the full
// fs.FileMode from the header: the type bits (fs.ModeSymlink, fs.ModeDevice,
// fs.ModeCharDevice, fs.ModeNamedPipe, fs.ModeDir), the special bits
// (fs.ModeSetuid, fs.ModeSetgid, fs.ModeSticky) and the permission bits.
//
// Hardlinks (tar.TypeLink) have no distinct io/fs mode bit, so they report as
// regular files. Symlinks and device/FIFO nodes carry no data body, so opening
// one yields a zero-length file; its type is still reported correctly here.
func (f *fileinfo) Mode() fs.FileMode {
	if f.h == nil {
		// Synthesized directory that only exists implicitly in the archive.
		return fs.ModeDir | 0o555
	}
	mode := f.h.FileInfo().Mode()
	if f.dir {
		// A node that is structurally a directory (has children) but whose
		// header is not TypeDir still reports as a directory.
		mode |= fs.ModeDir
	}
	return mode
}

func (f *fileinfo) ModTime() time.Time {
	if f.h == nil {
		return time.Time{}
	}
	return f.h.ModTime
}

func (f *fileinfo) IsDir() bool {
	return f.dir
}

func (f *file) Close() error {
	return nil
}

func (f *fileinfo) Sys() interface{} {
	if f.h == nil {
		return nil
	}
	h := *f.h
	return &h
}

func (f *file) Read(p []byte) (int, error) {
	return f.rdr.Read(p)
}

func (f *file) ReadAt(p []byte, off int64) (int, error) {
	return f.rdr.ReadAt(p, off)
}

func (f *file) Size() int64 {
	return f.rdr.Size()
}

func (f *file) Stat() (fs.FileInfo, error) {
	return newFileInfo(headerBase(f.idx.hdr), f.idx.hdr), nil
}
