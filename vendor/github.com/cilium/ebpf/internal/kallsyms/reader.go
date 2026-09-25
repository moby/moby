package kallsyms

import (
	"bufio"
	"bytes"
	"io"
)

// reader is a line and word-oriented reader built for reading /proc/kallsyms.
// It takes an io.Reader and iterates its contents line by line, then word by
// word.
//
// It's designed to allow partial reading of lines without paying the cost of
// allocating objects that will never be accessed, resulting in less work for
// the garbage collector.
type reader struct {
	s    *bufio.Scanner
	line []byte
	word []byte

	err error
}

func newReader(r io.Reader) *reader {
	return &reader{
		s: bufio.NewScanner(r),
	}
}

// Bytes returns the current word as a byte slice.
func (r *reader) Bytes() []byte {
	return r.word
}

// Text returns the output of Bytes as a string.
func (r *reader) Text() string {
	return string(r.Bytes())
}

// Line advances the reader to the next line in the input. Calling Line resets
// the current word, making [reader.Bytes] and [reader.Text] return empty
// values. Follow this up with a call to [reader.Word].
//
// Like [bufio.Scanner], [reader.Err] needs to be checked after Line returns
// false to determine if an error occurred during reading.
//
// Returns true if Line can be called again. Returns false if all lines in the
// input have been read.
func (r *reader) Line() bool {
	for r.s.Scan() {
		line := r.s.Bytes()
		if len(line) == 0 {
			continue
		}

		r.line = line
		r.word = nil

		return true
	}
	if err := r.s.Err(); err != nil {
		r.err = err
	}

	return false
}

// Word advances the reader to the next word in the current line.
//
// Returns true if a word is found and Word should be called again. Returns
// false when all words on the line have been read.
func (r *reader) Word() bool {
	line := bytes.TrimSpace(r.line)

	if len(line) == 0 {
		return false
	}

	var found bool
	r.word, r.line, found = bytes.Cut(line, []byte{' '})
	if !found {
		r.word, r.line, _ = bytes.Cut(line, []byte{'\t'})
	}
	return true
}

func (r *reader) Err() error {
	return r.err
}
