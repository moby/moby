// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"reflect"
	"strings"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
)

type objectValidator struct {
	Path                 pathSegments
	In                   string
	MaxProperties        *int64
	MinProperties        *int64
	Required             []string
	Properties           map[string]spec.Schema
	AdditionalProperties *spec.SchemaOrBool
	PatternProperties    map[string]spec.Schema
	Root                 any
	KnownFormats         strfmt.Registry
	Options              *SchemaValidatorOptions
}

func newObjectValidator(path pathSegments, in string,
	maxProperties, minProperties *int64, required []string, properties spec.SchemaProperties,
	additionalProperties *spec.SchemaOrBool, patternProperties spec.SchemaProperties,
	root any, formats strfmt.Registry, opts *SchemaValidatorOptions,
) *objectValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var v *objectValidator
	if opts.recycleValidators {
		v = validatorPools.objectValidators.Borrow()
	} else {
		v = new(objectValidator)
	}

	v.Path = path
	v.In = in
	v.MaxProperties = maxProperties
	v.MinProperties = minProperties
	v.Required = required
	v.Properties = properties
	v.AdditionalProperties = additionalProperties
	v.PatternProperties = patternProperties
	v.Root = root
	v.KnownFormats = formats
	v.Options = opts

	return v
}

func (o *objectValidator) Validate(data any) *Result {
	if o.Options.recycleValidators {
		defer func() {
			o.redeem()
		}()
	}

	var val map[string]any
	if data != nil {
		var ok bool
		val, ok = data.(map[string]any)
		if !ok {
			return errorHelp.sErrAt(o.Path, invalidObjectMsg(o.Path.dotted(), o.In), o.Options.recycleResult)
		}
	}
	numKeys := int64(len(val))

	if o.MinProperties != nil && numKeys < *o.MinProperties {
		return errorHelp.sErrAt(o.Path, errors.TooFewProperties(o.Path.dotted(), o.In, *o.MinProperties), o.Options.recycleResult)
	}
	if o.MaxProperties != nil && numKeys > *o.MaxProperties {
		return errorHelp.sErrAt(o.Path, errors.TooManyProperties(o.Path.dotted(), o.In, *o.MaxProperties), o.Options.recycleResult)
	}

	var res *Result
	if o.Options.recycleResult {
		res = validatorPools.results.Borrow()
	} else {
		res = new(Result)
	}

	o.precheck(res, val)

	// check validity of field names
	if o.AdditionalProperties != nil && !o.AdditionalProperties.Allows {
		// Case: additionalProperties: false
		o.validateNoAdditionalProperties(val, res)
	} else {
		// Cases: empty additionalProperties (implying: true), or additionalProperties: true, or additionalProperties: { <<schema>> }
		o.validateAdditionalProperties(val, res)
	}

	o.validatePropertiesSchema(val, res)

	// Check patternProperties
	// NOTE: it looks like we have done that twice in many cases
	for _, key := range sortedKeys(val) {
		value := val[key]
		_, regularProperty := o.Properties[key]
		matched, _, patterns := o.validatePatternProperty(key, value, res) // applies to regular properties as well
		if regularProperty || !matched {
			continue
		}

		for _, pName := range patterns {
			if v, ok := o.PatternProperties[pName]; ok {
				r := newSchemaValidator(&v, o.Root, o.Path.child(key), o.KnownFormats, o.Options).Validate(value)
				res.mergeForField(data.(map[string]any), key, r) //nolint:forcetypeassert // data is always map[string]any at this point
			}
		}
	}

	return res
}

func (o *objectValidator) Applies(source any, kind reflect.Kind) bool {
	// NOTE: this should also work for structs
	// there is a problem in the type validator where it will be unhappy about null values
	// so that requires more testing
	_, isSchema := source.(*spec.Schema)
	return isSchema && (kind == reflect.Map || kind == reflect.Struct)
}

// The three predicates below tell what kind of content the validated object
// is, so that schema-only checks are not run against plain data.
//
// Array indices are trimmed first: an element of an example is example data
// just as much as the example itself.

