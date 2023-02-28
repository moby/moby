package s3

import (
	"io"
)

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
}

type readerAtCloser struct {
	offset int64
	rc     io.ReadCloser
	ra     io.ReaderAt
	open   func(offset int64) (io.ReadCloser, error)
	closed bool
}

func toReaderAtCloser(open func(offset int64) (io.ReadCloser, error)) ReaderAtCloser {
	return &readerAtCloser{
		open: open,
	}
}

func (hrs *readerAtCloser) ReadAt(p []byte, off int64) (n int, err error) {
	if hrs.closed {
		return 0, io.EOF
	}

	if hrs.ra != nil {
		return hrs.ra.ReadAt(p, off)
	}

	if hrs.rc == nil || off != hrs.offset {
		if hrs.rc != nil {
			hrs.rc.Close()
			hrs.rc = nil
		}
		rc, err := hrs.open(off)
		if err != nil {
			return 0, err
		}
		hrs.rc = rc
	}
	if ra, ok := hrs.rc.(io.ReaderAt); ok {
		hrs.ra = ra
		n, err = ra.ReadAt(p, off)
	} else {
		for {
			var nn int
			nn, err = hrs.rc.Read(p)
			n += nn
			p = p[nn:]
			if nn == len(p) || err != nil {
				break
			}
		}
	}

	hrs.offset += int64(n)
	return
}

func (hrs *readerAtCloser) Close() error {
	if hrs.closed {
		return nil
	}
	hrs.closed = true
	if hrs.rc != nil {
		return hrs.rc.Close()
	}

	return nil
}
