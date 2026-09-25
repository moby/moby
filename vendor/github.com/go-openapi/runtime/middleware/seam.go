// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"path"
	"strings"

	"github.com/go-openapi/runtime/server-middleware/docui"
	"github.com/go-openapi/runtime/server-middleware/negotiate"
)

/////////////////////////////////////////////////////////:
// Seam to the negotiate options introduced in v0.29.5
/////////////////////////////////////////////////////////:

// NegotiateOption configures [NegotiateContentType] behaviour.
//
// Deprecated: moved to the [negotiate] package. Use [negotiate.Option] instead.
type NegotiateOption = negotiate.Option

// NegotiateContentType returns the best offered content type for the
// request's Accept header.
//
// Deprecated: moved to the [negotiate] package. Use [negotiate.ContentType] instead.
func NegotiateContentType(r *http.Request, offers []string, defaultOffer string, opts ...NegotiateOption) string {
	return negotiate.ContentType(r, offers, defaultOffer, opts...)
}

// NegotiateContentEncoding returns the best offered content encoding for
// the request's Accept-Encoding header.
//
// Deprecated: moved to the [negotiate] package. Use [negotiate.ContentEncoding] instead.
func NegotiateContentEncoding(r *http.Request, offers []string) string {
	return negotiate.ContentEncoding(r, offers)
}

// WithIgnoreParameters returns a [NegotiateOption] that strips MIME-type
// parameters from both Accept entries and offers before matching,
// restoring the pre-v0.30 behaviour.
//
// Deprecated: moved to the [negotiate] package. Use [negotiate.WithIgnoreParameters] instead.
func WithIgnoreParameters(ignore bool) NegotiateOption {
	return negotiate.WithIgnoreParameters(ignore)
}

/////////////////////////////////////////////////////////:
// Seam to the UI options
/////////////////////////////////////////////////////////:

// RapiDoc creates a [http.Handler] to serve a documentation site for a swagger spec.
//
// This allows for altering the spec before starting the [http] listener.
//
// Deprecated: moved to the [docui] package. Use [docui.RapiDoc] instead.
func RapiDoc(opts RapiDocOpts, next http.Handler) http.Handler {
	return docui.RapiDoc(next, opts.toFuncOptions()...)
}

// Redoc creates a [http.Handler] to serve a documentation site for a swagger spec.
//
// This allows for altering the spec before starting the [http] listener.
//
// Deprecated: moved to the [docui] package. Use [docui.Redoc] instead.
func Redoc(opts RedocOpts, next http.Handler) http.Handler {
	return docui.Redoc(next, opts.toFuncOptions()...)
}

// SwaggerUI creates a [http.Handler] to serve a documentation site for a swagger spec.
//
// This allows for altering the spec before starting the [http] listener.
//
// Deprecated: moved to the [docui] package. Use [docui.SwaggerUI] instead.
func SwaggerUI(opts SwaggerUIOpts, next http.Handler) http.Handler {
	return docui.SwaggerUI(next, opts.toFuncOptions()...)
}

// SwaggerUIOAuth2Callback creates a middleware that serves the OAuth2 callback page used by Swagger UI.
//
// Deprecated: moved to the [docui] package. Use [docui.SwaggerUIOAuth2Callback] instead.
func SwaggerUIOAuth2Callback(opts SwaggerUIOpts, next http.Handler) http.Handler {
	return docui.SwaggerUIOAuth2Callback(next, opts.toFuncOptions()...)
}

/////////////////////////////////////////////////////////:
// Seam to the spec middleware options
/////////////////////////////////////////////////////////:

// SpecOption can be applied to the [Spec] serving [middleware].
//
// Deprecated: moved to the [docui] package. Use [docui.SpecOption] instead.
type SpecOption func(*specOptions)

type specOptions struct {
	BasePath string
	Path     string
	Document string
}

func (o specOptions) fullPath() string {
	return path.Join(o.BasePath, o.Path, o.Document)
}

func specOptionsWithDefaults(basePath string, opts []SpecOption) specOptions {
	o := specOptions{
		BasePath: "/",
		Path:     "",
		Document: "swagger.json",
	}

	for _, apply := range opts {
		apply(&o)
	}
	if basePath != "" {
		o.BasePath = basePath
	}

	return o
}

// Spec creates a [middleware] to serve a swagger spec as a JSON document.
//
// This allows for altering the spec before starting the [http] listener.
//
// The basePath argument indicates the path of the spec document (defaults to "/").
// Additional [SpecOption] can be used to change the name of the document (defaults to "swagger.json").
//
// Deprecated: moved to the [docui] package as [docui.ServeSpec].
func Spec(basePath string, spec []byte, next http.Handler, opts ...SpecOption) http.Handler {
	o := specOptionsWithDefaults(basePath, opts)

	return docui.ServeSpec(spec, next, docui.WithSpecPath(o.fullPath()))

}

