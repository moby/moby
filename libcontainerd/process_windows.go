package libcontainerd

import (
	"io"

	"github.com/Microsoft/hcsshim"
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

type stdInCloser struct {
	io.WriteCloser
	hcsshim.Process
}

func createStdInCloser(pipe io.WriteCloser, process hcsshim.Process) *stdInCloser {
	return &stdInCloser{
		WriteCloser: pipe,
		Process:     process,
	}
}

func (stdin *stdInCloser) Close() error {
	if err := stdin.WriteCloser.Close(); err != nil {
		return err
	}

	return stdin.Process.CloseStdin()
}

func (stdin *stdInCloser) Write(p []byte) (n int, err error) {
	return stdin.WriteCloser.Write(p)
}
