package ioutils

import (
	"io"
	"io/ioutil"
	"os"
	"sync"
)

type WriterTruncator interface {
	io.Closer
	io.Writer
	Truncate() ([]byte, error)
}

type fileWriter struct {
	lock     sync.Mutex
	f        *os.File
	filePath string
}

func NewFileWriter(filePath string) (*fileWriter, error) {
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	w := &fileWriter{f: f}
	return w, nil
}

func (w *fileWriter) Write(b []byte) (int, error) {
	// make sure we aren't doing anything else with the file (like truncating it) while writing
	w.lock.Lock()
	i, err := w.f.Write(b)
	w.lock.Unlock()

	return i, err
}

func (w *fileWriter) Truncate() ([]byte, error) {
	// Lock here so any write requests don't hit the file while truncating
	w.lock.Lock()
	defer w.lock.Unlock()

	// Get the current data, which will be used in the return
	// fd might be closed, so read the file
	b, err := ioutil.ReadFile(w.f.Name())
	if err != nil {
		return nil, err
	}

	// fd might be closed, so use os.Truncate instead of file.Truncate
	if err := os.Truncate(w.f.Name(), 0); err != nil {
		return nil, err
	}

	return b, nil
}

func (w *fileWriter) Close() error {
	w.f.Close()
	return nil
}
