package auth

import (
	"context"
	"net/http"

	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/types"
)

// AuthBackend is the backend implementation used for authentication
type AuthBackend interface {
	Auth(ctx context.Context, authConfig *types.AuthConfig, userAgent string) (string, string, error)
}

// authRouter is a router to talk with the auth backend
type authRouter struct {
	routes  []router.Route
	backend AuthBackend
}

// Routes returns the available routes to the auth controller
func (r *authRouter) Routes() []router.Route {
	return r.routes
}

// Handler returns the HTTP handler for this router
func (r *authRouter) Handler() http.Handler {
	return router.MakeHandler(r.Routes())
}
