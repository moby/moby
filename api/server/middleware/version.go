package middleware // import "github.com/docker/docker/api/server/middleware"

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/versions"
)

// VersionMiddleware is a middleware that
// validates the client and server versions.
type VersionMiddleware struct {
	serverVersion string

	// defaultAPIVersion is the default API version provided by the API server,
	// specified as "major.minor". It is usually configured to the latest API
	// version [github.com/docker/docker/api.DefaultVersion].
	//
	// API requests for API versions greater than this version are rejected by
	// the server and produce a [versionUnsupportedError].
	defaultAPIVersion string

	// minAPIVersion is the minimum API version provided by the API server,
	// specified as "major.minor".
	//
	// API requests for API versions lower than this version are rejected by
	// the server and produce a [versionUnsupportedError].
	minAPIVersion string
}

// NewVersionMiddleware creates a VersionMiddleware with the given versions.
func NewVersionMiddleware(serverVersion, defaultAPIVersion, minAPIVersion string) (*VersionMiddleware, error) {
	if versions.LessThan(defaultAPIVersion, api.MinSupportedAPIVersion) || versions.GreaterThan(defaultAPIVersion, api.DefaultVersion) {
		return nil, fmt.Errorf("invalid default API version (%s): must be between %s and %s", defaultAPIVersion, api.MinSupportedAPIVersion, api.DefaultVersion)
	}
	if versions.LessThan(minAPIVersion, api.MinSupportedAPIVersion) || versions.GreaterThan(minAPIVersion, api.DefaultVersion) {
		return nil, fmt.Errorf("invalid minimum API version (%s): must be between %s and %s", minAPIVersion, api.MinSupportedAPIVersion, api.DefaultVersion)
	}
	if versions.GreaterThan(minAPIVersion, defaultAPIVersion) {
		return nil, fmt.Errorf("invalid API version: the minimum API version (%s) is higher than the default version (%s)", minAPIVersion, defaultAPIVersion)
	}
	return &VersionMiddleware{
		serverVersion:     serverVersion,
		defaultAPIVersion: defaultAPIVersion,
		minAPIVersion:     minAPIVersion,
	}, nil
}

type versionUnsupportedError struct {
	version, minVersion, maxVersion string
}

func (e versionUnsupportedError) Error() string {
	if e.minVersion != "" {
		return fmt.Sprintf("client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version", e.version, e.minVersion)
	}
	return fmt.Sprintf("client version %s is too new. Maximum supported API version is %s", e.version, e.maxVersion)
}

func (e versionUnsupportedError) InvalidParameter() {}

// WrapHandler returns a new handler function wrapping the previous one in the request chain.
func (v VersionMiddleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		w.Header().Set("Server", fmt.Sprintf("Docker/%s (%s)", v.serverVersion, runtime.GOOS))
		w.Header().Set("Api-Version", v.defaultAPIVersion)
		w.Header().Set("Ostype", runtime.GOOS)

		apiVersion := vars["version"]
		if apiVersion == "" {
			apiVersion = v.defaultAPIVersion
		}
		if versions.LessThan(apiVersion, v.minAPIVersion) {
			return versionUnsupportedError{version: apiVersion, minVersion: v.minAPIVersion}
		}
		if versions.GreaterThan(apiVersion, v.defaultAPIVersion) {
			return versionUnsupportedError{version: apiVersion, maxVersion: v.defaultAPIVersion}
		}
		ctx = context.WithValue(ctx, httputils.APIVersionKey{}, apiVersion)
		return handler(ctx, w, r, vars)
	}
}
