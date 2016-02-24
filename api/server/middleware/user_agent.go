package middleware

import (
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/version"
	"golang.org/x/net/context"
)

// NewUserAgentMiddleware creates a new UserAgent middleware.
func NewUserAgentMiddleware(versionCheck string) Middleware {
	serverVersion := version.Version(versionCheck)

	return func(handler httputils.APIFunc) httputils.APIFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
			if strings.Contains(r.Header.Get("User-Agent"), "Docker-Client/") {
				userAgent := strings.Split(r.Header.Get("User-Agent"), "/")

				// v1.20 onwards includes the GOOS of the client after the version
				// such as Docker/1.7.0 (linux)
				if len(userAgent) == 2 && strings.Contains(userAgent[1], " ") {
					userAgent[1] = strings.Split(userAgent[1], " ")[0]
				}

				if len(userAgent) == 2 && !serverVersion.Equal(version.Version(userAgent[1])) {
					logrus.Debugf("Client and server don't have the same version (client: %s, server: %s)", userAgent[1], serverVersion)
				}
			}
			return handler(ctx, w, r, vars)
		}
	}
}
