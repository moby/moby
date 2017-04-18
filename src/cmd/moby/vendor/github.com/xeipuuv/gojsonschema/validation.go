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
// description      Extends Schema and subSchema, implements the validation phase.
//
// created          28-02-2013

package gojsonschema

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Validate(ls JSONLoader, ld JSONLoader) (*Result, error) {

	var err error

	// load schema

	schema, err := NewSchema(ls)
	if err != nil {
		return nil, err
	}

	// begine validation

	return schema.Validate(ld)

}

func (v *Schema) Validate(l JSONLoader) (*Result, error) {

	// load document

	root, err := l.LoadJSON()
	if err != nil {
		return nil, err
	}

	// begin validation

	result := &Result{}
	context := newJsonContext(STRING_CONTEXT_ROOT, nil)
	v.rootSchema.validateRecursive(v.rootSchema, root, result, context)

	return result, nil

}

func (v *subSchema) subValidateWithContext(document interface{}, context *jsonContext) *Result {
	result := &Result{}
	v.validateRecursive(v, document, result, context)
	return result
}

// Walker function to validate the json recursively against the subSchema
func (v *subSchema) validateRecursive(currentSubSchema *subSchema, currentNode interface{}, result *Result, context *jsonContext) {

	if internalLogEnabled {
		internalLog("validateRecursive %s", context.String())
		internalLog(" %v", currentNode)
	}

	// Handle referenced schemas, returns directly when a $ref is found
	if currentSubSchema.refSchema != nil {
		v.validateRecursive(currentSubSchema.refSchema, currentNode, result, context)
		return
	}

	// Check for null value
	if currentNode == nil {
		if currentSubSchema.types.IsTyped() && !currentSubSchema.types.Contains(TYPE_NULL) {
			result.addError(
				new(InvalidTypeError),
				context,
				currentNode,
				ErrorDetails{
					"expected": currentSubSchema.types.String(),
					"given":    TYPE_NULL,
				},
			)
			return
		}

		currentSubSchema.validateSchema(currentSubSchema, currentNode, result, context)
		v.validateCommon(currentSubSchema, currentNode, result, context)

	} else { // Not a null value

		if isJsonNumber(currentNode) {

			value := currentNode.(json.Number)

			_, isValidInt64, _ := checkJsonNumber(value)

			validType := currentSubSchema.types.Contains(TYPE_NUMBER) || (isValidInt64 && currentSubSchema.types.Contains(TYPE_INTEGER))

			if currentSubSchema.types.IsTyped() && !validType {

				givenType := TYPE_INTEGER
				if !isValidInt64 {
					givenType = TYPE_NUMBER
				}

				result.addError(
					new(InvalidTypeError),
					context,
					currentNode,
					ErrorDetails{
						"expected": currentSubSchema.types.String(),
						"given":    givenType,
					},
				)
				return
			}

			currentSubSchema.validateSchema(currentSubSchema, value, result, context)
			v.validateNumber(currentSubSchema, value, result, context)
			v.validateCommon(currentSubSchema, value, result, context)
			v.validateString(currentSubSchema, value, result, context)

		} else {

			rValue := reflect.ValueOf(currentNode)
			rKind := rValue.Kind()

			switch rKind {

			// Slice => JSON array

			case reflect.Slice:

				if currentSubSchema.types.IsTyped() && !currentSubSchema.types.Contains(TYPE_ARRAY) {
					result.addError(
						new(InvalidTypeError),
						context,
						currentNode,
						ErrorDetails{
							"expected": currentSubSchema.types.String(),
							"given":    TYPE_ARRAY,
						},
					)
					return
				}

				castCurrentNode := currentNode.([]interface{})

				currentSubSchema.validateSchema(currentSubSchema, castCurrentNode, result, context)

				v.validateArray(currentSubSchema, castCurrentNode, result, context)
				v.validateCommon(currentSubSchema, castCurrentNode, result, context)

			// Map => JSON object

			case reflect.Map:
				if currentSubSchema.types.IsTyped() && !currentSubSchema.types.Contains(TYPE_OBJECT) {
					result.addError(
						new(InvalidTypeError),
						context,
						currentNode,
						ErrorDetails{
							"expected": currentSubSchema.types.String(),
							"given":    TYPE_OBJECT,
						},
					)
					return
				}

				castCurrentNode, ok := currentNode.(map[string]interface{})
				if !ok {
					castCurrentNode = convertDocumentNode(currentNode).(map[string]interface{})
				}

				currentSubSchema.validateSchema(currentSubSchema, castCurrentNode, result, context)

				v.validateObject(currentSubSchema, castCurrentNode, result, context)
				v.validateCommon(currentSubSchema, castCurrentNode, result, context)

				for _, pSchema := range currentSubSchema.propertiesChildren {
					nextNode, ok := castCurrentNode[pSchema.property]
					if ok {
						subContext := newJsonContext(pSchema.property, context)
						v.validateRecursive(pSchema, nextNode, result, subContext)
					}
				}

			// Simple JSON values : string, number, boolean

			case reflect.Bool:

				if currentSubSchema.types.IsTyped() && !currentSubSchema.types.Contains(TYPE_BOOLEAN) {
					result.addError(
						new(InvalidTypeError),
						context,
						currentNode,
						ErrorDetails{
							"expected": currentSubSchema.types.String(),
							"given":    TYPE_BOOLEAN,
						},
					)
					return
				}

				value := currentNode.(bool)

				currentSubSchema.validateSchema(currentSubSchema, value, result, context)
				v.validateNumber(currentSubSchema, value, result, context)
				v.validateCommon(currentSubSchema, value, result, context)
				v.validateString(currentSubSchema, value, result, context)

			case reflect.String:

				if currentSubSchema.types.IsTyped() && !currentSubSchema.types.Contains(TYPE_STRING) {
					result.addError(
						new(InvalidTypeError),
						context,
						currentNode,
						ErrorDetails{
							"expected": currentSubSchema.types.String(),
							"given":    TYPE_STRING,
						},
					)
					return
				}

				value := currentNode.(string)

				currentSubSchema.validateSchema(currentSubSchema, value, result, context)
				v.validateNumber(currentSubSchema, value, result, context)
				v.validateCommon(currentSubSchema, value, result, context)
				v.validateString(currentSubSchema, value, result, context)

			}

		}

	}

	result.incrementScore()
}

