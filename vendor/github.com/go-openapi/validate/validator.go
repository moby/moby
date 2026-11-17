// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"reflect"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
)

// An EntityValidator is an interface for things that can validate entities.
type EntityValidator interface {
	Validate(data any) *Result
}

type valueValidator interface {
	setPath(path pathSegments)
	Applies(source any, kind reflect.Kind) bool
	Validate(data any) *Result
}

type itemsValidator struct {
	items        *spec.Items
	root         any
	path         pathSegments
	in           string
	validators   [6]valueValidator
	KnownFormats strfmt.Registry
	Options      *SchemaValidatorOptions
}

func newItemsValidator(path pathSegments, in string, items *spec.Items, root any, formats strfmt.Registry, opts *SchemaValidatorOptions) *itemsValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var iv *itemsValidator
	if opts.recycleValidators {
		iv = validatorPools.itemsValidators.Borrow()
	} else {
		iv = new(itemsValidator)
	}

	iv.path = path
	iv.in = in
	iv.items = items
	iv.root = root
	iv.KnownFormats = formats
	iv.Options = opts
	iv.validators = [6]valueValidator{
		iv.typeValidator(),
		iv.stringValidator(),
		iv.formatValidator(),
		iv.numberValidator(),
		iv.sliceValidator(),
		iv.commonValidator(),
	}
	return iv
}

func (i *itemsValidator) Validate(index int, data any) *Result {
	if i.Options.recycleValidators {
		defer func() {
			i.redeemChildren()
			i.redeem()
		}()
	}

	tpe := reflect.TypeOf(data)
	kind := tpe.Kind()
	var result *Result
	if i.Options.recycleResult {
		result = validatorPools.results.Borrow()
	} else {
		result = new(Result)
	}

	path := i.path.item(index)

	for idx, validator := range i.validators {
		if !validator.Applies(i.root, kind) {
			if i.Options.recycleValidators {
				// Validate won't be called, so relinquish this validator
				if redeemableChildren, ok := validator.(interface{ redeemChildren() }); ok {
					redeemableChildren.redeemChildren()
				}
				if redeemable, ok := validator.(interface{ redeem() }); ok {
					redeemable.redeem()
				}
				i.validators[idx] = nil // prevents further (unsafe) usage
			}

			continue
		}

		validator.setPath(path)
		err := validator.Validate(data)
		if i.Options.recycleValidators {
			i.validators[idx] = nil // prevents further (unsafe) usage
		}
		if err != nil {
			result.Inc()
			if err.HasErrors() {
				result.Merge(err)

				break
			}

			result.Merge(err)
		}
	}

	return result
}

func (i *itemsValidator) typeValidator() valueValidator {
	return newTypeValidator(
		i.path,
		i.in,
		spec.StringOrArray([]string{i.items.Type}),
		i.items.Nullable,
		i.items.Format,
		i.Options,
	)
}

func (i *itemsValidator) commonValidator() valueValidator {
	return newBasicCommonValidator(
		nil, // located by the item index, set on each Validate call
		i.in,
		i.items.Default,
		i.items.Enum,
		i.Options,
	)
}

func (i *itemsValidator) sliceValidator() valueValidator {
	return newBasicSliceValidator(
		nil, // located by the item index, set on each Validate call
		i.in,
		i.items.Default,
		i.items.MaxItems,
		i.items.MinItems,
		i.items.UniqueItems,
		i.items.Items,
		i.root,
		i.KnownFormats,
		i.Options,
	)
}

func (i *itemsValidator) numberValidator() valueValidator {
	return newNumberValidator(
		nil, // located by the item index, set on each Validate call
		i.in,
		i.items.Default,
		i.items.MultipleOf,
		i.items.Maximum,
		i.items.ExclusiveMaximum,
		i.items.Minimum,
		i.items.ExclusiveMinimum,
		i.items.Type,
		i.items.Format,
		i.Options,
	)
}

