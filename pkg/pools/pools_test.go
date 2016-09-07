package pools

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestBufioReaderPoolGetWithNoReaderShouldCreateOne(c *check.C) {
	reader := BufioReader32KPool.Get(nil)
	if reader == nil {
		c.Fatalf("BufioReaderPool should have create a bufio.Reader but did not.")
	}
}

func (s *DockerSuite) TestBufioReaderPoolPutAndGet(c *check.C) {
	sr := bufio.NewReader(strings.NewReader("foobar"))
	reader := BufioReader32KPool.Get(sr)
	if reader == nil {
		c.Fatalf("BufioReaderPool should not return a nil reader.")
	}
	// verify the first 3 byte
	buf1 := make([]byte, 3)
	_, err := reader.Read(buf1)
	if err != nil {
		c.Fatal(err)
	}
	if actual := string(buf1); actual != "foo" {
		c.Fatalf("The first letter should have been 'foo' but was %v", actual)
	}
	BufioReader32KPool.Put(reader)
	// Try to read the next 3 bytes
	_, err = sr.Read(make([]byte, 3))
	if err == nil || err != io.EOF {
		c.Fatalf("The buffer should have been empty, issue an EOF error.")
	}
}

type simpleReaderCloser struct {
	io.Reader
	closed bool
}

func (r *simpleReaderCloser) Close() error {
	r.closed = true
	return nil
}

func (s *DockerSuite) TestNewReadCloserWrapperWithAReadCloser(c *check.C) {
	br := bufio.NewReader(strings.NewReader(""))
	sr := &simpleReaderCloser{
		Reader: strings.NewReader("foobar"),
		closed: false,
	}
	reader := BufioReader32KPool.NewReadCloserWrapper(br, sr)
	if reader == nil {
		c.Fatalf("NewReadCloserWrapper should not return a nil reader.")
	}
	// Verify the content of reader
	buf := make([]byte, 3)
	_, err := reader.Read(buf)
	if err != nil {
		c.Fatal(err)
	}
	if actual := string(buf); actual != "foo" {
		c.Fatalf("The first 3 letter should have been 'foo' but were %v", actual)
	}
	reader.Close()
	// Read 3 more bytes "bar"
	_, err = reader.Read(buf)
	if err != nil {
		c.Fatal(err)
	}
	if actual := string(buf); actual != "bar" {
		c.Fatalf("The first 3 letter should have been 'bar' but were %v", actual)
	}
	if !sr.closed {
		c.Fatalf("The ReaderCloser should have been closed, it is not.")
	}
}

func (s *DockerSuite) TestBufioWriterPoolGetWithNoReaderShouldCreateOne(c *check.C) {
	writer := BufioWriter32KPool.Get(nil)
	if writer == nil {
		c.Fatalf("BufioWriterPool should have create a bufio.Writer but did not.")
	}
}

func (s *DockerSuite) TestBufioWriterPoolPutAndGet(c *check.C) {
	buf := new(bytes.Buffer)
	bw := bufio.NewWriter(buf)
	writer := BufioWriter32KPool.Get(bw)
	if writer == nil {
		c.Fatalf("BufioReaderPool should not return a nil writer.")
	}
	written, err := writer.Write([]byte("foobar"))
	if err != nil {
		c.Fatal(err)
	}
	if written != 6 {
		c.Fatalf("Should have written 6 bytes, but wrote %v bytes", written)
	}
	// Make sure we Flush all the way ?
	writer.Flush()
	bw.Flush()
	if len(buf.Bytes()) != 6 {
		c.Fatalf("The buffer should contain 6 bytes ('foobar') but contains %v ('%v')", buf.Bytes(), string(buf.Bytes()))
	}
	// Reset the buffer
	buf.Reset()
	BufioWriter32KPool.Put(writer)
	// Try to write something
	if _, err = writer.Write([]byte("barfoo")); err != nil {
		c.Fatal(err)
	}
	// If we now try to flush it, it should panic (the writer is nil)
	// recover it
	defer func() {
		if r := recover(); r == nil {
			c.Fatal("Trying to flush the writter should have 'paniced', did not.")
		}
	}()
	writer.Flush()
}

type simpleWriterCloser struct {
	io.Writer
	closed bool
}

func (r *simpleWriterCloser) Close() error {
	r.closed = true
	return nil
}

func (s *DockerSuite) TestNewWriteCloserWrapperWithAWriteCloser(c *check.C) {
	buf := new(bytes.Buffer)
	bw := bufio.NewWriter(buf)
	sw := &simpleWriterCloser{
		Writer: new(bytes.Buffer),
		closed: false,
	}
	bw.Flush()
	writer := BufioWriter32KPool.NewWriteCloserWrapper(bw, sw)
	if writer == nil {
		c.Fatalf("BufioReaderPool should not return a nil writer.")
	}
	written, err := writer.Write([]byte("foobar"))
	if err != nil {
		c.Fatal(err)
	}
	if written != 6 {
		c.Fatalf("Should have written 6 bytes, but wrote %v bytes", written)
	}
	writer.Close()
	if !sw.closed {
		c.Fatalf("The ReaderCloser should have been closed, it is not.")
	}
}
