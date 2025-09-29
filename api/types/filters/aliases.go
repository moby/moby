/*
Package filters provides tools for encoding a mapping of keys to a set of
multiple values.
*/
package filters

import (
	"github.com/moby/moby/api/types/filters"
)

// Args stores a mapping of keys to a set of multiple values.
type Args = filters.Args

// KeyValuePair are used to initialize a new Args
type KeyValuePair = filters.KeyValuePair

// Arg creates a new KeyValuePair for initializing Args
func Arg(key, value string) filters.KeyValuePair {
	return filters.Arg(key, value)
}

// NewArgs returns a new Args populated with the initial args
func NewArgs(initialArgs ...filters.KeyValuePair) filters.Args {
	return filters.NewArgs(initialArgs...)
}

// ToJSON returns the Args as a JSON encoded string
func ToJSON(a Args) (string, error) {
	return filters.ToJSON(a)
}

// FromJSON decodes a JSON encoded string into Args
func FromJSON(p string) (filters.Args, error) {
	return filters.FromJSON(p)
}
