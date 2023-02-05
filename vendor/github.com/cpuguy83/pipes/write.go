package pipes

import (
	"os"
	"syscall"
)

type PipeWriter struct {
	fd *os.File
}

func (w *PipeWriter) Write(p []byte) (int, error) {
	return w.fd.Write(p)
}

func (w *PipeWriter) Close() error {
	return w.fd.Close()
}

func (w *PipeWriter) SyscallConn() (syscall.RawConn, error) {
	return w.fd.SyscallConn()
}
