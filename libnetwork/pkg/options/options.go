// The options package provides a way to pass unstructured sets of options to a
// component expecting a strongly-typed configuration structure.
package options

import (
	"fmt"
	"reflect"
)

type NoSuchFieldError struct {
	Field string
	Type  string
}

func (e NoSuchFieldError) Error() string {
	return fmt.Sprintf("no field %q in type %q", e.Field, e.Type)
}

type CannotSetFieldError struct {
	Field string
	Type  string
}

func (e CannotSetFieldError) Error() string {
	return fmt.Sprintf("cannot set field %q of type %q", e.Field, e.Type)
}

type Generic map[string]interface{}

func NewGeneric() Generic {
	return make(Generic)
}

func GenerateFromModel(options Generic, model interface{}) (interface{}, error) {
	modType := reflect.TypeOf(model)

	// If the model is of pointer type, we need to dereference for New.
	resType := reflect.TypeOf(model)
	if modType.Kind() == reflect.Ptr {
		resType = resType.Elem()
	}

	// Populate the result structure with the generic layout content.
	res := reflect.New(resType)
	for name, value := range options {
		field := res.Elem().FieldByName(name)
		if !field.IsValid() {
			return nil, NoSuchFieldError{name, reflect.TypeOf(model).Name()}
		}
		if !field.CanSet() {
			return nil, CannotSetFieldError{name, reflect.TypeOf(model).Name()}
		}
		field.Set(reflect.ValueOf(value))
	}

	// If the model is not of pointer type, return content of the result.
	if modType.Kind() == reflect.Ptr {
		return res.Interface(), nil
	}
	return res.Elem().Interface(), nil
}
