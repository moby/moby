// Copyright 2015 xeipuuv ( https://github.com/xeipuuv )
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

// author  			xeipuuv
// author-github 	https://github.com/xeipuuv
// author-mail		xeipuuv@gmail.com
//
// repository-name	gojsonpointer
// repository-desc	An implementation of JSON Pointer - Go language
//
// description		Main and unique file.
//
// created      	25-02-2013

package gojsonpointer

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	const_empty_pointer     = ``
	const_pointer_separator = `/`

	const_invalid_start = `JSON pointer must be empty or start with a "` + const_pointer_separator + `"`
)

type implStruct struct {
	mode string // "SET" or "GET"

	inDocument interface{}

	setInValue interface{}

	getOutNode interface{}
	getOutKind reflect.Kind
	outError   error
}

func NewJsonPointer(jsonPointerString string) (JsonPointer, error) {

	var p JsonPointer
	err := p.parse(jsonPointerString)
	return p, err

}

type JsonPointer struct {
	referenceTokens []string
}

// "Constructor", parses the given string JSON pointer
func (p *JsonPointer) parse(jsonPointerString string) error {

	var err error

	if jsonPointerString != const_empty_pointer {
		if !strings.HasPrefix(jsonPointerString, const_pointer_separator) {
			err = errors.New(const_invalid_start)
		} else {
			referenceTokens := strings.Split(jsonPointerString, const_pointer_separator)
			for _, referenceToken := range referenceTokens[1:] {
				p.referenceTokens = append(p.referenceTokens, referenceToken)
			}
		}
	}

	return err
}

// Uses the pointer to retrieve a value from a JSON document
func (p *JsonPointer) Get(document interface{}) (interface{}, reflect.Kind, error) {

	is := &implStruct{mode: "GET", inDocument: document}
	p.implementation(is)
	return is.getOutNode, is.getOutKind, is.outError

}

// Uses the pointer to update a value from a JSON document
func (p *JsonPointer) Set(document interface{}, value interface{}) (interface{}, error) {

	is := &implStruct{mode: "SET", inDocument: document, setInValue: value}
	p.implementation(is)
	return document, is.outError

}

// Both Get and Set functions use the same implementation to avoid code duplication
func (p *JsonPointer) implementation(i *implStruct) {

	kind := reflect.Invalid

	// Full document when empty
	if len(p.referenceTokens) == 0 {
		i.getOutNode = i.inDocument
		i.outError = nil
		i.getOutKind = kind
		i.outError = nil
		return
	}

	node := i.inDocument

	for ti, token := range p.referenceTokens {

		decodedToken := decodeReferenceToken(token)
		isLastToken := ti == len(p.referenceTokens)-1

		rValue := reflect.ValueOf(node)
		kind = rValue.Kind()

		switch kind {

		case reflect.Map:
			m := node.(map[string]interface{})
			if _, ok := m[decodedToken]; ok {
				node = m[decodedToken]
				if isLastToken && i.mode == "SET" {
					m[decodedToken] = i.setInValue
				}
			} else {
				i.outError = errors.New(fmt.Sprintf("Object has no key '%s'", token))
				i.getOutKind = kind
				i.getOutNode = nil
				return
			}

		case reflect.Slice:
			s := node.([]interface{})
			tokenIndex, err := strconv.Atoi(token)
			if err != nil {
				i.outError = errors.New(fmt.Sprintf("Invalid array index '%s'", token))
				i.getOutKind = kind
				i.getOutNode = nil
				return
			}
			sLength := len(s)
			if tokenIndex < 0 || tokenIndex >= sLength {
				i.outError = errors.New(fmt.Sprintf("Out of bound array[0,%d] index '%d'", sLength, tokenIndex))
				i.getOutKind = kind
				i.getOutNode = nil
				return
			}

			node = s[tokenIndex]
			if isLastToken && i.mode == "SET" {
				s[tokenIndex] = i.setInValue
			}

		default:
			i.outError = errors.New(fmt.Sprintf("Invalid token reference '%s'", token))
			i.getOutKind = kind
			i.getOutNode = nil
			return
		}

	}

	rValue := reflect.ValueOf(node)
	kind = rValue.Kind()

	i.getOutNode = node
	i.getOutKind = kind
	i.outError = nil
}

// Pointer to string representation function
func (p *JsonPointer) String() string {

	if len(p.referenceTokens) == 0 {
		return const_empty_pointer
	}

	pointerString := const_pointer_separator + strings.Join(p.referenceTokens, const_pointer_separator)

	return pointerString
}

// Specific JSON pointer encoding here
// ~0 => ~
// ~1 => /
// ... and vice versa

const (
	const_encoded_reference_token_0 = `~0`
	const_encoded_reference_token_1 = `~1`
	const_decoded_reference_token_0 = `~`
	const_decoded_reference_token_1 = `/`
)

func decodeReferenceToken(token string) string {
	step1 := strings.Replace(token, const_encoded_reference_token_1, const_decoded_reference_token_1, -1)
	step2 := strings.Replace(step1, const_encoded_reference_token_0, const_decoded_reference_token_0, -1)
	return step2
}

func encodeReferenceToken(token string) string {
	step1 := strings.Replace(token, const_decoded_reference_token_1, const_encoded_reference_token_1, -1)
	step2 := strings.Replace(step1, const_decoded_reference_token_0, const_encoded_reference_token_0, -1)
	return step2
}
