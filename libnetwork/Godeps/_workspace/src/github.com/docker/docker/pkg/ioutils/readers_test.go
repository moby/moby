package ioutils

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

func TestBufReader(t *testing.T) {
	reader, writer := io.Pipe()
	bufreader := NewBufReader(reader)

	// Write everything down to a Pipe
	// Usually, a pipe should block but because of the buffered reader,
	// the writes will go through
	done := make(chan bool)
	go func() {
		writer.Write([]byte("hello world"))
		writer.Close()
		done <- true
	}()

	// Drain the reader *after* everything has been written, just to verify
	// it is indeed buffering
	<-done
	output, err := ioutil.ReadAll(bufreader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, []byte("hello world")) {
		t.Error(string(output))
	}
}

type repeatedReader struct {
	readCount int
	maxReads  int
	data      []byte
}

func newRepeatedReader(max int, data []byte) *repeatedReader {
	return &repeatedReader{0, max, data}
}

func (r *repeatedReader) Read(p []byte) (int, error) {
	if r.readCount >= r.maxReads {
		return 0, io.EOF
	}
	r.readCount++
	n := copy(p, r.data)
	return n, nil
}

func testWithData(data []byte, reads int) {
	reader := newRepeatedReader(reads, data)
	bufReader := NewBufReader(reader)
	io.Copy(ioutil.Discard, bufReader)
}

func Benchmark1M10BytesReads(b *testing.B) {
	reads := 1000000
	readSize := int64(10)
	data := make([]byte, readSize)
	b.SetBytes(readSize * int64(reads))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testWithData(data, reads)
	}
}

func Benchmark1M1024BytesReads(b *testing.B) {
	reads := 1000000
	readSize := int64(1024)
	data := make([]byte, readSize)
	b.SetBytes(readSize * int64(reads))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testWithData(data, reads)
	}
}

func Benchmark10k32KBytesReads(b *testing.B) {
	reads := 10000
	readSize := int64(32 * 1024)
	data := make([]byte, readSize)
	b.SetBytes(readSize * int64(reads))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testWithData(data, reads)
	}
}
