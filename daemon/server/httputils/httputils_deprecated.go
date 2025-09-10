package httputils

import (
	"context"

	"github.com/moby/moby/api/types/versions"
)

// APIVersionKey is the client's requested API version.
//
// Deprecated: use [versions.WithVersion] or [versions.FromContext].
type APIVersionKey = versions.APIVersionKey //nolint:staticcheck // ignore SA1019: versions.APIVersionKey is deprecated.

// VersionFromContext returns an API version from the context using APIVersionKey.
//
// Deprecated: use [versions.FromContext].
func VersionFromContext(ctx context.Context) string {
	return versions.FromContext(ctx)
}
