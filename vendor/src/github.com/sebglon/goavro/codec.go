// Copyright 2015 LinkedIn Corp. Licensed under the Apache License,
// Version 2.0 (the "License"); you may not use this file except in
// compliance with the License.  You may obtain a copy of the License
// at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.Copyright [201X] LinkedIn Corp. Licensed under the Apache
// License, Version 2.0 (the "License"); you may not use this file
// except in compliance with the License.  You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.

// Package goavro is a library that encodes and decodes of Avro
// data. It provides an interface to encode data directly to io.Writer
// streams, and to decode data from io.Reader streams. Goavro fully
// adheres to version 1.7.7 of the Avro specification and data
// encoding.
package goavro

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const (
	mask = byte(127)
	flag = byte(128)
)

// ErrSchemaParse is returned when a Codec cannot be created due to an
// error while reading or parsing the schema.
type ErrSchemaParse struct {
	Message string
	Err     error
}

func (e ErrSchemaParse) Error() string {
	if e.Err == nil {
		return "cannot parse schema: " + e.Message
	}
	return "cannot parse schema: " + e.Message + ": " + e.Err.Error()
}

// ErrCodecBuild is returned when the encoder encounters an error.
type ErrCodecBuild struct {
	Message string
	Err     error
}

func (e ErrCodecBuild) Error() string {
	if e.Err == nil {
		return "cannot build " + e.Message
	}
	return "cannot build " + e.Message + ": " + e.Err.Error()
}

func newCodecBuildError(dataType string, a ...interface{}) *ErrCodecBuild {
	var err error
	var format, message string
	var ok bool
	if len(a) == 0 {
		return &ErrCodecBuild{dataType + ": no reason given", nil}
	}
	// if last item is error: save it
	if err, ok = a[len(a)-1].(error); ok {
		a = a[:len(a)-1] // pop it
	}
	// if items left, first ought to be format string
	if len(a) > 0 {
		if format, ok = a[0].(string); ok {
			a = a[1:] // unshift
			message = fmt.Sprintf(format, a...)
		}
	}
	if message != "" {
		message = ": " + message
	}
	return &ErrCodecBuild{dataType + message, err}
}

// Decoder interface specifies structures that may be decoded.
type Decoder interface {
	Decode(io.Reader) (interface{}, error)
}

// Encoder interface specifies structures that may be encoded.
type Encoder interface {
	Encode(io.Writer, interface{}) error
}

// The Codec interface supports both Decode and Encode operations.
type Codec interface {
	Decoder
	Encoder
	Schema() string
	NewWriter(...WriterSetter) (*Writer, error)
}

// CodecSetter functions are those those which are used to modify a
// newly instantiated Codec.
type CodecSetter func(Codec) error

type decoderFunction func(io.Reader) (interface{}, error)
type encoderFunction func(io.Writer, interface{}) error

type codec struct {
	nm     *name
	df     decoderFunction
	ef     encoderFunction
	schema string
}

// String returns a string representation of the codec.
func (c codec) String() string {
	return fmt.Sprintf("nm: %v, df: %v, ef: %v", c.nm, c.df, c.ef)
}

type symtab map[string]*codec // map full name to codec

// NewCodec creates a new object that supports both the Decode and
// Encode methods. It requires an Avro schema, expressed as a JSON
// string.
//
//   codec, err := goavro.NewCodec(someJSONSchema)
//   if err != nil {
//       return nil, err
//   }
//
//   // Decoding data uses codec created above, and an io.Reader,
//   // definition not shown:
//   datum, err := codec.Decode(r)
//   if err != nil {
//       return nil, err
//   }
//
//   // Encoding data uses codec created above, an io.Writer,
//   // definition not shown, and some data:
//   err := codec.Encode(w, datum)
//   if err != nil {
//       return nil, err
//   }
//
//   // Encoding data using bufio.Writer to buffer the writes
//   // during data encoding:
//
//   func encodeWithBufferedWriter(c Codec, w io.Writer, datum interface{}) error {
//	bw := bufio.NewWriter(w)
//	err := c.Encode(bw, datum)
//	if err != nil {
//		return err
//	}
//	return bw.Flush()
//   }
//
//   err := encodeWithBufferedWriter(codec, w, datum)
//   if err != nil {
//       return nil, err
//   }
func NewCodec(someJSONSchema string, setters ...CodecSetter) (Codec, error) {
	// unmarshal into schema blob
	var schema interface{}
	if err := json.Unmarshal([]byte(someJSONSchema), &schema); err != nil {
		return nil, &ErrSchemaParse{"cannot unmarshal JSON", err}
	}
	// remarshal back into compressed json
	compressedSchema, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal schema: %v", err)
	}

	// each codec gets a unified namespace of symbols to
	// respective codecs
	st := make(symtab)

	newCodec, err := st.buildCodec(nullNamespace, schema)
	if err != nil {
		return nil, err
	}

	for _, setter := range setters {
		err = setter(newCodec)
		if err != nil {
			return nil, err
		}
	}
	newCodec.schema = string(compressedSchema)
	return newCodec, nil
}

