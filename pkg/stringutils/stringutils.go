// Package stringutils provides helper functions for dealing with strings.
package stringutils

import (
	"bytes"
	"math/rand"
	"strings"

	"github.com/docker/docker/pkg/random"
)

// GenerateRandomAlphaOnlyString generates an alphabetical random string with length n.
func GenerateRandomAlphaOnlyString(n int) string {
	// make a really long string
	letters := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[random.Rand.Intn(len(letters))]
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

// Truncate truncates a string to maxlen.
func Truncate(s string, maxlen int) string {
	if len(s) <= maxlen {
		return s
	}
	return s[:maxlen]
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
			// Replace literal ' with a close ', a \', and a open '
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
