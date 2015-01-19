package ioutils

import (
	"io"
	"os"
	"sync"
)

type WriterTruncator interface {
	io.Closer
	io.Writer
	Truncate(w io.Writer) error
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

// Safely truncates the file
// Reads from the file, writes to the passed in writer
func (w *fileWriter) Truncate(out io.Writer) error {
	// Lock here so any write requests don't hit the file while truncating
	w.lock.Lock()
	defer func() {
		w.lock.Unlock()
	}()

	f, err := os.Open(w.f.Name())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, f); err != nil {
		return err
	}

	return os.Truncate(w.f.Name(), 0)
}

func (w *fileWriter) Close() error {
	w.f.Close()
	return nil
}
