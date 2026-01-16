// Copyright 2013 sigu-399 ( https://github.com/sigu-399 )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author       sigu-399
// author-github  https://github.com/sigu-399
// author-mail    sigu.399@gmail.com
//
// repository-name  jsonpointer
// repository-desc  An implementation of JSON Pointer - Go language
//
// description    Main and unique file.
//
// created        25-02-2013

package jsonpointer

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-openapi/swag/jsonname"
)

const (
	emptyPointer     = ``
	pointerSeparator = `/`
)

var (
	jsonPointableType = reflect.TypeOf(new(JSONPointable)).Elem()
	jsonSetableType   = reflect.TypeOf(new(JSONSetable)).Elem()
)

// JSONPointable is an interface for structs to implement when they need to customize the
// json pointer process
type JSONPointable interface {
	JSONLookup(string) (any, error)
}

// JSONSetable is an interface for structs to implement when they need to customize the
// json pointer process
type JSONSetable interface {
	JSONSet(string, any) error
}

// Pointer is a representation of a json pointer
type Pointer struct {
	referenceTokens []string
}

// New creates a new json pointer for the given string
func New(jsonPointerString string) (Pointer, error) {
	var p Pointer
	err := p.parse(jsonPointerString)

	return p, err
}

// Get uses the pointer to retrieve a value from a JSON document
func (p *Pointer) Get(document any) (any, reflect.Kind, error) {
	return p.get(document, jsonname.DefaultJSONNameProvider)
}

// Set uses the pointer to set a value from a JSON document
func (p *Pointer) Set(document any, value any) (any, error) {
	return document, p.set(document, value, jsonname.DefaultJSONNameProvider)
}

// DecodedTokens returns the decoded tokens of this JSON pointer
func (p *Pointer) DecodedTokens() []string {
	result := make([]string, 0, len(p.referenceTokens))
	for _, t := range p.referenceTokens {
		result = append(result, Unescape(t))
	}
	return result
}

// IsEmpty returns true if this is an empty json pointer
// this indicates that it points to the root document
func (p *Pointer) IsEmpty() bool {
	return len(p.referenceTokens) == 0
}

// Pointer to string representation function
func (p *Pointer) String() string {

	if len(p.referenceTokens) == 0 {
		return emptyPointer
	}

	return pointerSeparator + strings.Join(p.referenceTokens, pointerSeparator)
}

func (p *Pointer) Offset(document string) (int64, error) {
	dec := json.NewDecoder(strings.NewReader(document))
	var offset int64
	for _, ttk := range p.DecodedTokens() {
		tk, err := dec.Token()
		if err != nil {
			return 0, err
		}
		switch tk := tk.(type) {
		case json.Delim:
			switch tk {
			case '{':
				offset, err = offsetSingleObject(dec, ttk)
				if err != nil {
					return 0, err
				}
			case '[':
				offset, err = offsetSingleArray(dec, ttk)
				if err != nil {
					return 0, err
				}
			default:
				return 0, fmt.Errorf("invalid token %#v: %w", tk, ErrPointer)
			}
		default:
			return 0, fmt.Errorf("invalid token %#v: %w", tk, ErrPointer)
		}
	}
	return offset, nil
}

// "Constructor", parses the given string JSON pointer
func (p *Pointer) parse(jsonPointerString string) error {
	var err error

	if jsonPointerString != emptyPointer {
		if !strings.HasPrefix(jsonPointerString, pointerSeparator) {
			err = errors.Join(ErrInvalidStart, ErrPointer)
		} else {
			referenceTokens := strings.Split(jsonPointerString, pointerSeparator)
			p.referenceTokens = append(p.referenceTokens, referenceTokens[1:]...)
		}
	}

	return err
}

