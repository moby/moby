package streams

import (
	"io"

	"github.com/Microsoft/go-winio"
)

func openPipe(path string) (io.ReadCloser, io.WriteCloser, error) {
	// Open the pipe twice so we can close one side without closing the other.

	r, err := winio.DialPipe(path, nil)
	if err != nil {
		return nil, nil, err
	}

	w, err := winio.DialPipe(path, nil)
	if err != nil {
		r.Close()
		return nil, nil, err
	}
	return r, w, nil
}
