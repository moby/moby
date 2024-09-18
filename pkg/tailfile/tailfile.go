// Package tailfile provides helper functions to read the nth lines of any
// ReadSeeker.
package tailfile // import "github.com/docker/docker/pkg/tailfile"

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
)

const blockSize = 1024

var eol = []byte("\n")

// ErrNonPositiveLinesNumber is an error returned if the lines number was negative.
var ErrNonPositiveLinesNumber = errors.New("The number of lines to extract from the file must be positive")

// TailFile returns last n lines of the passed in file.
func TailFile(f *os.File, n int) ([][]byte, error) {
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	rAt := io.NewSectionReader(f, 0, size)
	r, nLines, err := NewTailReader(context.Background(), rAt, n)
	if err != nil {
		return nil, err
	}

	buf := make([][]byte, 0, nLines)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		buf = append(buf, scanner.Bytes())
	}
	return buf, nil
}

// SizeReaderAt is an interface used to get a ReaderAt as well as the size of the underlying reader.
// Note that the size of the underlying reader should not change when using this interface.
type SizeReaderAt interface {
	io.ReaderAt
	Size() int64
}

// NewTailReader scopes the passed in reader to just the last N lines passed in
func NewTailReader(ctx context.Context, r SizeReaderAt, reqLines int) (*io.SectionReader, int, error) {
	return NewTailReaderWithDelimiter(ctx, r, reqLines, eol)
}

// NewTailReaderWithDelimiter scopes the passed in reader to just the last N lines passed in
// In this case a "line" is defined by the passed in delimiter.
//
// Delimiter lengths should be generally small, no more than 12 bytes
func NewTailReaderWithDelimiter(ctx context.Context, r SizeReaderAt, reqLines int, delimiter []byte) (*io.SectionReader, int, error) {
	if reqLines < 1 {
		return nil, 0, ErrNonPositiveLinesNumber
	}
	if len(delimiter) == 0 {
		return nil, 0, errors.New("must provide a delimiter")
	}
	var (
		size      = r.Size()
		tailStart int64
		tailEnd   = size
		found     int
	)

	if int64(len(delimiter)) >= size {
		return io.NewSectionReader(bytes.NewReader(nil), 0, 0), 0, nil
	}

	scanner := newScanner(r, delimiter)
	for scanner.Scan(ctx) {
		if err := scanner.Err(); err != nil {
			return nil, 0, scanner.Err()
		}

		found++
		if found == 1 {
			tailEnd = scanner.End()
		}
		if found == reqLines {
			break
		}
	}

	tailStart = scanner.Start(ctx)

	if found == 0 {
		return io.NewSectionReader(bytes.NewReader(nil), 0, 0), 0, nil
	}

	if found < reqLines && tailStart != 0 {
		tailStart = 0
	}
	return io.NewSectionReader(r, tailStart, tailEnd-tailStart), found, nil
}

func newScanner(r SizeReaderAt, delim []byte) *scanner {
	size := r.Size()
	readSize := blockSize
	if readSize > int(size) {
		readSize = int(size)
	}
	// silly case...
	if len(delim) >= readSize/2 {
		readSize = len(delim)*2 + 2
	}

	return &scanner{
		r:     r,
		pos:   size,
		buf:   make([]byte, readSize),
		delim: delim,
	}
}

type scanner struct {
	r     SizeReaderAt
	pos   int64
	buf   []byte
	delim []byte
	err   error
	idx   int
}

func (s *scanner) Start(ctx context.Context) int64 {
	if s.idx > 0 {
		idx := bytes.LastIndex(s.buf[:s.idx], s.delim)
		if idx >= 0 {
			return s.pos + int64(idx) + int64(len(s.delim))
		}
	}

	// slow path
	buf := make([]byte, len(s.buf))
	copy(buf, s.buf)

	readAhead := &scanner{
		r:     s.r,
		pos:   s.pos,
		delim: s.delim,
		idx:   s.idx,
		buf:   buf,
	}

	if !readAhead.Scan(ctx) {
		return 0
	}
	return readAhead.End()
}

func (s *scanner) End() int64 {
	return s.pos + int64(s.idx) + int64(len(s.delim))
}

func (s *scanner) Err() error {
	return s.err
}

func (s *scanner) Scan(ctx context.Context) bool {
	if s.err != nil {
		return false
	}

	for {
		select {
		case <-ctx.Done():
			s.err = ctx.Err()
			return false
		default:
		}

		idx := s.idx - len(s.delim)
		if idx < 0 {
			readSize := int(s.pos)
			if readSize > len(s.buf) {
				readSize = len(s.buf)
			}

			if readSize < len(s.delim) {
				return false
			}

			offset := s.pos - int64(readSize)
			n, err := s.r.ReadAt(s.buf[:readSize], offset)
			if err != nil && err != io.EOF {
				s.err = err
				return false
			}

			s.pos -= int64(n)
			idx = n
		}

		s.idx = bytes.LastIndex(s.buf[:idx], s.delim)
		if s.idx >= 0 {
			return true
		}

		if len(s.delim) > 1 && s.pos > 0 {
			// in this case, there may be a partial delimiter at the front of the buffer, so set the position forward
			// up to the maximum size partial that could be there so it can be read again in the next iteration with any
			// potential remainder.
			// An example where delimiter is `####`:
			// [##asdfqwerty]
			//    ^
			// This resets the position to where the arrow is pointing.
			// It could actually check if a partial exists and at the front, but that is pretty similar to the indexing
			// code above though a bit more complex since each byte has to be checked (`len(delimiter)-1`) factorial).
			// It's much simpler and cleaner to just re-read `len(delimiter)-1` bytes again.
			s.pos += int64(len(s.delim)) - 1
		}
	}
}
