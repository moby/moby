package pipes

import (
	"os"
	"syscall"
)

type PipeReader struct {
	fd *os.File
}

func (r *PipeReader) Read(p []byte) (int, error) {
	return r.fd.Read(p)
}

func (r *PipeReader) Close() error {
	return r.fd.Close()
}

func (r *PipeReader) SyscallConn() (syscall.RawConn, error) {
	return r.fd.SyscallConn()
}