func (o *objectValidator) isProperties() bool {
	p := o.Path.trimIndexes()

	return p.last() == jsonProperties && p.beforeLast() != jsonProperties
}

func (o *objectValidator) isDefault() bool {
	p := o.Path.trimIndexes()

	return p.last() == jsonDefault && p.beforeLast() != jsonDefault
}

func (o *objectValidator) isExample() bool {
	p := o.Path.trimIndexes()
	last := p.last()

	return (last == swaggerExample || last == swaggerExamples) && p.beforeLast() != swaggerExample
}

func (o *objectValidator) checkArrayMustHaveItems(res *Result, val map[string]any) {
	// for swagger 2.0 schemas, there is an additional constraint to have array items defined explicitly.
	// with pure jsonschema draft 4, one may have arrays with undefined items (i.e. any type).
	if val == nil {
		return
	}

	t, typeFound := val[jsonType]
	if !typeFound {
		return
	}

	tpe, isString := t.(string)
	if !isString || tpe != arrayType {
		return
	}

	item, itemsKeyFound := val[jsonItems]
	if itemsKeyFound {
		return
	}

	res.addErrorsAt(o.Path, errors.Required(jsonItems, o.Path.dotted(), item))
}

func (o *objectValidator) checkItemsMustBeTypeArray(res *Result, val map[string]any) {
	if val == nil {
		return
	}

	if o.isProperties() || o.isDefault() || o.isExample() {
		return
	}

	_, itemsKeyFound := val[jsonItems]
	if !itemsKeyFound {
		return
	}

	t, typeFound := val[jsonType]
	if !typeFound {
		// there is no type
		res.addErrorsAt(o.Path, errors.Required(jsonType, o.Path.dotted(), t))
	}

	if tpe, isString := t.(string); !isString || tpe != arrayType {
		res.addErrorsAt(o.Path, errors.InvalidType(o.Path.dotted(), o.In, arrayType, nil))
	}
}

func (o *objectValidator) precheck(res *Result, val map[string]any) {
	if o.Options.EnableArrayMustHaveItemsCheck {
		o.checkArrayMustHaveItems(res, val)
	}
	if o.Options.EnableObjectArrayTypeCheck {
		o.checkItemsMustBeTypeArray(res, val)
	}
}

func (o *objectValidator) validateNoAdditionalProperties(val map[string]any, res *Result) {
	for _, k := range sortedKeys(val) {
		if k == "$schema" || k == "id" {
			// special properties "$schema" and "id" are ignored
			continue
		}

		_, regularProperty := o.Properties[k]
		if regularProperty {
			continue
		}

		matched := false
		for pk := range o.PatternProperties {
			re, err := compileRegexp(pk)
			if err != nil {
				continue
			}
			if matches := re.MatchString(k); matches {
				matched = true
				break
			}
		}
		if matched {
			continue
		}

		res.addErrorsAt(o.Path.child(k), errors.PropertyNotAllowed(o.Path.dotted(), o.In, k))

		// BUG(fredbi): This section should move to a part dedicated to spec validation as
		// it will conflict with regular schemas where a property "headers" is defined.

		//
		// Croaks a more explicit message on top of the standard one
		// on some recognized cases.
		//
		// NOTE: edge cases with invalid type assertion are simply ignored here.
		// NOTE: prefix your messages here by "IMPORTANT!" so there are not filtered
		// by higher level callers (the IMPORTANT! tag will be eventually
		// removed).
		if k != "headers" || val[k] == nil {
			continue
		}

		// $ref is forbidden in header
		headers, mapOk := val[k].(map[string]any)
		if !mapOk {
			continue
		}

		for _, headerKey := range sortedKeys(headers) {
			headerBody := headers[headerKey]
			if headerBody == nil {
				continue
			}

			headerSchema, mapOfMapOk := headerBody.(map[string]any)
			if !mapOfMapOk {
				continue
			}

			_, found := headerSchema["$ref"]
			if !found {
				continue
			}

			refString, stringOk := headerSchema["$ref"].(string)
			if !stringOk {
				continue
			}

			msg := strings.Join([]string{", one may not use $ref=\":", refString, "\""}, "")
			res.addErrorsAt(o.Path, refNotAllowedInHeaderMsg(o.Path.dotted(), headerKey, msg))
			/*
				case "$ref":
					if val[k] != nil {
						// Proposal for enhancement: check context of that ref: warn about siblings, check against invalid context
					}
			*/
		}
	}
}