// Different kinds of validation there, subSchema / common / array / object / string...
func (v *subSchema) validateSchema(currentSubSchema *subSchema, currentNode interface{}, result *Result, context *jsonContext) {

	if internalLogEnabled {
		internalLog("validateSchema %s", context.String())
		internalLog(" %v", currentNode)
	}

	if len(currentSubSchema.anyOf) > 0 {

		validatedAnyOf := false
		var bestValidationResult *Result

		for _, anyOfSchema := range currentSubSchema.anyOf {
			if !validatedAnyOf {
				validationResult := anyOfSchema.subValidateWithContext(currentNode, context)
				validatedAnyOf = validationResult.Valid()

				if !validatedAnyOf && (bestValidationResult == nil || validationResult.score > bestValidationResult.score) {
					bestValidationResult = validationResult
				}
			}
		}
		if !validatedAnyOf {

			result.addError(new(NumberAnyOfError), context, currentNode, ErrorDetails{})

			if bestValidationResult != nil {
				// add error messages of closest matching subSchema as
				// that's probably the one the user was trying to match
				result.mergeErrors(bestValidationResult)
			}
		}
	}

	if len(currentSubSchema.oneOf) > 0 {

		nbValidated := 0
		var bestValidationResult *Result

		for _, oneOfSchema := range currentSubSchema.oneOf {
			validationResult := oneOfSchema.subValidateWithContext(currentNode, context)
			if validationResult.Valid() {
				nbValidated++
			} else if nbValidated == 0 && (bestValidationResult == nil || validationResult.score > bestValidationResult.score) {
				bestValidationResult = validationResult
			}
		}

		if nbValidated != 1 {

			result.addError(new(NumberOneOfError), context, currentNode, ErrorDetails{})

			if nbValidated == 0 {
				// add error messages of closest matching subSchema as
				// that's probably the one the user was trying to match
				result.mergeErrors(bestValidationResult)
			}
		}

	}

	if len(currentSubSchema.allOf) > 0 {
		nbValidated := 0

		for _, allOfSchema := range currentSubSchema.allOf {
			validationResult := allOfSchema.subValidateWithContext(currentNode, context)
			if validationResult.Valid() {
				nbValidated++
			}
			result.mergeErrors(validationResult)
		}

		if nbValidated != len(currentSubSchema.allOf) {
			result.addError(new(NumberAllOfError), context, currentNode, ErrorDetails{})
		}
	}

	if currentSubSchema.not != nil {
		validationResult := currentSubSchema.not.subValidateWithContext(currentNode, context)
		if validationResult.Valid() {
			result.addError(new(NumberNotError), context, currentNode, ErrorDetails{})
		}
	}

	if currentSubSchema.dependencies != nil && len(currentSubSchema.dependencies) > 0 {
		if isKind(currentNode, reflect.Map) {
			for elementKey := range currentNode.(map[string]interface{}) {
				if dependency, ok := currentSubSchema.dependencies[elementKey]; ok {
					switch dependency := dependency.(type) {

					case []string:
						for _, dependOnKey := range dependency {
							if _, dependencyResolved := currentNode.(map[string]interface{})[dependOnKey]; !dependencyResolved {
								result.addError(
									new(MissingDependencyError),
									context,
									currentNode,
									ErrorDetails{"dependency": dependOnKey},
								)
							}
						}

					case *subSchema:
						dependency.validateRecursive(dependency, currentNode, result, context)

					}
				}
			}
		}
	}

	result.incrementScore()
}

