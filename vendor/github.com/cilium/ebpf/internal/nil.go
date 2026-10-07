package internal

import (
	"fmt"
	"reflect"
)

// IsNilPointer returns an error if i is a nil interface or a nil pointer.
// Otherwise, it returns nil.
func IsNilPointer(i any) error {
	if i == nil {
		return fmt.Errorf("nil interface")
	}
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Pointer {
		return nil
	}

	if v.IsNil() {
		return fmt.Errorf("nil %T", i)
	}

	return nil
}