func (i *itemsValidator) stringValidator() valueValidator {
	return newStringValidator(
		nil, // located by the item index, set on each Validate call
		i.in,
		i.items.Default,
		false, // Required
		false, // AllowEmpty
		i.items.MaxLength,
		i.items.MinLength,
		i.items.Pattern,
		i.Options,
	)
}

func (i *itemsValidator) formatValidator() valueValidator {
	return newFormatValidator(
		nil, // located by the item index, set on each Validate call
		i.in,
		i.items.Format,
		i.KnownFormats,
		i.Options,
	)
}

func (i *itemsValidator) redeem() {
	validatorPools.itemsValidators.Redeem(i)
}

func (i *itemsValidator) redeemChildren() {
	for idx, validator := range i.validators {
		if validator == nil {
			continue
		}
		if redeemableChildren, ok := validator.(interface{ redeemChildren() }); ok {
			redeemableChildren.redeemChildren()
		}
		if redeemable, ok := validator.(interface{ redeem() }); ok {
			redeemable.redeem()
		}
		i.validators[idx] = nil // free up allocated children if not in pool
	}
}

type basicCommonValidator struct {
	Path    pathSegments
	In      string
	Default any
	Enum    []any
	Options *SchemaValidatorOptions
}

func newBasicCommonValidator(path pathSegments, in string, def any, enum []any, opts *SchemaValidatorOptions) *basicCommonValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var b *basicCommonValidator
	if opts.recycleValidators {
		b = validatorPools.basicCommonValidators.Borrow()
	} else {
		b = new(basicCommonValidator)
	}

	b.Path = path
	b.In = in
	b.Default = def
	b.Enum = enum
	b.Options = opts

	return b
}

func (b *basicCommonValidator) Applies(source any, _ reflect.Kind) bool {
	switch source.(type) {
	case *spec.Parameter, *spec.Schema, *spec.Header:
		return true
	default:
		return false
	}
}

func (b *basicCommonValidator) Validate(data any) (res *Result) {
	if b.Options.recycleValidators {
		defer func() {
			b.redeem()
		}()
	}

	if len(b.Enum) == 0 {
		return nil
	}

	for _, enumValue := range b.Enum {
		actualType := reflect.TypeOf(enumValue)
		if actualType == nil { // Safeguard
			continue
		}

		expectedValue := reflect.ValueOf(data)
		if expectedValue.IsValid() &&
			expectedValue.Type().ConvertibleTo(actualType) &&
			reflect.DeepEqual(expectedValue.Convert(actualType).Interface(), enumValue) {
			return nil
		}
	}

	return errorHelp.sErrAt(b.Path, errors.EnumFail(b.Path.dotted(), b.In, data, b.Enum), b.Options.recycleResult)
}

func (b *basicCommonValidator) setPath(path pathSegments) {
	b.Path = path
}

func (b *basicCommonValidator) redeem() {
	validatorPools.basicCommonValidators.Redeem(b)
}

// A HeaderValidator has very limited subset of validations to apply.
type HeaderValidator struct {
	name         string
	header       *spec.Header
	validators   [6]valueValidator
	KnownFormats strfmt.Registry
	Options      *SchemaValidatorOptions
}

// NewHeaderValidator creates a new header validator object.
func NewHeaderValidator(name string, header *spec.Header, formats strfmt.Registry, options ...Option) *HeaderValidator {
	opts := new(SchemaValidatorOptions)
	for _, o := range options {
		o(opts)
	}

	return newHeaderValidator(name, header, formats, opts)
}

func newHeaderValidator(name string, header *spec.Header, formats strfmt.Registry, opts *SchemaValidatorOptions) *HeaderValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var p *HeaderValidator
	if opts.recycleValidators {
		p = validatorPools.headerValidators.Borrow()
	} else {
		p = new(HeaderValidator)
	}

	p.name = name
	p.header = header
	p.KnownFormats = formats
	p.Options = opts
	p.validators = [6]valueValidator{
		newTypeValidator(
			newPathSegments(name),
			"header",
			spec.StringOrArray([]string{header.Type}),
			header.Nullable,
			header.Format,
			p.Options,
		),
		p.stringValidator(),
		p.formatValidator(),
		p.numberValidator(),
		p.sliceValidator(),
		p.commonValidator(),
	}

	return p
}

