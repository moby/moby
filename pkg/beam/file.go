package beam

import (
	"os"
)

type File struct {
	f *os.File
}

func NewFile(f *os.File) *File {
	return &File{f}
}

func (f *File) Send(data []byte, s Stream) (err error) {
	if s != nil {
		Splice(s, DevNull)
	}
	_, err = f.f.Write(data)
	return
}

func (f *File) Receive() (data []byte, s Stream, err error) {
	var n int
	data = make([]byte, 4096)
	n, err = f.f.Read(data)
	return data[:n], nil, err
}

func (f *File) File() (*os.File, error) {
	return f.f, nil
}

func (f *File) Close() error {
	return f.f.Close()
}
