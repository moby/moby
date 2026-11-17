// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"reflect"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/jsonutils"
)

// SchemaValidator validates data against a JSON schema.
type SchemaValidator struct {
	// Path is the location of the validated value, in the legacy dot-separated
	// notation. It is what surfaces as the name of a validation error.
	//
	// Deprecated: a dotted path is ambiguous whenever a property name contains
	// a dot. Prefer the JSON pointer rendering of the same location.
	Path string

	// path is the same location, kept as JSON pointer reference tokens so that
	// children may be derived from it unambiguously.
	path pathSegments

	in           string
	Schema       *spec.Schema
	validators   [8]valueValidator
	Root         any
	KnownFormats strfmt.Registry
	Options      *SchemaValidatorOptions
}

// AgainstSchema validates the specified data against the provided schema, using a registry of supported formats.
//
// When no pre-parsed *[spec.Schema] structure is provided, it uses a JSON schema as default. See example.
func AgainstSchema(schema *spec.Schema, data any, formats strfmt.Registry, options ...Option) error {
	res := NewSchemaValidator(schema, nil, "", formats,
		append(options, WithRecycleValidators(true), withRecycleResults(true))...,
	).Validate(data)
	defer func() {
		redeemResult(res)
	}()

	if res.HasErrors() {
		return errors.CompositeValidationError(res.Errors...)
	}

	return nil
}

// NewSchemaValidator creates a new schema validator.
//
// Panics if the provided schema is invalid.
func NewSchemaValidator(schema *spec.Schema, rootSchema any, root string, formats strfmt.Registry, options ...Option) *SchemaValidator {
	opts := new(SchemaValidatorOptions)
	for _, o := range options {
		o(opts)
	}

	// the caller still owns this schema, and validation expands what it walks: work on a copy,
	// once, here - every validator below this one is then free to expand in place
	if !opts.ownSchemata && schema != nil {
		cloned, err := deepCloneSchema(*schema)
		if err != nil {
			panic(invalidSchemaProvidedMsg(err).Error())
		}

		if rootSchema == schema {
			rootSchema = &cloned
		}

		schema = &cloned
		opts.ownSchemata = true
	}

	return newSchemaValidator(schema, rootSchema, rootPathFromString(root), formats, opts)
}

// rootPathFromString interprets the root path of the exported constructors.
//
// The caller hands over an opaque string, so there is no telling which of its
// dots are separators and which belong to a name: it is taken as a single
// reference token.
func rootPathFromString(root string) pathSegments {
	if root == "" {
		return rootPath()
	}

	return newPathSegments(root)
}

func newSchemaValidator(schema *spec.Schema, rootSchema any, root pathSegments, formats strfmt.Registry, opts *SchemaValidatorOptions) *SchemaValidator {
	if schema == nil {
		return nil
	}

	if rootSchema == nil {
		rootSchema = schema
	}

	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	if schema.ID != "" || schema.Ref.String() != "" || schema.Ref.IsRoot() {
		err := spec.ExpandSchemaWithOptions(schema, rootSchema, nil, opts.expandOptions(""))
		if err != nil {
			msg := invalidSchemaProvidedMsg(err).Error()
			panic(msg)
		}
	}

	var s *SchemaValidator
	if opts.recycleValidators {
		s = validatorPools.schemaValidators.Borrow()
	} else {
		s = new(SchemaValidator)
	}

	s.path = root
	s.Path = root.dotted()
	s.in = "body"
	s.Schema = schema
	s.Root = rootSchema
	s.Options = opts
	s.KnownFormats = formats

	s.validators = [8]valueValidator{
		s.typeValidator(),
		s.schemaPropsValidator(),
		s.stringValidator(),
		s.formatValidator(),
		s.numberValidator(),
		s.sliceValidator(),
		s.commonValidator(),
		s.objectValidator(),
	}

	return s
}

// SetPath sets the path for this schema validator.
//
// Note that the sub-validators are built when the validator is created, so
// this only affects errors reported by this validator, not by its children.
func (s *SchemaValidator) SetPath(path string) {
	s.setPath(rootPathFromString(path))
}

// Applies returns true when this schema validator applies.
func (s *SchemaValidator) Applies(source any, _ reflect.Kind) bool {
	_, ok := source.(*spec.Schema)
	return ok
}

