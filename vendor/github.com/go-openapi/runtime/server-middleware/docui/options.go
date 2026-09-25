// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package docui

import (
	"net/http"
	"net/url"
	"strings"
)

const (
	// constants that are common to all UI-serving middlewares.
	defaultDocsPath  = "docs"
	defaultDocsURL   = "/swagger.json"
	defaultDocsTitle = "API Documentation"

	contentTypeHeader = "Content-Type"
	applicationJSON   = "application/json"
)

// UIMiddleware is a function returning a http middleware which accepts UI [Option].
type UIMiddleware func(...Option) func(http.Handler) http.Handler

// Option to tune your swagger documentation UI middleware.
//
// Options may be combined to alter the route at which the UI asset is served,
// the URL of the spec document, the source URL of the UI asset and the title of the UI page.
//
// The embedded js scriptlet served may be modified using [WithUITemplate].
type Option func(*options)

// SpecOption can be applied to the [ServeSpec] middleware.
type SpecOption func(*specOptions)

// SwaggerUIOptions define a group of extra options specific to the SwaggerUI component.
type SwaggerUIOptions struct {
	// OAuth2CallbackURL sets the URL called after OAuth2 login
	OAuth2CallbackURL string

	// Defines the URL of the swagger UI assets with presets.
	//
	// Default: https://unpkg.com/swagger-ui-dist/swagger-ui-standalone-preset.js
	SwaggerPresetURL string

	// Defines style sheet URL.
	//
	// Default: https://unpkg.com/swagger-ui-dist/swagger-ui.css
	SwaggerStylesURL string

	// Define the favicons URLs.
	//
	// Defaults:
	//
	//   - 16x16: https://unpkg.com/swagger-ui-dist/favicon-16x16.png
	//   - 32x32: https://unpkg.com/swagger-ui-dist/favicon-32x32.png
	Favicon32 string
	Favicon16 string
}

func (o *SwaggerUIOptions) applySwaggerUIDefaults() {
	if o.SwaggerPresetURL == "" {
		o.SwaggerPresetURL = swaggerPresetLatest
	}
	if o.SwaggerStylesURL == "" {
		o.SwaggerStylesURL = swaggerStylesLatest
	}
	if o.Favicon16 == "" || o.Favicon32 == "" {
		o.Favicon16 = swaggerFavicon16Latest
		o.Favicon32 = swaggerFavicon32Latest
	}
}

type (
	options struct {
		SwaggerUIOptions

		// BasePath for the UI, defaults to: /
		BasePath string

		// Path combines with BasePath to construct the path to the UI, defaults to: "docs".
		Path string

		// SpecURL is the URL of the spec document.
		SpecURL string

		// Title for the documentation site, default to: API documentation
		Title string

		// Template specifies a custom template to serve the UI
		Template string

		// AssetsURL points to the js asset that generates the documentation page.
		AssetsURL string
	}

	specOptions struct {
		Path     string
		Document string
	}
)

////////////////////////////////////////////////////////////
// Common UI options
////////////////////////////////////////////////////////////

// WithUIBasePath sets the base path from where to serve the UI assets.
//
// Default: "/"
func WithUIBasePath(base string) Option {
	return func(o *options) {
		if !strings.HasPrefix(base, "/") {
			base = "/" + base
		}
		o.BasePath = base
	}
}

// WithUIPath sets the path from where to serve the UI assets (i.e. /{basepath}/{path}).
//
// Default: "docs"
func WithUIPath(pth string) Option {
	return func(o *options) {
		o.Path = pth
	}
}

// WithUITitle sets the title of the UI.
//
// Default: "API documentation"
func WithUITitle(title string) Option {
	return func(o *options) {
		o.Title = title
	}
}

// WithUIAssetsURL sets the URL from where to fetch the js assets.
//
// Defaults:
//
//   - for Redoc: https://cdn.jsdelivr.net/npm/redoc/bundles/redoc.standalone.js
//   - for RapiDoc, this defaults to: https://unpkg.com/rapidoc/dist/rapidoc-min.js
//   - for SwaggerUI: https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js
func WithUIAssetsURL(assets string) Option {
	return func(o *options) {
		o.AssetsURL = assets
	}
}

// WithUITemplate allows to set a custom template for the UI.
//
// This allows the caller to fully customize the rendered UI, using the advanced options
// provided by any UI.
//
// The UI [middleware] will panic if the template does not parse or execute properly.
//
// Reference documentations to customize your js scriptlet:
//
//   - for Redoc: https://github.com/Redocly/redoc/blob/main/docs/deployment/html.md
//   - for RapiDoc: https://github.com/rapi-doc/RapiDoc
//   - for SwaggerUI: https://github.com/swagger-api/swagger-ui
func WithUITemplate[StringOrBytes ~string | ~[]byte](tpl StringOrBytes) Option {
	return func(o *options) {
		o.Template = string(tpl)
	}
}

// WithSpecURL sets the URL of the spec document.
//
// Defaults to: /swagger.json
func WithSpecURL(u string) Option {
	return func(o *options) {
		o.SpecURL = u
	}
}

////////////////////////////////////////////////////////////
// SwaggerUI UI options
////////////////////////////////////////////////////////////

func WithSwaggerUIOptions(opts SwaggerUIOptions) Option {
	return func(o *options) {
		o.SwaggerUIOptions = opts
	}
}

////////////////////////////////////////////////////////////
// Spec options
////////////////////////////////////////////////////////////

// WithSpecPath sets the path of the spec document.
//
// This is "/swagger.json" by default.
func WithSpecPath(pth string) SpecOption {
	return func(o *specOptions) {
		if pth == "" {
			return
		}

		o.Path = pth
	}
}

// WithSpecPathFromOptions reuses the same SpecPath as the one specified in
// a set of UI [Option] (extract the path from the URL provided by [WithSpecURL]).
func WithSpecPathFromOptions(opts ...Option) SpecOption {
	return func(o *specOptions) {
		uiOpts := optionsWithDefaults(opts)

		// If the spec URL is provided, there is a non-default path to serve the spec.
		//
		// This makes sure that the UI middleware is aligned with the Spec middleware.
		u, _ := url.Parse(uiOpts.SpecURL)

		if u.Path == "" {
			return
		}

		o.Path = u.Path
	}
}

func optionsWithDefaults(opts []Option, prepend ...Option) options {
	o := options{
		BasePath: "/",
		Path:     defaultDocsPath,
		SpecURL:  defaultDocsURL,
		Title:    defaultDocsTitle,
	}

	prepend = append(prepend, opts...)
	for _, apply := range prepend {
		apply(&o)
	}

	return o
}

func specOptionsWithDefaults(opts []SpecOption) specOptions {
	o := specOptions{
		Path: defaultDocsURL,
	}

	for _, apply := range opts {
		apply(&o)
	}

	if !strings.HasPrefix(o.Path, "/") {
		o.Path = "/" + o.Path
	}

	return o
}
