// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"reflect"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
)

type formatValidator struct {
	Path         pathSegments
	In           string
	Format       string
	KnownFormats strfmt.Registry
	Options      *SchemaValidatorOptions
}

func newFormatValidator(path pathSegments, in, format string, formats strfmt.Registry, opts *SchemaValidatorOptions) *formatValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	var f *formatValidator
	if opts.recycleValidators {
		f = validatorPools.formatValidators.Borrow()
	} else {
		f = new(formatValidator)
	}

	f.Path = path
	f.In = in
	f.Format = format
	f.KnownFormats = formats
	f.Options = opts

	return f
}

func (f *formatValidator) Applies(source any, kind reflect.Kind) bool {
	if source == nil || f.KnownFormats == nil {
		return false
	}

	switch source := source.(type) {
	case *spec.Items:
		return kind == reflect.String && f.KnownFormats.ContainsName(source.Format)
	case *spec.Parameter:
		return kind == reflect.String && f.KnownFormats.ContainsName(source.Format)
	case *spec.Schema:
		return kind == reflect.String && f.KnownFormats.ContainsName(source.Format)
	case *spec.Header:
		return kind == reflect.String && f.KnownFormats.ContainsName(source.Format)
	default:
		return false
	}
}

func (f *formatValidator) Validate(val any) *Result {
	if f.Options.recycleValidators {
		defer func() {
			f.redeem()
		}()
	}

	var result *Result
	if f.Options.recycleResult {
		result = validatorPools.results.Borrow()
	} else {
		result = new(Result)
	}

	str, ok := val.(string)
	if !ok {
		return result
	}

	if err := FormatOf(f.Path.dotted(), f.In, f.Format, str, f.KnownFormats); err != nil {
		result.addErrorsAt(f.Path, err)
	}

	return result
}

func (f *formatValidator) setPath(path pathSegments) {
	f.Path = path
}

func (f *formatValidator) redeem() {
	validatorPools.formatValidators.Redeem(f)
}
