/*
Package filters provides tools for encoding a mapping of keys to a set of
multiple values.
*/
package filters

import (
	"encoding/json"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/versions"
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

// ToParamWithVersion encodes Args as a JSON string. If version is less than 1.22
// then the encoded format will use an older legacy format where the values are a
// list of strings, instead of a set.
//
// Deprecated: do not use in any new code; use ToJSON instead
func ToParamWithVersion(version string, a filters.Args) (string, error) {
	if a.Len() == 0 {
		return "", nil
	}

	if version != "" && versions.LessThan(version, "1.22") {
		buf, err := json.Marshal(convertArgsToSlice(a))
		return string(buf), err
	}

	return ToJSON(a)
}

// FromJSON decodes a JSON encoded string into Args
func FromJSON(p string) (filters.Args, error) {
	return filters.FromJSON(p)
}

func convertArgsToSlice(f filters.Args) map[string][]string {
	m := map[string][]string{}
	for _, key := range f.Keys() {
		m[key] = f.Get(key)
	}
	return m
}
