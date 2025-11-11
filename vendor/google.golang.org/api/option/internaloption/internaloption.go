// Copyright 2020 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package internaloption contains options used internally by Google client code.
package internaloption

import (
	"context"
	"log/slog"

	"cloud.google.com/go/auth"
	"github.com/googleapis/gax-go/v2/internallog"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/internal"
	"google.golang.org/api/option"
)

type defaultEndpointOption string

func (o defaultEndpointOption) Apply(settings *internal.DialSettings) {
	settings.DefaultEndpoint = string(o)
}

// WithDefaultEndpoint is an option that indicates the default endpoint.
//
// It should only be used internally by generated clients.
//
// This is similar to WithEndpoint, but allows us to determine whether the user has overridden the default endpoint.
//
// Deprecated: WithDefaultEndpoint does not support setting the universe domain.
// Use WithDefaultEndpointTemplate and WithDefaultUniverseDomain to compose the
// default endpoint instead.
func WithDefaultEndpoint(url string) option.ClientOption {
	return defaultEndpointOption(url)
}

type defaultEndpointTemplateOption string

func (o defaultEndpointTemplateOption) Apply(settings *internal.DialSettings) {
	settings.DefaultEndpointTemplate = string(o)
}

// WithDefaultEndpointTemplate provides a template for creating the endpoint
// using a universe domain. See also WithDefaultUniverseDomain and
// option.WithUniverseDomain. The placeholder UNIVERSE_DOMAIN should be used
// instead of a concrete universe domain such as "googleapis.com".
//
// Example: WithDefaultEndpointTemplate("https://logging.UNIVERSE_DOMAIN/")
//
// It should only be used internally by generated clients.
func WithDefaultEndpointTemplate(url string) option.ClientOption {
	return defaultEndpointTemplateOption(url)
}

type defaultMTLSEndpointOption string

func (o defaultMTLSEndpointOption) Apply(settings *internal.DialSettings) {
	settings.DefaultMTLSEndpoint = string(o)
}

// WithDefaultMTLSEndpoint is an option that indicates the default mTLS endpoint.
//
// It should only be used internally by generated clients.
func WithDefaultMTLSEndpoint(url string) option.ClientOption {
	return defaultMTLSEndpointOption(url)
}

// SkipDialSettingsValidation bypasses validation on ClientOptions.
//
// It should only be used internally.
func SkipDialSettingsValidation() option.ClientOption {
	return skipDialSettingsValidation{}
}

type skipDialSettingsValidation struct{}

func (s skipDialSettingsValidation) Apply(settings *internal.DialSettings) {
	settings.SkipValidation = true
}

// EnableDirectPath returns a ClientOption that overrides the default
// attempt to use DirectPath.
//
// It should only be used internally by generated clients.
// This is an EXPERIMENTAL API and may be changed or removed in the future.
func EnableDirectPath(dp bool) option.ClientOption {
	return enableDirectPath(dp)
}

type enableDirectPath bool

func (e enableDirectPath) Apply(o *internal.DialSettings) {
	o.EnableDirectPath = bool(e)
}

// EnableDirectPathXds returns a ClientOption that overrides the default
// DirectPath type. It is only valid when DirectPath is enabled.
//
// It should only be used internally by generated clients.
// This is an EXPERIMENTAL API and may be changed or removed in the future.
func EnableDirectPathXds() option.ClientOption {
	return enableDirectPathXds(true)
}

type enableDirectPathXds bool

func (x enableDirectPathXds) Apply(o *internal.DialSettings) {
	o.EnableDirectPathXds = bool(x)
}

// AllowNonDefaultServiceAccount returns a ClientOption that overrides the default
// requirement for using the default service account for DirectPath.
//
// It should only be used internally by generated clients.
// This is an EXPERIMENTAL API and may be changed or removed in the future.
func AllowNonDefaultServiceAccount(nd bool) option.ClientOption {
	return allowNonDefaultServiceAccount(nd)
}

type allowNonDefaultServiceAccount bool

func (a allowNonDefaultServiceAccount) Apply(o *internal.DialSettings) {
	o.AllowNonDefaultServiceAccount = bool(a)
}

// WithDefaultAudience returns a ClientOption that specifies a default audience
// to be used as the audience field ("aud") for the JWT token authentication.
//
// It should only be used internally by generated clients.
func WithDefaultAudience(audience string) option.ClientOption {
	return withDefaultAudience(audience)
}

type withDefaultAudience string

func (w withDefaultAudience) Apply(o *internal.DialSettings) {
	o.DefaultAudience = string(w)
}

// WithDefaultScopes returns a ClientOption that overrides the default OAuth2
// scopes to be used for a service.
//
// It should only be used internally by generated clients.
func WithDefaultScopes(scope ...string) option.ClientOption {
	return withDefaultScopes(scope)
}

type withDefaultScopes []string

func (w withDefaultScopes) Apply(o *internal.DialSettings) {
	o.DefaultScopes = make([]string, len(w))
	copy(o.DefaultScopes, w)
}