func (v *subSchema) validateCommon(currentSubSchema *subSchema, value interface{}, result *Result, context *jsonContext) {

	if internalLogEnabled {
		internalLog("validateCommon %s", context.String())
		internalLog(" %v", value)
	}

	// enum:
	if len(currentSubSchema.enum) > 0 {
		has, err := currentSubSchema.ContainsEnum(value)
		if err != nil {
			result.addError(new(InternalError), context, value, ErrorDetails{"error": err})
		}
		if !has {
			result.addError(
				new(EnumError),
				context,
				value,
				ErrorDetails{
					"allowed": strings.Join(currentSubSchema.enum, ", "),
				},
			)
		}
	}

	result.incrementScore()
}

func (v *subSchema) validateArray(currentSubSchema *subSchema, value []interface{}, result *Result, context *jsonContext) {

	if internalLogEnabled {
		internalLog("validateArray %s", context.String())
		internalLog(" %v", value)
	}

	nbValues := len(value)

	// TODO explain
	if currentSubSchema.itemsChildrenIsSingleSchema {
		for i := range value {
			subContext := newJsonContext(strconv.Itoa(i), context)
			validationResult := currentSubSchema.itemsChildren[0].subValidateWithContext(value[i], subContext)
			result.mergeErrors(validationResult)
		}
	} else {
		if currentSubSchema.itemsChildren != nil && len(currentSubSchema.itemsChildren) > 0 {

			nbItems := len(currentSubSchema.itemsChildren)

			// while we have both schemas and values, check them against each other
			for i := 0; i != nbItems && i != nbValues; i++ {
				subContext := newJsonContext(strconv.Itoa(i), context)
				validationResult := currentSubSchema.itemsChildren[i].subValidateWithContext(value[i], subContext)
				result.mergeErrors(validationResult)
			}

			if nbItems < nbValues {
				// we have less schemas than elements in the instance array,
				// but that might be ok if "additionalItems" is specified.

				switch currentSubSchema.additionalItems.(type) {
				case bool:
					if !currentSubSchema.additionalItems.(bool) {
						result.addError(new(ArrayNoAdditionalItemsError), context, value, ErrorDetails{})
					}
				case *subSchema:
					additionalItemSchema := currentSubSchema.additionalItems.(*subSchema)
					for i := nbItems; i != nbValues; i++ {
						subContext := newJsonContext(strconv.Itoa(i), context)
						validationResult := additionalItemSchema.subValidateWithContext(value[i], subContext)
						result.mergeErrors(validationResult)
					}
				}
			}
		}
	}

	// minItems & maxItems
	if currentSubSchema.minItems != nil {
		if nbValues < int(*currentSubSchema.minItems) {
			result.addError(
				new(ArrayMinItemsError),
				context,
				value,
				ErrorDetails{"min": *currentSubSchema.minItems},
			)
		}
	}
	if currentSubSchema.maxItems != nil {
		if nbValues > int(*currentSubSchema.maxItems) {
			result.addError(
				new(ArrayMaxItemsError),
				context,
				value,
				ErrorDetails{"max": *currentSubSchema.maxItems},
			)
		}
	}

	// uniqueItems:
	if currentSubSchema.uniqueItems {
		var stringifiedItems []string
		for _, v := range value {
			vString, err := marshalToJsonString(v)
			if err != nil {
				result.addError(new(InternalError), context, value, ErrorDetails{"err": err})
			}
			if isStringInSlice(stringifiedItems, *vString) {
				result.addError(
					new(ItemsMustBeUniqueError),
					context,
					value,
					ErrorDetails{"type": TYPE_ARRAY},
				)
			}
			stringifiedItems = append(stringifiedItems, *vString)
		}
	}

	result.incrementScore()
}

