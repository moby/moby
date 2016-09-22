package dockerversion

import (
	"golang.org/x/net/context"
)

// APIVersionKey is the client's requested API version.
const APIVersionKey = "api-version"

// FromContext returns the API version stored in ctx, if one exists.
func FromContext(ctx context.Context) (v string) {
	if v, ok := ctx.Value(APIVersionKey).(string); ok {
		return v
	}
	return
}

// NewContext returns a new Context that carries the API version.
func NewContext(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, APIVersionKey, version)
}
