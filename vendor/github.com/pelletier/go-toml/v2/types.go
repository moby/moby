package toml

import (
	"encoding"
	"reflect"
	"time"
)

// isZeroer is used to check if a type has a custom IsZero method.
// This allows custom types to define their own zero-value semantics.
type isZeroer interface {
	IsZero() bool
}

var (
	timeType               = reflect.TypeOf((*time.Time)(nil)).Elem()
	textMarshalerType      = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	textUnmarshalerType    = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	isZeroerType           = reflect.TypeOf((*isZeroer)(nil)).Elem()
	mapStringInterfaceType = reflect.TypeOf(map[string]interface{}(nil))
	sliceInterfaceType     = reflect.TypeOf([]interface{}(nil))
	stringType             = reflect.TypeOf("")
)
