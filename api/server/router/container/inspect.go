package container // import "github.com/docker/docker/api/server/router/container"

import (
	"context"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/daemon/names"
)

func containerNotFound(id string) error {
	return objNotFoundError{"container", id}
}

type objNotFoundError struct {
	object string
	id     string
}

func (e objNotFoundError) Error() string {
	return "No such " + e.object + ": " + e.id
}

func (e objNotFoundError) NotFound() {}

// getContainersByName inspects container's configuration and serializes it as json.
func (s *containerRouter) getContainersByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if valid, err := names.ValidateName(vars["name"]); !valid && err != nil {
		// for compatible with old, we need to raise a fake containerNotFound error
		return containerNotFound(vars["name"])
	}
	displaySize := httputils.BoolValue(r, "size")

	version := httputils.VersionFromContext(ctx)
	json, err := s.backend.ContainerInspect(vars["name"], displaySize, version)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, json)
}
