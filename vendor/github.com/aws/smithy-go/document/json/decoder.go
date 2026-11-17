package json

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"

	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/document/internal/serde"
)

// DecoderOptions is the set of options that can be configured for a Decoder.
type DecoderOptions struct{}

// Decoder is a Smithy document decoder for JSON based protocols.
type Decoder struct {
	options DecoderOptions
}

// DecodeJSONInterface decodes the supported JSON input types and stores the result in the value pointed by toValue.
//
// If toValue is not a compatible type, or an error occurs while decoding DecodeJSONInterface will return an error.
//
// The supported input JSON types are:
//   bool -> JSON boolean
//   float64 -> JSON number
//   json.Number -> JSON number
//   string -> JSON string
//   []interface{} -> JSON array
//   map[string]interface{} -> JSON object
//   nil -> JSON null
//
func (d *Decoder) DecodeJSONInterface(input interface{}, toValue interface{}) error {
	if document.IsNoSerde(toValue) {
		return fmt.Errorf("unsupported type: %T", toValue)
	}

	v := reflect.ValueOf(toValue)

	if v.Kind() != reflect.Ptr || v.IsNil() || !v.IsValid() {
		return &document.InvalidUnmarshalError{Type: reflect.TypeOf(toValue)}
	}

	return d.decode(input, v, serde.Tag{})
}

func (d *Decoder) decode(jv interface{}, rv reflect.Value, tag serde.Tag) error {
	if jv == nil {
		rv := serde.Indirect(rv, true)
		return d.decodeJSONNull(rv)
	}

	rv = serde.Indirect(rv, false)

	if err := d.unsupportedType(jv, rv); err != nil {
		return err
	}

	switch tv := jv.(type) {
	case bool:
		return d.decodeJSONBoolean(tv, rv)
	case json.Number:
		return d.decodeJSONNumber(tv, rv)
	case float64:
		return d.decodeJSONFloat64(tv, rv)
	case string:
		return d.decodeJSONString(tv, rv)
	case []interface{}:
		return d.decodeJSONArray(tv, rv)
	case map[string]interface{}:
		return d.decodeJSONObject(tv, rv)
	default:
		return fmt.Errorf("unsupported json type, %T", tv)
	}
}

func (d *Decoder) decodeJSONNull(rv reflect.Value) error {
	if rv.IsValid() && rv.CanSet() {
		rv.Set(reflect.Zero(rv.Type()))
	}

	return nil
}

func (d *Decoder) decodeJSONBoolean(tv bool, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Bool, reflect.Interface:
		rv.Set(reflect.ValueOf(tv).Convert(rv.Type()))
	default:
		return &document.UnmarshalTypeError{Value: "bool", Type: rv.Type()}
	}

	return nil
}

func (d *Decoder) decodeJSONNumber(tv json.Number, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Interface:
		rv.Set(reflect.ValueOf(document.Number(tv)))
	case reflect.String:
		// covers document.Number
		rv.SetString(tv.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := tv.Int64()
		if err != nil {
			return err
		}
		if rv.OverflowInt(i) {
			return &document.UnmarshalTypeError{
				Value: fmt.Sprintf("number overflow, %s", tv.String()),
				Type:  rv.Type(),
			}
		}
		rv.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := document.Number(tv).Uint64()
		if err != nil {
			return err
		}
		if rv.OverflowUint(u) {
			return &document.UnmarshalTypeError{
				Value: fmt.Sprintf("number overflow, %s", tv.String()),
				Type:  rv.Type(),
			}
		}
		rv.SetUint(u)
	case reflect.Float32:
		f, err := document.Number(tv).Float32()
		if err != nil {
			return err
		}
		if rv.OverflowFloat(f) {
			return &document.UnmarshalTypeError{
				Value: fmt.Sprintf("float overflow, %s", tv.String()),
				Type:  rv.Type(),
			}
		}
		rv.SetFloat(f)
	case reflect.Float64:
		f, err := document.Number(tv).Float64()
		if err != nil {
			return err
		}
		if rv.OverflowFloat(f) {
			return &document.UnmarshalTypeError{
				Value: fmt.Sprintf("float overflow, %s", tv.String()),
				Type:  rv.Type(),
			}
		}
		rv.SetFloat(f)
	default:
		rvt := rv.Type()
		switch {
		case rvt.ConvertibleTo(serde.ReflectTypeOf.BigFloat):
			sv := tv.String()
			f, ok := (&big.Float{}).SetString(sv)
			if !ok {
				return &document.UnmarshalTypeError{
					Value: fmt.Sprintf("invalid number format, %s", sv),
					Type:  rv.Type(),
				}
			}
			rv.Set(reflect.ValueOf(*f).Convert(rvt))
		case rvt.ConvertibleTo(serde.ReflectTypeOf.BigInt):
			sv := tv.String()
			i, ok := (&big.Int{}).SetString(sv, 10)
			if !ok {
				return &document.UnmarshalTypeError{
					Value: fmt.Sprintf("invalid number format, %s", sv),
					Type:  rv.Type(),
				}
			}
			rv.Set(reflect.ValueOf(*i).Convert(rvt))
		default:
			return &document.UnmarshalTypeError{Value: "number", Type: rv.Type()}
		}
	}

	return nil
}

