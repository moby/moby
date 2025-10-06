/*
Package filters provides tools for encoding a mapping of keys to a set of
multiple values.
*/
package filters

import (
	"encoding/json"
)

// Args stores a mapping of keys to a set of multiple values.
type Args struct {
	fields map[string]map[string]bool
}

// KeyValuePair are used to initialize a new Args
type KeyValuePair struct {
	Key   string
	Value string
}

// Arg creates a new KeyValuePair for initializing Args
func Arg(key, value string) KeyValuePair {
	return KeyValuePair{Key: key, Value: value}
}

// NewArgs returns a new Args populated with the initial args
func NewArgs(initialArgs ...KeyValuePair) Args {
	args := Args{fields: map[string]map[string]bool{}}
	for _, arg := range initialArgs {
		args.Add(arg.Key, arg.Value)
	}
	return args
}

// MarshalJSON returns a JSON byte representation of the Args
func (args Args) MarshalJSON() ([]byte, error) {
	if len(args.fields) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(args.fields)
}

// ToJSON returns the Args as a JSON encoded string
func ToJSON(a Args) (string, error) {
	if a.Len() == 0 {
		return "", nil
	}
	buf, err := json.Marshal(a)
	return string(buf), err
}

// Add a new value to the set of values
func (args Args) Add(key, value string) {
	if _, ok := args.fields[key]; ok {
		args.fields[key][value] = true
	} else {
		args.fields[key] = map[string]bool{value: true}
	}
}

// Len returns the number of keys in the mapping
func (args Args) Len() int {
	return len(args.fields)
}
