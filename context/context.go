package context

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/pkg/version"
)

const (
	// RequestID is the unique ID for each http request
	RequestID = "request-id"

	// APIVersion is the client's requested API version
	APIVersion = "api-version"
)

// Context is just our own wrapper for the golang 'Context' - mainly
// so we can add our over version of the funcs.
type Context struct {
	context.Context
}

// Background creates a new Context based on golang's default one.
func Background() Context {
	return Context{context.Background()}
}

// WithValue will return a Context that has this new key/value pair
// associated with it. Just uses the golang version but then wraps it.
func WithValue(ctx Context, key, value interface{}) Context {
	return Context{context.WithValue(ctx, key, value)}
}

// RequestID is a utility func to make it easier to get the
// request ID associated with this Context/request.
func (ctx Context) RequestID() string {
	val := ctx.Value(RequestID)
	if val == nil {
		return ""
	}

	id, ok := val.(string)
	if !ok {
		// Ideally we shouldn't panic but we also should never get here
		panic("Context RequestID isn't a string")
	}
	return id
}

// Version is a utility func to make it easier to get the
// API version string associated with this Context/request.
func (ctx Context) Version() version.Version {
	val := ctx.Value(APIVersion)
	if val == nil {
		return version.Version("")
	}

	ver, ok := val.(version.Version)
	if !ok {
		// Ideally we shouldn't panic but we also should never get here
		panic("Context APIVersion isn't a version.Version")
	}
	return ver
}