// Decode will read from the specified io.Reader, and return the next
// datum from the stream, or an error explaining why the stream cannot
// be converted into the Codec's schema.
func (c codec) Decode(r io.Reader) (interface{}, error) {
	return c.df(r)
}

// Encode will write the specified datum to the specified io.Writer,
// or return an error explaining why the datum cannot be converted
// into the Codec's schema.
func (c codec) Encode(w io.Writer, datum interface{}) error {
	return c.ef(w, datum)
}

func (c codec) Schema() string {
	return c.schema
}

// NewWriter creates a new Writer that encodes using the given Codec.
//
// The following two code examples produce identical results:
//
//    // method 1:
//    fw, err := codec.NewWriter(goavro.ToWriter(w))
//    if err != nil {
//    	log.Fatal(err)
//    }
//    defer fw.Close()
//
//    // method 2:
//    fw, err := goavro.NewWriter(goavro.ToWriter(w), goavro.UseCodec(codec))
//    if err != nil {
//    	log.Fatal(err)
//    }
//    defer fw.Close()
func (c codec) NewWriter(setters ...WriterSetter) (*Writer, error) {
	setters = append(setters, UseCodec(c))
	return NewWriter(setters...)
}

var (
	nullCodec, booleanCodec, intCodec, longCodec, floatCodec, doubleCodec, bytesCodec, stringCodec *codec
)

func init() {
	// NOTE: use Go type names because for runtime resolution of
	// union member, it gets the Go type name of the datum sent to
	// the union encoder, and uses that string as a key into the
	// encoders map
	nullCodec = &codec{nm: &name{n: "null"}, df: nullDecoder, ef: nullEncoder}
	booleanCodec = &codec{nm: &name{n: "bool"}, df: booleanDecoder, ef: booleanEncoder}
	intCodec = &codec{nm: &name{n: "int32"}, df: intDecoder, ef: intEncoder}
	longCodec = &codec{nm: &name{n: "int64"}, df: longDecoder, ef: longEncoder}
	floatCodec = &codec{nm: &name{n: "float32"}, df: floatDecoder, ef: floatEncoder}
	doubleCodec = &codec{nm: &name{n: "float64"}, df: doubleDecoder, ef: doubleEncoder}
	bytesCodec = &codec{nm: &name{n: "[]uint8"}, df: bytesDecoder, ef: bytesEncoder}
	stringCodec = &codec{nm: &name{n: "string"}, df: stringDecoder, ef: stringEncoder}
}

func (st symtab) buildCodec(enclosingNamespace string, schema interface{}) (*codec, error) {
	switch schemaType := schema.(type) {
	case string:
		return st.buildString(enclosingNamespace, schemaType, schema)
	case []interface{}:
		return st.makeUnionCodec(enclosingNamespace, schema)
	case map[string]interface{}:
		return st.buildMap(enclosingNamespace, schema.(map[string]interface{}))
	default:
		return nil, newCodecBuildError("unknown", "schema type: %T", schema)
	}
}

func (st symtab) buildMap(enclosingNamespace string, schema map[string]interface{}) (*codec, error) {
	t, ok := schema["type"]
	if !ok {
		return nil, newCodecBuildError("map", "ought have type: %v", schema)
	}
	switch t.(type) {
	case string:
		// EXAMPLE: "type":"int"
		// EXAMPLE: "type":"enum"
		return st.buildString(enclosingNamespace, t.(string), schema)
	case map[string]interface{}, []interface{}:
		// EXAMPLE: "type":{"type":fixed","name":"fixed_16","size":16}
		// EXAMPLE: "type":["null","int"]
		return st.buildCodec(enclosingNamespace, t)
	default:
		return nil, newCodecBuildError("map", "type ought to be either string, map[string]interface{}, or []interface{}; received: %T", t)
	}
}

