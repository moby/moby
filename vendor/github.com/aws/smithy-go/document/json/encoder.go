package json

import (
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"strings"

	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/document/internal/serde"
	smithyjson "github.com/aws/smithy-go/encoding/json"
)

// EncoderOptions is the set of options that can be configured for an Encoder.
type EncoderOptions struct{}

// Encoder is a Smithy document decoder for JSON based protocols.
type Encoder struct {
	options EncoderOptions
}

// Encode returns the JSON encoding of v.
func (e *Encoder) Encode(v interface{}) ([]byte, error) {
	encoder := smithyjson.NewEncoder()

	if err := e.encode(jsonValueProvider(encoder.Value), reflect.ValueOf(v), serde.Tag{}); err != nil {
		return nil, err
	}

	encodedBytes := encoder.Bytes()

	if len(encodedBytes) == 0 {
		return nil, nil
	}

	return encodedBytes, nil
}

// valueProvider is an interface for retrieving a JSON Value type used for encoding.
type valueProvider interface {
	GetValue() smithyjson.Value
}

// jsonValueProvider is a valueProvider that returns the JSON value encoder as is.
type jsonValueProvider smithyjson.Value

func (p jsonValueProvider) GetValue() smithyjson.Value {
	return smithyjson.Value(p)
}

// jsonObjectKeyProvider is a valueProvider that returns a JSON value type for encoding a value for the given JSON object
// key.
type jsonObjectKeyProvider struct {
	Object *smithyjson.Object
	Key    string
}

func (p jsonObjectKeyProvider) GetValue() smithyjson.Value {
	return p.Object.Key(p.Key)
}

// jsonArrayProvider is a valueProvider that returns a JSON value type for encoding a value in the given JSON array.
type jsonArrayProvider struct {
	Array *smithyjson.Array
}

func (p jsonArrayProvider) GetValue() smithyjson.Value {
	return p.Array.Value()
}

func (e *Encoder) encode(vp valueProvider, rv reflect.Value, tag serde.Tag) error {
	// Zero values are serialized as null, or skipped if omitEmpty.
	if serde.IsZeroValue(rv) {
		if tag.OmitEmpty {
			return nil
		}
		return e.encodeZeroValue(vp, rv)
	}

	// Handle both pointers and interface conversion into types
	rv = serde.ValueElem(rv)

	switch rv.Kind() {
	case reflect.Invalid:
		if tag.OmitEmpty {
			return nil
		}
		vp.GetValue().Null()
		return nil

	case reflect.Struct:
		return e.encodeStruct(vp, rv)

	case reflect.Map:
		return e.encodeMap(vp, rv)

	case reflect.Slice, reflect.Array:
		return e.encodeSlice(vp, rv)

	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		// skip unsupported types
		return nil

	default:
		return e.encodeScalar(vp, rv)
	}
}

func (e *Encoder) encodeZeroValue(vp valueProvider, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Invalid:
		vp.GetValue().Null()
	case reflect.Array:
		vp.GetValue().Array().Close()
	case reflect.Map, reflect.Slice:
		vp.GetValue().Null()
	case reflect.String:
		vp.GetValue().String("")
	case reflect.Bool:
		vp.GetValue().Boolean(false)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		vp.GetValue().Long(0)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		vp.GetValue().ULong(0)
	case reflect.Float32, reflect.Float64:
		vp.GetValue().Double(0)
	case reflect.Interface, reflect.Ptr:
		vp.GetValue().Null()
	default:
		return &document.InvalidMarshalError{Message: fmt.Sprintf("unknown value type: %s", rv.String())}
	}

	return nil
}

