// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import "github.com/go-openapi/swag/mangling"

// Option configures the behavior of a new [Spec] analyzer.
type Option func(*analyzerOptions)

type analyzerOptions struct {
	manglerOpts []mangling.Option
}

// WithManglerOptions sets the name mangler options used when building
// Go identifiers from specification names (e.g. parameter names).
func WithManglerOptions(opts ...mangling.Option) Option {
	return func(o *analyzerOptions) {
		o.manglerOpts = append(o.manglerOpts, opts...)
	}
}
