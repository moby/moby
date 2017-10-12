// Package stringutils provides helper functions for dealing with strings.
package stringutils

import (
	"bytes"
	"math/rand"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/width"
)

// GenerateRandomAlphaOnlyString generates an alphabetical random string with length n.
func GenerateRandomAlphaOnlyString(n int) string {
	// make a really long string
	letters := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// GenerateRandomASCIIString generates an ASCII random string with length n.
func GenerateRandomASCIIString(n int) string {
	chars := "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"~!@#$%^&*()-_+={}[]\\|<,>.?/\"';:` "
	res := make([]byte, n)
	for i := 0; i < n; i++ {
		res[i] = chars[rand.Intn(len(chars))]
	}
	return string(res)
}

// Ellipsis truncates a string to fit within maxlen, and appends ellipsis (...).
// For maxlen of 3 and lower, no ellipsis is appended.
func Ellipsis(s string, maxlen int) string {
	byteLen := len(s)
	if byteLen == utf8.RuneCountInString(s) {
		r := []rune(s)
		if byteLen <= maxlen {
			return s
		}
		if maxlen <= 3 {
			return string(r[:maxlen])
		}
		return string(r[:maxlen-3]) + "..."
	}

	var display []int
	displayLen := 0
	rs := []rune(s)
	for _, r := range rs {
		//In a broad sense, wide characters include East Asian Wide, East Asian Fullwidth, and East Asian Ambiguous,
		//see http://unicode.org/reports/tr11/
		kind := width.LookupRune(r).Kind()
		if kind == width.EastAsianAmbiguous || kind == width.EastAsianWide || kind == width.EastAsianFullwidth {
			displayLen += 2
			display = append(display, displayLen)
		} else {
			displayLen++
			display = append(display, displayLen)
		}
	}
	if displayLen <= maxlen {
		return s
	}
	if maxlen <= 3 {
		for i := range display {
			if display[i] <= maxlen && display[i+1] > maxlen {
				return string(rs[:i+1])
			}
		}
	}
	for i := range display {
		if display[i] <= maxlen-3 && display[i+1] > maxlen-3 {
			s = string(rs[:i+1]) + "..."
		}
	}
	return s
}

// Truncate truncates a string to maxlen.
func Truncate(s string, maxlen int) string {
	r := []rune(s)
	if len(r) <= maxlen {
		return s
	}
	return string(r[:maxlen])
}

// InSlice tests whether a string is contained in a slice of strings or not.
// Comparison is case insensitive
func InSlice(slice []string, s string) bool {
	for _, ss := range slice {
		if strings.ToLower(s) == strings.ToLower(ss) {
			return true
		}
	}
	return false
}

func quote(word string, buf *bytes.Buffer) {
	// Bail out early for "simple" strings
	if word != "" && !strings.ContainsAny(word, "\\'\"`${[|&;<>()~*?! \t\n") {
		buf.WriteString(word)
		return
	}

	buf.WriteString("'")

	for i := 0; i < len(word); i++ {
		b := word[i]
		if b == '\'' {
			// Replace literal ' with a close ', a \', and an open '
			buf.WriteString("'\\''")
		} else {
			buf.WriteByte(b)
		}
	}

	buf.WriteString("'")
}

// ShellQuoteArguments takes a list of strings and escapes them so they will be
// handled right when passed as arguments to a program via a shell
func ShellQuoteArguments(args []string) string {
	var buf bytes.Buffer
	for i, arg := range args {
		if i != 0 {
			buf.WriteByte(' ')
		}
		quote(arg, &buf)
	}
	return buf.String()
}