func (d *Decoder) decodeJSONFloat64(tv float64, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Interface:
		rv.Set(reflect.ValueOf(tv))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, accuracy := big.NewFloat(tv).Int64()
		if accuracy != big.Exact || rv.OverflowInt(i) {
			return &document.UnmarshalTypeError{
				Value: fmt.Sprintf("number overflow, %e", tv),
				Type:  rv.Type(),
			}
		}
		rv.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, accuracy := big.NewFloat(tv).Uint64()
		if accuracy != big.Exact || rv.OverflowUint(u) {
			return &document.UnmarshalTypeError{
				Value: fmt.Sprintf("number overflow, %e", tv),
				Type:  rv.Type(),
			}
		}
		rv.SetUint(u)
	case reflect.Float32, reflect.Float64:
		if rv.OverflowFloat(tv) {
			return &document.UnmarshalTypeError{
				Value: fmt.Sprintf("float overflow, %e", tv),
				Type:  rv.Type(),
			}
		}
		rv.SetFloat(tv)
	default:
		rvt := rv.Type()
		switch {
		case rvt.ConvertibleTo(serde.ReflectTypeOf.BigFloat):
			f := big.NewFloat(tv)
			rv.Set(reflect.ValueOf(*f).Convert(rvt))
		case rvt.ConvertibleTo(serde.ReflectTypeOf.BigInt):
			i, accuracy := big.NewFloat(tv).Int(nil)
			if accuracy != big.Exact {
				return &document.UnmarshalTypeError{
					Value: fmt.Sprintf("int overflow, %e", tv),
					Type:  rv.Type(),
				}
			}
			rv.Set(reflect.ValueOf(*i).Convert(rvt))
		default:
			return &document.UnmarshalTypeError{Value: "number", Type: rv.Type()}
		}
	}

	return nil
}

func (d *Decoder) decodeJSONArray(tv []interface{}, rv reflect.Value) error {
	var isArray bool

	switch rv.Kind() {
	case reflect.Slice:
		// Make room for the slice elements if needed
		if rv.IsNil() || rv.Cap() < len(tv) {
			rv.Set(reflect.MakeSlice(rv.Type(), 0, len(tv)))
		}
	case reflect.Array:
		// Limited to capacity of existing array.
		isArray = true
	case reflect.Interface:
		s := make([]interface{}, len(tv))
		for i, av := range tv {
			if err := d.decode(av, reflect.ValueOf(&s[i]).Elem(), serde.Tag{}); err != nil {
				return err
			}
		}
		rv.Set(reflect.ValueOf(s))
		return nil
	default:
		return &document.UnmarshalTypeError{Value: "list", Type: rv.Type()}
	}

	// If rv is not a slice, array
	for i := 0; i < rv.Cap() && i < len(tv); i++ {
		if !isArray {
			rv.SetLen(i + 1)
		}
		if err := d.decode(tv[i], rv.Index(i), serde.Tag{}); err != nil {
			return err
		}
	}

	return nil
}

func (d *Decoder) decodeJSONString(tv string, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.String:
		rv.SetString(tv)
	case reflect.Interface:
		// Ensure type aliasing is handled properly
		rv.Set(reflect.ValueOf(tv).Convert(rv.Type()))
	default:
		return &document.UnmarshalTypeError{Value: "string", Type: rv.Type()}
	}

	return nil
}

func (d *Decoder) decodeJSONObject(tv map[string]interface{}, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Map:
		t := rv.Type()
		if t.Key().Kind() != reflect.String {
			return &document.UnmarshalTypeError{Value: "map string key", Type: t.Key()}
		}
		if rv.IsNil() {
			rv.Set(reflect.MakeMap(t))
		}
	case reflect.Struct:
		if rv.CanInterface() && document.IsNoSerde(rv.Interface()) {
			return &document.UnmarshalTypeError{
				Value: fmt.Sprintf("unsupported type"),
				Type:  rv.Type(),
			}
		}
	case reflect.Interface:
		rv.Set(reflect.MakeMap(serde.ReflectTypeOf.MapStringToInterface))
		rv = rv.Elem()
	default:
		return &document.UnmarshalTypeError{Value: "map", Type: rv.Type()}
	}

	if rv.Kind() == reflect.Map {
		for k, kv := range tv {
			key := reflect.New(rv.Type().Key()).Elem()
			key.SetString(k)
			elem := reflect.New(rv.Type().Elem()).Elem()
			if err := d.decode(kv, elem, serde.Tag{}); err != nil {
				return err
			}
			rv.SetMapIndex(key, elem)
		}
	} else if rv.Kind() == reflect.Struct {
		fields := serde.GetStructFields(rv.Type())
		for k, kv := range tv {
			if f, ok := fields.FieldByName(k); ok {
				fv := serde.DecoderFieldByIndex(rv, f.Index)
				if err := d.decode(kv, fv, f.Tag); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (d *Decoder) unsupportedType(jv interface{}, rv reflect.Value) error {
	if rv.Kind() == reflect.Interface && rv.NumMethod() != 0 {
		return &document.UnmarshalTypeError{Value: "non-empty interface", Type: rv.Type()}
	}

	if rv.Type().ConvertibleTo(serde.ReflectTypeOf.Time) {
		return &document.UnmarshalTypeError{
			Type:  rv.Type(),
			Value: fmt.Sprintf("time value: %v", jv),
		}
	}
	return nil
}