func (p *Pointer) get(node any, nameProvider *jsonname.NameProvider) (any, reflect.Kind, error) {
	if nameProvider == nil {
		nameProvider = jsonname.DefaultJSONNameProvider
	}

	kind := reflect.Invalid

	// Full document when empty
	if len(p.referenceTokens) == 0 {
		return node, kind, nil
	}

	for _, token := range p.referenceTokens {
		decodedToken := Unescape(token)

		r, knd, err := getSingleImpl(node, decodedToken, nameProvider)
		if err != nil {
			return nil, knd, err
		}
		node = r
	}

	rValue := reflect.ValueOf(node)
	kind = rValue.Kind()

	return node, kind, nil
}

func (p *Pointer) set(node, data any, nameProvider *jsonname.NameProvider) error {
	knd := reflect.ValueOf(node).Kind()

	if knd != reflect.Pointer && knd != reflect.Struct && knd != reflect.Map && knd != reflect.Slice && knd != reflect.Array {
		return errors.Join(
			ErrUnsupportedValueType,
			ErrPointer,
		)
	}

	if nameProvider == nil {
		nameProvider = jsonname.DefaultJSONNameProvider
	}

	// Full document when empty
	if len(p.referenceTokens) == 0 {
		return nil
	}

	lastI := len(p.referenceTokens) - 1
	for i, token := range p.referenceTokens {
		isLastToken := i == lastI
		decodedToken := Unescape(token)

		if isLastToken {

			return setSingleImpl(node, data, decodedToken, nameProvider)
		}

		// Check for nil during traversal
		if isNil(node) {
			return fmt.Errorf("cannot traverse through nil value at %q: %w", decodedToken, ErrPointer)
		}

		rValue := reflect.Indirect(reflect.ValueOf(node))
		kind := rValue.Kind()

		if rValue.Type().Implements(jsonPointableType) {
			r, err := node.(JSONPointable).JSONLookup(decodedToken)
			if err != nil {
				return err
			}
			fld := reflect.ValueOf(r)
			if fld.CanAddr() && fld.Kind() != reflect.Interface && fld.Kind() != reflect.Map && fld.Kind() != reflect.Slice && fld.Kind() != reflect.Pointer {
				node = fld.Addr().Interface()
				continue
			}
			node = r
			continue
		}

		switch kind { //nolint:exhaustive
		case reflect.Struct:
			nm, ok := nameProvider.GetGoNameForType(rValue.Type(), decodedToken)
			if !ok {
				return fmt.Errorf("object has no field %q: %w", decodedToken, ErrPointer)
			}
			fld := rValue.FieldByName(nm)
			if fld.CanAddr() && fld.Kind() != reflect.Interface && fld.Kind() != reflect.Map && fld.Kind() != reflect.Slice && fld.Kind() != reflect.Pointer {
				node = fld.Addr().Interface()
				continue
			}
			node = fld.Interface()

		case reflect.Map:
			kv := reflect.ValueOf(decodedToken)
			mv := rValue.MapIndex(kv)

			if !mv.IsValid() {
				return fmt.Errorf("object has no key %q: %w", decodedToken, ErrPointer)
			}
			if mv.CanAddr() && mv.Kind() != reflect.Interface && mv.Kind() != reflect.Map && mv.Kind() != reflect.Slice && mv.Kind() != reflect.Pointer {
				node = mv.Addr().Interface()
				continue
			}
			node = mv.Interface()

		case reflect.Slice:
			tokenIndex, err := strconv.Atoi(decodedToken)
			if err != nil {
				return err
			}
			sLength := rValue.Len()
			if tokenIndex < 0 || tokenIndex >= sLength {
				return fmt.Errorf("index out of bounds array[0,%d] index '%d': %w", sLength, tokenIndex, ErrPointer)
			}

			elem := rValue.Index(tokenIndex)
			if elem.CanAddr() && elem.Kind() != reflect.Interface && elem.Kind() != reflect.Map && elem.Kind() != reflect.Slice && elem.Kind() != reflect.Pointer {
				node = elem.Addr().Interface()
				continue
			}
			node = elem.Interface()

		default:
			return fmt.Errorf("invalid token reference %q: %w", decodedToken, ErrPointer)
		}
	}

	return nil
}