func (o *objectValidator) validateAdditionalProperties(val map[string]any, res *Result) {
	for _, key := range sortedKeys(val) {
		value := val[key]
		_, regularProperty := o.Properties[key]
		if regularProperty {
			continue
		}

		// Validates property against "patternProperties" if applicable
		// BUG(fredbi): succeededOnce is always false

		// NOTE: how about regular properties which do not match patternProperties?
		matched, succeededOnce, _ := o.validatePatternProperty(key, value, res)
		if matched || succeededOnce {
			continue
		}

		if o.AdditionalProperties == nil || o.AdditionalProperties.Schema == nil {
			continue
		}

		// Cases: properties which are not regular properties and have not been matched by the PatternProperties validator
		// AdditionalProperties as Schema
		r := newSchemaValidator(o.AdditionalProperties.Schema, o.Root, o.Path.child(key), o.KnownFormats, o.Options).Validate(value)
		res.mergeForField(val, key, r)
	}
	// Valid cases: additionalProperties: true or undefined
}

func (o *objectValidator) validatePropertiesSchema(val map[string]any, res *Result) {
	createdFromDefaults := map[string]struct{}{}

	// Property types:
	// - regular Property
	pSchema := validatorPools.schemas.Borrow() // recycle a spec.Schema object which lifespan extends only to the validation of properties
	defer func() {
		validatorPools.schemas.Redeem(pSchema)
	}()

	for _, pName := range sortedKeys(o.Properties) {
		*pSchema = o.Properties[pName]
		rName := o.Path.child(pName)

		// Recursively validates each property against its schema
		v, ok := val[pName]
		if ok {
			r := newSchemaValidator(pSchema, o.Root, rName, o.KnownFormats, o.Options).Validate(v)
			res.mergeForField(val, pName, r)

			continue
		}

		if pSchema.Default != nil {
			// if a default value is defined, creates the property from defaults
			// NOTE: JSON schema does not enforce default values to be valid against schema. Swagger does.
			createdFromDefaults[pName] = struct{}{}
			if !o.Options.skipSchemataResult {
				res.addPropertySchemata(val, pName, pSchema) // this shallow-clones the content of the pSchema pointer
			}
		}
	}

	if len(o.Required) == 0 {
		return
	}

	// Check required properties
	for _, k := range o.Required {
		v, ok := val[k]
		if ok {
			continue
		}
		_, isCreatedFromDefaults := createdFromDefaults[k]
		if isCreatedFromDefaults {
			continue
		}

		// located on the object that lacks the property: the property itself
		// has no node to point at, and the object has to be amended
		res.addErrorsAt(o.Path, errors.Required(o.Path.child(k).dotted(), o.In, v))
	}
}

// NOTE: succeededOnce is not used anywhere.
func (o *objectValidator) validatePatternProperty(key string, value any, result *Result) (bool, bool, []string) {
	if len(o.PatternProperties) == 0 {
		return false, false, nil
	}

	matched := false
	succeededOnce := false
	patterns := make([]string, 0, len(o.PatternProperties))

	schema := validatorPools.schemas.Borrow()
	defer func() {
		validatorPools.schemas.Redeem(schema)
	}()

	for _, k := range sortedKeys(o.PatternProperties) {
		re, err := compileRegexp(k)
		if err != nil {
			continue
		}

		match := re.MatchString(key)
		if !match {
			continue
		}

		*schema = o.PatternProperties[k]
		patterns = append(patterns, k)
		matched = true
		validator := newSchemaValidator(schema, o.Root, o.Path.child(key), o.KnownFormats, o.Options)

		res := validator.Validate(value)
		result.Merge(res)
	}

	return matched, succeededOnce, patterns
}

func (o *objectValidator) setPath(path pathSegments) {
	o.Path = path
}

func (o *objectValidator) redeem() {
	validatorPools.objectValidators.Redeem(o)
}
