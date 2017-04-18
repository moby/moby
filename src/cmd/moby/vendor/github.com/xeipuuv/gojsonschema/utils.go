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

// author           xeipuuv
// author-github    https://github.com/xeipuuv
// author-mail      xeipuuv@gmail.com
//
// repository-name  gojsonschema
// repository-desc  An implementation of JSON Schema, based on IETF's draft v4 - Go language.
//
// description      Various utility functions.
//
// created          26-02-2013

package gojsonschema

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

func isKind(what interface{}, kind reflect.Kind) bool {
	target := what
	if isJsonNumber(what) {
		// JSON Numbers are strings!
		target = *mustBeNumber(what)
	}
	return reflect.ValueOf(target).Kind() == kind
}

func existsMapKey(m map[string]interface{}, k string) bool {
	_, ok := m[k]
	return ok
}

func isStringInSlice(s []string, what string) bool {
	for i := range s {
		if s[i] == what {
			return true
		}
	}
	return false
}

func marshalToJsonString(value interface{}) (*string, error) {

	mBytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	sBytes := string(mBytes)
	return &sBytes, nil
}

func isJsonNumber(what interface{}) bool {

	switch what.(type) {

	case json.Number:
		return true
	}

	return false
}

func checkJsonNumber(what interface{}) (isValidFloat64 bool, isValidInt64 bool, isValidInt32 bool) {

	jsonNumber := what.(json.Number)

	f64, errFloat64 := jsonNumber.Float64()
	s64 := strconv.FormatFloat(f64, 'f', -1, 64)
	_, errInt64 := strconv.ParseInt(s64, 10, 64)

	isValidFloat64 = errFloat64 == nil
	isValidInt64 = errInt64 == nil

	_, errInt32 := strconv.ParseInt(s64, 10, 32)
	isValidInt32 = isValidInt64 && errInt32 == nil

	return

}

// same as ECMA Number.MAX_SAFE_INTEGER and Number.MIN_SAFE_INTEGER
const (
	max_json_float = float64(1<<53 - 1)  // 9007199254740991.0 	 2^53 - 1
	min_json_float = -float64(1<<53 - 1) //-9007199254740991.0	-2^53 - 1
)

func isFloat64AnInteger(f float64) bool {

	if math.IsNaN(f) || math.IsInf(f, 0) || f < min_json_float || f > max_json_float {
		return false
	}

	return f == float64(int64(f)) || f == float64(uint64(f))
}

func mustBeInteger(what interface{}) *int {

	if isJsonNumber(what) {

		number := what.(json.Number)

		_, _, isValidInt32 := checkJsonNumber(number)

		if isValidInt32 {

			int64Value, err := number.Int64()
			if err != nil {
				return nil
			}

			int32Value := int(int64Value)
			return &int32Value

		} else {
			return nil
		}

	}

	return nil
}

func mustBeNumber(what interface{}) *float64 {

	if isJsonNumber(what) {

		number := what.(json.Number)
		float64Value, err := number.Float64()

		if err == nil {
			return &float64Value
		} else {
			return nil
		}

	}

	return nil

}

// formats a number so that it is displayed as the smallest string possible
func resultErrorFormatJsonNumber(n json.Number) string {

	if int64Value, err := n.Int64(); err == nil {
		return fmt.Sprintf("%d", int64Value)
	}

	float64Value, _ := n.Float64()

	return fmt.Sprintf("%g", float64Value)
}

// formats a number so that it is displayed as the smallest string possible
func resultErrorFormatNumber(n float64) string {

	if isFloat64AnInteger(n) {
		return fmt.Sprintf("%d", int64(n))
	}

	return fmt.Sprintf("%g", n)
}

func convertDocumentNode(val interface{}) interface{} {

	if lval, ok := val.([]interface{}); ok {

		res := []interface{}{}
		for _, v := range lval {
			res = append(res, convertDocumentNode(v))
		}

		return res

	}

	if mval, ok := val.(map[interface{}]interface{}); ok {

		res := map[string]interface{}{}

		for k, v := range mval {
			res[k.(string)] = convertDocumentNode(v)
		}

		return res

	}

	return val
}
