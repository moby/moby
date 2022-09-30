package loggerutils // import "github.com/docker/docker/daemon/logger/loggerutils"

import (
	"io"
	"io/fs"
	"os"
	"runtime"
)

type fileConvertFn func(dst io.WriteSeeker, src io.ReadSeeker) error

type stfID uint64

// sharedTempFileConverter converts files using a user-supplied function and
// writes the results to temporary files which are automatically cleaned up on
// close. If another request is made to convert the same file, the conversion
// result and temporary file are reused if they have not yet been cleaned up.
//
// A file is considered the same as another file using the os.SameFile function,
// which compares file identity (e.g. device and inode numbers on Linux) and is
// robust to file renames. Input files are assumed to be immutable; no attempt
// is made to ascertain whether the file contents have changed between requests.
//
// One file descriptor is used per source file, irrespective of the number of
// concurrent readers of the converted contents.
type sharedTempFileConverter struct {
	// The directory where temporary converted files are to be written to.
	// If set to the empty string, the default directory for temporary files
	// is used.
	TempDir string

	conv fileConvertFn
	st   chan stfcState
}

type stfcState struct {
	fl     map[stfID]sharedTempFile
	nextID stfID
}

type sharedTempFile struct {
	src  os.FileInfo // Info about the source file for path-independent identification with os.SameFile.
	fd   *os.File
	size int64
	ref  int                       // Reference count of open readers on the temporary file.
	wait []chan<- stfConvertResult // Wait list for the conversion to complete.
}

type stfConvertResult struct {
	fr  *sharedFileReader
	err error
}

func newSharedTempFileConverter(conv fileConvertFn) *sharedTempFileConverter {
	st := make(chan stfcState, 1)
	st <- stfcState{fl: make(map[stfID]sharedTempFile)}
	return &sharedTempFileConverter{conv: conv, st: st}
}

// Do returns a reader for the contents of f as converted by the c.C function.
// It is the caller's responsibility to close the returned reader.
//
// This function is safe for concurrent use by multiple goroutines.
func (c *sharedTempFileConverter) Do(f *os.File) (*sharedFileReader, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	st := <-c.st
	for id, tf := range st.fl {
		// os.SameFile can have false positives if one of the files was
		// deleted before the other file was created -- such as during
		// log rotations... https://github.com/golang/go/issues/36895
		// Weed out those false positives by also comparing the files'
		// ModTime, which conveniently also handles the case of true
		// positives where the file has also been modified since it was
		// first converted.
		if os.SameFile(tf.src, stat) && tf.src.ModTime() == stat.ModTime() {
			return c.openExisting(st, id, tf)
		}
	}
	return c.openNew(st, f, stat)
}

func (c *sharedTempFileConverter) openNew(st stfcState, f *os.File, stat os.FileInfo) (*sharedFileReader, error) {
	// Record that we are starting to convert this file so that any other
	// requests for the same source file while the conversion is in progress
	// can join.
	id := st.nextID
	st.nextID++
	st.fl[id] = sharedTempFile{src: stat}
	c.st <- st

	dst, size, convErr := c.convert(f)

	st = <-c.st
	flid := st.fl[id]

	if convErr != nil {
		// Conversion failed. Delete it from the state so that future
		// requests to convert the same file can try again fresh.
		delete(st.fl, id)
		c.st <- st
		for _, w := range flid.wait {
			w <- stfConvertResult{err: convErr}
		}
		return nil, convErr
	}

	flid.fd = dst
	flid.size = size
	flid.ref = len(flid.wait) + 1
	for _, w := range flid.wait {
		// Each waiter needs its own reader with an independent read pointer.
		w <- stfConvertResult{fr: flid.Reader(c, id)}
	}
	flid.wait = nil
	st.fl[id] = flid
	c.st <- st
	return flid.Reader(c, id), nil
}

func (c *sharedTempFileConverter) openExisting(st stfcState, id stfID, v sharedTempFile) (*sharedFileReader, error) {
	if v.fd != nil {
		// Already converted.
		v.ref++
		st.fl[id] = v
		c.st <- st
		return v.Reader(c, id), nil
	}
	// The file has not finished being converted.
	// Add ourselves to the wait list. "Don't call us; we'll call you."
	wait := make(chan stfConvertResult, 1)
	v.wait = append(v.wait, wait)
	st.fl[id] = v
	c.st <- st

	res := <-wait
	return res.fr, res.err
}

func (c *sharedTempFileConverter) convert(f *os.File) (converted *os.File, size int64, err error) {
	dst, err := os.CreateTemp(c.TempDir, "dockerdtemp.*")
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = dst.Close()
		// Delete the temporary file immediately so that final cleanup
		// of the file on disk is deferred to the OS once we close all
		// our file descriptors (or the process dies). Assuming no early
		// returns due to errors, the file will be open by this process
		// with a read-only descriptor at this point. As we don't care
		// about being able to reuse the file name -- it's randomly
		// generated and unique -- we can safely use os.Remove on
		// Windows.
		_ = os.Remove(dst.Name())
	}()
	err = c.conv(dst, f)
	if err != nil {
		return nil, 0, err
	}
	// Close the exclusive read-write file descriptor, catching any delayed
	// write errors (and on Windows, releasing the share-locks on the file)
	if err := dst.Close(); err != nil {
		_ = os.Remove(dst.Name())
		return nil, 0, err
	}
	// Open the file again read-only (without locking the file against
	// deletion on Windows).
	converted, err = open(dst.Name())
	if err != nil {
		return nil, 0, err
	}

	// The position of the file's read pointer doesn't matter as all readers
	// will be accessing the file through its io.ReaderAt interface.
	size, err = converted.Seek(0, io.SeekEnd)
	if err != nil {
		_ = converted.Close()
		return nil, 0, err
	}
	return converted, size, nil
}

type sharedFileReader struct {
	*io.SectionReader

	c      *sharedTempFileConverter
	id     stfID
	closed bool
}

func (stf sharedTempFile) Reader(c *sharedTempFileConverter, id stfID) *sharedFileReader {
	rdr := &sharedFileReader{SectionReader: io.NewSectionReader(stf.fd, 0, stf.size), c: c, id: id}
	runtime.SetFinalizer(rdr, (*sharedFileReader).Close)
	return rdr
}

func (r *sharedFileReader) Close() error {
	if r.closed {
		return fs.ErrClosed
	}

	st := <-r.c.st
	flid, ok := st.fl[r.id]
	if !ok {
		panic("invariant violation: temp file state missing from map")
	}
	flid.ref--
	lastRef := flid.ref <= 0
	if lastRef {
		delete(st.fl, r.id)
	} else {
		st.fl[r.id] = flid
	}
	r.closed = true
	r.c.st <- st

	if lastRef {
		return flid.fd.Close()
	}
	runtime.SetFinalizer(r, nil)
	return nil
}
