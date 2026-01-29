package ssa

import (
	"fmt"
	"strings"
)

// Signature is a function prototype.
type Signature struct {
	// ID is a unique identifier for this signature used to lookup.
	ID SignatureID
	// Params and Results are the types of the parameters and results of the function.
	Params, Results []Type

	// used is true if this is used by the currently-compiled function.
	// Debugging only.
	used bool
}

// String implements fmt.Stringer.
func (s *Signature) String() string {
	str := strings.Builder{}
	str.WriteString(s.ID.String())
	str.WriteString(": ")
	if len(s.Params) > 0 {
		for _, typ := range s.Params {
			str.WriteString(typ.String())
		}
	} else {
		str.WriteByte('v')
	}
	str.WriteByte('_')
	if len(s.Results) > 0 {
		for _, typ := range s.Results {
			str.WriteString(typ.String())
		}
	} else {
		str.WriteByte('v')
	}
	return str.String()
}

// SignatureID is an unique identifier used to lookup.
type SignatureID int

// String implements fmt.Stringer.
func (s SignatureID) String() string {
	return fmt.Sprintf("sig%d", s)
}
