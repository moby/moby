package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig"
	"golang.org/x/net/context"
)

// @Title postAuth
// @Description Authenticate the client against a remote registry
// @Param   version     path    string     false        "API version number"
// @Param   authConfig  body    []byte     true         "Auth credentials and configuration"
// @Success 200 {object} types.AuthResponse
// @Router /auth [post]
func (s *Server) postAuth(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var config *cliconfig.AuthConfig
	err := json.NewDecoder(r.Body).Decode(&config)
	r.Body.Close()
	if err != nil {
		return err
	}
	status, err := s.daemon.RegistryService.Auth(config)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, &types.AuthResponse{
		Status: status,
	})
}
