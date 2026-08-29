// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag/loading"
)

// SchemaValidatorOptions defines optional rules for schema validation.
type SchemaValidatorOptions struct {
	EnableObjectArrayTypeCheck    bool
	EnableArrayMustHaveItemsCheck bool
	recycleValidators             bool
	recycleResult                 bool
	skipSchemataResult            bool
	pathLoaderWithOptions         func(string, ...loading.Option) (json.RawMessage, error)
}

// Option sets optional rules for schema validation.
type Option func(*SchemaValidatorOptions)

// EnableObjectArrayTypeCheck activates the swagger rule: an items must be in type: array.
func EnableObjectArrayTypeCheck(enable bool) Option {
	return func(svo *SchemaValidatorOptions) {
		svo.EnableObjectArrayTypeCheck = enable
	}
}

// EnableArrayMustHaveItemsCheck activates the swagger rule: an array must have items defined.
func EnableArrayMustHaveItemsCheck(enable bool) Option {
	return func(svo *SchemaValidatorOptions) {
		svo.EnableArrayMustHaveItemsCheck = enable
	}
}

// SwaggerSchema activates swagger schema validation rules.
func SwaggerSchema(enable bool) Option {
	return func(svo *SchemaValidatorOptions) {
		svo.EnableObjectArrayTypeCheck = enable
		svo.EnableArrayMustHaveItemsCheck = enable
	}
}

// WithRecycleValidators saves memory allocations and makes validators
// available for a single use of Validate() only.
//
// When a validator is recycled, called MUST not call the Validate() method twice.
func WithRecycleValidators(enable bool) Option {
	return func(svo *SchemaValidatorOptions) {
		svo.recycleValidators = enable
	}
}

func withRecycleResults(enable bool) Option {
	return func(svo *SchemaValidatorOptions) {
		svo.recycleResult = enable
	}
}

// WithSkipSchemataResult skips the deep audit payload stored in validation Result.
func WithSkipSchemataResult(enable bool) Option {
	return func(svo *SchemaValidatorOptions) {
		svo.skipSchemataResult = enable
	}
}

// WithPathLoader injects the document loader used to resolve remote and relative $ref while
// validating a schema or specification. It matches the option-aware loader signature of
// github.com/go-openapi/swag/loading (and go-openapi/loads).
//
// This lets validation resolve references through a caller-provided loader instead of the spec
// package's global default. The loader may carry any loading options — a custom HTTP client or
// timeout, authentication or custom headers, an embedded or rooted file system, and so on.
//
// One important use is confining loading of untrusted input: build the loader with loading.WithRoot
// (to confine local reads) and loading.WithHTTPClient (to restrict remote fetches), or use a
// restricted loader from go-openapi/loads. Left unset, the spec package default loader is used.
func WithPathLoader(loader func(string, ...loading.Option) (json.RawMessage, error)) Option {
	return func(svo *SchemaValidatorOptions) {
		svo.pathLoaderWithOptions = loader
	}
}

// Options returns the current set of options.
func (svo SchemaValidatorOptions) Options() []Option {
	return []Option{
		EnableObjectArrayTypeCheck(svo.EnableObjectArrayTypeCheck),
		EnableArrayMustHaveItemsCheck(svo.EnableArrayMustHaveItemsCheck),
		WithRecycleValidators(svo.recycleValidators),
		withRecycleResults(svo.recycleResult),
		WithSkipSchemataResult(svo.skipSchemataResult),
		WithPathLoader(svo.pathLoaderWithOptions),
	}
}

// expandOptions builds the spec expand options for schema/$ref expansion during validation,
// carrying the injected loader (when set) so resolution can be confined. relativeBase is used for
// base-path-relative resolution; it is ignored by [spec.ExpandSchemaWithOptions], which derives the
// base from the root.
func (svo *SchemaValidatorOptions) expandOptions(relativeBase string) *spec.ExpandOptions {
	return &spec.ExpandOptions{
		RelativeBase:          relativeBase,
		PathLoaderWithOptions: svo.pathLoaderWithOptions,
	}
}
