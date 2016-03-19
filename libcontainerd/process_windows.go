package libcontainerd

import (
	"io"
)

// process keeps the state for both main container process and exec process.
type process struct {
	processCommon

	// Platform specific fields are below here. There are none presently on Windows.
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
