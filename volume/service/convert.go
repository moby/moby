package service

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/volume"
	"github.com/sirupsen/logrus"
)

// convertOpts are used to pass options to `volumeToAPI`
type convertOpt interface {
	isConvertOpt()
}

type useCachedPath bool

func (useCachedPath) isConvertOpt() {}

type calcSize bool

func (calcSize) isConvertOpt() {}

type pathCacher interface {
	CachedPath() string
}

func (s *VolumesService) volumesToAPI(ctx context.Context, volumes []volume.Volume, opts ...convertOpt) []*types.Volume {
	var (
		out        = make([]*types.Volume, 0, len(volumes))
		getSize    bool
		cachedPath bool
	)

	for _, o := range opts {
		switch t := o.(type) {
		case calcSize:
			getSize = bool(t)
		case useCachedPath:
			cachedPath = bool(t)
		}
	}
	for _, v := range volumes {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		apiV := volumeToAPIType(v)

		if cachedPath {
			if vv, ok := v.(pathCacher); ok {
				apiV.Mountpoint = vv.CachedPath()
			}
		} else {
			apiV.Mountpoint = v.Path()
		}

		if getSize {
			p := v.Path()
			if apiV.Mountpoint == "" {
				apiV.Mountpoint = p
			}
			sz, err := directory.Size(ctx, p)
			if err != nil {
				logrus.WithError(err).WithField("volume", v.Name()).Warnf("Failed to determine size of volume")
				sz = -1
			}
			apiV.UsageData = &types.VolumeUsageData{Size: sz, RefCount: int64(s.vs.CountReferences(v))}
		}

		out = append(out, &apiV)
	}
	return out
}

func volumeToAPIType(v volume.Volume) types.Volume {
	createdAt, _ := v.CreatedAt()
	tv := types.Volume{
		Name:      v.Name(),
		Driver:    v.DriverName(),
		CreatedAt: createdAt.Format(time.RFC3339),
	}
	if v, ok := v.(volume.DetailedVolume); ok {
		tv.Labels = v.Labels()
		tv.Options = v.Options()
		tv.Scope = v.Scope()
	}
	if cp, ok := v.(pathCacher); ok {
		tv.Mountpoint = cp.CachedPath()
	}
	return tv
}

func filtersToBy(filter filters.Args, acceptedFilters map[string]bool) (By, error) {
	if err := filter.Validate(acceptedFilters); err != nil {
		return nil, err
	}
	var bys []By
	if drivers := filter.Get("driver"); len(drivers) > 0 {
		bys = append(bys, ByDriver(drivers...))
	}
	if filter.Contains("name") {
		bys = append(bys, CustomFilter(func(v volume.Volume) bool {
			return filter.Match("name", v.Name())
		}))
	}
	bys = append(bys, byLabelFilter(filter))

	if filter.Contains("dangling") {
		var dangling bool
		if filter.ExactMatch("dangling", "true") || filter.ExactMatch("dangling", "1") {
			dangling = true
		} else if !filter.ExactMatch("dangling", "false") && !filter.ExactMatch("dangling", "0") {
			return nil, invalidFilter{"dangling", filter.Get("dangling")}
		}
		bys = append(bys, ByReferenced(!dangling))
	}

	var by By
	switch len(bys) {
	case 0:
	case 1:
		by = bys[0]
	default:
		by = And(bys...)
	}
	return by, nil
}