// Validate the value of the header against its schema.
func (p *HeaderValidator) Validate(data any) *Result {
	if p.Options.recycleValidators {
		defer func() {
			p.redeemChildren()
			p.redeem()
		}()
	}

	if data == nil {
		return nil
	}

	var result *Result
	if p.Options.recycleResult {
		result = validatorPools.results.Borrow()
	} else {
		result = new(Result)
	}

	tpe := reflect.TypeOf(data)
	kind := tpe.Kind()

	for idx, validator := range p.validators {
		if !validator.Applies(p.header, kind) {
			if p.Options.recycleValidators {
				// Validate won't be called, so relinquish this validator
				if redeemableChildren, ok := validator.(interface{ redeemChildren() }); ok {
					redeemableChildren.redeemChildren()
				}
				if redeemable, ok := validator.(interface{ redeem() }); ok {
					redeemable.redeem()
				}
				p.validators[idx] = nil // prevents further (unsafe) usage
			}

			continue
		}

		err := validator.Validate(data)
		if p.Options.recycleValidators {
			p.validators[idx] = nil // prevents further (unsafe) usage
		}
		if err != nil {
			if err.HasErrors() {
				result.Merge(err)
				break
			}
			result.Merge(err)
		}
	}

	return result
}

func (p *HeaderValidator) commonValidator() valueValidator {
	return newBasicCommonValidator(
		newPathSegments(p.name),
		"response",
		p.header.Default,
		p.header.Enum,
		p.Options,
	)
}

func (p *HeaderValidator) sliceValidator() valueValidator {
	return newBasicSliceValidator(
		newPathSegments(p.name),
		"response",
		p.header.Default,
		p.header.MaxItems,
		p.header.MinItems,
		p.header.UniqueItems,
		p.header.Items,
		p.header,
		p.KnownFormats,
		p.Options,
	)
}

func (p *HeaderValidator) numberValidator() valueValidator {
	return newNumberValidator(
		newPathSegments(p.name),
		"response",
		p.header.Default,
		p.header.MultipleOf,
		p.header.Maximum,
		p.header.ExclusiveMaximum,
		p.header.Minimum,
		p.header.ExclusiveMinimum,
		p.header.Type,
		p.header.Format,
		p.Options,
	)
}

func (p *HeaderValidator) stringValidator() valueValidator {
	return newStringValidator(
		newPathSegments(p.name),
		"response",
		p.header.Default,
		true,
		false,
		p.header.MaxLength,
		p.header.MinLength,
		p.header.Pattern,
		p.Options,
	)
}

func (p *HeaderValidator) formatValidator() valueValidator {
	return newFormatValidator(
		newPathSegments(p.name),
		"response",
		p.header.Format,
		p.KnownFormats,
		p.Options,
	)
}

func (p *HeaderValidator) redeem() {
	validatorPools.headerValidators.Redeem(p)
}

func (p *HeaderValidator) redeemChildren() {
	for idx, validator := range p.validators {
		if validator == nil {
			continue
		}
		if redeemableChildren, ok := validator.(interface{ redeemChildren() }); ok {
			redeemableChildren.redeemChildren()
		}
		if redeemable, ok := validator.(interface{ redeem() }); ok {
			redeemable.redeem()
		}
		p.validators[idx] = nil // free up allocated children if not in pool
	}
}

// A ParamValidator has very limited subset of validations to apply.
type ParamValidator struct {
	param        *spec.Parameter
	validators   [6]valueValidator
	KnownFormats strfmt.Registry
	Options      *SchemaValidatorOptions
}

// NewParamValidator creates a new param validator object.
func NewParamValidator(param *spec.Parameter, formats strfmt.Registry, options ...Option) *ParamValidator {
	opts := new(SchemaValidatorOptions)
	for _, o := range options {
		o(opts)
	}

	return newParamValidator(param, formats, opts)
}

