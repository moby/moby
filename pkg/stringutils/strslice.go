package stringutils

import (
	"encoding/json"
	"strings"
)

// StrSlice representes a string or an array of strings.
// We need to override the json decoder to accept both options.
type StrSlice struct {
	parts []string
}

// MarshalJSON Marshals (or serializes) the StrSlice into the json format.
// This method is needed to implement json.Marshaller.
func (e *StrSlice) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte{}, nil
	}
	return json.Marshal(e.Slice())
}

// UnmarshalJSON decodes the byte slice whether it's a string or an array of strings.
// This method is needed to implement json.Unmarshaler.
func (e *StrSlice) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	p := make([]string, 0, 1)
	if err := json.Unmarshal(b, &p); err != nil {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		p = append(p, s)
	}

	e.parts = p
	return nil
}

// Len returns the number of parts of the StrSlice.
func (e *StrSlice) Len() int {
	if e == nil {
		return 0
	}
	return len(e.parts)
}

// Slice gets the parts of the StrSlice as a Slice of string.
func (e *StrSlice) Slice() []string {
	if e == nil {
		return nil
	}
	return e.parts
}

// ToString gets space separated string of all the parts.
func (e *StrSlice) ToString() string {
	s := e.Slice()
	if s == nil {
		return ""
	}
	return strings.Join(s, " ")
}

// NewStrSlice creates an StrSlice based on the specified parts (as strings).
func NewStrSlice(parts ...string) *StrSlice {
	return &StrSlice{parts}
}
