package versions

import (
	"context"
)

// APIVersionKey is the API version used for the request's context.
//
// Deprecated: use [WithVersion].
type APIVersionKey = apiVersionKey

// apiVersionKey is the API version used for the request's context.
type apiVersionKey struct{}

// WithVersion creates a new context from the given parent context, with
// the given API version attached as value. It returns the parent context
// unmodified if the API version is empty. Similarly, if the parent context
// already has the given version as value, no new context is created, and
// the parent is returned unmodified.
//
// WithVersion uses [context.WithValue], and it panics if a nil-context
// is passed as parent.
func WithVersion(parent context.Context, version string) context.Context {
	if version == "" {
		return parent
	}
	if v := FromContext(parent); v == version {
		return parent
	}
	return context.WithValue(parent, apiVersionKey{}, version)
}

// FromContext returns an API version from the context. It panics if the
// context value does not have the right type.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if val := ctx.Value(apiVersionKey{}); val != nil {
		return val.(string)
	}

	return ""
}
