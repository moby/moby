package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// Implement io.Reader
type errorReader struct{}

func (r *errorReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("error reader always fail")
}

func TestReadCloserWrapperClose(t *testing.T) {
	reader := strings.NewReader("A string reader")
	wrapper := NewReadCloserWrapper(reader, func() error {
		return fmt.Errorf("This will be called when closing")
	})
	err := wrapper.Close()
	if err == nil || !strings.Contains(err.Error(), "This will be called when closing") {
		t.Fatalf("readCloserWrapper should have call the anonymous func and thus, fail.")
	}
}

func TestReaderErrWrapperReadOnError(t *testing.T) {
	called := false
	reader := &errorReader{}
	wrapper := NewReaderErrWrapper(reader, func() {
		called = true
	})
	_, err := wrapper.Read([]byte{})
	assert.Check(t, is.Error(err, "error reader always fail"))
	if !called {
		t.Fatalf("readErrWrapper should have call the anonymous function on failure")
	}
}

func TestReaderErrWrapperRead(t *testing.T) {
	reader := strings.NewReader("a string reader.")
	wrapper := NewReaderErrWrapper(reader, func() {
		t.Fatalf("readErrWrapper should not have called the anonymous function")
	})
	// Read 20 byte (should be ok with the string above)
	num, err := wrapper.Read(make([]byte, 20))
	if err != nil {
		t.Fatal(err)
	}
	if num != 16 {
		t.Fatalf("readerErrWrapper should have read 16 byte, but read %d", num)
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
	cancelReadCloser := NewCancelReadCloser(ctx, io.NopCloser(&perpetualReader{}))
	for {
		var buf [128]byte
		_, err := cancelReadCloser.Read(buf[:])
		if err == context.DeadlineExceeded {
			break
		} else if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}
	}
}
