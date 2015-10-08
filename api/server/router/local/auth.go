package local

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig"
	"golang.org/x/net/context"
)

func (s *router) postAuth(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var config *cliconfig.AuthConfig
	err := json.NewDecoder(r.Body).Decode(&config)
	r.Body.Close()
	if err != nil {
		return err
	}
	status, err := s.daemon.AuthenticateToRegistry(config)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &types.AuthResponse{
		Status: status,
	})
}
