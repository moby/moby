package xml

import (
	"bytes"
	"unicode/utf8"

	"github.com/aws/smithy-go/traits"
)

type writer struct {
	buf *bytes.Buffer
}

func newWriter() *writer {
	return &writer{
		buf: bytes.NewBuffer(nil),
	}
}

type attr struct {
	name, value string
}

func (w *writer) writeChardata(s string) {
	escapeString(w.buf, s)
}

func (w *writer) writeStart(name string, ns *traits.XMLNamespace, attrs []attr) {
	w.buf.WriteByte('<')
	escapeString(w.buf, name)
	if ns != nil {
		w.buf.WriteString(" xmlns")
		if ns.Prefix != "" {
			w.buf.WriteByte(':')
			escapeString(w.buf, ns.Prefix)
		}
		w.buf.WriteString(`="`)
		escapeString(w.buf, ns.URI)
		w.buf.WriteByte('"')
	}
	for _, a := range attrs {
		w.buf.WriteByte(' ')
		escapeString(w.buf, a.name)
		w.buf.WriteString(`="`)
		escapeString(w.buf, a.value)
		w.buf.WriteByte('"')
	}
	w.buf.WriteByte('>')
}

func (w *writer) writeEnd(name string) {
	w.buf.WriteString("</")
	escapeString(w.buf, name)
	w.buf.WriteByte('>')
}

func (w *writer) writeInner(inner *writer) {
	w.buf.Write(inner.buf.Bytes())
}

func (w *writer) Bytes() []byte {
	return w.buf.Bytes()
}

// copied from smithy-go/encoding/xml/escape.go

var (
	escQuot     = []byte("&#34;")
	escApos     = []byte("&#39;")
	escAmp      = []byte("&amp;")
	escLT       = []byte("&lt;")
	escGT       = []byte("&gt;")
	escTab      = []byte("&#x9;")
	escNL       = []byte("&#xA;")
	escCR       = []byte("&#xD;")
	escFFFD     = []byte("\uFFFD")
	escNextLine = []byte("&#x85;")
	escLS       = []byte("&#x2028;")
)

func isInCharacterRange(r rune) bool {
	return r == 0x09 ||
		r == 0x0A ||
		r == 0x0D ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}

func escapeString(w *bytes.Buffer, s string) {
	var esc []byte
	last := 0
	for i := 0; i < len(s); {
		r, width := utf8.DecodeRuneInString(s[i:])
		i += width
		switch r {
		case '"':
			esc = escQuot
		case '\'':
			esc = escApos
		case '&':
			esc = escAmp
		case '<':
			esc = escLT
		case '>':
			esc = escGT
		case '\t':
			esc = escTab
		case '\n':
			esc = escNL
		case '\r':
			esc = escCR
		case '\u0085':
			esc = escNextLine
		case '\u2028':
			esc = escLS
		default:
			if !isInCharacterRange(r) || (r == 0xFFFD && width == 1) {
				esc = escFFFD
				break
			}
			continue
		}
		w.WriteString(s[last : i-width])
		w.Write(esc)
		last = i
	}
	w.WriteString(s[last:])
}
