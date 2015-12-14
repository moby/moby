// +build experimental

package local

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	dkrouter "github.com/docker/docker/api/server/router"
	"github.com/docker/docker/runconfig"
	"golang.org/x/net/context"
)

func addExperimentalRoutes(r *router) {
	newRoutes := []dkrouter.Route{
		NewPostRoute("/containers/{name:.*}/checkpoint", r.postContainersCheckpoint),
		NewPostRoute("/containers/{name:.*}/restore", r.postContainersRestore),
	}

	r.routes = append(r.routes, newRoutes...)
}

func (s *router) postContainersCheckpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	criuOpts := &runconfig.CriuConfig{}
	if err := json.NewDecoder(r.Body).Decode(criuOpts); err != nil {
		return err
	}

	if err := s.daemon.ContainerCheckpoint(vars["name"], criuOpts); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *router) postContainersRestore(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	criuOpts := &runconfig.CriuConfig{}
	if err := json.NewDecoder(r.Body).Decode(&criuOpts); err != nil {
		return err
	}
	force := httputils.BoolValueOrDefault(r, "force", false)
	if err := s.daemon.ContainerRestore(vars["name"], criuOpts, force); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
