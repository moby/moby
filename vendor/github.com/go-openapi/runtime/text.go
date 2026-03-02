// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/go-openapi/swag/jsonutils"
)

// TextConsumer creates a new text consumer
func TextConsumer() Consumer {
	return ConsumerFunc(func(reader io.Reader, data any) error {
		if reader == nil {
			return errors.New("TextConsumer requires a reader") // early exit
		}

		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(reader)
		if err != nil {
			return err
		}
		b := buf.Bytes()

		// If the buffer is empty, no need to unmarshal it, which causes a panic.
		if len(b) == 0 {
			return nil
		}

		if tu, ok := data.(encoding.TextUnmarshaler); ok {
			err := tu.UnmarshalText(b)
			if err != nil {
				return fmt.Errorf("text consumer: %v", err)
			}

			return nil
		}

		t := reflect.TypeOf(data)
		if data != nil && t.Kind() == reflect.Ptr {
			v := reflect.Indirect(reflect.ValueOf(data))
			if t.Elem().Kind() == reflect.String {
				v.SetString(string(b))
				return nil
			}
		}

		return fmt.Errorf("%v (%T) is not supported by the TextConsumer, %s",
			data, data, "can be resolved by supporting TextUnmarshaler interface")
	})
}

// TextProducer creates a new text producer
func TextProducer() Producer {
	return ProducerFunc(func(writer io.Writer, data any) error {
		if writer == nil {
			return errors.New("TextProducer requires a writer") // early exit
		}

		if data == nil {
			return errors.New("no data given to produce text from")
		}

		if tm, ok := data.(encoding.TextMarshaler); ok {
			txt, err := tm.MarshalText()
			if err != nil {
				return fmt.Errorf("text producer: %v", err)
			}
			_, err = writer.Write(txt)
			return err
		}

		if str, ok := data.(error); ok {
			_, err := writer.Write([]byte(str.Error()))
			return err
		}

		if str, ok := data.(fmt.Stringer); ok {
			_, err := writer.Write([]byte(str.String()))
			return err
		}

		v := reflect.Indirect(reflect.ValueOf(data))
		if t := v.Type(); t.Kind() == reflect.Struct || t.Kind() == reflect.Slice {
			b, err := jsonutils.WriteJSON(data)
			if err != nil {
				return err
			}
			_, err = writer.Write(b)
			return err
		}
		if v.Kind() != reflect.String {
			return fmt.Errorf("%T is not a supported type by the TextProducer", data)
		}

		_, err := writer.Write([]byte(v.String()))
		return err
	})
}
