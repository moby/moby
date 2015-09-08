package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/version"
	restful "github.com/emicklei/go-restful"
)

func (s *Server) postAuth(version version.Version, w *restful.Response, r *restful.Request) error {
	var config *cliconfig.AuthConfig
	err := json.NewDecoder(r.Request.Body).Decode(&config)
	r.Request.Body.Close()
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
