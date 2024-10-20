// Package stringid provides helper functions for dealing with string identifiers
package stringid // import "github.com/docker/docker/pkg/stringid"

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const (
	shortLen = 12
	fullLen  = 64
)

// TruncateID returns a shorthand version of a string identifier for convenience.
// A collision with other shorthands is very unlikely, but possible.
// In case of a collision a lookup with TruncIndex.Get() will fail, and the caller
// will need to use a longer prefix, or the full-length Id.
func TruncateID(id string) string {
	if i := strings.IndexRune(id, ':'); i >= 0 {
		id = id[i+1:]
	}
	if len(id) > shortLen {
		id = id[:shortLen]
	}
	return id
}

// GenerateRandomID returns a unique, 64-character ID consisting of a-z, 0-9.
// It guarantees that the ID, when truncated ([TruncateID]) does not consist
// of numbers only, so that the truncated ID can be used as hostname for
// containers.
func GenerateRandomID() string {
	b := make([]byte, 32)
	for {
		if _, err := rand.Read(b); err != nil {
			panic(err) // This shouldn't happen
		}
		id := hex.EncodeToString(b)

		// make sure that the truncated ID does not consist of only numeric
		// characters, as it's used as default hostname for containers.
		//
		// See:
		// - https://github.com/moby/moby/issues/3869
		// - https://bugzilla.redhat.com/show_bug.cgi?id=1059122
		if allNum(id[:shortLen]) {
			// all numbers; try again
			continue
		}
		return id
	}
}

// allNum checks whether id consists of only numbers (0-9).
func allNum(id string) bool {
	for _, c := range []byte(id) {
		if c > '9' || c < '0' {
			return false
		}
	}
	return true
}
