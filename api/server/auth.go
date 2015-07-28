package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/version"
)

func (s *Server) postAuth(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
