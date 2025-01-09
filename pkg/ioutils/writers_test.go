package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"bytes"
	"testing"
)

func TestWriteCloserWrapperClose(t *testing.T) {
	called := false
	writer := bytes.NewBuffer([]byte{})
	wrapper := NewWriteCloserWrapper(writer, func() error {
		called = true
		return nil
	})
	if err := wrapper.Close(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatalf("writeCloserWrapper should have call the anonymous function.")
	}
}

func TestNopWriteCloser(t *testing.T) {
	writer := bytes.NewBuffer([]byte{})
	wrapper := NopWriteCloser(writer)
	if err := wrapper.Close(); err != nil {
		t.Fatal("NopWriteCloser always return nil on Close.")
	}
}

func TestNopWriter(t *testing.T) {
	nw := &NopWriter{}
	l, err := nw.Write([]byte{'c'})
	if err != nil {
		t.Fatal(err)
	}
	if l != 1 {
		t.Fatalf("Expected 1 got %d", l)
	}
}
