// Package stringid provides helper functions for dealing with string identifiers
package stringid // import "github.com/moby/moby/pkg/stringid"

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const shortLen = 12

var (
	validShortID = regexp.MustCompile("^[a-f0-9]{12}$")
	validHex     = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// IsShortID determines if an arbitrary string *looks like* a short ID.
func IsShortID(id string) bool {
	return validShortID.MatchString(id)
}

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

// GenerateRandomID returns a unique id.
func GenerateRandomID() string {
	b := make([]byte, 32)
	for {
		if _, err := rand.Read(b); err != nil {
			panic(err) // This shouldn't happen
		}
		id := hex.EncodeToString(b)
		// if we try to parse the truncated for as an int and we don't have
		// an error then the value is all numeric and causes issues when
		// used as a hostname. ref #3869
		if _, err := strconv.ParseInt(TruncateID(id), 10, 64); err == nil {
			continue
		}
		return id
	}
}

// ValidateID checks whether an ID string is a valid image ID.
func ValidateID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return fmt.Errorf("image ID %q is invalid", id)
	}
	return nil
}
