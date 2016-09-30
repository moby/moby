package libcontainerd

import (
	"io"

	"github.com/Microsoft/hcsshim"
	"github.com/docker/docker/pkg/ioutils"
)

// process keeps the state for both main container process and exec process.
type process struct {
	processCommon

	// Platform specific fields are below here.

	// commandLine is to support returning summary information for docker top
	commandLine string
	hcsProcess  hcsshim.Process
}

func openReaderFromPipe(p io.ReadCloser) io.Reader {
	r, w := io.Pipe()
	go func() {
		if _, err := io.Copy(w, p); err != nil {
			r.CloseWithError(err)
		}
		w.Close()
		p.Close()
	}()
	return r
}

func createStdInCloser(pipe io.WriteCloser, process hcsshim.Process) io.WriteCloser {
	return ioutils.NewWriteCloserWrapper(pipe, func() error {
		if err := pipe.Close(); err != nil {
			return err
		}

		// We do not need to lock container ID here, even though
		// we are calling into hcsshim. This is safe, because the
		// only place that closes this process handle is this method.
		err := process.CloseStdin()
		if err != nil && !hcsshim.IsNotExist(err) {
			// This error will occur if the compute system is currently shutting down
			if perr, ok := err.(*hcsshim.ProcessError); ok && perr.Err != hcsshim.ErrVmcomputeOperationInvalidState {
				return err
			}
		}

		return err
	})
}
