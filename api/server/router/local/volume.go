package local

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

func (s *router) getVolumesList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	volumes, err := s.daemon.Volumes(r.Form.Get("filters"))
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &types.VolumesListResponse{Volumes: volumes})
}

func (s *router) getVolumeByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	v, err := s.daemon.VolumeInspect(vars["name"])
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, v)
}

func (s *router) postVolumesCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

	volume, err := s.daemon.VolumeCreate(req.Name, req.Driver, req.DriverOpts)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusCreated, volume)
}

func (s *router) deleteVolumes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	if err := s.daemon.VolumeRm(vars["name"]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
