package system // import "github.com/docker/docker/api/server/router/system"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router/build"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	timetypes "github.com/docker/docker/api/types/time"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func optionsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.WriteHeader(http.StatusOK)
	return nil
}

func (s *systemRouter) pingHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")

	builderVersion := build.BuilderVersion(*s.features)
	if bv := builderVersion; bv != "" {
		w.Header().Set("Builder-Version", string(bv))
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", "0")
		return nil
	}
	_, err := w.Write([]byte{'O', 'K'})
	return err
}

func (s *systemRouter) getInfo(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	info := s.backend.SystemInfo()

	if s.cluster != nil {
		info.Swarm = s.cluster.Info()
		info.Warnings = append(info.Warnings, info.Swarm.Warnings...)
	}

	if versions.LessThan(httputils.VersionFromContext(ctx), "1.25") {
		// TODO: handle this conversion in engine-api
		type oldInfo struct {
			*types.Info
			ExecutionDriver string
		}
		old := &oldInfo{
			Info:            info,
			ExecutionDriver: "<not supported>",
		}
		nameOnlySecurityOptions := []string{}
		kvSecOpts, err := types.DecodeSecurityOptions(old.SecurityOptions)
		if err != nil {
			return err
		}
		for _, s := range kvSecOpts {
			nameOnlySecurityOptions = append(nameOnlySecurityOptions, s.Name)
		}
		old.SecurityOptions = nameOnlySecurityOptions
		return httputils.WriteJSON(w, http.StatusOK, old)
	}
	if versions.LessThan(httputils.VersionFromContext(ctx), "1.39") {
		if info.KernelVersion == "" {
			info.KernelVersion = "<unknown>"
		}
		if info.OperatingSystem == "" {
			info.OperatingSystem = "<unknown>"
		}
	}
	return httputils.WriteJSON(w, http.StatusOK, info)
}

func (s *systemRouter) getVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	info := s.backend.SystemVersion()

	return httputils.WriteJSON(w, http.StatusOK, info)
}

func (s *systemRouter) getDiskUsage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	version := httputils.VersionFromContext(ctx)

	var getContainers, getImages, getVolumes, getBuildCache bool
	typeStrs, ok := r.Form["type"]
	if versions.LessThan(version, "1.42") || !ok {
		getContainers, getImages, getVolumes, getBuildCache = true, true, true, true
	} else {
		for _, typ := range typeStrs {
			switch types.DiskUsageObject(typ) {
			case types.ContainerObject:
				getContainers = true
			case types.ImageObject:
				getImages = true
			case types.VolumeObject:
				getVolumes = true
			case types.BuildCacheObject:
				getBuildCache = true
			default:
				return invalidRequestError{Err: fmt.Errorf("unknown object type: %s", typ)}
			}
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	var systemDiskUsage *types.DiskUsage
	if getContainers || getImages || getVolumes {
		eg.Go(func() error {
			var err error
			systemDiskUsage, err = s.backend.SystemDiskUsage(ctx, DiskUsageOptions{
				Containers: getContainers,
				Images:     getImages,
				Volumes:    getVolumes,
			})
			return err
		})
	}

	var buildCache []*types.BuildCache
	if getBuildCache {
		eg.Go(func() error {
			var err error
			buildCache, err = s.builder.DiskUsage(ctx)
			if err != nil {
				return errors.Wrap(err, "error getting build cache usage")
			}
			if buildCache == nil {
				// Ensure empty `BuildCache` field is represented as empty JSON array(`[]`)
				// instead of `null` to be consistent with `Images`, `Containers` etc.
				buildCache = []*types.BuildCache{}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	var builderSize int64
	if versions.LessThan(version, "1.42") {
		for _, b := range buildCache {
			builderSize += b.Size
		}
	}

	du := types.DiskUsage{
		BuildCache:  buildCache,
		BuilderSize: builderSize,
	}
	if systemDiskUsage != nil {
		du.LayersSize = systemDiskUsage.LayersSize
		du.Images = systemDiskUsage.Images
		du.Containers = systemDiskUsage.Containers
		du.Volumes = systemDiskUsage.Volumes
	}
	return httputils.WriteJSON(w, http.StatusOK, du)
}

type invalidRequestError struct {
	Err error
}

func (e invalidRequestError) Error() string {
	return e.Err.Error()
}

func (e invalidRequestError) InvalidParameter() {}

func (s *systemRouter) getEvents(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	since, err := eventTime(r.Form.Get("since"))
	if err != nil {
		return err
	}
	until, err := eventTime(r.Form.Get("until"))
	if err != nil {
		return err
	}

	var (
		timeout        <-chan time.Time
		onlyPastEvents bool
	)
	if !until.IsZero() {
		if until.Before(since) {
			return invalidRequestError{fmt.Errorf("`since` time (%s) cannot be after `until` time (%s)", r.Form.Get("since"), r.Form.Get("until"))}
		}

		now := time.Now()

		onlyPastEvents = until.Before(now)

		if !onlyPastEvents {
			dur := until.Sub(now)
			timer := time.NewTimer(dur)
			defer timer.Stop()
			timeout = timer.C
		}
	}

	ef, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	output.Flush()

	enc := json.NewEncoder(output)

	buffered, l := s.backend.SubscribeToEvents(since, until, ef)
	defer s.backend.UnsubscribeFromEvents(l)

	for _, ev := range buffered {
		if err := enc.Encode(ev); err != nil {
			return err
		}
	}

	if onlyPastEvents {
		return nil
	}

	for {
		select {
		case ev := <-l:
			jev, ok := ev.(events.Message)
			if !ok {
				logrus.Warnf("unexpected event message: %q", ev)
				continue
			}
			if err := enc.Encode(jev); err != nil {
				return err
			}
		case <-timeout:
			return nil
		case <-ctx.Done():
			logrus.Debug("Client context cancelled, stop sending events")
			return nil
		}
	}
}

func (s *systemRouter) postAuth(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var config *types.AuthConfig
	err := json.NewDecoder(r.Body).Decode(&config)
	r.Body.Close()
	if err != nil {
		return err
	}
	status, token, err := s.backend.AuthenticateToRegistry(ctx, config)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &registry.AuthenticateOKBody{
		Status:        status,
		IdentityToken: token,
	})
}

func eventTime(formTime string) (time.Time, error) {
	t, tNano, err := timetypes.ParseTimestamps(formTime, -1)
	if err != nil {
		return time.Time{}, err
	}
	if t == -1 {
		return time.Time{}, nil
	}
	return time.Unix(t, tNano), nil
}
