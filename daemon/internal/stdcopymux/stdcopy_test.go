package stdcopymux

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/moby/moby/client/pkg/stdcopy"
)

const startingBufLen = 32*1024 + 8 /* stdwriterPrefixLen */ + 1

func TestNewStdWriter(t *testing.T) {
	writer := NewStdWriter(io.Discard, stdcopy.Stdout)
	if writer == nil {
		t.Fatalf("NewStdWriter with an invalid StdType should not return nil.")
	}
}

func TestWriteWithUninitializedStdWriter(t *testing.T) {
	writer := stdWriter{
		Writer: nil,
		prefix: byte(stdcopy.Stdout),
	}
	n, err := writer.Write([]byte("Something here"))
	if n != 0 || err == nil {
		t.Fatalf("Should fail when given an incomplete or uninitialized StdWriter")
	}
}

func TestWriteWithNilBytes(t *testing.T) {
	writer := NewStdWriter(io.Discard, stdcopy.Stdout)
	n, err := writer.Write(nil)
	if err != nil {
		t.Fatalf("Shouldn't have fail when given no data")
	}
	if n > 0 {
		t.Fatalf("Write should have written 0 byte, but has written %d", n)
	}
}

func TestWrite(t *testing.T) {
	writer := NewStdWriter(io.Discard, stdcopy.Stdout)
	data := []byte("Test StdWrite.Write")
	n, err := writer.Write(data)
	if err != nil {
		t.Fatalf("Error while writing with StdWrite")
	}
	if n != len(data) {
		t.Fatalf("Write should have written %d byte but wrote %d.", len(data), n)
	}
}

type errWriter struct {
	n   int
	err error
}

func (f *errWriter) Write(buf []byte) (int, error) {
	return f.n, f.err
}

func TestWriteWithWriterError(t *testing.T) {
	expectedError := errors.New("expected")
	expectedReturnedBytes := 10
	writer := NewStdWriter(&errWriter{
		n:   stdWriterPrefixLen + expectedReturnedBytes,
		err: expectedError,
	}, stdcopy.Stdout)
	data := []byte("This won't get written, sigh")
	n, err := writer.Write(data)
	if !errors.Is(err, expectedError) {
		t.Fatalf("Didn't get expected error.")
	}
	if n != expectedReturnedBytes {
		t.Fatalf("Didn't get expected written bytes %d, got %d.",
			expectedReturnedBytes, n)
	}
}

func TestWriteDoesNotReturnNegativeWrittenBytes(t *testing.T) {
	writer := NewStdWriter(&errWriter{n: -1}, stdcopy.Stdout)
	data := []byte("This won't get written, sigh")
	actual, _ := writer.Write(data)
	if actual != 0 {
		t.Fatalf("Expected returned written bytes equal to 0, got %d", actual)
	}
}

func getSrcBuffer(stdOutBytes, stdErrBytes []byte) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	dstOut := NewStdWriter(buffer, stdcopy.Stdout)
	_, err := dstOut.Write(stdOutBytes)
	if err != nil {
		return buffer, err
	}
	dstErr := NewStdWriter(buffer, stdcopy.Stderr)
	_, err = dstErr.Write(stdErrBytes)
	return buffer, err
}

func TestStdCopyWriteAndRead(t *testing.T) {
	stdOutBytes := []byte(strings.Repeat("o", startingBufLen))
	stdErrBytes := []byte(strings.Repeat("e", startingBufLen))
	buffer, err := getSrcBuffer(stdOutBytes, stdErrBytes)
	if err != nil {
		t.Fatal(err)
	}
	written, err := stdcopy.StdCopy(io.Discard, io.Discard, buffer)
	if err != nil {
		t.Fatal(err)
	}
	expectedTotalWritten := len(stdOutBytes) + len(stdErrBytes)
	if written != int64(expectedTotalWritten) {
		t.Fatalf("Expected to have total of %d bytes written, got %d", expectedTotalWritten, written)
	}
}

type customReader struct {
	n            int
	err          error
	totalCalls   int
	correctCalls int
	src          *bytes.Buffer
}

func (f *customReader) Read(buf []byte) (int, error) {
	f.totalCalls++
	if f.totalCalls <= f.correctCalls {
		return f.src.Read(buf)
	}
	return f.n, f.err
}

func TestStdCopyReturnsErrorReadingHeader(t *testing.T) {
	expectedError := errors.New("error")
	reader := &customReader{
		err: expectedError,
	}
	written, err := stdcopy.StdCopy(io.Discard, io.Discard, reader)
	if written != 0 {
		t.Fatalf("Expected 0 bytes read, got %d", written)
	}
	if !errors.Is(err, expectedError) {
		t.Fatalf("Didn't get expected error")
	}
}

