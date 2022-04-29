package volume // import "github.com/docker/docker/api/server/router/volume"

import (
	"context"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/volume/service/opts"
	"github.com/pkg/errors"
)

func (v *volumeRouter) getVolumesList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	filters, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return errors.Wrap(err, "error reading volume filters")
	}
	volumes, warnings, err := v.backend.List(ctx, filters)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &volume.ListResponse{Volumes: volumes, Warnings: warnings})
}

func (v *volumeRouter) getVolumeByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	vol, err := v.backend.Get(ctx, vars["name"], opts.WithGetResolveStatus)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, vol)
}

func (v *volumeRouter) postVolumesCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var req volume.CreateOptions
	if err := httputils.ReadJSON(r, &req); err != nil {
		return err
	}

	vol, err := v.backend.Create(ctx, req.Name, req.Driver, opts.WithCreateOptions(req.DriverOpts), opts.WithCreateLabels(req.Labels))
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusCreated, vol)
}

func (v *volumeRouter) deleteVolumes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	force := httputils.BoolValue(r, "force")
	if err := v.backend.Remove(ctx, vars["name"], opts.WithPurgeOnError(force)); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (v *volumeRouter) postVolumesPrune(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	pruneFilters, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	pruneReport, err := v.backend.Prune(ctx, pruneFilters)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, pruneReport)
}
