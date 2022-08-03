package volume // import "github.com/docker/docker/api/server/router/volume"

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/volume/service/opts"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// clusterVolumesVersion defines the API version that swarm cluster volume
	// functionality was introduced. avoids the use of magic numbers.
	clusterVolumesVersion = "1.42"
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

	version := httputils.VersionFromContext(ctx)
	if versions.GreaterThanOrEqualTo(version, clusterVolumesVersion) && v.cluster.IsManager() {
		clusterVolumes, swarmErr := v.cluster.GetVolumes(volume.ListOptions{Filters: filters})
		if swarmErr != nil {
			// if there is a swarm error, we may not want to error out right
			// away. the local list probably worked. instead, let's do what we
			// do if there's a bad driver while trying to list: add the error
			// to the warnings. don't do this if swarm is not initialized.
			warnings = append(warnings, swarmErr.Error())
		}
		// add the cluster volumes to the return
		volumes = append(volumes, clusterVolumes...)
	}

	return httputils.WriteJSON(w, http.StatusOK, &volume.ListResponse{Volumes: volumes, Warnings: warnings})
}

func (v *volumeRouter) getVolumeByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	version := httputils.VersionFromContext(ctx)

	// re: volume name duplication
	//
	// we prefer to get volumes locally before attempting to get them from the
	// cluster. Local volumes can only be looked up by name, but cluster
	// volumes can also be looked up by ID.
	vol, err := v.backend.Get(ctx, vars["name"], opts.WithGetResolveStatus)

	// if the volume is not found in the regular volume backend, and the client
	// is using an API version greater than 1.42 (when cluster volumes were
	// introduced), then check if Swarm has the volume.
	if errdefs.IsNotFound(err) && versions.GreaterThanOrEqualTo(version, clusterVolumesVersion) && v.cluster.IsManager() {
		swarmVol, err := v.cluster.GetVolume(vars["name"])
		// if swarm returns an error and that error indicates that swarm is not
		// initialized, return original NotFound error. Otherwise, we'd return
		// a weird swarm unavailable error on non-swarm engines.
		if err != nil {
			return err
		}
		vol = &swarmVol
	} else if err != nil {
		// otherwise, if this isn't NotFound, or this isn't a high enough version,
		// just return the error by itself.
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

	var (
		vol     *volume.Volume
		err     error
		version = httputils.VersionFromContext(ctx)
	)

	// if the ClusterVolumeSpec is filled in, then this is a cluster volume
	// and is created through the swarm cluster volume backend.
	//
	// re: volume name duplication
	//
	// As it happens, there is no good way to prevent duplication of a volume
	// name between local and cluster volumes. This is because Swarm volumes
	// can be created from any manager node, bypassing most of the protections
	// we could put into the engine side.
	//
	// Instead, we will allow creating a volume with a duplicate name, which
	// should not break anything.
	if req.ClusterVolumeSpec != nil && versions.GreaterThanOrEqualTo(version, clusterVolumesVersion) {
		logrus.Debug("using cluster volume")
		vol, err = v.cluster.CreateVolume(req)
	} else {
		logrus.Debug("using regular volume")
		vol, err = v.backend.Create(ctx, req.Name, req.Driver, opts.WithCreateOptions(req.DriverOpts), opts.WithCreateLabels(req.Labels))
	}

	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusCreated, vol)
}

func (v *volumeRouter) putVolumesUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if !v.cluster.IsManager() {
		return errdefs.Unavailable(errors.New("volume update only valid for cluster volumes, but swarm is unavailable"))
	}

	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	rawVersion := r.URL.Query().Get("version")
	version, err := strconv.ParseUint(rawVersion, 10, 64)
	if err != nil {
		err = fmt.Errorf("invalid swarm object version '%s': %v", rawVersion, err)
		return errdefs.InvalidParameter(err)
	}

	var req volume.UpdateOptions
	if err := httputils.ReadJSON(r, &req); err != nil {
		return err
	}

	return v.cluster.UpdateVolume(vars["name"], version, req)
}

func (v *volumeRouter) deleteVolumes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	force := httputils.BoolValue(r, "force")

	version := httputils.VersionFromContext(ctx)

	err := v.backend.Remove(ctx, vars["name"], opts.WithPurgeOnError(force))
	// when a removal is forced, if the volume does not exist, no error will be
	// returned. this means that to ensure forcing works on swarm volumes as
	// well, we should always also force remove against the cluster.
	if err != nil || force {
		if versions.GreaterThanOrEqualTo(version, clusterVolumesVersion) && v.cluster.IsManager() {
			if errdefs.IsNotFound(err) || force {
				err := v.cluster.RemoveVolume(vars["name"], force)
				if err != nil {
					return err
				}
			}
		} else {
			return err
		}
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
