// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package system // import "github.com/docker/docker/api/server/router/system"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router/build"
	"github.com/docker/docker/api/types"
	buildtypes "github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	timetypes "github.com/docker/docker/api/types/time"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func optionsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.WriteHeader(http.StatusOK)
	return nil
}

func (s *systemRouter) pingHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")

	builderVersion := build.BuilderVersion(s.features())
	if bv := builderVersion; bv != "" {
		w.Header().Set("Builder-Version", string(bv))
	}

	w.Header().Set("Swarm", s.swarmStatus())

	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", "0")
		return nil
	}
	_, err := w.Write([]byte{'O', 'K'})
	return err
}

func (s *systemRouter) swarmStatus() string {
	if s.cluster != nil {
		if p, ok := s.cluster.(StatusProvider); ok {
			return p.Status()
		}
	}
	return string(swarm.LocalNodeStateInactive)
}

func (s *systemRouter) getInfo(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	version := httputils.VersionFromContext(ctx)
	info, _, _ := s.collectSystemInfo.Do(ctx, version, func(ctx context.Context) (*infoResponse, error) {
		info, err := s.backend.SystemInfo(ctx)
		if err != nil {
			return nil, err
		}

		if s.cluster != nil {
			info.Swarm = s.cluster.Info(ctx)
			info.Warnings = append(info.Warnings, info.Swarm.Warnings...)
		}

		if versions.LessThan(version, "1.25") {
			// TODO: handle this conversion in engine-api
			kvSecOpts, err := system.DecodeSecurityOptions(info.SecurityOptions)
			if err != nil {
				info.Warnings = append(info.Warnings, err.Error())
			}
			var nameOnly []string
			for _, so := range kvSecOpts {
				nameOnly = append(nameOnly, so.Name)
			}
			info.SecurityOptions = nameOnly
		}
		if versions.LessThan(version, "1.39") {
			if info.KernelVersion == "" {
				info.KernelVersion = "<unknown>"
			}
			if info.OperatingSystem == "" {
				info.OperatingSystem = "<unknown>"
			}
		}
		if versions.LessThan(version, "1.44") {
			for k, rt := range info.Runtimes {
				// Status field introduced in API v1.44.
				info.Runtimes[k] = system.RuntimeWithStatus{Runtime: rt.Runtime}
			}
		}
		if versions.LessThan(version, "1.46") {
			// Containerd field introduced in API v1.46.
			info.Containerd = nil
		}
		if versions.LessThan(version, "1.47") {
			// Field is omitted in API 1.48 and up, but should still be included
			// in older versions, even if no values are set.
			info.RegistryConfig.ExtraFields = map[string]any{
				"AllowNondistributableArtifactsCIDRs":     json.RawMessage(nil),
				"AllowNondistributableArtifactsHostnames": json.RawMessage(nil),
			}
		}
		if versions.LessThan(version, "1.49") {
			// FirewallBackend field introduced in API v1.49.
			info.FirewallBackend = nil
		}

		extraFields := map[string]any{}
		if versions.LessThan(version, "1.49") {
			// Expected commits are omitted in API 1.49, but should still be
			// included in older versions.
			info.ContainerdCommit.Expected = info.ContainerdCommit.ID //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.49.
			info.RuncCommit.Expected = info.RuncCommit.ID             //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.49.
			info.InitCommit.Expected = info.InitCommit.ID             //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.49.
		}
		if versions.GreaterThanOrEqualTo(version, "1.42") {
			info.KernelMemory = false
		}
		if versions.LessThan(version, "1.50") {
			info.DiscoveredDevices = nil

			// These fields are omitted in > API 1.49, and always false
			// older API versions.
			extraFields = map[string]any{
				"BridgeNfIptables":  json.RawMessage("false"),
				"BridgeNfIp6tables": json.RawMessage("false"),
			}
		}
		return &infoResponse{Info: info, extraFields: extraFields}, nil
	})

	return httputils.WriteJSON(w, http.StatusOK, info)
}

func (s *systemRouter) getVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	info, err := s.backend.SystemVersion(ctx)
	if err != nil {
		return err
	}

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
		getContainers, getImages, getVolumes, getBuildCache = true, true, true, s.builder != nil
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

	var systemDiskUsage *system.DiskUsage
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

	var buildCache []*buildtypes.CacheRecord
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
				buildCache = []*buildtypes.CacheRecord{}
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
			// Parents field was added in API 1.42 to replace the Parent field.
			b.Parents = nil
		}
	}
	if versions.GreaterThanOrEqualTo(version, "1.42") {
		for _, b := range buildCache {
			// Parent field is deprecated in API v1.42 and up, as it is deprecated
			// in BuildKit. Empty the field to omit it in the API response.
			b.Parent = "" //nolint:staticcheck // ignore SA1019 (Parent field is deprecated)
		}
	}
	if versions.LessThan(version, "1.44") && systemDiskUsage != nil && systemDiskUsage.Images != nil {
		for _, b := range systemDiskUsage.Images.Items {
			b.VirtualSize = b.Size //nolint:staticcheck // ignore SA1019: field is deprecated, but still set on API < v1.44.
		}
	}

	du := system.DiskUsage{}
	if getBuildCache {
		du.BuildCache = &buildtypes.CacheDiskUsage{
			TotalSize: builderSize,
			Items:     buildCache,
		}
	}
	if systemDiskUsage != nil {
		du.Images = systemDiskUsage.Images
		du.Containers = systemDiskUsage.Containers
		du.Volumes = systemDiskUsage.Volumes
	}

	// Use the old struct for the API return value.
	var v types.DiskUsage
	if du.Images != nil {
		v.LayersSize = du.Images.TotalSize
		v.Images = du.Images.Items
	}
	if du.Containers != nil {
		v.Containers = du.Containers.Items
	}
	if du.Volumes != nil {
		v.Volumes = du.Volumes.Items
	}
	if du.BuildCache != nil {
		v.BuildCache = du.BuildCache.Items
	}
	v.BuilderSize = builderSize
	return httputils.WriteJSON(w, http.StatusOK, v)
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
	w.WriteHeader(http.StatusOK)
	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	output.Flush()

	enc := json.NewEncoder(output)

	buffered, l := s.backend.SubscribeToEvents(since, until, ef)
	defer s.backend.UnsubscribeFromEvents(l)

	shouldSkip := func(ev events.Message) bool { return false }
	if versions.LessThan(httputils.VersionFromContext(ctx), "1.46") {
		// Image create events were added in API 1.46
		shouldSkip = func(ev events.Message) bool {
			return ev.Type == "image" && ev.Action == "create"
		}
	}

	for _, ev := range buffered {
		if shouldSkip(ev) {
			continue
		}
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
				log.G(ctx).Warnf("unexpected event message: %q", ev)
				continue
			}
			if shouldSkip(jev) {
				continue
			}
			if err := enc.Encode(jev); err != nil {
				return err
			}
		case <-timeout:
			return nil
		case <-ctx.Done():
			log.G(ctx).Debug("Client context cancelled, stop sending events")
			return nil
		}
	}
}

func (s *systemRouter) postAuth(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var config *registry.AuthConfig
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
