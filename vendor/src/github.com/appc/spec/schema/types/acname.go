package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	valchars = `abcdefghijklmnopqrstuvwxyz0123456789.-/`
)

// ACName (an App-Container Name) is a format used by keys in different
// formats of the App Container Standard. An ACName is restricted to
// characters accepted by the DNS RFC[1] and "/"; all alphabetical characters
// must be lowercase only.
//
// [1] http://tools.ietf.org/html/rfc1123#page-13
type ACName string

func (n ACName) String() string {
	return string(n)
}

func (n *ACName) Set(s string) error {
	nn, err := NewACName(s)
	if err == nil {
		*n = *nn
	}
	return err
}

// Equals checks whether a given ACName is equal to this one.
func (n ACName) Equals(o ACName) bool {
	return strings.ToLower(string(n)) == strings.ToLower(string(o))
}

func (n ACName) Empty() bool {
	return n.String() == ""
}

// NewACName generates a new ACName from a string. If the given string is
// not a valid ACName, nil and an error are returned.
func NewACName(s string) (*ACName, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("ACName cannot be empty")
	}
	for _, c := range s {
		if !strings.ContainsRune(valchars, c) {
			msg := fmt.Sprintf("invalid char in ACName: %c", c)
			return nil, ACNameError(msg)
		}
	}
	return (*ACName)(&s), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (n *ACName) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	nn, err := NewACName(s)
	if err != nil {
		return err
	}
	*n = *nn
	return nil
}

// MarshalJSON implements the json.Marshaler interface
func (n *ACName) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.String())
}
