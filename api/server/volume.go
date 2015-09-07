package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/context"
)

// @Title getVolumesList
// @Description Get the list of volumes registered
// @Param   version     path    string     false        "API version number"
// @Param   filters     form    string     false        "Filters for the list of volumes"
// @Success 200 {object} types.VolumesListResponse
// @SubApi /volumes
// @Router /volumes [get]
func (s *Server) getVolumesList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	volumes, err := s.daemon.Volumes(r.Form.Get("filters"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, &types.VolumesListResponse{Volumes: volumes})
}

// @Title getVolumeByName
// @Description Get the list of volumes registered
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Name of the volume"
// @Success 200 {object} types.Volume
// @SubApi /volumes
// @Router /volumes/:name [get]
func (s *Server) getVolumeByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	v, err := s.daemon.VolumeInspect(vars["name"])
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, v)
}

// @Title postVolumesCreate
// @Description Create a new volume in the host
// @Param   version     path    string     false        "API version number"
// @Param   request     form    []byte     true         "Volume configuration"
// @Success 201 {object} types.Volume
// @SubApi /volumes
// @Router /volumes [post]
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

	volume, err := s.daemon.VolumeCreate(req.Name, req.Driver, req.DriverOpts)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, volume)
}

// @Title deleteVolumes
// @Description Delete the volume from the host
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Name of the volume"
// @Success 204
// @SubApi /volumes
// @Router /volumes/:name [delete]
func (s *Server) deleteVolumes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if err := s.daemon.VolumeRm(vars["name"]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