func TestStdCopyReturnsErrorReadingFrame(t *testing.T) {
	expectedError := errors.New("error")
	stdOutBytes := []byte(strings.Repeat("o", startingBufLen))
	stdErrBytes := []byte(strings.Repeat("e", startingBufLen))
	buffer, err := getSrcBuffer(stdOutBytes, stdErrBytes)
	if err != nil {
		t.Fatal(err)
	}
	reader := &customReader{
		correctCalls: 1,
		n:            stdWriterPrefixLen + 1,
		err:          expectedError,
		src:          buffer,
	}
	written, err := stdcopy.StdCopy(io.Discard, io.Discard, reader)
	if written != 0 {
		t.Fatalf("Expected 0 bytes read, got %d", written)
	}
	if !errors.Is(err, expectedError) {
		t.Fatalf("Didn't get expected error")
	}
}

func TestStdCopyDetectsCorruptedFrame(t *testing.T) {
	stdOutBytes := []byte(strings.Repeat("o", startingBufLen))
	stdErrBytes := []byte(strings.Repeat("e", startingBufLen))
	buffer, err := getSrcBuffer(stdOutBytes, stdErrBytes)
	if err != nil {
		t.Fatal(err)
	}
	reader := &customReader{
		correctCalls: 1,
		n:            stdWriterPrefixLen + 1,
		err:          io.EOF,
		src:          buffer,
	}
	written, err := stdcopy.StdCopy(io.Discard, io.Discard, reader)
	if written != startingBufLen {
		t.Fatalf("Expected %d bytes read, got %d", startingBufLen, written)
	}
	if err != nil {
		t.Fatal("Didn't get nil error")
	}
}

func TestStdCopyWithInvalidInputHeader(t *testing.T) {
	dstOut := NewStdWriter(io.Discard, stdcopy.Stdout)
	dstErr := NewStdWriter(io.Discard, stdcopy.Stderr)
	src := strings.NewReader("Invalid input")
	_, err := stdcopy.StdCopy(dstOut, dstErr, src)
	if err == nil {
		t.Fatal("StdCopy with invalid input header should fail.")
	}
}

func TestStdCopyWithCorruptedPrefix(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	src := bytes.NewReader(data)
	written, err := stdcopy.StdCopy(nil, nil, src)
	if err != nil {
		t.Fatalf("StdCopy should not return an error with corrupted prefix.")
	}
	if written != 0 {
		t.Fatalf("StdCopy should have written 0, but has written %d", written)
	}
}

func TestStdCopyReturnsWriteErrors(t *testing.T) {
	stdOutBytes := []byte(strings.Repeat("o", startingBufLen))
	stdErrBytes := []byte(strings.Repeat("e", startingBufLen))
	buffer, err := getSrcBuffer(stdOutBytes, stdErrBytes)
	if err != nil {
		t.Fatal(err)
	}
	expectedError := errors.New("expected")

	dstOut := &errWriter{err: expectedError}

	written, err := stdcopy.StdCopy(dstOut, io.Discard, buffer)
	if written != 0 {
		t.Fatalf("StdCopy should have written 0, but has written %d", written)
	}
	if !errors.Is(err, expectedError) {
		t.Fatalf("Didn't get expected error, got %v", err)
	}
}

func TestStdCopyDetectsNotFullyWrittenFrames(t *testing.T) {
	stdOutBytes := []byte(strings.Repeat("o", startingBufLen))
	stdErrBytes := []byte(strings.Repeat("e", startingBufLen))
	buffer, err := getSrcBuffer(stdOutBytes, stdErrBytes)
	if err != nil {
		t.Fatal(err)
	}
	dstOut := &errWriter{n: startingBufLen - 10}

	written, err := stdcopy.StdCopy(dstOut, io.Discard, buffer)
	if written != 0 {
		t.Fatalf("StdCopy should have return 0 written bytes, but returned %d", written)
	}
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("Didn't get expected io.ErrShortWrite error")
	}
}

// TestStdCopyReturnsErrorFromSystem tests that StdCopy correctly returns an
// error, when that error is muxed into the Systemerr stream.
func TestStdCopyReturnsErrorFromSystem(t *testing.T) {
	// write in the basic messages, just so there's some fluff in there
	stdOutBytes := []byte(strings.Repeat("o", startingBufLen))
	stdErrBytes := []byte(strings.Repeat("e", startingBufLen))
	buffer, err := getSrcBuffer(stdOutBytes, stdErrBytes)
	if err != nil {
		t.Fatal(err)
	}
	// add in an error message on the Systemerr stream
	systemErrBytes := []byte(strings.Repeat("S", startingBufLen))
	systemWriter := NewStdWriter(buffer, stdcopy.Systemerr)
	_, err = systemWriter.Write(systemErrBytes)
	if err != nil {
		t.Fatal(err)
	}

	// now copy and demux. we should expect an error containing the string we
	// wrote out
	_, err = stdcopy.StdCopy(io.Discard, io.Discard, buffer)
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), string(systemErrBytes)) {
		t.Fatal("expected error to contain message")
	}
}

func BenchmarkWrite(b *testing.B) {
	w := NewStdWriter(io.Discard, stdcopy.Stdout)
	data := []byte("Test line for testing stdwriter performance\n")
	data = bytes.Repeat(data, 100)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := w.Write(data); err != nil {
			b.Fatal(err)
		}
	}
}
