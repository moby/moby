package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/context"
)

func (s *Server) getVolumesList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	volumes, err := s.daemon.Volumes(ctx, r.Form.Get("filters"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, &types.VolumesListResponse{Volumes: volumes})
}

func (s *Server) getVolumeByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	v, err := s.daemon.VolumeInspect(ctx, vars["name"])
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, v)
}

func (s *Server) postVolumesCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	if err := checkForJSON(r); err != nil {
		return err
	}

	var req types.VolumeCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	volume, err := s.daemon.VolumeCreate(ctx, req.Name, req.Driver, req.DriverOpts)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, volume)
}

func (s *Server) deleteVolumes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if err := s.daemon.VolumeRm(ctx, vars["name"]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
