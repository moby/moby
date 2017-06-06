package cpio

import (
	"io"
	"fmt"
)

// A writer enables sequential writing of cpio archives.
// A call to WriteHeader begins a new file. Every call to
// write afterwards appends to that file, writing at most
// hdr.Size bytes in total.
type Writer struct {
	w               io.Writer
	inode           int64
	length          int64
	remaining_bytes int
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w:     w,
		inode: 721,
	}
}

func assemble(mode, typev int64) int64 {
	return mode&0xFFF | ((typev & 0xF) << 12)
}

func (w *Writer) WriteHeader(hdr *Header) error {
	// Bring last file to the defined length
	e := w.zeros(int64(w.remaining_bytes))
	if e != nil {
		return e
	}
	e = w.pad(4)
	if e != nil {
		return e
	}
	bname := []byte(hdr.Name)
	nlinks := 1
	if hdr.Type == TYPE_DIR {
		nlinks = 2
	}
	shdr := fmt.Sprintf("%s%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x",
		"070701",
		w.inode,
		assemble(hdr.Mode, hdr.Type),
		hdr.Uid,
		hdr.Gid,
		nlinks,
		hdr.Mtime,
		hdr.Size,
		3, // major
		1, // minor
		hdr.Devmajor,
		hdr.Devminor,
		len(bname)+1, // +1 for terminating zero
		0)            // check
	_, e = w.countedWrite([]byte(shdr))
	if e != nil {
		return e
	}

	_, e = w.countedWrite(bname)
	if e != nil {
		return e
	}

	_, e = w.countedWrite([]byte{0})
	if e != nil {
		return e
	}
	w.inode++
	w.remaining_bytes = int(hdr.Size)
	return w.pad(4)
}

func (w *Writer) zeros(num int64) error {
	for ; num > 0; num-- {
		_, e := w.countedWrite([]byte{0})
		if e != nil {
			return e
		}
	}
	return nil
}

// Brings the length of the file to a multiple of mod
func (w *Writer) pad(mod int64) error {

	return w.zeros((mod - (w.length % mod)) % mod)
}

func (w *Writer) Write(b []byte) (n int, e error) {
	if len(b) > w.remaining_bytes {
		b = b[0:w.remaining_bytes]
	}
	n, e = w.countedWrite(b)
	w.remaining_bytes -= n
	return
}

func (w *Writer) countedWrite(b []byte) (n int, e error) {
	n, e = w.w.Write(b)
	w.length += int64(n)
	return n, e
}

// Writes trailer
// Does not close underlying writer
func (w *Writer) Close() error {
	e := w.WriteHeader(&trailer)
	if e != nil {
		return e
	}
	return w.pad(512)
}