func (st symtab) buildString(enclosingNamespace, typeName string, schema interface{}) (*codec, error) {
	switch typeName {
	case "null":
		return nullCodec, nil
	case "boolean":
		return booleanCodec, nil
	case "int":
		return intCodec, nil
	case "long":
		return longCodec, nil
	case "float":
		return floatCodec, nil
	case "double":
		return doubleCodec, nil
	case "bytes":
		return bytesCodec, nil
	case "string":
		return stringCodec, nil
	case "record":
		return st.makeRecordCodec(enclosingNamespace, schema)
	case "enum":
		return st.makeEnumCodec(enclosingNamespace, schema)
	case "fixed":
		return st.makeFixedCodec(enclosingNamespace, schema)
	case "map":
		return st.makeMapCodec(enclosingNamespace, schema)
	case "array":
		return st.makeArrayCodec(enclosingNamespace, schema)
	default:
		t, err := newName(nameName(typeName), nameEnclosingNamespace(enclosingNamespace))
		if err != nil {
			return nil, newCodecBuildError(typeName, "could not normalize name: %q: %q: %s", enclosingNamespace, typeName, err)
		}
		c, ok := st[t.n]
		if !ok {
			return nil, newCodecBuildError("unknown", "unknown type name: %s", t.n)
		}
		return c, nil
	}
}

type unionEncoder struct {
	ef    encoderFunction
	index int32
}

func (st symtab) makeUnionCodec(enclosingNamespace string, schema interface{}) (*codec, error) {
	errorNamespace := "null namespace"
	if enclosingNamespace != nullNamespace {
		errorNamespace = enclosingNamespace
	}
	friendlyName := fmt.Sprintf("union (%s)", errorNamespace)

	// schema checks
	schemaArray, ok := schema.([]interface{})
	if !ok {
		return nil, newCodecBuildError(friendlyName, "ought to be array: %T", schema)
	}
	if len(schemaArray) == 0 {
		return nil, newCodecBuildError(friendlyName, " ought have at least one member")
	}

	// setup
	nameToUnionEncoder := make(map[string]unionEncoder)
	indexToDecoder := make([]decoderFunction, len(schemaArray))
	allowedNames := make([]string, len(schemaArray))

	for idx, unionMemberSchema := range schemaArray {
		c, err := st.buildCodec(enclosingNamespace, unionMemberSchema)
		if err != nil {
			return nil, newCodecBuildError(friendlyName, "member ought to be decodable: %s", err)
		}
		allowedNames[idx] = c.nm.n
		indexToDecoder[idx] = c.df
		nameToUnionEncoder[c.nm.n] = unionEncoder{ef: c.ef, index: int32(idx)}
	}

	invalidType := "datum ought match schema: expected: "
	invalidType += strings.Join(allowedNames, ", ")
	invalidType += "; received: "

	nm, _ := newName(nameName("union"))
	friendlyName = fmt.Sprintf("union (%s)", nm.n)

	return &codec{
		nm: nm,
		df: func(r io.Reader) (interface{}, error) {
			i, err := intDecoder(r)
			if err != nil {
				return nil, newEncoderError(friendlyName, err)
			}
			idx, ok := i.(int32)
			if !ok {
				return nil, newEncoderError(friendlyName, "expected: int; received: %T", i)
			}
			index := int(idx)
			if index < 0 || index >= len(indexToDecoder) {
				return nil, newEncoderError(friendlyName, "index must be between 0 and %d; read index: %d", len(indexToDecoder)-1, index)
			}
			return indexToDecoder[index](r)
		},
		ef: func(w io.Writer, datum interface{}) error {
			var err error
			var name string
			switch datum.(type) {
			default:
				name = reflect.TypeOf(datum).String()
			case map[string]interface{}:
				name = "map"
			case []interface{}:
				name = "array"
			case nil:
				name = "null"
			case Enum:
				name = datum.(Enum).Name
			case *Record:
				name = datum.(*Record).Name
			}

			ue, ok := nameToUnionEncoder[name]
			if !ok {
				return newEncoderError(friendlyName, invalidType+name)
			}
			if err = intEncoder(w, ue.index); err != nil {
				return newEncoderError(friendlyName, err)
			}
			if err = ue.ef(w, datum); err != nil {
				return newEncoderError(friendlyName, err)
			}
			return nil
		},
	}, nil
}

