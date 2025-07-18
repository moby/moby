// Package stringid provides helper functions for dealing with string identifiers
package stringid

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	shortLen = 12
	fullLen  = 64
	padding  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // Padding added to the UUID to make it produce the same length as the old ID format.
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
	uuidv7 := uuid.Must(uuid.NewV7()).String()
	return strings.ReplaceAll(uuidv7, "-", "") + padding
}

// UUIDv7 format (with hyphens):
//
//	UUIDv7: 01956c20-e1a6-73bd-959f-f5d3bcd6bd77 36 chars
//	UUIDv7: 01956c20e1a673bd959ff5d3bcd6bd77     32 chars (with hyphens removed)
//	        │           ││
//	        │           │└────────────────────── Random Data (Remaining Bits, 19 characters)
//	        │           └─────────────────────── UUID Version (v7, 4-bit, 1 character)
//	        └─────────────────────────────────── Timestamp (Milliseconds since Unix epoch, 48 bit, 12 characters)
const shortIDUUIDSuffix = "7aaaaaaaaaaaaaaaaaaa"

func toUUID(id string) (uuid.UUID, error) {
	if len(id) == shortLen {
		// short ID is only the timestamp: append the UUID version (v7),
		// and a fixed "random" suffix" to allow parsing as UUIDv7
		return uuid.Parse(id + shortIDUUIDSuffix)
	}
	return uuid.Parse(id[:32])
}

func getTimestamp(id string) time.Time {
	uuidV7, err := toUUID(id)
	if err != nil {
		panic(err)
	}
	return time.Unix(uuidV7.Time().UnixTime())
}
