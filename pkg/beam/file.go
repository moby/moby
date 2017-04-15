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

func (f *File) Send(msg Message) (err error) {
	if msg.Stream != nil {
		Splice(msg.Stream, DevNull)
	}
	_, err = f.f.Write(msg.Data)
	return
}

func (f *File) Receive() (msg Message, err error) {
	data := make([]byte, 4096)
	n, err := f.f.Read(data)
	msg.Data = data[:n]
	return
}

func (f *File) File() (*os.File, error) {
	return f.f, nil
}

func (f *File) Close() error {
	return f.f.Close()
}
