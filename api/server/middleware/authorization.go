package middleware

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/authorization"
	"golang.org/x/net/context"
)

// NewAuthorizationMiddleware creates a new Authorization middleware.
func NewAuthorizationMiddleware(plugins []authorization.Plugin) Middleware {
	return func(handler httputils.APIFunc) httputils.APIFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
			// FIXME: fill when authN gets in
			// User and UserAuthNMethod are taken from AuthN plugins
			// Currently tracked in https://github.com/docker/docker/pull/13994
			user := ""
			userAuthNMethod := ""
			authCtx := authorization.NewCtx(plugins, user, userAuthNMethod, r.Method, r.RequestURI)

			if err := authCtx.AuthZRequest(w, r); err != nil {
				logrus.Errorf("AuthZRequest for %s %s returned error: %s", r.Method, r.RequestURI, err)
				return err
			}

			rw := authorization.NewResponseModifier(w)

			if err := handler(ctx, rw, r, vars); err != nil {
				logrus.Errorf("Handler for %s %s returned error: %s", r.Method, r.RequestURI, err)
				return err
			}

			if err := authCtx.AuthZResponse(rw, r); err != nil {
				logrus.Errorf("AuthZResponse for %s %s returned error: %s", r.Method, r.RequestURI, err)
				return err
			}
			return nil
		}
	}
}