func (e *Encoder) encodeStruct(vp valueProvider, rv reflect.Value) error {
	if rv.CanInterface() && document.IsNoSerde(rv.Interface()) {
		return &document.UnmarshalTypeError{
			Value: fmt.Sprintf("unsupported type"),
			Type:  rv.Type(),
		}
	}

	switch {
	case rv.Type().ConvertibleTo(serde.ReflectTypeOf.Time):
		return &document.InvalidMarshalError{
			Message: fmt.Sprintf("unsupported type %s", rv.Type().String()),
		}
	case rv.Type().ConvertibleTo(serde.ReflectTypeOf.BigFloat):
		fallthrough
	case rv.Type().ConvertibleTo(serde.ReflectTypeOf.BigInt):
		return e.encodeNumber(vp, rv)
	}

	object := vp.GetValue().Object()
	defer object.Close()

	fields := serde.GetStructFields(rv.Type())
	for _, f := range fields.All() {
		if f.Name == "" {
			return &document.InvalidMarshalError{Message: "map key cannot be empty"}
		}

		fv, found := serde.EncoderFieldByIndex(rv, f.Index)
		if !found {
			continue
		}

		err := e.encode(jsonObjectKeyProvider{
			Object: object,
			Key:    f.Name,
		}, fv, f.Tag)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) encodeMap(vp valueProvider, rv reflect.Value) error {
	object := vp.GetValue().Object()
	defer object.Close()

	rawKeys := rv.MapKeys()
	keys := make([]*mapKey, 0, len(rawKeys))
	for _, raw := range rawKeys {
		keys = append(keys, &mapKey{
			Value:      raw,
			Underlying: fmt.Sprint(raw.Interface()),
		})
	}

	slices.SortFunc(keys, sortMapKeys)
	for _, key := range keys {
		if key.Underlying == "" {
			return &document.InvalidMarshalError{Message: "map key cannot be empty"}
		}

		ev := rv.MapIndex(key.Value)
		err := e.encode(jsonObjectKeyProvider{
			Object: object,
			Key:    key.Underlying,
		}, ev, serde.Tag{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) encodeSlice(value valueProvider, rv reflect.Value) error {
	array := value.GetValue().Array()
	defer array.Close()

	for i := 0; i < rv.Len(); i++ {
		err := e.encode(jsonArrayProvider{Array: array}, rv.Index(i), serde.Tag{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) encodeScalar(vp valueProvider, rv reflect.Value) error {
	if rv.Type() == serde.ReflectTypeOf.DocumentNumber {
		number := rv.Interface().(document.Number)
		if !isValidJSONNumber(number.String()) {
			return &document.InvalidMarshalError{Message: fmt.Sprintf("invalid number literal: %s", number)}
		}
		vp.GetValue().Write([]byte(number))
		return nil
	}

	switch rv.Kind() {
	case reflect.Bool:
		vp.GetValue().Boolean(rv.Bool())
	case reflect.String:
		vp.GetValue().String(rv.String())
	default:
		// Fallback to encoding numbers, will return invalid type if not supported
		err := e.encodeNumber(vp, rv)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) encodeNumber(vp valueProvider, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		vp.GetValue().Long(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		vp.GetValue().ULong(rv.Uint())
	case reflect.Float32:
		vp.GetValue().Float(float32(rv.Float()))
	case reflect.Float64:
		vp.GetValue().Double(rv.Float())
	default:
		rvt := rv.Type()
		switch {
		case rvt.ConvertibleTo(serde.ReflectTypeOf.BigInt):
			bi := rv.Convert(serde.ReflectTypeOf.BigInt).Interface().(big.Int)
			vp.GetValue().BigInteger(&bi)
		case rvt.ConvertibleTo(serde.ReflectTypeOf.BigFloat):
			bf := rv.Convert(serde.ReflectTypeOf.BigFloat).Interface().(big.Float)
			vp.GetValue().BigDecimal(&bf)
		default:
			return &document.InvalidMarshalError{Message: fmt.Sprintf("incompatible type: %s", rvt.String())}
		}
	}

	return nil
}

// isValidJSONNumber reports whether s is a valid JSON number literal.
// From https://golang.org/src/encoding/json/encode.go#L652 isValidNumber
// Copyright 2010 The Go Authors.
func isValidJSONNumber(s string) bool {
	// This function implements the JSON numbers grammar.
	// See https://tools.ietf.org/html/rfc7159#section-6
	// and https://www.json.org/img/number.png

	if s == "" {
		return false
	}

	// Optional -
	if s[0] == '-' {
		s = s[1:]
		if s == "" {
			return false
		}
	}

	// Digits
	switch {
	default:
		return false

	case s[0] == '0':
		s = s[1:]

	case '1' <= s[0] && s[0] <= '9':
		s = s[1:]
		for len(s) > 0 && '0' <= s[0] && s[0] <= '9' {
			s = s[1:]
		}
	}

	// . followed by 1 or more digits.
	if len(s) >= 2 && s[0] == '.' && '0' <= s[1] && s[1] <= '9' {
		s = s[2:]
		for len(s) > 0 && '0' <= s[0] && s[0] <= '9' {
			s = s[1:]
		}
	}

	// e or E followed by an optional - or + and
	// 1 or more digits.
	if len(s) >= 2 && (s[0] == 'e' || s[0] == 'E') {
		s = s[1:]
		if s[0] == '+' || s[0] == '-' {
			s = s[1:]
			if s == "" {
				return false
			}
		}
		for len(s) > 0 && '0' <= s[0] && s[0] <= '9' {
			s = s[1:]
		}
	}

	// Make sure we are at the end.
	return s == ""
}

// cache struct to cache a map key's reflect.Value with its underlying value,
// since repeated calls to reflect.Value.Interface() may be expensive
type mapKey struct {
	Value      reflect.Value
	Underlying string
}

func sortMapKeys(i, j *mapKey) int {
	return strings.Compare(i.Underlying, j.Underlying)
}
