/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package filters

import (
	"unicode"
	"unicode/utf8"
)

const (
	tokenEOF = -(iota + 1)
	tokenQuoted
	tokenValue
	tokenField
	tokenSeparator
	tokenOperator
	tokenIllegal
)

type token rune

func (t token) String() string {
	switch t {
	case tokenEOF:
		return "EOF"
	case tokenQuoted:
		return "Quoted"
	case tokenValue:
		return "Value"
	case tokenField:
		return "Field"
	case tokenSeparator:
		return "Separator"
	case tokenOperator:
		return "Operator"
	case tokenIllegal:
		return "Illegal"
	}

	return string(t)
}

func (t token) GoString() string {
	return "token" + t.String()
}

type scanner struct {
	input string
	pos   int
	ppos  int // bounds the current rune in the string
	value bool
	err   string
}

func (s *scanner) init(input string) {
	s.input = input
	s.pos = 0
	s.ppos = 0
}

func (s *scanner) next() rune {
	if s.pos >= len(s.input) {
		return tokenEOF
	}
	s.pos = s.ppos

	r, w := utf8.DecodeRuneInString(s.input[s.ppos:])
	s.ppos += w
	if r == utf8.RuneError {
		if w > 0 {
			s.error("rune error")
			return tokenIllegal
		}
		return tokenEOF
	}

	if r == 0 {
		s.error("unexpected null")
		return tokenIllegal
	}

	return r
}

func (s *scanner) peek() rune {
	pos := s.pos
	ppos := s.ppos
	ch := s.next()
	s.pos = pos
	s.ppos = ppos
	return ch
}

func (s *scanner) scan() (nextp int, tk token, text string) {
	var (
		ch  = s.next()
		pos = s.pos
	)

chomp:
	switch {
	case ch == tokenEOF:
	case ch == tokenIllegal:
	case isQuoteRune(ch):
		if !s.scanQuoted(ch) {
			return pos, tokenIllegal, s.input[pos:s.ppos]
		}
		return pos, tokenQuoted, s.input[pos:s.ppos]
	case isSeparatorRune(ch):
		s.value = false
		return pos, tokenSeparator, s.input[pos:s.ppos]
	case isOperatorRune(ch):
		s.scanOperator()
		s.value = true
		return pos, tokenOperator, s.input[pos:s.ppos]
	case unicode.IsSpace(ch):
		// chomp
		ch = s.next()
		pos = s.pos
		goto chomp
	case s.value:
		s.scanValue()
		s.value = false
		return pos, tokenValue, s.input[pos:s.ppos]
	case isFieldRune(ch):
		s.scanField()
		return pos, tokenField, s.input[pos:s.ppos]
	}

	return s.pos, token(ch), ""
}

func (s *scanner) scanField() {
	for {
		ch := s.peek()
		if !isFieldRune(ch) {
			break
		}
		s.next()
	}
}

func (s *scanner) scanOperator() {
	for {
		ch := s.peek()
		switch ch {
		case '=', '!', '~':
			s.next()
		default:
			return
		}
	}
}

func (s *scanner) scanValue() {
	for {
		ch := s.peek()
		if !isValueRune(ch) {
			break
		}
		s.next()
	}
}

func (s *scanner) scanQuoted(quote rune) bool {
	var illegal bool
	ch := s.next() // read character after quote
	for ch != quote {
		if ch == '\n' || ch < 0 {
			s.error("quoted literal not terminated")
			return false
		}
		if ch == '\\' {
			var legal bool
			ch, legal = s.scanEscape(quote)
			if !legal {
				illegal = true
			}
		} else {
			ch = s.next()
		}
	}
	return !illegal
}

func (s *scanner) scanEscape(quote rune) (ch rune, legal bool) {
	ch = s.next() // read character after '/'
	switch ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', quote:
		// nothing to do
		ch = s.next()
		legal = true
	case '0', '1', '2', '3', '4', '5', '6', '7':
		ch, legal = s.scanDigits(ch, 8, 3)
	case 'x':
		ch, legal = s.scanDigits(s.next(), 16, 2)
	case 'u':
		ch, legal = s.scanDigits(s.next(), 16, 4)
	case 'U':
		ch, legal = s.scanDigits(s.next(), 16, 8)
	default:
		s.error("illegal escape sequence")
	}
	return
}

func (s *scanner) scanDigits(ch rune, base, n int) (rune, bool) {
	for n > 0 && digitVal(ch) < base {
		ch = s.next()
		n--
	}
	if n > 0 {
		s.error("illegal numeric escape sequence")
		return ch, false
	}
	return ch, true
}

func (s *scanner) error(msg string) {
	if s.err == "" {
		s.err = msg
	}
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= ch && ch <= 'f':
		return int(ch - 'a' + 10)
	case 'A' <= ch && ch <= 'F':
		return int(ch - 'A' + 10)
	}
	return 16 // larger than any legal digit val
}

func isFieldRune(r rune) bool {
	return (r == '_' || isAlphaRune(r) || isDigitRune(r))
}

func isAlphaRune(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}

func isDigitRune(r rune) bool {
	return r >= '0' && r <= '9'
}

func isOperatorRune(r rune) bool {
	switch r {
	case '=', '!', '~':
		return true
	}

	return false
}

func isQuoteRune(r rune) bool {
	switch r {
	case '/', '|', '"': // maybe add single quoting?
		return true
	}

	return false
}

func isSeparatorRune(r rune) bool {
	switch r {
	case ',', '.':
		return true
	}

	return false
}

func isValueRune(r rune) bool {
	return r != ',' && !unicode.IsSpace(r) &&
		(unicode.IsLetter(r) ||
			unicode.IsDigit(r) ||
			unicode.IsNumber(r) ||
			unicode.IsGraphic(r) ||
			unicode.IsPunct(r))
}