// Enum is an abstract data type used to hold data corresponding to an Avro enum. Whenever an Avro
// schema specifies an enum, this library's Decode method will return an Enum initialized to the
// enum's name and value read from the io.Reader. Likewise, when using Encode to convert data to an
// Avro record, it is necessary to crate and send an Enum instance to the Encode method.
type Enum struct {
	Name, Value string
}

func (st symtab) makeEnumCodec(enclosingNamespace string, schema interface{}) (*codec, error) {
	errorNamespace := "null namespace"
	if enclosingNamespace != nullNamespace {
		errorNamespace = enclosingNamespace
	}
	friendlyName := fmt.Sprintf("enum (%s)", errorNamespace)

	// schema checks
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil, newCodecBuildError(friendlyName, "expected: map[string]interface{}; received: %T", schema)
	}
	nm, err := newName(nameEnclosingNamespace(enclosingNamespace), nameSchema(schemaMap))
	if err != nil {
		return nil, err
	}
	friendlyName = fmt.Sprintf("enum (%s)", nm.n)

	s, ok := schemaMap["symbols"]
	if !ok {
		return nil, newCodecBuildError(friendlyName, "ought to have symbols key")
	}
	symtab, ok := s.([]interface{})
	if !ok || len(symtab) == 0 {
		return nil, newCodecBuildError(friendlyName, "symbols ought to be non-empty array")
	}
	for _, v := range symtab {
		_, ok := v.(string)
		if !ok {
			return nil, newCodecBuildError(friendlyName, "symbols array member ought to be string")
		}
	}
	c := &codec{
		nm: nm,
		df: func(r io.Reader) (interface{}, error) {
			someValue, err := longDecoder(r)
			if err != nil {
				return nil, newDecoderError(friendlyName, err)
			}
			index, ok := someValue.(int64)
			if !ok {
				return nil, newDecoderError(friendlyName, "expected long; received: %T", someValue)
			}
			if index < 0 || index >= int64(len(symtab)) {
				return nil, newDecoderError(friendlyName, "index must be between 0 and %d", len(symtab)-1)
			}
			return symtab[index], nil
		},
		ef: func(w io.Writer, datum interface{}) error {
			someEnum, ok := datum.(Enum)
			if !ok {
				return newEncoderError(friendlyName, "expected: Enum; received: %T", datum)
			}
			someString := someEnum.Value
			for idx, symbol := range symtab {
				if symbol == someString {
					if err := longEncoder(w, int64(idx)); err != nil {
						return newEncoderError(friendlyName, err)
					}
					return nil
				}
			}
			return newEncoderError(friendlyName, "symbol not defined: %s", someString)
		},
	}
	st[nm.n] = c
	return c, nil
}

func (st symtab) makeFixedCodec(enclosingNamespace string, schema interface{}) (*codec, error) {
	errorNamespace := "null namespace"
	if enclosingNamespace != nullNamespace {
		errorNamespace = enclosingNamespace
	}
	friendlyName := fmt.Sprintf("fixed (%s)", errorNamespace)

	// schema checks
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil, newCodecBuildError(friendlyName, "expected: map[string]interface{}; received: %T", schema)
	}
	nm, err := newName(nameSchema(schemaMap), nameEnclosingNamespace(enclosingNamespace))
	if err != nil {
		return nil, err
	}
	friendlyName = fmt.Sprintf("fixed (%s)", nm.n)
	s, ok := schemaMap["size"]
	if !ok {
		return nil, newCodecBuildError(friendlyName, "ought to have size key")
	}
	fs, ok := s.(float64)
	if !ok {
		return nil, newCodecBuildError(friendlyName, "size ought to be number: %T", s)
	}
	size := int32(fs)
	c := &codec{
		nm: nm,
		df: func(r io.Reader) (interface{}, error) {
			buf := make([]byte, size)
			n, err := r.Read(buf)
			if err != nil {
				return nil, newDecoderError(friendlyName, err)
			}
			if n < int(size) {
				return nil, newDecoderError(friendlyName, "buffer underrun")
			}
			return buf, nil
		},
		ef: func(w io.Writer, datum interface{}) error {
			someBytes, ok := datum.([]byte)
			if !ok {
				return newEncoderError(friendlyName, "expected: []byte; received: %T", datum)
			}
			if len(someBytes) != int(size) {
				return newEncoderError(friendlyName, "expected: %d bytes; received: %d", size, len(someBytes))
			}
			n, err := w.Write(someBytes)
			if err != nil {
				return newEncoderError(friendlyName, err)
			}
			if n != int(size) {
				return newEncoderError(friendlyName, "buffer underrun")
			}
			return nil
		},
	}
	st[nm.n] = c
	return c, nil
}