func (v *subSchema) validateObject(currentSubSchema *subSchema, value map[string]interface{}, result *Result, context *jsonContext) {

	if internalLogEnabled {
		internalLog("validateObject %s", context.String())
		internalLog(" %v", value)
	}

	// minProperties & maxProperties:
	if currentSubSchema.minProperties != nil {
		if len(value) < int(*currentSubSchema.minProperties) {
			result.addError(
				new(ArrayMinPropertiesError),
				context,
				value,
				ErrorDetails{"min": *currentSubSchema.minProperties},
			)
		}
	}
	if currentSubSchema.maxProperties != nil {
		if len(value) > int(*currentSubSchema.maxProperties) {
			result.addError(
				new(ArrayMaxPropertiesError),
				context,
				value,
				ErrorDetails{"max": *currentSubSchema.maxProperties},
			)
		}
	}

	// required:
	for _, requiredProperty := range currentSubSchema.required {
		_, ok := value[requiredProperty]
		if ok {
			result.incrementScore()
		} else {
			result.addError(
				new(RequiredError),
				context,
				value,
				ErrorDetails{"property": requiredProperty},
			)
		}
	}

	// additionalProperty & patternProperty:
	if currentSubSchema.additionalProperties != nil {

		switch currentSubSchema.additionalProperties.(type) {
		case bool:

			if !currentSubSchema.additionalProperties.(bool) {

				for pk := range value {

					found := false
					for _, spValue := range currentSubSchema.propertiesChildren {
						if pk == spValue.property {
							found = true
						}
					}

					pp_has, pp_match := v.validatePatternProperty(currentSubSchema, pk, value[pk], result, context)

					if found {

						if pp_has && !pp_match {
							result.addError(
								new(AdditionalPropertyNotAllowedError),
								context,
								value[pk],
								ErrorDetails{"property": pk},
							)
						}

					} else {

						if !pp_has || !pp_match {
							result.addError(
								new(AdditionalPropertyNotAllowedError),
								context,
								value[pk],
								ErrorDetails{"property": pk},
							)
						}

					}
				}
			}

		case *subSchema:

			additionalPropertiesSchema := currentSubSchema.additionalProperties.(*subSchema)
			for pk := range value {

				found := false
				for _, spValue := range currentSubSchema.propertiesChildren {
					if pk == spValue.property {
						found = true
					}
				}

				pp_has, pp_match := v.validatePatternProperty(currentSubSchema, pk, value[pk], result, context)

				if found {

					if pp_has && !pp_match {
						validationResult := additionalPropertiesSchema.subValidateWithContext(value[pk], context)
						result.mergeErrors(validationResult)
					}

				} else {

					if !pp_has || !pp_match {
						validationResult := additionalPropertiesSchema.subValidateWithContext(value[pk], context)
						result.mergeErrors(validationResult)
					}

				}

			}
		}
	} else {

		for pk := range value {

			pp_has, pp_match := v.validatePatternProperty(currentSubSchema, pk, value[pk], result, context)

			if pp_has && !pp_match {

				result.addError(
					new(InvalidPropertyPatternError),
					context,
					value[pk],
					ErrorDetails{
						"property": pk,
						"pattern":  currentSubSchema.PatternPropertiesString(),
					},
				)
			}

		}
	}

	result.incrementScore()
}

func (v *subSchema) validatePatternProperty(currentSubSchema *subSchema, key string, value interface{}, result *Result, context *jsonContext) (has bool, matched bool) {

	if internalLogEnabled {
		internalLog("validatePatternProperty %s", context.String())
		internalLog(" %s %v", key, value)
	}

	has = false

	validatedkey := false

	for pk, pv := range currentSubSchema.patternProperties {
		if matches, _ := regexp.MatchString(pk, key); matches {
			has = true
			subContext := newJsonContext(key, context)
			validationResult := pv.subValidateWithContext(value, subContext)
			result.mergeErrors(validationResult)
			if validationResult.Valid() {
				validatedkey = true
			}
		}
	}

	if !validatedkey {
		return has, false
	}

	result.incrementScore()

	return has, true
}

