package errtype

import (
	"fmt"
)

type NotFoundError struct {
	property string
	value    string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no such %s: %s", e.property, e.value)
}

func NewNotFoundError(p, v string) error {
	return &NotFoundError{
		property: p,
		value:    v,
	}
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotFoundError)
	return ok
}
