package tar2go

import (
	"archive/tar"
	"io/fs"
)

var (
	_ fs.FS          = &filesystem{}
	_ fs.StatFS      = &filesystem{}
	_ fs.ReadDirFS   = &filesystem{}
	_ readLinkFS     = &filesystem{}
	_ fs.File        = &file{}
	_ fs.ReadDirFile = &dirFile{}
)

// readLinkFS mirrors io/fs.ReadLinkFS, which was added in Go 1.25. We assert
// against a local copy so the compile-time interface check works at the module's
// Go version without naming the newer symbol. The method set is identical, so a
// *filesystem also satisfies fs.ReadLinkFS structurally for Go 1.25+ consumers.
type readLinkFS interface {
	fs.FS
	ReadLink(name string) (string, error)
	Lstat(name string) (fs.FileInfo, error)
}

type filesystem struct {
	idx *Index
}

func (f *filesystem) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// Fast path: an exact regular-file lookup avoids building the directory
	// tree (and scanning the whole archive).
	if idx, err := f.idx.indexWithLock(name); err == nil && idx.hdr.Typeflag != tar.TypeDir {
		return newFile(idx), nil
	}

	node, err := f.idx.lookupWithLock(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if node.isDir() {
		return newDirFile(f.idx, node, name), nil
	}
	return newFile(node.idx), nil
}

func (f *filesystem) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	if idx, err := f.idx.indexWithLock(name); err == nil && idx.hdr.Typeflag != tar.TypeDir {
		return newFileInfo(headerBase(idx.hdr), idx.hdr), nil
	}

	node, err := f.idx.lookupWithLock(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	return node.info(), nil
}

func (f *filesystem) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	entries, err := f.idx.readDir(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	return entries, nil
}

// Lstat returns file information for name without following a trailing symlink.
// tar2go never auto-follows symlinks, so Lstat behaves the same as Stat: a
// symlink reports fs.ModeSymlink rather than the type of its target.
//
// Together with ReadLink this satisfies io/fs.ReadLinkFS (Go 1.25+); consumers
// detect it via structural typing, so the interface is not named here.
func (f *filesystem) Lstat(name string) (fs.FileInfo, error) {
	fi, err := f.Stat(name)
	if err != nil {
		// Normalize the op so errors read as an Lstat failure.
		if pe, ok := err.(*fs.PathError); ok {
			pe.Op = "lstat"
		}
		return nil, err
	}
	return fi, nil
}

// ReadLink returns the target of the symlink named by name. It returns an error
// for entries that are not symlinks, mirroring os.Readlink on a non-symlink.
func (f *filesystem) ReadLink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	if idx, err := f.idx.indexWithLock(name); err == nil {
		return readlink(name, idx.hdr)
	}

	node, err := f.idx.lookupWithLock(name)
	if err != nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
	}
	return readlink(name, node.hdr)
}

func readlink(name string, h *tar.Header) (string, error) {
	if h == nil || h.Typeflag != tar.TypeSymlink {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}
	return h.Linkname, nil
}
