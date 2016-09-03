package ioutils

import (
	"bytes"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestFixedBufferWrite(c *check.C) {
	buf := &fixedBuffer{buf: make([]byte, 0, 64)}
	n, err := buf.Write([]byte("hello"))
	if err != nil {
		c.Fatal(err)
	}

	if n != 5 {
		c.Fatalf("expected 5 bytes written, got %d", n)
	}

	if string(buf.buf[:5]) != "hello" {
		c.Fatalf("expected \"hello\", got %q", string(buf.buf[:5]))
	}

	n, err = buf.Write(bytes.Repeat([]byte{1}, 64))
	if err != errBufferFull {
		c.Fatalf("expected errBufferFull, got %v - %v", err, buf.buf[:64])
	}
}

func (s *DockerSuite) TestFixedBufferRead(c *check.C) {
	buf := &fixedBuffer{buf: make([]byte, 0, 64)}
	if _, err := buf.Write([]byte("hello world")); err != nil {
		c.Fatal(err)
	}

	b := make([]byte, 5)
	n, err := buf.Read(b)
	if err != nil {
		c.Fatal(err)
	}

	if n != 5 {
		c.Fatalf("expected 5 bytes read, got %d - %s", n, buf.String())
	}

	if string(b) != "hello" {
		c.Fatalf("expected \"hello\", got %q", string(b))
	}

	n, err = buf.Read(b)
	if err != nil {
		c.Fatal(err)
	}

	if n != 5 {
		c.Fatalf("expected 5 bytes read, got %d", n)
	}

	if string(b) != " worl" {
		c.Fatalf("expected \" worl\", got %s", string(b))
	}

	b = b[:1]
	n, err = buf.Read(b)
	if err != nil {
		c.Fatal(err)
	}

	if n != 1 {
		c.Fatalf("expected 1 byte read, got %d - %s", n, buf.String())
	}

	if string(b) != "d" {
		c.Fatalf("expected \"d\", got %s", string(b))
	}
}
