package toml

import (
	"errors"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
)

// DecodeError represents an error encountered during the parsing or decoding
// of a TOML document.
//
// In addition to the error message, it contains the position in the document
// where it happened, as well as a human-readable representation that shows
// where the error occurred in the document.
type DecodeError struct {
	message string
	line    int
	column  int
	key     Key

	human string
}

// StrictMissingError occurs in a TOML document that does not have a
// corresponding field in the target value. It contains all the missing fields
// in Errors.
//
// Emitted by Decoder when DisallowUnknownFields() was called.
type StrictMissingError struct {
	// One error per field that could not be found.
	Errors []DecodeError
}

// Error returns the canonical string for this error.
func (s *StrictMissingError) Error() string {
	return "strict mode: fields in the document are missing in the target struct"
}

// String returns a human readable description of all errors.
func (s *StrictMissingError) String() string {
	var buf strings.Builder

	for i, e := range s.Errors {
		if i > 0 {
			buf.WriteString("\n---\n")
		}
		buf.WriteString(e.String())
	}

	return buf.String()
}

// Unwrap returns wrapped decode errors
//
// Implements errors.Join() interface.
func (s *StrictMissingError) Unwrap() []error {
	errs := make([]error, len(s.Errors))
	for i := range s.Errors {
		errs[i] = &s.Errors[i]
	}
	return errs
}

// Key represents a TOML key as a sequence of key parts.
type Key []string

// Error returns the error message contained in the DecodeError.
func (e *DecodeError) Error() string {
	return "toml: " + e.message
}

// String returns the human-readable contextualized error. This string is
// multi-line.
func (e *DecodeError) String() string {
	return e.human
}

// Position returns the (line, column) pair indicating where the error
// occurred in the document. Positions are 1-indexed.
func (e *DecodeError) Position() (row int, column int) {
	return e.line, e.column
}

// Key that was being processed when the error occurred.
func (e *DecodeError) Key() Key {
	return e.key
}

// wrapDecodeError creates a DecodeError from a ParserError. The highlight of
// the ParserError needs to be a subslice of the document.
func wrapDecodeError(document []byte, de *unstable.ParserError) *DecodeError {
	if de == nil {
		return nil
	}
	return newDecodeError(document, de.Highlight, de.Key, de.Message)
}

// newDecodeError creates a DecodeError pointing at the given highlight, which
// needs to be a subslice of the document.
func newDecodeError(document []byte, highlight []byte, key Key, message string) *DecodeError {
	offset := subsliceOffset(document, highlight)

	errLineIdx, errColumn := positionAt(document, offset)

	human := buildHumanContext(document, errLineIdx, errColumn, len(highlight), message)

	return &DecodeError{
		message: message,
		line:    errLineIdx + 1,
		column:  errColumn,
		key:     key,
		human:   human,
	}
}

// subsliceOffset returns the offset of the subslice b within the document.
func subsliceOffset(document, b []byte) int {
	// Highlights are subslices of the document, which means they share the
	// same backing array, and their capacity counts the bytes between their
	// start and the end of the backing array.
	offset := cap(document) - cap(b)
	if offset < 0 || offset+len(b) > len(document) {
		panic(errors.New("highlight is not a subslice of the document"))
	}
	return offset
}

// positionAt returns the 0-indexed line and the 1-indexed column of the given
// offset in the document.
func positionAt(document []byte, offset int) (lineIdx int, column int) {
	lineStart := 0
	for i := 0; i < offset; i++ {
		if document[i] == '\n' {
			lineIdx++
			lineStart = i + 1
		}
	}
	return lineIdx, offset - lineStart + 1
}

// docLines splits the document into lines, removing the trailing newline
// characters.
func docLines(document []byte) []string {
	s := string(document)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

// buildHumanContext renders the human-readable multi-line context of an
// error: a window of up to 3 lines before and after the error line, with
// the error position underlined.
func buildHumanContext(document []byte, errLineIdx, errColumn, highlightLen int, message string) string {
	lines := docLines(document)

	const window = 3
	firstIdx := errLineIdx - window
	if firstIdx < 0 {
		firstIdx = 0
	}
	lastIdx := errLineIdx + window
	if lastIdx > len(lines)-1 {
		lastIdx = len(lines) - 1
	}
	// Empty lines at the edges of the window are dropped, unless the error
	// is about that very position.
	for firstIdx < errLineIdx && lines[firstIdx] == "" {
		firstIdx++
	}
	for lastIdx > errLineIdx && lines[lastIdx] == "" {
		lastIdx--
	}

	// Width of the column of line numbers.
	width := len(strconv.Itoa(lastIdx + 1))

	var buf strings.Builder

	writeLine := func(idx int) {
		number := strconv.Itoa(idx + 1)
		for i := len(number); i < width; i++ {
			buf.WriteByte(' ')
		}
		buf.WriteString(number)
		buf.WriteByte('|')
		if len(lines[idx]) > 0 {
			buf.WriteByte(' ')
			buf.WriteString(lines[idx])
		}
		buf.WriteByte('\n')
	}

	for idx := firstIdx; idx <= errLineIdx; idx++ {
		writeLine(idx)
	}

	// Underline the error.
	for i := 0; i < width; i++ {
		buf.WriteByte(' ')
	}
	buf.WriteString("| ")
	for i := 1; i < errColumn; i++ {
		buf.WriteByte(' ')
	}
	// The highlight cannot extend past the end of its line.
	tildes := highlightLen
	if errLineIdx < len(lines) {
		if avail := len(lines[errLineIdx]) - errColumn + 1; tildes > avail {
			tildes = avail
		}
	}
	if tildes < 1 {
		tildes = 1
	}
	for i := 0; i < tildes; i++ {
		buf.WriteByte('~')
	}
	if message != "" {
		buf.WriteByte(' ')
		buf.WriteString(message)
	}
	buf.WriteByte('\n')

	for idx := errLineIdx + 1; idx <= lastIdx; idx++ {
		writeLine(idx)
	}

	return strings.TrimSuffix(buf.String(), "\n")
}
