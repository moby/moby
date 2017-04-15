package beam

import (
	"bytes"
	"io"
	"sync"
)

func NewReader(s Stream) io.Reader {
	return &streamReader{s}
}

type streamReader struct {
	Stream
}

// FIXME: group data + stream into a Msg struct
func (r *streamReader) Read(data []byte) (n int, err error) {
	msg, err := r.Receive()
	if err != nil {
		return 0, err
	}
	return bytes.NewReader(msg.Data).Read(data)
}

func NewWriter(s Stream) io.Writer {
	return &streamWriter{s}
}

type streamWriter struct {
	Stream
}

func (w *streamWriter) Write(data []byte) (n int, err error) {
	err = w.Send(Message{Data: data})
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
			msg            Message
		)
		msg, errRcv = src.Receive()
		if msg.Data != nil || msg.Stream != nil {
			errSnd = dst.Send(msg)
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

func devNull(msg Message) (err error) {
	if msg.Stream != nil {
		go Copy(Func(devNull), msg.Stream)
	}
	return
}

var DevNull Func = devNull
