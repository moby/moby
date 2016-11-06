package volume

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	volumetypes "github.com/docker/docker/api/types/volume"
	volumestore "github.com/docker/docker/volume/store"
	"golang.org/x/net/context"
)

func (v *volumeRouter) getVolumesList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	volumes, warnings, err := v.backend.Volumes(r.Form.Get("filters"))
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &volumetypes.VolumesListOKBody{Volumes: volumes, Warnings: warnings})
}

func (v *volumeRouter) getVolumeByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	volume, err := v.backend.VolumeInspect(vars["name"])
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, volume)
}

func (v *volumeRouter) postVolumesCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	var req volumetypes.VolumesCreateBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	volume, err := v.backend.VolumeCreate(req.Name, req.Driver, req.DriverOpts, req.Labels)
	if volumestore.IsAlreadyExists(err) {
		version := httputils.VersionFromContext(ctx)
		if versions.GreaterThanOrEqualTo(version, "1.29") {
			return httputils.WriteJSON(w, http.StatusNotModified, volume)
		}
		return httputils.WriteJSON(w, http.StatusCreated, volume)
	}
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusCreated, volume)
}

func (v *volumeRouter) deleteVolumes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	force := httputils.BoolValue(r, "force")
	if err := v.backend.VolumeRm(vars["name"], force); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (v *volumeRouter) postVolumesPrune(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	pruneFilters, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	pruneReport, err := v.backend.VolumesPrune(ctx, pruneFilters)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, pruneReport)
}
