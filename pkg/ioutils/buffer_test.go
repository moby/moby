package ioutils

import (
	"bytes"
	"testing"
)

func TestFixedBufferWrite(t *testing.T) {
	buf := &fixedBuffer{buf: make([]byte, 0, 64)}
	n, err := buf.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	if n != 5 {
		t.Fatalf("expected 5 bytes written, got %d", n)
	}

	if string(buf.buf[:5]) != "hello" {
		t.Fatalf("expected \"hello\", got %q", string(buf.buf[:5]))
	}

	n, err = buf.Write(bytes.Repeat([]byte{1}, 64))
	if n != 59 {
		t.Fatalf("expected 59 bytes written before buffer is full, got %d", n)
	}
	if err != errBufferFull {
		t.Fatalf("expected errBufferFull, got %v - %v", err, buf.buf[:64])
	}
}

func TestFixedBufferRead(t *testing.T) {
	buf := &fixedBuffer{buf: make([]byte, 0, 64)}
	if _, err := buf.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 5)
	n, err := buf.Read(b)
	if err != nil {
		t.Fatal(err)
	}

	if n != 5 {
		t.Fatalf("expected 5 bytes read, got %d - %s", n, buf.String())
	}

	if string(b) != "hello" {
		t.Fatalf("expected \"hello\", got %q", string(b))
	}

	n, err = buf.Read(b)
	if err != nil {
		t.Fatal(err)
	}

	if n != 5 {
		t.Fatalf("expected 5 bytes read, got %d", n)
	}

	if string(b) != " worl" {
		t.Fatalf("expected \" worl\", got %s", string(b))
	}

	b = b[:1]
	n, err = buf.Read(b)
	if err != nil {
		t.Fatal(err)
	}

	if n != 1 {
		t.Fatalf("expected 1 byte read, got %d - %s", n, buf.String())
	}

	if string(b) != "d" {
		t.Fatalf("expected \"d\", got %s", string(b))
	}
}