func isNil(input any) bool {
	if input == nil {
		return true
	}

	kind := reflect.TypeOf(input).Kind()
	switch kind { //nolint:exhaustive
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Chan:
		return reflect.ValueOf(input).IsNil()
	default:
		return false
	}
}

// GetForToken gets a value for a json pointer token 1 level deep
func GetForToken(document any, decodedToken string) (any, reflect.Kind, error) {
	return getSingleImpl(document, decodedToken, jsonname.DefaultJSONNameProvider)
}

// SetForToken gets a value for a json pointer token 1 level deep
func SetForToken(document any, decodedToken string, value any) (any, error) {
	return document, setSingleImpl(document, value, decodedToken, jsonname.DefaultJSONNameProvider)
}

func getSingleImpl(node any, decodedToken string, nameProvider *jsonname.NameProvider) (any, reflect.Kind, error) {
	rValue := reflect.Indirect(reflect.ValueOf(node))
	kind := rValue.Kind()
	if isNil(node) {
		return nil, kind, fmt.Errorf("nil value has no field %q: %w", decodedToken, ErrPointer)
	}

	switch typed := node.(type) {
	case JSONPointable:
		r, err := typed.JSONLookup(decodedToken)
		if err != nil {
			return nil, kind, err
		}
		return r, kind, nil
	case *any: // case of a pointer to interface, that is not resolved by reflect.Indirect
		return getSingleImpl(*typed, decodedToken, nameProvider)
	}

	switch kind { //nolint:exhaustive
	case reflect.Struct:
		nm, ok := nameProvider.GetGoNameForType(rValue.Type(), decodedToken)
		if !ok {
			return nil, kind, fmt.Errorf("object has no field %q: %w", decodedToken, ErrPointer)
		}
		fld := rValue.FieldByName(nm)
		return fld.Interface(), kind, nil

	case reflect.Map:
		kv := reflect.ValueOf(decodedToken)
		mv := rValue.MapIndex(kv)

		if mv.IsValid() {
			return mv.Interface(), kind, nil
		}
		return nil, kind, fmt.Errorf("object has no key %q: %w", decodedToken, ErrPointer)

	case reflect.Slice:
		tokenIndex, err := strconv.Atoi(decodedToken)
		if err != nil {
			return nil, kind, err
		}
		sLength := rValue.Len()
		if tokenIndex < 0 || tokenIndex >= sLength {
			return nil, kind, fmt.Errorf("index out of bounds array[0,%d] index '%d': %w", sLength-1, tokenIndex, ErrPointer)
		}

		elem := rValue.Index(tokenIndex)
		return elem.Interface(), kind, nil

	default:
		return nil, kind, fmt.Errorf("invalid token reference %q: %w", decodedToken, ErrPointer)
	}
}

func setSingleImpl(node, data any, decodedToken string, nameProvider *jsonname.NameProvider) error {
	rValue := reflect.Indirect(reflect.ValueOf(node))

	// Check for nil to prevent panic when calling rValue.Type()
	if isNil(node) {
		return fmt.Errorf("cannot set field %q on nil value: %w", decodedToken, ErrPointer)
	}

	if ns, ok := node.(JSONSetable); ok { // pointer impl
		return ns.JSONSet(decodedToken, data)
	}

	if rValue.Type().Implements(jsonSetableType) {
		return node.(JSONSetable).JSONSet(decodedToken, data)
	}

	switch rValue.Kind() { //nolint:exhaustive
	case reflect.Struct:
		nm, ok := nameProvider.GetGoNameForType(rValue.Type(), decodedToken)
		if !ok {
			return fmt.Errorf("object has no field %q: %w", decodedToken, ErrPointer)
		}
		fld := rValue.FieldByName(nm)
		if fld.IsValid() {
			fld.Set(reflect.ValueOf(data))
		}
		return nil

	case reflect.Map:
		kv := reflect.ValueOf(decodedToken)
		rValue.SetMapIndex(kv, reflect.ValueOf(data))
		return nil

	case reflect.Slice:
		tokenIndex, err := strconv.Atoi(decodedToken)
		if err != nil {
			return err
		}
		sLength := rValue.Len()
		if tokenIndex < 0 || tokenIndex >= sLength {
			return fmt.Errorf("index out of bounds array[0,%d] index '%d': %w", sLength, tokenIndex, ErrPointer)
		}

		elem := rValue.Index(tokenIndex)
		if !elem.CanSet() {
			return fmt.Errorf("can't set slice index %s to %v: %w", decodedToken, data, ErrPointer)
		}
		elem.Set(reflect.ValueOf(data))
		return nil

	default:
		return fmt.Errorf("invalid token reference %q: %w", decodedToken, ErrPointer)
	}
}

