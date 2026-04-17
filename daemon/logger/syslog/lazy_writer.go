package syslog

import (
	"errors"
	"sync"

	syslog "github.com/RackSec/srslog"
)

// Wraps a `syslog.Writer` delaying connection until `GetOrConnect` is called.
type lazyWriter struct {
	connect func() (*syslog.Writer, error)

	mu     *sync.Mutex
	writer *syslog.Writer
}

func newLazyWriter(connect func() (*syslog.Writer, error)) lazyWriter {
	return lazyWriter{
		connect: connect,
		mu:      &sync.Mutex{},
		writer:  nil,
	}
}

// Return the connected `Writer` or attempt to connect.
func (w *lazyWriter) GetOrConnect() (*syslog.Writer, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer == nil {
		var err error
		w.writer, err = w.connect()
		if err != nil {
			return nil, err
		}
	}
	return w.writer, nil
}
func (w *lazyWriter) Close() (err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.writer != nil {
		err = w.writer.Close()
	}

	w.writer = nil
	w.connect = func() (*syslog.Writer, error) {
		return nil, errors.New("LazyWriter is closed")
	}
	return err
}