func newParamValidator(param *spec.Parameter, formats strfmt.Registry, opts *SchemaValidatorOptions) *ParamValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var p *ParamValidator
	if opts.recycleValidators {
		p = validatorPools.paramValidators.Borrow()
	} else {
		p = new(ParamValidator)
	}

	p.param = param
	p.KnownFormats = formats
	p.Options = opts
	p.validators = [6]valueValidator{
		newTypeValidator(
			newPathSegments(param.Name),
			param.In,
			spec.StringOrArray([]string{param.Type}),
			param.Nullable,
			param.Format,
			p.Options,
		),
		p.stringValidator(),
		p.formatValidator(),
		p.numberValidator(),
		p.sliceValidator(),
		p.commonValidator(),
	}

	return p
}

// Validate the data against the description of the parameter.
func (p *ParamValidator) Validate(data any) *Result {
	if data == nil {
		return nil
	}

	var result *Result
	if p.Options.recycleResult {
		result = validatorPools.results.Borrow()
	} else {
		result = new(Result)
	}

	tpe := reflect.TypeOf(data)
	kind := tpe.Kind()

	if p.Options.recycleValidators {
		defer func() {
			p.redeemChildren()
			p.redeem()
		}()
	}

	// Proposal for enhancement: validate type
	for idx, validator := range p.validators {
		if !validator.Applies(p.param, kind) {
			if p.Options.recycleValidators {
				// Validate won't be called, so relinquish this validator
				if redeemableChildren, ok := validator.(interface{ redeemChildren() }); ok {
					redeemableChildren.redeemChildren()
				}
				if redeemable, ok := validator.(interface{ redeem() }); ok {
					redeemable.redeem()
				}
				p.validators[idx] = nil // prevents further (unsafe) usage
			}

			continue
		}

		err := validator.Validate(data)
		if p.Options.recycleValidators {
			p.validators[idx] = nil // prevents further (unsafe) usage
		}
		if err != nil {
			if err.HasErrors() {
				result.Merge(err)
				break
			}
			result.Merge(err)
		}
	}

	return result
}

func (p *ParamValidator) commonValidator() valueValidator {
	return newBasicCommonValidator(
		newPathSegments(p.param.Name),
		p.param.In,
		p.param.Default,
		p.param.Enum,
		p.Options,
	)
}

func (p *ParamValidator) sliceValidator() valueValidator {
	return newBasicSliceValidator(
		newPathSegments(p.param.Name),
		p.param.In,
		p.param.Default,
		p.param.MaxItems,
		p.param.MinItems,
		p.param.UniqueItems,
		p.param.Items,
		p.param,
		p.KnownFormats,
		p.Options,
	)
}

func (p *ParamValidator) numberValidator() valueValidator {
	return newNumberValidator(
		newPathSegments(p.param.Name),
		p.param.In,
		p.param.Default,
		p.param.MultipleOf,
		p.param.Maximum,
		p.param.ExclusiveMaximum,
		p.param.Minimum,
		p.param.ExclusiveMinimum,
		p.param.Type,
		p.param.Format,
		p.Options,
	)
}

func (p *ParamValidator) stringValidator() valueValidator {
	return newStringValidator(
		newPathSegments(p.param.Name),
		p.param.In,
		p.param.Default,
		p.param.Required,
		p.param.AllowEmptyValue,
		p.param.MaxLength,
		p.param.MinLength,
		p.param.Pattern,
		p.Options,
	)
}

func (p *ParamValidator) formatValidator() valueValidator {
	return newFormatValidator(
		newPathSegments(p.param.Name),
		p.param.In,
		p.param.Format,
		p.KnownFormats,
		p.Options,
	)
}

func (p *ParamValidator) redeem() {
	validatorPools.paramValidators.Redeem(p)
}

func (p *ParamValidator) redeemChildren() {
	for idx, validator := range p.validators {
		if validator == nil {
			continue
		}
		if redeemableChildren, ok := validator.(interface{ redeemChildren() }); ok {
			redeemableChildren.redeemChildren()
		}
		if redeemable, ok := validator.(interface{ redeem() }); ok {
			redeemable.redeem()
		}
		p.validators[idx] = nil // free up allocated children if not in pool
	}
}

