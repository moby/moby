package toml

import (
	"encoding"
	"reflect"
	"time"
)

// isZeroer is used to check whether a value is the zero value for its type,
// as defined by the type itself.
type isZeroer interface {
	IsZero() bool
}

var isZeroerType = reflect.TypeOf(new(isZeroer)).Elem()

var (
	timeType               = reflect.TypeOf(time.Time{})
	textMarshalerType      = reflect.TypeOf(new(encoding.TextMarshaler)).Elem()
	textUnmarshalerType    = reflect.TypeOf(new(encoding.TextUnmarshaler)).Elem()
	mapStringInterfaceType = reflect.TypeOf(map[string]interface{}(nil))
	sliceInterfaceType     = reflect.TypeOf([]interface{}(nil))
	stringType             = reflect.TypeOf("")
)
