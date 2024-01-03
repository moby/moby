package termtest // import "github.com/docker/docker/integration/internal/termtest"

import (
	"errors"
	"regexp"

	"github.com/Azure/go-ansiterm"
)

var stripOSC = regexp.MustCompile(`\x1b\][^\x1b\a]*(\x1b\\|\a)`)

// StripANSICommands attempts to strip ANSI console escape and control sequences
// from s, returning a string containing only the final printed characters which
// would be visible onscreen if the string was to be processed by a terminal
// emulator. Basic cursor positioning and screen erase control sequences are
// parsed and processed such that the output of simple CLI commands passed
// through a Windows Pseudoterminal and then this function yields the same
// string as if the output of those commands was redirected to a file.
//
// The only correct way to represent the result of processing ANSI console
// output would be a two-dimensional array of an emulated terminal's display
// buffer. That would be awkward to test against, so this function instead
// attempts to render to a one-dimensional string without extra padding. This is
// an inherently lossy process, and any attempts to render a string containing
// many cursor positioning commands are unlikely to yield satisfactory results.
// Handlers for several ANSI control sequences are also unimplemented; attempts
// to parse a string containing one will panic.
func StripANSICommands(s string, opts ...ansiterm.Option) (string, error) {
	// Work around https://github.com/Azure/go-ansiterm/issues/34
	s = stripOSC.ReplaceAllLiteralString(s, "")

	var h stringHandler
	p := ansiterm.CreateParser("Ground", &h, opts...)
	_, err := p.Parse([]byte(s))
	return h.String(), err
}

type stringHandler struct {
	ansiterm.AnsiEventHandler
	cursor int
	b      []byte
}

func (h *stringHandler) Print(b byte) error {
	if h.cursor == len(h.b) {
		h.b = append(h.b, b)
	} else {
		h.b[h.cursor] = b
	}
	h.cursor++
	return nil
}

func (h *stringHandler) Execute(b byte) error {
	switch b {
	case '\b':
		if h.cursor > 0 {
			if h.cursor == len(h.b) && h.b[h.cursor-1] == ' ' {
				h.b = h.b[:len(h.b)-1]
			}
			h.cursor--
		}
	case '\r', '\n':
		h.Print(b)
	}
	return nil
}

// Erase Display
func (h *stringHandler) ED(v int) error {
	switch v {
	case 1: // Erase from start to cursor.
		for i := 0; i < h.cursor; i++ {
			h.b[i] = ' '
		}
	case 2, 3: // Erase whole display.
		h.b = make([]byte, h.cursor)
		for i := range h.b {
			h.b[i] = ' '
		}
	default: // Erase from cursor to end of display.
		h.b = h.b[:h.cursor+1]
	}
	return nil
}

// CUrsor Position
func (h *stringHandler) CUP(x, y int) error {
	if x > 1 {
		return errors.New("termtest: cursor position not supported for X > 1")
	}
	if y > len(h.b) {
		for n := len(h.b) - y; n > 0; n-- {
			h.b = append(h.b, ' ')
		}
	}
	h.cursor = y - 1
	return nil
}

func (h stringHandler) DECTCEM(bool) error      { return nil } // Text Cursor Enable
func (h stringHandler) SGR(v []int) error       { return nil } // Set Graphics Rendition
func (h stringHandler) DA(attrs []string) error { return nil }
func (h stringHandler) Flush() error            { return nil }

func (h *stringHandler) String() string {
	return string(h.b)
}