type basicSliceValidator struct {
	Path         pathSegments
	In           string
	Default      any
	MaxItems     *int64
	MinItems     *int64
	UniqueItems  bool
	Items        *spec.Items
	Source       any
	KnownFormats strfmt.Registry
	Options      *SchemaValidatorOptions
}

func newBasicSliceValidator(
	path pathSegments, in string,
	def any, maxItems, minItems *int64, uniqueItems bool, items *spec.Items,
	source any, formats strfmt.Registry,
	opts *SchemaValidatorOptions,
) *basicSliceValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var s *basicSliceValidator
	if opts.recycleValidators {
		s = validatorPools.basicSliceValidators.Borrow()
	} else {
		s = new(basicSliceValidator)
	}

	s.Path = path
	s.In = in
	s.Default = def
	s.MaxItems = maxItems
	s.MinItems = minItems
	s.UniqueItems = uniqueItems
	s.Items = items
	s.Source = source
	s.KnownFormats = formats
	s.Options = opts

	return s
}

func (s *basicSliceValidator) Applies(source any, kind reflect.Kind) bool {
	switch source.(type) {
	case *spec.Parameter, *spec.Items, *spec.Header:
		return kind == reflect.Slice
	default:
		return false
	}
}

func (s *basicSliceValidator) Validate(data any) *Result {
	if s.Options.recycleValidators {
		defer func() {
			s.redeem()
		}()
	}
	val := reflect.ValueOf(data)

	size := int64(val.Len())
	if s.MinItems != nil {
		if err := MinItems(s.Path.dotted(), s.In, size, *s.MinItems); err != nil {
			return errorHelp.sErrAt(s.Path, err, s.Options.recycleResult)
		}
	}

	if s.MaxItems != nil {
		if err := MaxItems(s.Path.dotted(), s.In, size, *s.MaxItems); err != nil {
			return errorHelp.sErrAt(s.Path, err, s.Options.recycleResult)
		}
	}

	if s.UniqueItems {
		if err := UniqueItems(s.Path.dotted(), s.In, data); err != nil {
			return errorHelp.sErrAt(s.Path, err, s.Options.recycleResult)
		}
	}

	if s.Items == nil {
		return nil
	}

	for i := range int(size) {
		itemsValidator := newItemsValidator(s.Path, s.In, s.Items, s.Source, s.KnownFormats, s.Options)
		ele := val.Index(i)
		if err := itemsValidator.Validate(i, ele.Interface()); err != nil {
			if err.HasErrors() {
				return err
			}
			if err.wantsRedeemOnMerge {
				redeemResult(err)
			}
		}
	}

	return nil
}

func (s *basicSliceValidator) setPath(path pathSegments) {
	s.Path = path
}

func (s *basicSliceValidator) redeem() {
	validatorPools.basicSliceValidators.Redeem(s)
}

type numberValidator struct {
	Path             pathSegments
	In               string
	Default          any
	MultipleOf       *float64
	Maximum          *float64
	ExclusiveMaximum bool
	Minimum          *float64
	ExclusiveMinimum bool
	// Allows for more accurate behavior regarding integers
	Type    string
	Format  string
	Options *SchemaValidatorOptions
}

func newNumberValidator(
	path pathSegments, in string, def any,
	multipleOf, maximum *float64, exclusiveMaximum bool, minimum *float64, exclusiveMinimum bool,
	typ, format string,
	opts *SchemaValidatorOptions,
) *numberValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var n *numberValidator
	if opts.recycleValidators {
		n = validatorPools.numberValidators.Borrow()
	} else {
		n = new(numberValidator)
	}

	n.Path = path
	n.In = in
	n.Default = def
	n.MultipleOf = multipleOf
	n.Maximum = maximum
	n.ExclusiveMaximum = exclusiveMaximum
	n.Minimum = minimum
	n.ExclusiveMinimum = exclusiveMinimum
	n.Type = typ
	n.Format = format
	n.Options = opts

	return n
}

