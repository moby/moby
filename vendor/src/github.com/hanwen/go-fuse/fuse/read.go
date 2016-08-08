package fuse

import (
	"io"
	"syscall"
)

// ReadResultData is the read return for returning bytes directly.
type readResultData struct {
	// Raw bytes for the read.
	Data []byte
}

func (r *readResultData) Size() int {
	return len(r.Data)
}

func (r *readResultData) Done() {
}

func (r *readResultData) Bytes(buf []byte) ([]byte, Status) {
	return r.Data, OK
}

func ReadResultData(b []byte) ReadResult {
	return &readResultData{b}
}

func ReadResultFd(fd uintptr, off int64, sz int) ReadResult {
	return &readResultFd{fd, off, sz}
}

// ReadResultFd is the read return for zero-copy file data.
type readResultFd struct {
	// Splice from the following file.
	Fd uintptr

	// Offset within Fd, or -1 to use current offset.
	Off int64

	// Size of data to be loaded. Actual data available may be
	// less at the EOF.
	Sz int
}

// Reads raw bytes from file descriptor if necessary, using the passed
// buffer as storage.
func (r *readResultFd) Bytes(buf []byte) ([]byte, Status) {
	sz := r.Sz
	if len(buf) < sz {
		sz = len(buf)
	}

	n, err := syscall.Pread(int(r.Fd), buf[:sz], r.Off)
	if err == io.EOF {
		err = nil
	}

	if n < 0 {
		n = 0
	}

	return buf[:n], ToStatus(err)
}

func (r *readResultFd) Size() int {
	return r.Sz
}

func (r *readResultFd) Done() {
}
