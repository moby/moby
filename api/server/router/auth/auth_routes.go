package auth

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/types"
)

// NewRouter initializes a new auth router
func NewRouter(backend AuthBackend) router.Router {
	r := &authRouter{backend: backend}
	r.routes = []router.Route{
		router.NewPostRoute("/auth", r.auth),
		router.NewPostRoute("/auth/token/debug", r.debugToken),
	}
	return r
}

// auth implements the authentication route in the docker remote API
func (r *authRouter) auth(w http.ResponseWriter, req *http.Request, vars map[string]string) error {
	var config types.AuthConfig
	err := json.NewDecoder(req.Body).Decode(&config)
	if err != nil {
		return err
	}

	status, token, err := r.backend.Auth(req.Context(), &config, req.UserAgent())
	if err != nil {
		return err
	}

	response := types.AuthResponse{
		Status:        status,
		IdentityToken: token,
	}

	return httputils.WriteJSON(w, http.StatusOK, response)
}
