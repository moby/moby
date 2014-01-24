package beam

import (
	"fmt"
	"os"
)

type File struct {
	f *os.File
}

func (f *File) Send(data []byte, s Stream) (err error) {
	if s != nil {
		return fmt.Errorf("Operation not supported")
	}
	_, err = f.f.Write(data)
	return
}

func (f *File) Receive() (data []byte, s Stream, err error) {
	data = make([]byte, 4096)
	_, err = f.f.Read(data)
	return data, nil, err
}

func (f *File) File() (*os.File, error) {
	return f.f, nil
}

func (f *File) Close() error {
	return f.f.Close()
}
