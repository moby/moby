package beam

import (
	"bytes"
	"sync"
	"io"
)

func NewReader(s Stream) io.Reader {
	return &streamReader{s}
}

type streamReader struct {
	Stream
}

// FIXME: group data + stream into a Msg struct
func (r *streamReader) Read(data []byte) (n int, err error) {
	var msg []byte
	msg, _, err = r.Receive()
	if err != nil {
		return 0, err
	}
	return bytes.NewReader(msg).Read(data)
}

func NewWriter(s Stream) io.Writer {
	return &streamWriter{s}
}

type streamWriter struct {
	Stream
}

func (w *streamWriter) Write(data []byte) (n int, err error) {
	err = w.Send(data, nil)
	if err != nil {
		n = 0
	} else {
		n = len(data)
	}
	return
}


func Splice(a, b Stream) (firstErr error) {
	var wg sync.WaitGroup
	wg.Add(2)
	halfSplice := func(src, dst Stream) {
		err := Copy(dst, src)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		dst.Close()
		wg.Done()
	}
	go halfSplice(a, b)
	go halfSplice(b, a)
	wg.Wait()
	return
}


func Copy(dst, src Stream) error {
	for {
		var (
			errSnd, errRcv error
			data []byte
			s Stream
		)
		data, s, errRcv = src.Receive()
		if data != nil || s != nil {
			errSnd = dst.Send(data, s)
		}
		// Note: the order of evaluation of errors is important here.
		if errRcv != nil && errRcv != io.EOF {
			return errRcv
		}
		if errSnd != nil {
			return errSnd
		}
		if errRcv == io.EOF {
			break
		}
	}
	return nil
}

func devNull(data []byte, stream Stream) error {
	if stream != nil {
		go Copy(Func(devNull), stream)
	}
	return nil
}

var DevNull Func = devNull