func (n *numberValidator) Applies(source any, kind reflect.Kind) bool {
	switch source.(type) {
	case *spec.Parameter, *spec.Schema, *spec.Items, *spec.Header:
		isInt := kind >= reflect.Int && kind <= reflect.Uint64
		isFloat := kind == reflect.Float32 || kind == reflect.Float64
		return isInt || isFloat
	default:
		return false
	}
}

// Validate provides a validator for generic JSON numbers,
//
// By default, numbers are internally represented as float64.
// Formats float, or float32 may alter this behavior by mapping to float32.
// A special validation process is followed for integers, with optional "format":
// this is an attempt to provide a validation with native types.
//
// NOTE: since the constraint specified (boundary, multipleOf) is unmarshalled
// as float64, loss of information remains possible (e.g. on very large integers).
//
// Since this value directly comes from the unmarshalling, it is not possible
// at this stage of processing to check further and guarantee the correctness of such values.
//
// Normally, the JSON Number.MAX_SAFE_INTEGER (resp. Number.MIN_SAFE_INTEGER)
// would check we do not get such a loss.
//
// If this is the case, replace AddErrors() by AddWarnings() and IsValid() by !HasWarnings().
//
// Proposal for enhancement: consider replacing boundary check errors by simple warnings.
//
// NOTE: default boundaries with MAX_SAFE_INTEGER are not checked (specific to json.Number?)
func (n *numberValidator) Validate(val any) *Result {
	if n.Options.recycleValidators {
		defer func() {
			n.redeem()
		}()
	}

	var res, resMultiple, resMinimum, resMaximum *Result
	if n.Options.recycleResult {
		res = validatorPools.results.Borrow()
	} else {
		res = new(Result)
	}

	// Used only to attempt to validate constraint on value,
	// even though value or constraint specified do not match type and format
	data := valueHelp.asFloat64(val)

	// Is the provided value within the range of the specified numeric type and format?
	res.addErrorsAt(n.Path, IsValueValidAgainstRange(val, n.Type, n.Format, "Checked", n.Path.dotted()))

	if n.MultipleOf != nil {
		resMultiple = validatorPools.results.Borrow()

		// Is the constraint specifier within the range of the specific numeric type and format?
		resMultiple.addErrorsAt(n.Path, IsValueValidAgainstRange(*n.MultipleOf, n.Type, n.Format, "MultipleOf", n.Path.dotted()))
		if resMultiple.IsValid() {
			// Constraint validated with compatible types
			if err := MultipleOfNativeType(n.Path.dotted(), n.In, val, *n.MultipleOf); err != nil {
				resMultiple.Merge(errorHelp.sErrAt(n.Path, err, n.Options.recycleResult))
			}
		} else {
			// Constraint nevertheless validated, converted as general number
			if err := MultipleOf(n.Path.dotted(), n.In, data, *n.MultipleOf); err != nil {
				resMultiple.Merge(errorHelp.sErrAt(n.Path, err, n.Options.recycleResult))
			}
		}
	}

	if n.Maximum != nil {
		resMaximum = validatorPools.results.Borrow()

		// Is the constraint specifier within the range of the specific numeric type and format?
		resMaximum.addErrorsAt(n.Path, IsValueValidAgainstRange(*n.Maximum, n.Type, n.Format, "Maximum boundary", n.Path.dotted()))
		if resMaximum.IsValid() {
			// Constraint validated with compatible types
			if err := MaximumNativeType(n.Path.dotted(), n.In, val, *n.Maximum, n.ExclusiveMaximum); err != nil {
				resMaximum.Merge(errorHelp.sErrAt(n.Path, err, n.Options.recycleResult))
			}
		} else {
			// Constraint nevertheless validated, converted as general number
			if err := Maximum(n.Path.dotted(), n.In, data, *n.Maximum, n.ExclusiveMaximum); err != nil {
				resMaximum.Merge(errorHelp.sErrAt(n.Path, err, n.Options.recycleResult))
			}
		}
	}

	if n.Minimum != nil {
		resMinimum = validatorPools.results.Borrow()

		// Is the constraint specifier within the range of the specific numeric type and format?
		resMinimum.addErrorsAt(n.Path, IsValueValidAgainstRange(*n.Minimum, n.Type, n.Format, "Minimum boundary", n.Path.dotted()))
		if resMinimum.IsValid() {
			// Constraint validated with compatible types
			if err := MinimumNativeType(n.Path.dotted(), n.In, val, *n.Minimum, n.ExclusiveMinimum); err != nil {
				resMinimum.Merge(errorHelp.sErrAt(n.Path, err, n.Options.recycleResult))
			}
		} else {
			// Constraint nevertheless validated, converted as general number
			if err := Minimum(n.Path.dotted(), n.In, data, *n.Minimum, n.ExclusiveMinimum); err != nil {
				resMinimum.Merge(errorHelp.sErrAt(n.Path, err, n.Options.recycleResult))
			}
		}
	}
	res.Merge(resMultiple, resMinimum, resMaximum)
	res.Inc()

	return res
}

