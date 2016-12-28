// Copyright 2012-2015 Apcera Inc. All rights reserved.

package builtin

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"unsafe"
)

// DefaultEncoder implementation for EncodedConn.
// This encoder will leave []byte and string untouched, but will attempt to
// turn numbers into appropriate strings that can be decoded. It will also
// propely encoded and decode bools. If will encode a struct, but if you want
// to properly handle structures you should use JsonEncoder.
type DefaultEncoder struct {
	// Empty
}

var trueB = []byte("true")
var falseB = []byte("false")
var nilB = []byte("")

// Encode
func (je *DefaultEncoder) Encode(subject string, v interface{}) ([]byte, error) {
	switch arg := v.(type) {
	case string:
		bytes := *(*[]byte)(unsafe.Pointer(&arg))
		return bytes, nil
	case []byte:
		return arg, nil
	case bool:
		if arg {
			return trueB, nil
		} else {
			return falseB, nil
		}
	case nil:
		return nilB, nil
	default:
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "%+v", arg)
		return buf.Bytes(), nil
	}
}

// Decode
func (je *DefaultEncoder) Decode(subject string, data []byte, vPtr interface{}) error {
	// Figure out what it's pointing to...
	sData := *(*string)(unsafe.Pointer(&data))
	switch arg := vPtr.(type) {
	case *string:
		*arg = sData
		return nil
	case *[]byte:
		*arg = data
		return nil
	case *int:
		n, err := strconv.ParseInt(sData, 10, 64)
		if err != nil {
			return err
		}
		*arg = int(n)
		return nil
	case *int32:
		n, err := strconv.ParseInt(sData, 10, 64)
		if err != nil {
			return err
		}
		*arg = int32(n)
		return nil
	case *int64:
		n, err := strconv.ParseInt(sData, 10, 64)
		if err != nil {
			return err
		}
		*arg = int64(n)
		return nil
	case *float32:
		n, err := strconv.ParseFloat(sData, 32)
		if err != nil {
			return err
		}
		*arg = float32(n)
		return nil
	case *float64:
		n, err := strconv.ParseFloat(sData, 64)
		if err != nil {
			return err
		}
		*arg = float64(n)
		return nil
	case *bool:
		b, err := strconv.ParseBool(sData)
		if err != nil {
			return err
		}
		*arg = b
		return nil
	default:
		vt := reflect.TypeOf(arg).Elem()
		return fmt.Errorf("nats: Default Encoder can't decode to type %s", vt)
	}
}