// WithSpecPath sets the path to be joined to the base path of the
// spec-serving middleware (see [docui.ServeSpec]).
//
// This is empty by default.
func WithSpecPath(pth string) SpecOption {
	return func(o *specOptions) {
		o.Path = pth
	}
}

// WithSpecDocument sets the name of the JSON document served as a spec.
//
// By default, this is "swagger.json".
func WithSpecDocument(doc string) SpecOption {
	return func(o *specOptions) {
		if doc == "" {
			return
		}

		o.Document = doc
	}
}

// UIOptions defines common options for UI serving middlewares.
//
// Deprecated: use instead the function options provided by [docui].
type UIOptions struct {
	// BasePath for the UI, defaults to: /
	BasePath string

	// Path combines with BasePath to construct the path to the UI, defaults to: "docs".
	Path string

	// SpecURL is the URL of the spec document.
	//
	// Defaults to: /swagger.json
	SpecURL string

	// Title for the documentation site, default to: API documentation
	Title string

	// Template specifies a custom template to serve the UI
	Template string
}

// toFuncOptions bridges the deprecated options struct with the newer function options in [docui].
func (o UIOptions) toFuncOptions() []docui.Option {
	const structMembers = 5
	opts := make([]docui.Option, 0, structMembers)

	if o.BasePath != "" {
		opts = append(opts, docui.WithUIBasePath(o.BasePath))
	}

	if o.Path != "" {
		opts = append(opts, docui.WithUIPath(o.Path))
	}

	if o.SpecURL != "" {
		opts = append(opts, docui.WithSpecURL(o.SpecURL))
	}

	if o.Title != "" {
		opts = append(opts, docui.WithUITitle(o.Title))
	}

	if o.Template != "" {
		opts = append(opts, docui.WithUITemplate(o.Template))
	}

	return opts
}

// RapiDocOpts configures the [RapiDoc] middlewares.
//
// Deprecated: use instead the function options provided by [docui].
type RapiDocOpts struct {
	// BasePath for the UI, defaults to: /
	BasePath string

	// Path combines with BasePath to construct the path to the UI, defaults to: "docs".
	Path string

	// SpecURL is the URL of the spec document.
	//
	// Defaults to: /swagger.json
	SpecURL string

	// Title for the documentation site, default to: API documentation
	Title string

	// Template specifies a custom template to serve the UI
	Template string

	// RapiDocURL points to the js asset that generates the rapidoc site.
	//
	// Defaults to https://unpkg.com/rapidoc/dist/rapidoc-min.js
	RapiDocURL string
}

func (o RapiDocOpts) toFuncOptions() []docui.Option {
	const structMembers = 6
	opts := make([]docui.Option, 0, structMembers)

	if o.BasePath != "" {
		opts = append(opts, docui.WithUIBasePath(o.BasePath))
	}

	if o.Path != "" {
		opts = append(opts, docui.WithUIPath(o.Path))
	}

	if o.SpecURL != "" {
		opts = append(opts, docui.WithSpecURL(o.SpecURL))
	}

	if o.Title != "" {
		opts = append(opts, docui.WithUITitle(o.Title))
	}

	if o.Template != "" {
		opts = append(opts, docui.WithUITemplate(o.Template))
	}

	if o.RapiDocURL != "" {
		opts = append(opts, docui.WithUIAssetsURL(o.RapiDocURL))
	}

	return opts
}

// RedocOpts configures the [Redoc] middlewares.
//
// Deprecated: use instead the function options provided by [docui].
type RedocOpts struct {
	// BasePath for the UI, defaults to: /
	BasePath string

	// Path combines with BasePath to construct the path to the UI, defaults to: "docs".
	Path string

	// SpecURL is the URL of the spec document.
	//
	// Defaults to: /swagger.json
	SpecURL string

	// Title for the documentation site, default to: API documentation
	Title string

	// Template specifies a custom template to serve the UI
	Template string

	// RedocURL points to the js that generates the redoc site.
	//
	// Defaults to: https://cdn.jsdelivr.net/npm/redoc/bundles/redoc.standalone.js
	RedocURL string
}

func (o RedocOpts) toFuncOptions() []docui.Option {
	const structMembers = 6
	opts := make([]docui.Option, 0, structMembers)

	if o.BasePath != "" {
		opts = append(opts, docui.WithUIBasePath(o.BasePath))
	}

	if o.Path != "" {
		opts = append(opts, docui.WithUIPath(o.Path))
	}

	if o.SpecURL != "" {
		opts = append(opts, docui.WithSpecURL(o.SpecURL))
	}

	if o.Title != "" {
		opts = append(opts, docui.WithUITitle(o.Title))
	}

	if o.Template != "" {
		opts = append(opts, docui.WithUITemplate(o.Template))
	}

	if o.RedocURL != "" {
		opts = append(opts, docui.WithUIAssetsURL(o.RedocURL))
	}

	return opts
}