// Validate validates the data against the schema.
//
//nolint:gocognit // refactor in a forthcoming PR
func (s *SchemaValidator) Validate(data any) *Result {
	if s == nil {
		return emptyResult
	}

	if s.Options.recycleValidators {
		defer func() {
			s.redeemChildren()
			s.redeem() // one-time use validator
		}()
	}

	var result *Result
	if s.Options.recycleResult {
		result = validatorPools.results.Borrow()
		result.data = data
	} else {
		result = &Result{data: data}
	}

	if s.Schema != nil && !s.Options.skipSchemataResult {
		result.addRootObjectSchemata(s.Schema)
	}

	if data == nil {
		// early exit with minimal validation
		result.Merge(s.validators[0].Validate(data)) // type validator
		result.Merge(s.validators[6].Validate(data)) // common validator

		if s.Options.recycleValidators {
			s.validators[0] = nil
			s.validators[6] = nil
		}

		return result
	}

	tpe := reflect.TypeOf(data)
	kind := tpe.Kind()
	for kind == reflect.Ptr {
		tpe = tpe.Elem()
		kind = tpe.Kind()
	}
	d := data

	if kind == reflect.Struct {
		// NOTE: since reflect retrieves the true nature of types
		// this means that all strfmt types passed here (e.g. strfmt.Datetime, etc..)
		// are converted here to strings, and structs are systematically converted
		// to map[string]interface{}.
		var dd any
		if err := jsonutils.FromDynamicJSON(data, &dd); err != nil {
			result.addErrorsAt(s.path, err)
			result.Inc()

			return result
		}

		d = dd
	}

	// Proposal for enhancement: this part should be handed over to type validator
	// Handle special case of json.Number data (number marshalled as string)
	isnumber := s.Schema != nil && (s.Schema.Type.Contains(numberType) || s.Schema.Type.Contains(integerType))
	if num, ok := data.(json.Number); ok && isnumber {
		if s.Schema.Type.Contains(integerType) { // avoid lossy conversion
			in, erri := num.Int64()
			if erri != nil {
				result.addErrorsAt(s.path, invalidTypeConversionMsg(s.Path, erri))
				result.Inc()

				return result
			}
			d = in
		} else {
			nf, errf := num.Float64()
			if errf != nil {
				result.addErrorsAt(s.path, invalidTypeConversionMsg(s.Path, errf))
				result.Inc()

				return result
			}
			d = nf
		}

		tpe = reflect.TypeOf(d)
		kind = tpe.Kind()
	}

	for idx, v := range s.validators {
		if !v.Applies(s.Schema, kind) {
			if s.Options.recycleValidators {
				// Validate won't be called, so relinquish this validator
				if redeemableChildren, ok := v.(interface{ redeemChildren() }); ok {
					redeemableChildren.redeemChildren()
				}
				if redeemable, ok := v.(interface{ redeem() }); ok {
					redeemable.redeem()
				}
				s.validators[idx] = nil // prevents further (unsafe) usage
			}

			continue
		}

		result.Merge(v.Validate(d))
		if s.Options.recycleValidators {
			s.validators[idx] = nil // prevents further (unsafe) usage
		}
		result.Inc()
	}
	result.Inc()

	return result
}

func (s *SchemaValidator) typeValidator() valueValidator {
	return newTypeValidator(
		s.path,
		s.in,
		s.Schema.Type,
		s.Schema.Nullable,
		s.Schema.Format,
		s.Options,
	)
}

func (s *SchemaValidator) commonValidator() valueValidator {
	return newBasicCommonValidator(
		s.path,
		s.in,
		s.Schema.Default,
		s.Schema.Enum,
		s.Options,
	)
}

func (s *SchemaValidator) sliceValidator() valueValidator {
	return newSliceValidator(
		s.path,
		s.in,
		s.Schema.MaxItems,
		s.Schema.MinItems,
		s.Schema.UniqueItems,
		s.Schema.AdditionalItems,
		s.Schema.Items,
		s.Root,
		s.KnownFormats,
		s.Options,
	)
}

func (s *SchemaValidator) numberValidator() valueValidator {
	return newNumberValidator(
		s.path,
		s.in,
		s.Schema.Default,
		s.Schema.MultipleOf,
		s.Schema.Maximum,
		s.Schema.ExclusiveMaximum,
		s.Schema.Minimum,
		s.Schema.ExclusiveMinimum,
		"",
		"",
		s.Options,
	)
}

func (s *SchemaValidator) stringValidator() valueValidator {
	return newStringValidator(
		s.path,
		s.in,
		nil,
		false,
		false,
		s.Schema.MaxLength,
		s.Schema.MinLength,
		s.Schema.Pattern,
		s.Options,
	)
}

func (s *SchemaValidator) formatValidator() valueValidator {
	return newFormatValidator(
		s.path,
		s.in,
		s.Schema.Format,
		s.KnownFormats,
		s.Options,
	)
}

func (s *SchemaValidator) schemaPropsValidator() valueValidator {
	sch := s.Schema
	return newSchemaPropsValidator(
		s.path, s.in, sch.AllOf, sch.OneOf, sch.AnyOf, sch.Not, sch.Dependencies, s.Root, s.KnownFormats,
		s.Options,
	)
}

func (s *SchemaValidator) objectValidator() valueValidator {
	return newObjectValidator(
		s.path,
		s.in,
		s.Schema.MaxProperties,
		s.Schema.MinProperties,
		s.Schema.Required,
		s.Schema.Properties,
		s.Schema.AdditionalProperties,
		s.Schema.PatternProperties,
		s.Root,
		s.KnownFormats,
		s.Options,
	)
}

func (s *SchemaValidator) setPath(path pathSegments) {
	s.path = path
	s.Path = path.dotted()
}

func (s *SchemaValidator) redeem() {
	validatorPools.schemaValidators.Redeem(s)
}

func (s *SchemaValidator) redeemChildren() {
	for i, validator := range s.validators {
		if validator == nil {
			continue
		}
		if redeemableChildren, ok := validator.(interface{ redeemChildren() }); ok {
			redeemableChildren.redeemChildren()
		}
		if redeemable, ok := validator.(interface{ redeem() }); ok {
			redeemable.redeem()
		}
		s.validators[i] = nil // free up allocated children if not in pool
	}
}