// WithDefaultUniverseDomain returns a ClientOption that sets the default universe domain.
//
// It should only be used internally by generated clients.
//
// This is similar to the public WithUniverse, but allows us to determine whether the user has
// overridden the default universe.
func WithDefaultUniverseDomain(ud string) option.ClientOption {
	return withDefaultUniverseDomain(ud)
}

type withDefaultUniverseDomain string

func (w withDefaultUniverseDomain) Apply(o *internal.DialSettings) {
	o.DefaultUniverseDomain = string(w)
}

// EnableJwtWithScope returns a ClientOption that specifies if scope can be used
// with self-signed JWT.
//
// EnableJwtWithScope is ignored when option.WithUniverseDomain is set
// to a value other than the Google Default Universe (GDU) of "googleapis.com".
// For non-GDU domains, token exchange is impossible and services must
// support self-signed JWTs with scopes.
func EnableJwtWithScope() option.ClientOption {
	return enableJwtWithScope(true)
}

type enableJwtWithScope bool

func (w enableJwtWithScope) Apply(o *internal.DialSettings) {
	o.EnableJwtWithScope = bool(w)
}

// WithCredentials returns a client option to specify credentials which will be used to authenticate API calls.
// This credential takes precedence over all other credential options.
func WithCredentials(creds *google.Credentials) option.ClientOption {
	return (*withCreds)(creds)
}

type withCreds google.Credentials

func (w *withCreds) Apply(o *internal.DialSettings) {
	o.InternalCredentials = (*google.Credentials)(w)
}

// EnableNewAuthLibrary returns a ClientOption that specifies if libraries in this
// module to delegate auth to our new library. This option will be removed in
// the future once all clients have been moved to the new auth layer.
func EnableNewAuthLibrary() option.ClientOption {
	return enableNewAuthLibrary(true)
}

type enableNewAuthLibrary bool

func (w enableNewAuthLibrary) Apply(o *internal.DialSettings) {
	o.EnableNewAuthLibrary = bool(w)
}

// EnableAsyncRefreshDryRun returns a ClientOption that specifies if libraries in this
// module should asynchronously refresh auth token in parallel to sync refresh.
//
// This option can be used to determine whether refreshing the token asymnchronously
// prior to its actual expiry works without any issues in a particular environment.
//
// errHandler function will be called when there is an error while refreshing
// the token asynchronously.
//
// This is an EXPERIMENTAL option and will be removed in the future.
// TODO(b/372244283): Remove after b/358175516 has been fixed
func EnableAsyncRefreshDryRun(errHandler func()) option.ClientOption {
	return enableAsyncRefreshDryRun{
		errHandler: errHandler,
	}
}

// TODO(b/372244283): Remove after b/358175516 has been fixed
type enableAsyncRefreshDryRun struct {
	errHandler func()
}

// TODO(b/372244283): Remove after b/358175516 has been fixed
func (w enableAsyncRefreshDryRun) Apply(o *internal.DialSettings) {
	o.EnableAsyncRefreshDryRun = w.errHandler
}

// EmbeddableAdapter is a no-op option.ClientOption that allow libraries to
// create their own client options by embedding this type into their own
// client-specific option wrapper. See example for usage.
type EmbeddableAdapter struct{}

func (*EmbeddableAdapter) Apply(_ *internal.DialSettings) {}

// GetLogger is a helper for client libraries to extract the [slog.Logger] from
// the provided options or return a default logger if one is not found.
//
// It should only be used internally by generated clients. This is an EXPERIMENTAL API
// and may be changed or removed in the future.
func GetLogger(opts []option.ClientOption) *slog.Logger {
	var ds internal.DialSettings
	for _, opt := range opts {
		opt.Apply(&ds)
	}
	return internallog.New(ds.Logger)
}

// AuthCreds returns [cloud.google.com/go/auth.Credentials] using the following
// options provided via [option.ClientOption], including legacy oauth2/google
// options, in this order:
//
// * [option.WithAuthCredentials]
// * [option/internaloption.WithCredentials] (internal use only)
// * [option.WithCredentials]
// * [option.WithTokenSource]
//
// If there are no applicable credentials options, then it passes the
// following options to [cloud.google.com/go/auth/credentials.DetectDefault] and
// returns the result:
//
// * [option.WithAudiences]
// * [option.WithCredentialsFile]
// * [option.WithCredentialsJSON]
// * [option.WithScopes]
// * [option/internaloption.WithDefaultScopes] (internal use only)
// * [option/internaloption.EnableJwtWithScope] (internal use only)
//
// This function should only be used internally by generated clients. This is an
// EXPERIMENTAL API and may be changed or removed in the future.
func AuthCreds(ctx context.Context, opts []option.ClientOption) (*auth.Credentials, error) {
	var ds internal.DialSettings
	for _, opt := range opts {
		opt.Apply(&ds)
	}
	return internal.AuthCreds(ctx, &ds)
}