func (st symtab) makeRecordCodec(enclosingNamespace string, schema interface{}) (*codec, error) {
	errorNamespace := "null namespace"
	if enclosingNamespace != nullNamespace {
		errorNamespace = enclosingNamespace
	}
	friendlyName := fmt.Sprintf("record (%s)", errorNamespace)

	// delegate schema checks to NewRecord()
	recordTemplate, err := NewRecord(recordSchemaRaw(schema), RecordEnclosingNamespace(enclosingNamespace))
	if err != nil {
		return nil, err
	}

	if len(recordTemplate.Fields) == 0 {
		return nil, newCodecBuildError(friendlyName, "fields ought to be non-empty array")
	}

	fieldCodecs := make([]*codec, len(recordTemplate.Fields))
	for idx, field := range recordTemplate.Fields {
		var err error
		fieldCodecs[idx], err = st.buildCodec(recordTemplate.n.namespace(), field.schema)
		if err != nil {
			return nil, newCodecBuildError(friendlyName, "record field ought to be codec: %+v", st, err)
		}
	}

	friendlyName = fmt.Sprintf("record (%s)", recordTemplate.Name)

	c := &codec{
		nm: recordTemplate.n,
		df: func(r io.Reader) (interface{}, error) {
			someRecord, _ := NewRecord(recordSchemaRaw(schema), RecordEnclosingNamespace(enclosingNamespace))
			for idx, codec := range fieldCodecs {
				value, err := codec.Decode(r)
				if err != nil {
					return nil, newDecoderError(friendlyName, err)
				}
				someRecord.Fields[idx].Datum = value
			}
			return someRecord, nil
		},
		ef: func(w io.Writer, datum interface{}) error {
			someRecord, ok := datum.(*Record)
			if !ok {
				return newEncoderError(friendlyName, "expected: Record; received: %T", datum)
			}
			if someRecord.Name != recordTemplate.Name {
				return newEncoderError(friendlyName, "expected: %v; received: %v", recordTemplate.Name, someRecord.Name)
			}
			for idx, field := range someRecord.Fields {
				var value interface{}
				// check whether field datum is valid
				if reflect.ValueOf(field.Datum).IsValid() {
					value = field.Datum
				} else if field.hasDefault {
					value = field.defval
				} else {
					return newEncoderError(friendlyName, "field has no data and no default set: %v", field.Name)
				}
				err = fieldCodecs[idx].Encode(w, value)
				if err != nil {
					return newEncoderError(friendlyName, err)
				}
			}
			return nil
		},
	}
	st[recordTemplate.Name] = c
	return c, nil
}

