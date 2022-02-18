package container // import "github.com/moby/moby/api/server/router/container"

import (
	"context"
	"net/http"

	"github.com/moby/moby/api/server/httputils"
)

// getContainersByName inspects container's configuration and serializes it as json.
func (s *containerRouter) getContainersByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	displaySize := httputils.BoolValue(r, "size")

	version := httputils.VersionFromContext(ctx)
	json, err := s.backend.ContainerInspect(vars["name"], displaySize, version)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, json)
}
