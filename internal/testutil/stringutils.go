package testutil

import "math/rand/v2"

const letters = "abcdefghijklmnopqrstuvwxyz"

// RandomAlpha generates a lowercase alphabetical random string with length n.
func RandomAlpha(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))] // #nosec G404 -- ignore "Use of weak random number generator (math/rand or math/rand/v2 instead of crypto/rand)"
	}
	return string(b)
}
