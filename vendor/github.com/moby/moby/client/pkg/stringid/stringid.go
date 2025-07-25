// Package stringid provides helper functions for dealing with string identifiers.
//
// It is similar to the package used by the daemon, but for presentational
// purposes in the client.
package stringid

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const (
	shortLen = 12
	fullLen  = 64
)

// TruncateID returns a shorthand version of a string identifier for presentation.
// For convenience, it accepts both digests ("sha256:xxxx") and IDs without an
// algorithm prefix. It truncates the algorithm (if any) before truncating the
// ID. The length of the truncated ID is currently fixed, but users should make
// no assumptions of this to not change; it is merely a prefix of the ID that
// provides enough uniqueness for common scenarios.
//
// Truncated IDs ("ID-prefixes") usually can be used to uniquely identify an
// object (such as a container or network), but collisions may happen, in
// which case an "ambiguous result" error is produced. In case of a collision,
// the caller should try with a longer prefix or the full-length ID.
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
func GenerateRandomID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // This shouldn't happen
	}
	id := hex.EncodeToString(b)
	return id
}
