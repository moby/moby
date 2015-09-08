package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/version"
	restful "github.com/emicklei/go-restful"
)

func (s *Server) getVolumesList(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	volumes, err := s.daemon.Volumes(r.Request.Form.Get("filters"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, &types.VolumesListResponse{Volumes: volumes})
}

func (s *Server) getVolumeByName(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	v, err := s.daemon.VolumeInspect(r.PathParameter("name"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, v)
}

func (s *Server) postVolumesCreate(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	if err := checkForJSON(r.Request); err != nil {
		return err
	}

	var req types.VolumeCreateRequest
	if err := json.NewDecoder(r.Request.Body).Decode(&req); err != nil {
		return err
	}

	volume, err := s.daemon.VolumeCreate(req.Name, req.Driver, req.DriverOpts)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, volume)
}

func (s *Server) deleteVolumes(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}
	if err := s.daemon.VolumeRm(r.PathParameter("name")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
