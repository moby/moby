package converter

import (
	"fmt"
	"reflect"
)

// NewChain takes a set of structs, in order, to allow for accurate chain.Convert(from, &to) calls. NewChain should
// be called with struct values in a manner similar to this:
// converter.NewChain(v1.Document{}, v2.Document{}, v3.Document{})
func NewChain(structs ...interface{}) Chain {
	out := Chain{}
	for _, s := range structs {
		typ := reflect.TypeOf(s)
		if isPtr(typ) { // these shouldn't be pointers, but check just to be safe
			typ = typ.Elem()
		}
		out.Types = append(out.Types, typ)
	}
	return out
}

// Chain holds a set of types with which to migrate through when a `chain.Convert` call is made
type Chain struct {
	Types []reflect.Type
}

// Convert converts from one type in the chain to the target type, calling each conversion in between
func (c Chain) Convert(from interface{}, to interface{}) (err error) {
	fromValue := reflect.ValueOf(from)
	fromType := fromValue.Type()

	// handle incoming pointers
	for isPtr(fromType) {
		fromValue = fromValue.Elem()
		fromType = fromType.Elem()
	}

	toValuePtr := reflect.ValueOf(to)
	toTypePtr := toValuePtr.Type()

	if !isPtr(toTypePtr) {
		return fmt.Errorf("TO struct provided not a pointer, unable to set values: %v", to)
	}

	// toValue must be a pointer but need a reference to the struct type directly
	toValue := toValuePtr.Elem()
	toType := toValue.Type()

	fromIdx := -1
	toIdx := -1

	for i, typ := range c.Types {
		if typ == fromType {
			fromIdx = i
		}
		if typ == toType {
			toIdx = i
		}
	}

	if fromIdx == -1 {
		return fmt.Errorf("invalid FROM type provided, not in the conversion chain: %s", fromType.Name())
	}

	if toIdx == -1 {
		return fmt.Errorf("invalid TO type provided, not in the conversion chain: %s", toType.Name())
	}

	last := from
	for i := fromIdx; i != toIdx; {
		// skip the first index, because that is the from type - start with the next conversion in the chain
		if fromIdx < toIdx {
			i++
		} else {
			i--
		}

		var next interface{}
		if i == toIdx {
			next = to
		} else {
			nextVal := reflect.New(c.Types[i])
			next = nextVal.Interface() // this will be a pointer, which is fine to pass to both from and to in Convert
		}

		if err = Convert(last, next); err != nil {
			return err
		}

		last = next
	}

	return nil
}