func offsetSingleObject(dec *json.Decoder, decodedToken string) (int64, error) {
	for dec.More() {
		offset := dec.InputOffset()
		tk, err := dec.Token()
		if err != nil {
			return 0, err
		}
		switch tk := tk.(type) {
		case json.Delim:
			switch tk {
			case '{':
				if err = drainSingle(dec); err != nil {
					return 0, err
				}
			case '[':
				if err = drainSingle(dec); err != nil {
					return 0, err
				}
			}
		case string:
			if tk == decodedToken {
				return offset, nil
			}
		default:
			return 0, fmt.Errorf("invalid token %#v: %w", tk, ErrPointer)
		}
	}
	return 0, fmt.Errorf("token reference %q not found: %w", decodedToken, ErrPointer)
}

func offsetSingleArray(dec *json.Decoder, decodedToken string) (int64, error) {
	idx, err := strconv.Atoi(decodedToken)
	if err != nil {
		return 0, fmt.Errorf("token reference %q is not a number: %v: %w", decodedToken, err, ErrPointer)
	}
	var i int
	for i = 0; i < idx && dec.More(); i++ {
		tk, err := dec.Token()
		if err != nil {
			return 0, err
		}

		if delim, isDelim := tk.(json.Delim); isDelim {
			switch delim {
			case '{':
				if err = drainSingle(dec); err != nil {
					return 0, err
				}
			case '[':
				if err = drainSingle(dec); err != nil {
					return 0, err
				}
			}
		}
	}

	if !dec.More() {
		return 0, fmt.Errorf("token reference %q not found: %w", decodedToken, ErrPointer)
	}
	return dec.InputOffset(), nil
}

// drainSingle drains a single level of object or array.
// The decoder has to guarantee the beginning delim (i.e. '{' or '[') has been consumed.
func drainSingle(dec *json.Decoder) error {
	for dec.More() {
		tk, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, isDelim := tk.(json.Delim); isDelim {
			switch delim {
			case '{':
				if err = drainSingle(dec); err != nil {
					return err
				}
			case '[':
				if err = drainSingle(dec); err != nil {
					return err
				}
			}
		}
	}

	// Consumes the ending delim
	if _, err := dec.Token(); err != nil {
		return err
	}
	return nil
}

// Specific JSON pointer encoding here
// ~0 => ~
// ~1 => /
// ... and vice versa

const (
	encRefTok0 = `~0`
	encRefTok1 = `~1`
	decRefTok0 = `~`
	decRefTok1 = `/`
)

var (
	encRefTokReplacer = strings.NewReplacer(encRefTok1, decRefTok1, encRefTok0, decRefTok0)
	decRefTokReplacer = strings.NewReplacer(decRefTok1, encRefTok1, decRefTok0, encRefTok0)
)

// Unescape unescapes a json pointer reference token string to the original representation
func Unescape(token string) string {
	return encRefTokReplacer.Replace(token)
}

// Escape escapes a pointer reference token string
func Escape(token string) string {
	return decRefTokReplacer.Replace(token)
}
