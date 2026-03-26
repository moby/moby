// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

import (
	"encoding"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
)

// Format represents a string format.
//
// All implementations of Format provide a string representation and text
// marshaling/unmarshaling interface to be used by encoders (e.g. encoding/[json]).
type Format interface {
	String() string
	encoding.TextMarshaler
	encoding.TextUnmarshaler
}

// Registry is a registry of string formats, with a validation method.
type Registry interface {
	Add(name string, strfmt Format, validator Validator) bool
	DelByName(name string) bool
	GetType(name string) (reflect.Type, bool)
	ContainsName(name string) bool
	Validates(name, data string) bool
	Parse(name, data string) (any, error)
	MapStructureHookFunc() mapstructure.DecodeHookFunc
}
