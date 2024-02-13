package tar2go

import (
	"io/fs"
)

var (
	_ fs.FS   = &filesystem{}
	_ fs.File = &file{}
)

type filesystem struct {
	idx *Index
}

func (f *filesystem) Open(name string) (fs.File, error) {
	idx, err := f.idx.indexWithLock(name)
	if err != nil {
		return nil, &fs.PathError{Path: name, Op: "open", Err: err}
	}
	return newFile(idx), nil
}

func (f *filesystem) Stat(name string) (fs.FileInfo, error) {
	idx, err := f.idx.indexWithLock(name)
	if err != nil {
		return nil, &fs.PathError{Path: name, Op: "stat", Err: err}
	}
	return &fileinfo{h: idx.hdr}, nil
}
