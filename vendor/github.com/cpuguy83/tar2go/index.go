package tar2go

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sync"
)

var (
	// ErrDelete should be returned by an UpdaterFn when the file should be deleted.
	ErrDelete = errors.New("delete")
)

// Index is a tar index that can be used to read files from a tar.
type Index struct {
	rdr *io.SectionReader
	tar *tar.Reader
	mu  sync.Mutex
	idx map[string]*indexReader
}

// NewIndex creates a new Index from the passed in io.ReaderAt.
func NewIndex(rdr io.ReaderAt) *Index {
	ras, ok := rdr.(ReaderAtSized)
	var size int64
	if !ok {
		size = 1<<63 - 1
	} else {
		size = ras.Size()
	}
	return &Index{
		rdr: io.NewSectionReader(rdr, 0, size),
		idx: make(map[string]*indexReader),
	}
}

func (i *Index) indexWithLock(name string) (*indexReader, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.index(name)
}

func filterFSPrefix(name string) string {
	if len(name) <= 1 {
		return name
	}
	if name[0] == '/' {
		return name[1:]
	}
	if len(name) > 2 && name[0] == '.' && name[1] == '/' {
		return name[2:]
	}
	return name
}

// This function must be called with the lock held.
func (i *Index) index(name string) (*indexReader, error) {
	name = filterFSPrefix(name)
	if rdr, ok := i.idx[name]; ok {
		return rdr, nil
	}

	if i.tar == nil {
		i.tar = tar.NewReader(i.rdr)
	}

	for {
		hdr, err := i.tar.Next()
		if err != nil {
			if err == io.EOF {
				return nil, fs.ErrNotExist
			}
			return nil, fmt.Errorf("error indexing tar: %w", err)
		}

		pos, err := i.rdr.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("error getting file offset: %w", err)
		}
		rdr := &indexReader{rdr: i.rdr, offset: pos, size: hdr.Size, hdr: hdr}
		hdrName := filterFSPrefix(hdr.Name)
		i.idx[hdrName] = rdr

		if hdrName == name {
			return rdr, nil
		}
	}
}

// Reader returns an io.ReaderAt that can be used to read the entire tar.
func (i *Index) Reader() *io.SectionReader {
	return io.NewSectionReader(i.rdr, 0, i.rdr.Size())
}

// FS returns an fs.FS that can be used to read the files in the tar.
func (i *Index) FS() fs.FS {
	return &filesystem{idx: i}
}

// ReaderAtSized is an io.ReaderAt that also implements a Size method.
type ReaderAtSized interface {
	io.ReaderAt
	Size() int64
}

// UpdaterFn is a function that is passed the name of the file and a ReaderAtSized
type UpdaterFn func(string, ReaderAtSized) (ReaderAtSized, bool, error)

// Replace replaces the file with the passed in name with the passed in ReaderAtSized.
// If the passed in ReaderAtSized is nil, the file will be deleted.
// If the file does not exist, it will be added.
//
// This function does not update the actual tar file, it only updates the index.
func (i *Index) Replace(name string, rdr ReaderAtSized) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// index may overwrite it this replacement.
	i.index(name)

	if rdr == nil {
		delete(i.idx, name)
		return nil
	}

	i.idx[name] = &indexReader{rdr: rdr, offset: 0, size: rdr.Size(), hdr: &tar.Header{
		Name: name,
		Size: rdr.Size(),
	}}
	return nil
}

// Update creates a new tar with the files updated by the passed in updater function.
// The output tar is written to the passed in io.Writer
func (i *Index) Update(w io.Writer, updater UpdaterFn) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	rdr := i.Reader()
	tr := tar.NewReader(rdr)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading tar: %w", err)
		}

		offset, err := rdr.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("error getting file offset: %w", err)
		}

		ra, updated, err := updater(hdr.Name, io.NewSectionReader(i.rdr, offset, hdr.Size))
		if err != nil {
			if err == ErrDelete {
				continue
			}
			return fmt.Errorf("error updating file %s: %w", hdr.Name, err)
		}

		if updated {
			hdr.Size = ra.Size()
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("error writing tar header: %w", err)
		}

		if _, err := io.Copy(tw, io.NewSectionReader(ra, 0, ra.Size())); err != nil {
			return fmt.Errorf("error writing tar file: %w", err)
		}
	}
}

type indexReader struct {
	rdr    io.ReaderAt
	offset int64
	size   int64
	hdr    *tar.Header
}

func (r *indexReader) Reader() *io.SectionReader {
	return io.NewSectionReader(r.rdr, r.offset, r.size)
}
