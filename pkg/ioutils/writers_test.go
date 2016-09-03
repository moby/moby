package ioutils

import (
	"bytes"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestWriteCloserWrapperClose(c *check.C) {
	called := false
	writer := bytes.NewBuffer([]byte{})
	wrapper := NewWriteCloserWrapper(writer, func() error {
		called = true
		return nil
	})
	if err := wrapper.Close(); err != nil {
		c.Fatal(err)
	}
	if !called {
		c.Fatalf("writeCloserWrapper should have call the anonymous function.")
	}
}

func (s *DockerSuite) TestNopWriteCloser(c *check.C) {
	writer := bytes.NewBuffer([]byte{})
	wrapper := NopWriteCloser(writer)
	if err := wrapper.Close(); err != nil {
		c.Fatal("NopWriteCloser always return nil on Close.")
	}

}

func (s *DockerSuite) TestNopWriter(c *check.C) {
	nw := &NopWriter{}
	l, err := nw.Write([]byte{'c'})
	if err != nil {
		c.Fatal(err)
	}
	if l != 1 {
		c.Fatalf("Expected 1 got %d", l)
	}
}

func (s *DockerSuite) TestWriteCounter(c *check.C) {
	dummy1 := "This is a dummy string."
	dummy2 := "This is another dummy string."
	totalLength := int64(len(dummy1) + len(dummy2))

	reader1 := strings.NewReader(dummy1)
	reader2 := strings.NewReader(dummy2)

	var buffer bytes.Buffer
	wc := NewWriteCounter(&buffer)

	reader1.WriteTo(wc)
	reader2.WriteTo(wc)

	if wc.Count != totalLength {
		c.Errorf("Wrong count: %d vs. %d", wc.Count, totalLength)
	}

	if buffer.String() != dummy1+dummy2 {
		c.Error("Wrong message written")
	}
}
