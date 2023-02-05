package pipes

import (
	"io"
	"syscall"

	"golang.org/x/sys/unix"
)

func (r *PipeReader) WriteTo(w io.Writer) (int64, error) {
	if wc, ok := w.(syscall.Conn); ok {
		if raw, err := wc.SyscallConn(); err == nil {
			handled, n, err := r.writeTo(raw)
			if handled || err == nil {
				return n, err
			}
		}
	}

	return io.Copy(w, r.fd)
}

func (r *PipeReader) writeTo(w syscall.RawConn) (bool, int64, error) {
	rc, err := r.SyscallConn()
	if err != nil {
		return false, 0, err
	}

	var (
		copied    int64
		readErr   error
		spliceErr error
	)

	// Beceause the writer may not be pollable we need to call `Read` first (which we know is pollable).
	err = rc.Read(func(rfd uintptr) bool {
		readErr = w.Write(func(wfd uintptr) bool {
			var n int64
			n, spliceErr = splice(int(rfd), int(wfd), 0)
			if n > 0 {
				copied += n
			}
			return true
		})

		if readErr != nil {
			return true
		}
		return spliceErr != unix.EAGAIN
	})

	if err != nil {
		return copied > 0, copied, err
	}

	if readErr != nil {
		return copied > 0, copied, readErr
	}

	return copied > 0, copied, err
}