func (st symtab) makeMapCodec(enclosingNamespace string, schema interface{}) (*codec, error) {
	errorNamespace := "null namespace"
	if enclosingNamespace != nullNamespace {
		errorNamespace = enclosingNamespace
	}
	friendlyName := fmt.Sprintf("map (%s)", errorNamespace)

	// schema checks
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil, newCodecBuildError(friendlyName, "expected: map[string]interface{}; received: %T", schema)
	}
	v, ok := schemaMap["values"]
	if !ok {
		return nil, newCodecBuildError(friendlyName, "ought to have values key")
	}
	valuesCodec, err := st.buildCodec(enclosingNamespace, v)
	if err != nil {
		return nil, newCodecBuildError(friendlyName, err)
	}

	nm := &name{n: "map"}
	friendlyName = fmt.Sprintf("map (%s)", nm.n)

	return &codec{
		nm: nm,
		df: func(r io.Reader) (interface{}, error) {
			data := make(map[string]interface{})
			someValue, err := longDecoder(r)
			if err != nil {
				return nil, newDecoderError(friendlyName, err)
			}
			blockCount := someValue.(int64)

			for blockCount != 0 {
				if blockCount < 0 {
					blockCount = -blockCount
					// next long is size of block, for which we have no use
					_, err := longDecoder(r)
					if err != nil {
						return nil, newDecoderError(friendlyName, err)
					}
				}
				for i := int64(0); i < blockCount; i++ {
					someValue, err := stringDecoder(r)
					if err != nil {
						return nil, newDecoderError(friendlyName, err)
					}
					mapKey, ok := someValue.(string)
					if !ok {
						return nil, newDecoderError(friendlyName, "map key ought to be string")
					}
					datum, err := valuesCodec.df(r)
					if err != nil {
						return nil, err
					}
					data[mapKey] = datum
				}
				// decode next blockcount
				someValue, err = longDecoder(r)
				if err != nil {
					return nil, newDecoderError(friendlyName, err)
				}
				blockCount = someValue.(int64)
			}
			return data, nil
		},
		ef: func(w io.Writer, datum interface{}) error {
			dict, ok := datum.(map[string]interface{})
			if !ok {
				return newEncoderError(friendlyName, "expected: map[string]interface{}; received: %T", datum)
			}
			if len(dict) > 0 {
				if err = longEncoder(w, int64(len(dict))); err != nil {
					return newEncoderError(friendlyName, err)
				}
				for k, v := range dict {
					if err = stringEncoder(w, k); err != nil {
						return newEncoderError(friendlyName, err)
					}
					if err = valuesCodec.ef(w, v); err != nil {
						return newEncoderError(friendlyName, err)
					}
				}
			}
			if err = longEncoder(w, int64(0)); err != nil {
				return newEncoderError(friendlyName, err)
			}
			return nil
		},
	}, nil
}

func (st symtab) makeArrayCodec(enclosingNamespace string, schema interface{}) (*codec, error) {
	errorNamespace := "null namespace"
	if enclosingNamespace != nullNamespace {
		errorNamespace = enclosingNamespace
	}
	friendlyName := fmt.Sprintf("array (%s)", errorNamespace)

	// schema checks
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil, newCodecBuildError(friendlyName, "expected: map[string]interface{}; received: %T", schema)
	}
	v, ok := schemaMap["items"]
	if !ok {
		return nil, newCodecBuildError(friendlyName, "ought to have items key")
	}
	valuesCodec, err := st.buildCodec(enclosingNamespace, v)
	if err != nil {
		return nil, newCodecBuildError(friendlyName, err)
	}

	const itemsPerArrayBlock = 10
	nm := &name{n: "array"}
	friendlyName = fmt.Sprintf("array (%s)", nm.n)

	return &codec{
		nm: nm,
		df: func(r io.Reader) (interface{}, error) {
			data := make([]interface{}, 0)

			someValue, err := longDecoder(r)
			if err != nil {
				return nil, newDecoderError(friendlyName, err)
			}
			blockCount := someValue.(int64)

			for blockCount != 0 {
				if blockCount < 0 {
					blockCount = -blockCount
					// read and discard number of bytes in block
					_, err = longDecoder(r)
					if err != nil {
						return nil, newDecoderError(friendlyName, err)
					}
				}
				for i := int64(0); i < blockCount; i++ {
					datum, err := valuesCodec.df(r)
					if err != nil {
						return nil, newDecoderError(friendlyName, err)
					}
					data = append(data, datum)
				}
				someValue, err = longDecoder(r)
				if err != nil {
					return nil, newDecoderError(friendlyName, err)
				}
				blockCount = someValue.(int64)
			}
			return data, nil
		},
		ef: func(w io.Writer, datum interface{}) error {
			someArray, ok := datum.([]interface{})
			if !ok {
				return newEncoderError(friendlyName, "expected: []interface{}; received: %T", datum)
			}
			for leftIndex := 0; leftIndex < len(someArray); leftIndex += itemsPerArrayBlock {
				rightIndex := leftIndex + itemsPerArrayBlock
				if rightIndex > len(someArray) {
					rightIndex = len(someArray)
				}
				items := someArray[leftIndex:rightIndex]
				err = longEncoder(w, int64(len(items)))
				if err != nil {
					return newEncoderError(friendlyName, err)
				}
				for _, item := range items {
					err = valuesCodec.ef(w, item)
					if err != nil {
						return newEncoderError(friendlyName, err)
					}
				}
			}
			return longEncoder(w, int64(0))
		},
	}, nil
}
