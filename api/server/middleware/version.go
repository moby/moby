package middleware

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/version"
	"golang.org/x/net/context"
)

type badRequestError struct {
	error
}

func (badRequestError) HTTPErrorStatusCode() int {
	return http.StatusBadRequest
}

// NewVersionMiddleware creates a new Version middleware.
func NewVersionMiddleware(versionCheck string, defaultVersion, minVersion version.Version) Middleware {
	serverVersion := version.Version(versionCheck)

	return func(handler httputils.APIFunc) httputils.APIFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
			apiVersion := version.Version(vars["version"])
			if apiVersion == "" {
				apiVersion = defaultVersion
			}

			if apiVersion.GreaterThan(defaultVersion) {
				return badRequestError{fmt.Errorf("client is newer than server (client API version: %s, server API version: %s)", apiVersion, defaultVersion)}
			}
			if apiVersion.LessThan(minVersion) {
				return badRequestError{fmt.Errorf("client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version", apiVersion, minVersion)}
			}

			header := fmt.Sprintf("Docker/%s (%s)", serverVersion, runtime.GOOS)
			w.Header().Set("Server", header)
			ctx = context.WithValue(ctx, httputils.APIVersionKey, apiVersion)
			return handler(ctx, w, r, vars)
		}
	}
}
