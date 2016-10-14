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

package goavro

import (
	"fmt"
	"io"
	"math"
)

type ByteWriter interface {
	Grow(int)
	WriteByte(byte) error
}

type StringWriter interface {
	WriteString(string) (int, error)
}

// ErrEncoder is returned when the encoder encounters an error.
type ErrEncoder struct {
	Message string
	Err     error
}

func (e ErrEncoder) Error() string {
	if e.Err == nil {
		return "cannot encode " + e.Message
	}
	return "cannot encode " + e.Message + ": " + e.Err.Error()
}

func newEncoderError(dataType string, a ...interface{}) *ErrEncoder {
	var err error
	var format, message string
	var ok bool
	if len(a) == 0 {
		return &ErrEncoder{dataType + ": no reason given", nil}
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
	return &ErrEncoder{dataType + message, err}
}

func nullEncoder(_ io.Writer, _ interface{}) error {
	return nil
}

func booleanEncoder(w io.Writer, datum interface{}) error {
	someBoolean, ok := datum.(bool)
	if !ok {
		return newEncoderError("boolean", "expected: bool; received: %T", datum)
	}

	var b byte
	if someBoolean {
		b = byte(1)
	}

	var err error
	if bw, ok := w.(ByteWriter); ok {
		err = bw.WriteByte(b)
	} else {
		bb := make([]byte, 1)
		bb[0] = b
		_, err = w.Write(bb)
	}
	if err != nil {
		return newEncoderError("boolean", err)
	}
	return nil
}

func writeInt(w io.Writer, byteCount int, encoded uint64) error {
	var err error
	var bb []byte
	bw, ok := w.(ByteWriter)
	// To avoid reallocations, grow capacity to the largest possible size
	// for this integer
	if ok {
		bw.Grow(byteCount)
	} else {
		bb = make([]byte, 0, byteCount)
	}

	if encoded == 0 {
		if bw != nil {
			err = bw.WriteByte(0)
			if err != nil {
				return err
			}
		} else {
			bb = append(bb, byte(0))
		}
	} else {
		for encoded > 0 {
			b := byte(encoded & 127)
			encoded = encoded >> 7
			if !(encoded == 0) {
				b |= 128
			}
			if bw != nil {
				err = bw.WriteByte(b)
				if err != nil {
					return err
				}
			} else {
				bb = append(bb, b)
			}
		}
	}
	if bw == nil {
		_, err := w.Write(bb)
		return err
	}
	return nil

}

func intEncoder(w io.Writer, datum interface{}) error {
	downShift := uint32(31)
	someInt, ok := datum.(int32)
	if !ok {
		return newEncoderError("int", "expected: int32; received: %T", datum)
	}
	encoded := uint64((uint32(someInt) << 1) ^ uint32(someInt>>downShift))
	const maxByteSize = 5
	return writeInt(w, maxByteSize, encoded)
}

func longEncoder(w io.Writer, datum interface{}) error {
	downShift := uint32(63)
	someInt, ok := datum.(int64)
	if !ok {
		return newEncoderError("long", "expected: int64; received: %T", datum)
	}
	encoded := ((uint64(someInt) << 1) ^ uint64(someInt>>downShift))
	const maxByteSize = 10
	return writeInt(w, maxByteSize, encoded)
}

func writeFloat(w io.Writer, byteCount int, bits uint64) error {
	var err error
	var bb []byte
	bw, ok := w.(ByteWriter)
	if ok {
		bw.Grow(byteCount)
	} else {
		bb = make([]byte, 0, byteCount)
	}
	for i := 0; i < byteCount; i++ {
		if bw != nil {
			err = bw.WriteByte(byte(bits & 255))
			if err != nil {
				return err
			}
		} else {
			bb = append(bb, byte(bits&255))
		}
		bits = bits >> 8
	}
	if bw == nil {
		_, err = w.Write(bb)
		return err
	}
	return nil
}

func floatEncoder(w io.Writer, datum interface{}) error {
	someFloat, ok := datum.(float32)
	if !ok {
		return newEncoderError("float", "expected: float32; received: %T", datum)
	}
	bits := uint64(math.Float32bits(someFloat))
	const byteCount = 4
	return writeFloat(w, byteCount, bits)
}

func doubleEncoder(w io.Writer, datum interface{}) error {
	someFloat, ok := datum.(float64)
	if !ok {
		return newEncoderError("double", "expected: float64; received: %T", datum)
	}
	bits := uint64(math.Float64bits(someFloat))
	const byteCount = 8
	return writeFloat(w, byteCount, bits)
}

func bytesEncoder(w io.Writer, datum interface{}) error {
	someBytes, ok := datum.([]byte)
	if !ok {
		return newEncoderError("bytes", "expected: []byte; received: %T", datum)
	}
	err := longEncoder(w, int64(len(someBytes)))
	if err != nil {
		return newEncoderError("bytes", err)
	}
	_, err = w.Write(someBytes)
	return err
}

func stringEncoder(w io.Writer, datum interface{}) error {
	someString, ok := datum.(string)
	if !ok {
		return newEncoderError("string", "expected: string; received: %T", datum)
	}
	err := longEncoder(w, int64(len(someString)))
	if err != nil {
		return newEncoderError("string", err)
	}
	if sw, ok := w.(StringWriter); ok {
		_, err = sw.WriteString(someString)
	} else {
		_, err = w.Write([]byte(someString))
	}
	return err
}