func (v *subSchema) validateString(currentSubSchema *subSchema, value interface{}, result *Result, context *jsonContext) {

	// Ignore JSON numbers
	if isJsonNumber(value) {
		return
	}

	// Ignore non strings
	if !isKind(value, reflect.String) {
		return
	}

	if internalLogEnabled {
		internalLog("validateString %s", context.String())
		internalLog(" %v", value)
	}

	stringValue := value.(string)

	// minLength & maxLength:
	if currentSubSchema.minLength != nil {
		if utf8.RuneCount([]byte(stringValue)) < int(*currentSubSchema.minLength) {
			result.addError(
				new(StringLengthGTEError),
				context,
				value,
				ErrorDetails{"min": *currentSubSchema.minLength},
			)
		}
	}
	if currentSubSchema.maxLength != nil {
		if utf8.RuneCount([]byte(stringValue)) > int(*currentSubSchema.maxLength) {
			result.addError(
				new(StringLengthLTEError),
				context,
				value,
				ErrorDetails{"max": *currentSubSchema.maxLength},
			)
		}
	}

	// pattern:
	if currentSubSchema.pattern != nil {
		if !currentSubSchema.pattern.MatchString(stringValue) {
			result.addError(
				new(DoesNotMatchPatternError),
				context,
				value,
				ErrorDetails{"pattern": currentSubSchema.pattern},
			)

		}
	}

	// format
	if currentSubSchema.format != "" {
		if !FormatCheckers.IsFormat(currentSubSchema.format, stringValue) {
			result.addError(
				new(DoesNotMatchFormatError),
				context,
				value,
				ErrorDetails{"format": currentSubSchema.format},
			)
		}
	}

	result.incrementScore()
}

func (v *subSchema) validateNumber(currentSubSchema *subSchema, value interface{}, result *Result, context *jsonContext) {

	// Ignore non numbers
	if !isJsonNumber(value) {
		return
	}

	if internalLogEnabled {
		internalLog("validateNumber %s", context.String())
		internalLog(" %v", value)
	}

	number := value.(json.Number)
	float64Value, _ := number.Float64()

	// multipleOf:
	if currentSubSchema.multipleOf != nil {

		if !isFloat64AnInteger(float64Value / *currentSubSchema.multipleOf) {
			result.addError(
				new(MultipleOfError),
				context,
				resultErrorFormatJsonNumber(number),
				ErrorDetails{"multiple": *currentSubSchema.multipleOf},
			)
		}
	}

	//maximum & exclusiveMaximum:
	if currentSubSchema.maximum != nil {
		if currentSubSchema.exclusiveMaximum {
			if float64Value >= *currentSubSchema.maximum {
				result.addError(
					new(NumberLTError),
					context,
					resultErrorFormatJsonNumber(number),
					ErrorDetails{
						"max": resultErrorFormatNumber(*currentSubSchema.maximum),
					},
				)
			}
		} else {
			if float64Value > *currentSubSchema.maximum {
				result.addError(
					new(NumberLTEError),
					context,
					resultErrorFormatJsonNumber(number),
					ErrorDetails{
						"max": resultErrorFormatNumber(*currentSubSchema.maximum),
					},
				)
			}
		}
	}

	//minimum & exclusiveMinimum:
	if currentSubSchema.minimum != nil {
		if currentSubSchema.exclusiveMinimum {
			if float64Value <= *currentSubSchema.minimum {
				result.addError(
					new(NumberGTError),
					context,
					resultErrorFormatJsonNumber(number),
					ErrorDetails{
						"min": resultErrorFormatNumber(*currentSubSchema.minimum),
					},
				)
			}
		} else {
			if float64Value < *currentSubSchema.minimum {
				result.addError(
					new(NumberGTEError),
					context,
					resultErrorFormatJsonNumber(number),
					ErrorDetails{
						"min": resultErrorFormatNumber(*currentSubSchema.minimum),
					},
				)
			}
		}
	}

	result.incrementScore()
}
