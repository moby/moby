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

type fileinfo struct {
	h *tar.Header
}

func (f *fileinfo) Name() string {
	return f.h.Name
}

func (f *fileinfo) Size() int64 {
	return f.h.Size
}

func (f *fileinfo) Mode() fs.FileMode {
	return fs.FileMode(f.h.Mode)
}

func (f *fileinfo) ModTime() time.Time {
	return f.h.ModTime
}

func (f *fileinfo) IsDir() bool {
	return f.h.Typeflag == tar.TypeDir
}

func (f *file) Close() error {
	return nil
}

func (f *fileinfo) Sys() interface{} {
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
	return &fileinfo{h: f.idx.hdr}, nil
}
