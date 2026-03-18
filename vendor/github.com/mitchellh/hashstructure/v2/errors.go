package hashstructure

import (
	"fmt"
)

// ErrNotStringer is returned when there's an error with hash:"string"
type ErrNotStringer struct {
	Field string
}

// Error implements error for ErrNotStringer
func (ens *ErrNotStringer) Error() string {
	return fmt.Sprintf("hashstructure: %s has hash:\"string\" set, but does not implement fmt.Stringer", ens.Field)
}

// ErrFormat is returned when an invalid format is given to the Hash function.
type ErrFormat struct{}

func (*ErrFormat) Error() string {
	return "format must be one of the defined Format values in the hashstructure library"
}
