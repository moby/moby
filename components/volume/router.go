package volume

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	apirouter "github.com/docker/docker/api/server/router"
	"github.com/docker/docker/components/volume/types"
	"golang.org/x/net/context"
)

// router is a router to talk with the volumes controller
type router struct {
	backend *backend
}

func newRouter(b *backend) *router {
	return &router{backend: b}
}

// Routes returns the available routes to the volumes controller
func (v *router) Routes() []apirouter.Route {
	return []apirouter.Route{
		// GET
		apirouter.NewGetRoute("/volumes", v.getVolumesList),
		apirouter.NewGetRoute("/volumes/{name:.*}", v.getVolumeByName),
		// POST
		apirouter.NewPostRoute("/volumes/create", v.postVolumesCreate),
		// DELETE
		apirouter.NewDeleteRoute("/volumes/{name:.*}", v.deleteVolumes),
	}
}

func (v *router) getVolumesList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	volumes, warnings, err := v.backend.List(r.Form.Get("filters"))
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &types.VolumesListResponse{Volumes: volumes, Warnings: warnings})
}

func (v *router) getVolumeByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	volume, err := v.backend.Inspect(vars["name"])
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, volume)
}

func (v *router) postVolumesCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	var req types.VolumeCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	volume, err := v.backend.Create(req.Name, req.Driver, "", req.DriverOpts, req.Labels)
	if err != nil {
		return err
	}
	apiV := volumeToAPIType(volume)
	apiV.Mountpoint = volume.Path()

	return httputils.WriteJSON(w, http.StatusCreated, apiV)
}

func (v *router) deleteVolumes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	force := httputils.BoolValue(r, "force")
	if err := v.backend.RemoveByName(vars["name"], force); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