func (n *numberValidator) setPath(path pathSegments) {
	n.Path = path
}

func (n *numberValidator) redeem() {
	validatorPools.numberValidators.Redeem(n)
}

type stringValidator struct {
	Path            pathSegments
	In              string
	Default         any
	Required        bool
	AllowEmptyValue bool
	MaxLength       *int64
	MinLength       *int64
	Pattern         string
	Options         *SchemaValidatorOptions
}

func newStringValidator(
	path pathSegments, in string,
	def any, required, allowEmpty bool, maxLength, minLength *int64, pattern string,
	opts *SchemaValidatorOptions,
) *stringValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var s *stringValidator
	if opts.recycleValidators {
		s = validatorPools.stringValidators.Borrow()
	} else {
		s = new(stringValidator)
	}

	s.Path = path
	s.In = in
	s.Default = def
	s.Required = required
	s.AllowEmptyValue = allowEmpty
	s.MaxLength = maxLength
	s.MinLength = minLength
	s.Pattern = pattern
	s.Options = opts

	return s
}

func (s *stringValidator) Applies(source any, kind reflect.Kind) bool {
	switch source.(type) {
	case *spec.Parameter, *spec.Schema, *spec.Items, *spec.Header:
		return kind == reflect.String
	default:
		return false
	}
}

func (s *stringValidator) Validate(val any) *Result {
	if s.Options.recycleValidators {
		defer func() {
			s.redeem()
		}()
	}

	data, ok := val.(string)
	if !ok {
		return errorHelp.sErrAt(s.Path, errors.InvalidType(s.Path.dotted(), s.In, stringType, val), s.Options.recycleResult)
	}

	if s.Required && !s.AllowEmptyValue && (s.Default == nil || s.Default == "") {
		if err := RequiredString(s.Path.dotted(), s.In, data); err != nil {
			return errorHelp.sErrAt(s.Path, err, s.Options.recycleResult)
		}
	}

	if s.MaxLength != nil {
		if err := MaxLength(s.Path.dotted(), s.In, data, *s.MaxLength); err != nil {
			return errorHelp.sErrAt(s.Path, err, s.Options.recycleResult)
		}
	}

	if s.MinLength != nil {
		if err := MinLength(s.Path.dotted(), s.In, data, *s.MinLength); err != nil {
			return errorHelp.sErrAt(s.Path, err, s.Options.recycleResult)
		}
	}

	if s.Pattern != "" {
		if err := Pattern(s.Path.dotted(), s.In, data, s.Pattern); err != nil {
			return errorHelp.sErrAt(s.Path, err, s.Options.recycleResult)
		}
	}
	return nil
}

func (s *stringValidator) setPath(path pathSegments) {
	s.Path = path
}

func (s *stringValidator) redeem() {
	validatorPools.stringValidators.Redeem(s)
}