// SwaggerUIOpts configures the [SwaggerUI] [middleware].
//
// Deprecated: use instead the function options provided by [docui].
type SwaggerUIOpts struct {
	// BasePath for the API, defaults to: /
	BasePath string

	// Path combines with BasePath to construct the path to the UI, defaults to: "docs".
	Path string

	// SpecURL is the URL of the spec document.
	//
	// Defaults to: /swagger.json
	SpecURL string

	// Title for the documentation site, default to: API documentation
	Title string

	// Template specifies a custom template to serve the UI
	Template string

	// OAuthCallbackURL the url called after OAuth2 login
	//
	// NOTE: in the new [docui.SwaggerUIOptions] type, this field is named `OAuth2CallbackURL`,
	// which is more appropriate.
	OAuthCallbackURL string

	// The three components needed to embed swagger-ui

	// SwaggerURL points to the js that generates the SwaggerUI site.
	//
	// Defaults to: https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js
	SwaggerURL string

	SwaggerPresetURL string
	SwaggerStylesURL string

	Favicon32 string
	Favicon16 string
}

func (o SwaggerUIOpts) toFuncOptions() []docui.Option {
	const structMembers = 6
	opts := make([]docui.Option, 0, structMembers)

	if o.BasePath != "" {
		opts = append(opts, docui.WithUIBasePath(o.BasePath))
	}

	if o.Path != "" {
		opts = append(opts, docui.WithUIPath(o.Path))
	}

	if o.SpecURL != "" {
		opts = append(opts, docui.WithSpecURL(o.SpecURL))
	}

	if o.Title != "" {
		opts = append(opts, docui.WithUITitle(o.Title))
	}

	if o.Template != "" {
		opts = append(opts, docui.WithUITemplate(o.Template))
	}

	if o.SwaggerURL != "" {
		opts = append(opts, docui.WithUIAssetsURL(o.SwaggerURL))
	}

	var empty SwaggerUIOpts
	if o != empty {
		swaggeruiOpts := docui.SwaggerUIOptions{
			OAuth2CallbackURL: o.OAuthCallbackURL,
			SwaggerPresetURL:  o.SwaggerPresetURL,
			SwaggerStylesURL:  o.SwaggerStylesURL,
			Favicon32:         o.Favicon32,
			Favicon16:         o.Favicon16,
		}
		opts = append(opts, docui.WithSwaggerUIOptions(swaggeruiOpts))
	}

	return opts
}

// UIOption can be applied to UI serving [middleware] to alter the default
// behavior.
//
// Deprecated: use instead the function options provided by [docui].
type UIOption func(*UIOptions)

// uiOptionsWithDefaults applies the given options on top of an empty
// [UIOptions]. Per-flavor handlers ([SwaggerUI], [Redoc], [RapiDoc])
// fill in the remaining defaults via [UIOptions.EnsureDefaults] when
// the option struct is used.
func uiOptionsWithDefaults(opts []UIOption) UIOptions {
	var o UIOptions
	for _, apply := range opts {
		apply(&o)
	}

	return o
}

// WithUIBasePath sets the base path from where to serve the UI assets.
//
// Deprecated: use instead the function options provided by [docui].
func WithUIBasePath(base string) UIOption {
	return func(o *UIOptions) {
		if !strings.HasPrefix(base, "/") {
			base = "/" + base
		}
		o.BasePath = base
	}
}

// WithUIPath sets the path from where to serve the UI assets (i.e. /{basepath}/{path}.
//
// Deprecated: use instead the function options provided by [docui].
func WithUIPath(pth string) UIOption {
	return func(o *UIOptions) {
		o.Path = pth
	}
}

// WithUISpecURL sets the path from where to serve swagger spec document.
//
// This may be specified as a full URL or a path.
//
// By default, this is "/swagger.json".
//
// Deprecated: use instead the function options provided by [docui].
func WithUISpecURL(specURL string) UIOption {
	return func(o *UIOptions) {
		o.SpecURL = specURL
	}
}

// WithUITitle sets the title of the UI.
//
// Deprecated: use instead the function options provided by [docui].
func WithUITitle(title string) UIOption {
	return func(o *UIOptions) {
		o.Title = title
	}
}

// WithTemplate allows to set a custom template for the UI.
//
// UI [middleware] will panic if the template does not parse or execute properly.
//
// Deprecated: use instead the function options provided by [docui].
func WithTemplate(tpl string) UIOption {
	return func(o *UIOptions) {
		o.Template = tpl
	}
}
