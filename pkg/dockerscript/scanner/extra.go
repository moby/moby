package scanner

import (
	"strings"
	"unicode"
)

// extra functions used to hijack the upstream text/scanner

func detectIdent(ch rune) bool {
	if unicode.IsLetter(ch) {
		return true
	}
	if unicode.IsDigit(ch) {
		return true
	}
	if strings.ContainsRune("_:/+-@%^.!=", ch) {
		return true
	}
	return false
}
