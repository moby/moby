package middleware

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/docker/docker/pkg/version"
	"golang.org/x/net/context"
)

type badRequestError struct {
	error
}

func (badRequestError) HTTPErrorStatusCode() int {
	return http.StatusBadRequest
}

// VersionMiddleware is a middleware that
// validates the client and server versions.
type VersionMiddleware struct {
	serverVersion  version.Version
	defaultVersion version.Version
	minVersion     version.Version
}

// NewVersionMiddleware creates a new VersionMiddleware
// with the default versions.
func NewVersionMiddleware(s, d, m version.Version) VersionMiddleware {
	return VersionMiddleware{
		serverVersion:  s,
		defaultVersion: d,
		minVersion:     m,
	}
}

// WrapHandler returns a new handler function wrapping the previous one in the request chain.
func (v VersionMiddleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		apiVersion := version.Version(vars["version"])
		if apiVersion == "" {
			apiVersion = v.defaultVersion
		}

		if apiVersion.GreaterThan(v.defaultVersion) {
			return badRequestError{fmt.Errorf("client is newer than server (client API version: %s, server API version: %s)", apiVersion, v.defaultVersion)}
		}
		if apiVersion.LessThan(v.minVersion) {
			return badRequestError{fmt.Errorf("client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version", apiVersion, v.minVersion)}
		}

		header := fmt.Sprintf("Docker/%s (%s)", v.serverVersion, runtime.GOOS)
		w.Header().Set("Server", header)
		ctx = context.WithValue(ctx, "api-version", apiVersion)
		return handler(ctx, w, r, vars)
	}

}
