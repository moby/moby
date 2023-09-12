package container // import "github.com/docker/docker/api/server/router/container"

import (
	"context"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/versions"
)

// getContainersByName inspects container's configuration and serializes it as json.
func (s *containerRouter) getContainersByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	displaySize := httputils.BoolValue(r, "size")

	version := versions.FromContext(ctx)
	json, err := s.backend.ContainerInspect(ctx, vars["name"], displaySize, version)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, json)
}
