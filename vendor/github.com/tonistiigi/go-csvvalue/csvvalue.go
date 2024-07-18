// Package csvvalue provides an efficient parser for a single line CSV value.
// It is more efficient than the standard library csv package for parsing many
// small values. For multi-line CSV parsing, the standard library is recommended.
package csvvalue

import (
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

var errInvalidDelim = errors.New("csv: invalid field or comment delimiter")

var defaultParser = NewParser()

// Fields parses the line with default parser and returns
// slice of fields for the record. If dst is nil, a new slice is allocated.
func Fields(inp string, dst []string) ([]string, error) {
	return defaultParser.Fields(inp, dst)
}

// Parser is a CSV parser for a single line value.
type Parser struct {
	Comma            rune
	LazyQuotes       bool
	TrimLeadingSpace bool
}

// NewParser returns a new Parser with default settings.
func NewParser() *Parser {
	return &Parser{Comma: ','}
}

// Fields parses the line and returns slice of fields for the record.
// If dst is nil, a new slice is allocated.
// For backward compatibility, a trailing newline is allowed.
func (r *Parser) Fields(line string, dst []string) ([]string, error) {
	if !validDelim(r.Comma) {
		return nil, errInvalidDelim
	}

	if cap(dst) == 0 {
		// imprecise estimate, strings.Count is fast
		dst = make([]string, 0, 1+strings.Count(line, string(r.Comma)))
	} else {
		dst = dst[:0]
	}

	const quoteLen = len(`"`)
	var (
		pos      int
		commaLen = utf8.RuneLen(r.Comma)
		trim     = r.TrimLeadingSpace
	)

	// allow trailing newline for compatibility
	if n := len(line); n > 0 && line[n-1] == '\n' {
		if n > 1 && line[n-2] == '\r' {
			line = line[:n-2]
		} else {
			line = line[:n-1]
		}
	}

	if len(line) == 0 {
		return nil, io.EOF
	}

parseField:
	for {
		if trim {
			i := strings.IndexFunc(line, func(r rune) bool {
				return !unicode.IsSpace(r)
			})
			if i < 0 {
				i = len(line)
			}
			line = line[i:]
			pos += i
		}
		if len(line) == 0 || line[0] != '"' {
			// Non-quoted string field
			i := strings.IndexRune(line, r.Comma)
			var field string
			if i >= 0 {
				field = line[:i]
			} else {
				field = line
			}
			// Check to make sure a quote does not appear in field.
			if !r.LazyQuotes {
				if j := strings.IndexRune(field, '"'); j >= 0 {
					return nil, parseErr(pos+j, csv.ErrBareQuote)
				}
			}
			dst = append(dst, field)
			if i >= 0 {
				line = line[i+commaLen:]
				pos += i + commaLen
				continue
			}
			break
		}
		// Quoted string field
		line = line[quoteLen:]
		pos += quoteLen
		halfOpen := false
		for {
			i := strings.IndexRune(line, '"')
			if i >= 0 {
				// Hit next quote.
				if !halfOpen {
					dst = append(dst, line[:i])
				} else {
					appendToLast(dst, line[:i])
				}
				halfOpen = false
				line = line[i+quoteLen:]
				pos += i + quoteLen
				switch rn := nextRune(line); {
				case rn == '"':
					// `""` sequence (append quote).
					appendToLast(dst, "\"")
					line = line[quoteLen:]
					pos += quoteLen
					halfOpen = true
				case rn == r.Comma:
					// `",` sequence (end of field).
					line = line[commaLen:]
					pos += commaLen
					continue parseField
				case len(line) == 0:
					break parseField
				case r.LazyQuotes:
					// `"` sequence (bare quote).
					appendToLast(dst, "\"")
					halfOpen = true
				default:
					// `"*` sequence (invalid non-escaped quote).
					return nil, parseErr(pos-quoteLen, csv.ErrQuote)
				}
			} else {
				if !r.LazyQuotes {
					return nil, parseErr(pos, csv.ErrQuote)
				}
				// Hit end of line (copy all data so far).
				dst = append(dst, line)
				break parseField
			}
		}
	}
	return dst, nil
}

func validDelim(r rune) bool {
	return r != 0 && r != '"' && r != '\r' && r != '\n' && utf8.ValidRune(r) && r != utf8.RuneError
}

func appendToLast(dst []string, s string) {
	dst[len(dst)-1] += s
}

func nextRune(b string) rune {
	r, _ := utf8.DecodeRuneInString(b)
	return r
}

func parseErr(pos int, err error) error {
	return &csv.ParseError{StartLine: 1, Line: 1, Column: pos + 1, Err: err}
}
