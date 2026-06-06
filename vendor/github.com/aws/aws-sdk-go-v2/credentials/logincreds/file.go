package logincreds

import (
	"io"
	"os"
)

var openFile func(string) (io.ReadCloser, error) = func(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

var createFile func(string, os.FileMode) (io.WriteCloser, error) = func(name string, mode os.FileMode) (io.WriteCloser, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
}
