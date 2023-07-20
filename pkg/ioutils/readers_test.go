package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func TestReadCloserWrapperClose(t *testing.T) {
	const text = "hello world"
	testErr := errors.New("this will be called when closing")
	wrapper := NewReadCloserWrapper(strings.NewReader(text), func() error {
		return testErr
	})

	buf, err := io.ReadAll(wrapper)
	if err != nil {
		t.Errorf("io.ReadAll(wrapper) err = %v", err)
	}
	if string(buf) != text {
		t.Errorf("expected %v, got: %v", text, string(buf))
	}
	err = wrapper.Close()
	if !errors.Is(err, testErr) {
		// readCloserWrapper should have called the anonymous func and thus, fail
		t.Errorf("expected %v, got: %v", testErr, err)
	}
}

func TestReaderErrWrapperReadOnError(t *testing.T) {
	called := false
	expectedErr := errors.New("error reader always fail")
	wrapper := NewReaderErrWrapper(iotest.ErrReader(expectedErr), func() {
		called = true
	})
	_, err := wrapper.Read([]byte{})
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got: %v", expectedErr, err)
	}
	if !called {
		t.Fatalf("readErrWrapper should have called the anonymous function on failure")
	}
}

func TestReaderErrWrapperRead(t *testing.T) {
	const text = "hello world"
	wrapper := NewReaderErrWrapper(strings.NewReader(text), func() {
		t.Fatalf("readErrWrapper should not have called the anonymous function")
	})
	num, err := wrapper.Read(make([]byte, len(text)+10))
	if err != nil {
		t.Error(err)
	}
	if expected := len(text); num != expected {
		t.Errorf("readerErrWrapper should have read %d byte, but read %d", expected, num)
	}
}

type perpetualReader struct{}

func (p *perpetualReader) Read(buf []byte) (n int, err error) {
	for i := 0; i != len(buf); i++ {
		buf[i] = 'a'
	}
	return len(buf), nil
}

func TestCancelReadCloser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	crc := NewCancelReadCloser(ctx, io.NopCloser(&perpetualReader{}))
	for {
		var buf [128]byte
		_, err := crc.Read(buf[:])
		if err == context.DeadlineExceeded {
			break
		} else if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}
	}
}
