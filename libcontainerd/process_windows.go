package libcontainerd

import (
	"io"
	"strconv"
	"strings"
	"sync"

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

type autoClosingReader struct {
	io.ReadCloser
	sync.Once
}

func (r *autoClosingReader) Read(b []byte) (n int, err error) {
	n, err = r.ReadCloser.Read(b)
	if err == io.EOF {
		r.Once.Do(func() { r.ReadCloser.Close() })
	}
	return
}

// fixStdinBackspaceBehavior works around a bug in Windows before build 14350
// where it interpreted DEL as VK_DELETE instead of as VK_BACK. This replaces
// DEL with BS to work around this.
func fixStdinBackspaceBehavior(w io.WriteCloser, osversion string, tty bool) io.WriteCloser {
	if !tty {
		return w
	}
	v := strings.Split(osversion, ".")
	if len(v) < 3 {
		return w
	}

	if build, err := strconv.Atoi(v[2]); err != nil || build >= 14350 {
		return w
	}

	return &delToBsWriter{w}
}

type delToBsWriter struct {
	io.WriteCloser
}

func (w *delToBsWriter) Write(b []byte) (int, error) {
	const (
		backspace = 0x8
		del       = 0x7f
	)
	bc := make([]byte, len(b))
	for i, c := range b {
		if c == del {
			bc[i] = backspace
		} else {
			bc[i] = c
		}
	}
	return w.WriteCloser.Write(bc)
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
