package system

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/containerd/log"
	"github.com/golang/gddo/httputil"
	"github.com/moby/moby/api/pkg/authconfig"
	"github.com/moby/moby/api/types"
	buildtypes "github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/daemon/internal/compat"
	"github.com/moby/moby/v2/daemon/internal/filters"
	"github.com/moby/moby/v2/daemon/internal/timestamp"
	"github.com/moby/moby/v2/daemon/internal/versions"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/httputils"
	"github.com/moby/moby/v2/daemon/server/router/build"
	"github.com/moby/moby/v2/pkg/ioutils"
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
	info, _, _ := s.collectSystemInfo.Do(ctx, version, func(ctx context.Context) (*compat.Wrapper, error) {
		info, err := s.backend.SystemInfo(ctx)
		if err != nil {
			return nil, err
		}

		if s.cluster != nil {
			info.Swarm = s.cluster.Info(ctx)
			info.Warnings = append(info.Warnings, info.Swarm.Warnings...)
		}

		var legacyOptions []compat.Option
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
			legacyOptions = append(legacyOptions, compat.WithExtraFields(map[string]any{
				"RegistryConfig": map[string]any{
					"AllowNondistributableArtifactsCIDRs":     json.RawMessage(nil),
					"AllowNondistributableArtifactsHostnames": json.RawMessage(nil),
				},
			}))
		}
		if versions.LessThan(version, "1.49") {
			// FirewallBackend field introduced in API v1.49.
			info.FirewallBackend = nil

			// Expected commits are omitted in API 1.49, but should still be
			// included in older versions.
			legacyOptions = append(legacyOptions, compat.WithExtraFields(map[string]any{
				"ContainerdCommit": map[string]any{"Expected": info.ContainerdCommit.ID},
				"RuncCommit":       map[string]any{"Expected": info.RuncCommit.ID},
				"InitCommit":       map[string]any{"Expected": info.InitCommit.ID},
			}))
		}
		if versions.LessThan(version, "1.50") {
			info.DiscoveredDevices = nil

			// These fields are omitted in > API 1.49, and always false
			// older API versions.
			legacyOptions = append(legacyOptions, compat.WithExtraFields(map[string]any{
				"BridgeNfIptables":  json.RawMessage("false"),
				"BridgeNfIp6tables": json.RawMessage("false"),
			}))
		}
		return compat.Wrap(info, legacyOptions...), nil
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
			switch system.DiskUsageObject(typ) {
			case system.ContainerObject:
				getContainers = true
			case system.ImageObject:
				getImages = true
			case system.VolumeObject:
				getVolumes = true
			case system.BuildCacheObject:
				getBuildCache = true
			default:
				return invalidRequestError{Err: fmt.Errorf("unknown object type: %s", typ)}
			}
		}
	}

	var verbose bool
	if versions.LessThan(version, "1.52") {
		// In versions prior to 1.52, verbose is always true.
		verbose = true
	}
	if v := r.Form.Get("verbose"); v != "" {
		var err error
		verbose, err = strconv.ParseBool(v)
		if err != nil {
			return invalidRequestError{Err: fmt.Errorf("invalid value for verbose: %s", v)}
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	var systemDiskUsage *backend.DiskUsage
	if getContainers || getImages || getVolumes {
		eg.Go(func() error {
			var err error
			systemDiskUsage, err = s.backend.SystemDiskUsage(ctx, backend.DiskUsageOptions{
				Containers: getContainers,
				Images:     getImages,
				Volumes:    getVolumes,
				Verbose:    verbose,
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

	var v system.DiskUsage
	if systemDiskUsage != nil && systemDiskUsage.Images != nil {
		v.ImageUsage = &system.ImagesDiskUsage{
			ActiveImages: systemDiskUsage.Images.ActiveCount,
			Reclaimable:  systemDiskUsage.Images.Reclaimable,
			TotalImages:  systemDiskUsage.Images.TotalCount,
			TotalSize:    systemDiskUsage.Images.TotalSize,
		}

		if versions.LessThan(version, "1.52") {
			v.LayersSize = systemDiskUsage.Images.TotalSize //nolint: staticcheck,SA1019: v.LayersSize is deprecated: kept to maintain backwards compatibility with API < v1.52, use [ImagesDiskUsage.TotalSize] instead.
			v.Images = systemDiskUsage.Images.Items         //nolint: staticcheck,SA1019: v.Images is deprecated: kept to maintain backwards compatibility with API < v1.52, use [ImagesDiskUsage.Items] instead.
		} else if verbose {
			v.ImageUsage.Items = systemDiskUsage.Images.Items
		}
	}
	if systemDiskUsage != nil && systemDiskUsage.Containers != nil {
		v.ContainerUsage = &system.ContainersDiskUsage{
			ActiveContainers: systemDiskUsage.Containers.ActiveCount,
			Reclaimable:      systemDiskUsage.Containers.Reclaimable,
			TotalContainers:  systemDiskUsage.Containers.TotalCount,
			TotalSize:        systemDiskUsage.Containers.TotalSize,
		}

		if versions.LessThan(version, "1.52") {
			v.Containers = systemDiskUsage.Containers.Items //nolint: staticcheck,SA1019: v.Containers is deprecated: kept to maintain backwards compatibility with API < v1.52, use [ContainersDiskUsage.Items] instead.
		} else if verbose {
			v.ContainerUsage.Items = systemDiskUsage.Containers.Items
		}
	}
	if systemDiskUsage != nil && systemDiskUsage.Volumes != nil {
		v.VolumeUsage = &system.VolumesDiskUsage{
			ActiveVolumes: systemDiskUsage.Volumes.ActiveCount,
			TotalSize:     systemDiskUsage.Volumes.TotalSize,
			Reclaimable:   systemDiskUsage.Volumes.Reclaimable,
			TotalVolumes:  systemDiskUsage.Volumes.TotalCount,
		}

		if versions.LessThan(version, "1.52") {
			v.Volumes = systemDiskUsage.Volumes.Items //nolint: staticcheck,SA1019: v.Volumes is deprecated: kept to maintain backwards compatibility with API < v1.52, use [VolumesDiskUsage.Items] instead.
		} else if verbose {
			v.VolumeUsage.Items = systemDiskUsage.Volumes.Items
		}
	}
	if getBuildCache {
		v.BuildCacheUsage = &system.BuildCacheDiskUsage{
			TotalBuildCacheRecords: int64(len(buildCache)),
		}

		activeCount := v.BuildCacheUsage.TotalBuildCacheRecords
		var totalSize, reclaimable int64

		for _, b := range buildCache {
			if versions.LessThan(version, "1.42") {
				totalSize += b.Size
			}

			if !b.InUse {
				activeCount--
			}
			if !b.InUse && !b.Shared {
				reclaimable += b.Size
			}
		}

		v.BuildCacheUsage.ActiveBuildCacheRecords = activeCount
		v.BuildCacheUsage.TotalSize = totalSize
		v.BuildCacheUsage.Reclaimable = reclaimable

		if versions.LessThan(version, "1.52") {
			v.BuildCache = buildCache //nolint: staticcheck,SA1019: v.BuildCache is deprecated: kept to maintain backwards compatibility with API < v1.52, use [BuildCacheDiskUsage.Items] instead.
		} else if verbose {
			v.BuildCacheUsage.Items = buildCache
		}
	}
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

	contentType := httputil.NegotiateContentType(r, []string{
		types.MediaTypeNDJSON,
		types.MediaTypeJSONSequence,
	}, types.MediaTypeJSON) // output isn't actually JSON but API used to  this content-type
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	output.Flush()

	encode := httputils.NewJSONStreamEncoder(output, contentType)

	buffered, l := s.backend.SubscribeToEvents(since, until, ef)
	defer s.backend.UnsubscribeFromEvents(l)

	shouldSkip := func(ev events.Message) bool { return false }
	if versions.LessThan(httputils.VersionFromContext(ctx), "1.46") {
		// Image create events were added in API 1.46
		shouldSkip = func(ev events.Message) bool {
			return ev.Type == events.ImageEventType && ev.Action == events.ActionCreate
		}
	}

	var includeLegacyFields bool
	if versions.LessThan(httputils.VersionFromContext(ctx), "1.52") {
		includeLegacyFields = true
	}

	for _, ev := range buffered {
		if shouldSkip(ev) {
			continue
		}
		if includeLegacyFields {
			if err := encode(backFillLegacy(&ev)); err != nil {
				return err
			}
			continue
		}
		if err := encode(ev); err != nil {
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
			if includeLegacyFields {
				if err := encode(backFillLegacy(&jev)); err != nil {
					return err
				}
				continue
			}
			if err := encode(jev); err != nil {
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
	config, err := authconfig.DecodeRequestBody(r.Body)
	if err != nil {
		return err
	}
	token, err := s.backend.AuthenticateToRegistry(ctx, config)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, &registry.AuthResponse{
		Status:        "Login Succeeded",
		IdentityToken: token,
	})
}

func eventTime(formTime string) (time.Time, error) {
	t, tNano, err := timestamp.ParseTimestamps(formTime, -1)
	if err != nil {
		return time.Time{}, err
	}
	if t == -1 {
		return time.Time{}, nil
	}
	return time.Unix(t, tNano), nil
}

// These fields were deprecated in docker v1.10, API v1.22, but not removed
// from the API responses. Unfortunately, the Docker CLI (and compose indirectly),
// continued using these fields up until v25.0.0, and panic if the fields are
// omitted, or left empty (due to a bug), see: https://github.com/moby/moby/pull/50832#issuecomment-3276600925
func backFillLegacy(ev *events.Message) any {
	switch ev.Type {
	case events.ContainerEventType:
		return compat.Wrap(ev, compat.WithExtraFields(map[string]any{
			"id":     ev.Actor.ID,
			"status": ev.Action,
			"from":   ev.Actor.Attributes["image"],
		}))
	case events.ImageEventType:
		return compat.Wrap(ev, compat.WithExtraFields(map[string]any{
			"id":     ev.Actor.ID,
			"status": ev.Action,
		}))
	default:
		return &ev
	}
}
