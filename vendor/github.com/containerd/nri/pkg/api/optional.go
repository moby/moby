/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

import (
	"os"
	"slices"
)

//
// XXX FIXME:
//
// The optional interface constructor should be updated/split up
// to avoid having to take an interface{} argument. Instead The
// optional types should have a
//   - constructor taking the underlying native type
//   - a Copy() function for copying them
//   - a FromPointer constructor to create them from an optionally nil
//     pointer to the underlying native type (to help constructing from
//     structures that use a pointer to the native underlying type to
//     denote optionality (OCI Spec mostly))
// Creating from any other type should use one of these with any explicit
// cast for the argument as necessary.
//

// String creates an Optional wrapper from its argument.
func String(v interface{}) *OptionalString {
	var value string

	switch o := v.(type) {
	case string:
		value = o
	case *string:
		if o == nil {
			return nil
		}
		value = *o
	case *OptionalString:
		if o == nil {
			return nil
		}
		value = o.Value
	default:
		return nil
	}

	return &OptionalString{
		Value: value,
	}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalString) Get() *string {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// RepeatedString creates an Optional wrapper from its argument.
func RepeatedString(v interface{}) *OptionalRepeatedString {
	var value []string

	switch o := v.(type) {
	case []string:
		value = o
	case *[]string:
		if o == nil {
			return nil
		}
		value = *o
	case *OptionalRepeatedString:
		if o == nil {
			return nil
		}
		value = o.Value
	default:
		return nil
	}

	return &OptionalRepeatedString{
		Value: slices.Clone(value),
	}
}

// Get returns nil if its value is unset or a pointer to a copy of the value.
func (o *OptionalRepeatedString) Get() *[]string {
	if o == nil {
		return nil
	}
	v := slices.Clone(o.Value)
	return &v
}

// Int creates an Optional wrapper from its argument.
func Int(v interface{}) *OptionalInt {
	var value int64

	switch o := v.(type) {
	case int:
		value = int64(o)
	case *int:
		if o == nil {
			return nil
		}
		value = int64(*o)
	case *OptionalInt:
		if o == nil {
			return nil
		}
		value = o.Value
	default:
		return nil
	}

	return &OptionalInt{
		Value: value,
	}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalInt) Get() *int {
	if o == nil {
		return nil
	}
	v := int(o.Value)
	return &v
}

// Int32 creates an Optional wrapper from its argument.
func Int32(v interface{}) *OptionalInt32 {
	var value int32

	switch o := v.(type) {
	case int32:
		value = o
	case *int32:
		if o == nil {
			return nil
		}
		value = *o
	case *OptionalInt32:
		if o == nil {
			return nil
		}
		value = o.Value
	default:
		return nil
	}

	return &OptionalInt32{
		Value: value,
	}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalInt32) Get() *int32 {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// UInt32 creates an Optional wrapper from its argument.
func UInt32(v interface{}) *OptionalUInt32 {
	var value uint32

	switch o := v.(type) {
	case uint32:
		value = o
	case *uint32:
		if o == nil {
			return nil
		}
		value = *o
	case *OptionalUInt32:
		if o == nil {
			return nil
		}
		value = o.Value
	default:
		return nil
	}

	return &OptionalUInt32{
		Value: value,
	}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalUInt32) Get() *uint32 {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// Int64 creates an Optional wrapper from its argument.
func Int64(v interface{}) *OptionalInt64 {
	var value int64

	switch o := v.(type) {
	case int:
		value = int64(o)
	case uint:
		value = int64(o)
	case uint64:
		value = int64(o)
	case int64:
		value = o
	case *int64:
		if o == nil {
			return nil
		}
		value = *o
	case *uint64:
		if o == nil {
			return nil
		}
		value = int64(*o)
	case *OptionalInt64:
		if o == nil {
			return nil
		}
		value = o.Value
	default:
		return nil
	}

	return &OptionalInt64{
		Value: value,
	}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalInt64) Get() *int64 {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// UInt64 creates an Optional wrapper from its argument.
func UInt64(v interface{}) *OptionalUInt64 {
	var value uint64

	switch o := v.(type) {
	case int:
		value = uint64(o)
	case uint:
		value = uint64(o)
	case int64:
		value = uint64(o)
	case uint64:
		value = o
	case *int64:
		if o == nil {
			return nil
		}
		value = uint64(*o)
	case *uint64:
		if o == nil {
			return nil
		}
		value = *o
	case *OptionalUInt64:
		if o == nil {
			return nil
		}
		value = o.Value
	default:
		return nil
	}

	return &OptionalUInt64{
		Value: value,
	}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalUInt64) Get() *uint64 {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// Bool creates an Optional wrapper from its argument.
func Bool(v interface{}) *OptionalBool {
	var value bool

	switch o := v.(type) {
	case bool:
		value = o
	case *bool:
		if o == nil {
			return nil
		}
		value = *o
	case *OptionalBool:
		if o == nil {
			return nil
		}
		value = o.Value
	default:
		return nil
	}

	return &OptionalBool{
		Value: value,
	}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalBool) Get() *bool {
	if o == nil {
		return nil
	}
	v := o.Value
	return &v
}

// FileMode creates an Optional wrapper from its argument.
func FileMode(v interface{}) *OptionalFileMode {
	var value os.FileMode

	switch o := v.(type) {
	case *os.FileMode:
		if o == nil {
			return nil
		}
		value = *o
	case os.FileMode:
		value = o
	case *OptionalFileMode:
		if o == nil {
			return nil
		}
		value = os.FileMode(o.Value)
	case uint32:
		value = os.FileMode(o)
	default:
		return nil
	}

	return &OptionalFileMode{
		Value: uint32(value),
	}
}

// Get returns nil if its value is unset or a pointer to the value itself.
func (o *OptionalFileMode) Get() *os.FileMode {
	if o == nil {
		return nil
	}
	v := os.FileMode(o.Value)
	return &v
}
