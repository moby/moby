package verifier

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/solver/result"
	"github.com/pkg/errors"
)

func CheckInvalidPlatforms[T comparable](ctx context.Context, res *result.Result[T]) ([]client.VertexWarning, error) {
	req, err := getRequestOpts(res)
	if err != nil {
		return nil, err
	}

	if req.Request != "" {
		return nil, nil
	}

	if _, ok := res.Metadata[exptypes.ExporterPlatformsKey]; len(res.Refs) > 0 && !ok {
		return nil, errors.Errorf("build result contains multiple refs without platforms mapping")
	}

	isMap := len(res.Refs) > 0

	ps, err := exptypes.ParsePlatforms(res.Metadata)
	if err != nil {
		return nil, err
	}

	warnings := []client.VertexWarning{}
	reqMap := map[string]struct{}{}
	reqList := []exptypes.Platform{}

	for _, v := range req.Platforms {
		p, err := platforms.Parse(v)
		if err != nil {
			warnings = append(warnings, client.VertexWarning{
				Short: []byte(fmt.Sprintf("Invalid platform result requested %q: %s", v, err.Error())),
			})
		}
		p = platforms.Normalize(p)
		_, ok := reqMap[platforms.Format(p)]
		if ok {
			warnings = append(warnings, client.VertexWarning{
				Short: []byte(fmt.Sprintf("Duplicate platform result requested %q", v)),
			})
		}
		reqMap[platforms.Format(p)] = struct{}{}
		reqList = append(reqList, exptypes.Platform{Platform: p})
	}

	if len(warnings) > 0 {
		return warnings, nil
	}

	if len(reqMap) == 1 && len(ps.Platforms) == 1 {
		pp := platforms.Normalize(ps.Platforms[0].Platform)
		if _, ok := reqMap[platforms.Format(pp)]; !ok {
			return []client.VertexWarning{{
				Short: []byte(fmt.Sprintf("Requested platform %q does not match result platform %q", req.Platforms[0], platforms.Format(pp))),
			}}, nil
		}
		return nil, nil
	}

	if !isMap && len(reqMap) > 1 {
		return []client.VertexWarning{{
			Short: []byte("Multiple platforms requested but result is not multi-platform"),
		}}, nil
	}

	mismatch := len(reqMap) != len(ps.Platforms)

	if !mismatch {
		for _, p := range ps.Platforms {
			pp := platforms.Normalize(p.Platform)
			if _, ok := reqMap[platforms.Format(pp)]; !ok {
				mismatch = true
				break
			}
		}
	}

	if mismatch {
		return []client.VertexWarning{{
			Short: []byte(fmt.Sprintf("Requested platforms %s do not match result platforms %s", platformsString(reqList), platformsString(ps.Platforms))),
		}}, nil
	}

	return nil, nil
}

func platformsString(ps []exptypes.Platform) string {
	var ss []string
	for _, p := range ps {
		ss = append(ss, platforms.Format(platforms.Normalize(p.Platform)))
	}
	sort.Strings(ss)
	return strings.Join(ss, ",")
}
