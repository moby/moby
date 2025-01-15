package exptypes

import (
	"encoding/json"
	"fmt"

	"github.com/containerd/platforms"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func ParsePlatforms(meta map[string][]byte) (Platforms, error) {
	if platformsBytes, ok := meta[ExporterPlatformsKey]; ok {
		var ps Platforms
		if len(platformsBytes) > 0 {
			if err := json.Unmarshal(platformsBytes, &ps); err != nil {
				return Platforms{}, errors.Wrapf(err, "failed to parse platforms passed to provenance processor")
			}
		}
		if len(ps.Platforms) == 0 {
			return Platforms{}, errors.Errorf("invalid empty platforms index for exporter")
		}
		for i, p := range ps.Platforms {
			if p.ID == "" {
				return Platforms{}, errors.Errorf("invalid empty platform key for exporter")
			}
			if p.Platform.OS == "" || p.Platform.Architecture == "" {
				return Platforms{}, errors.Errorf("invalid platform value %v for exporter", p.Platform)
			}
			ps.Platforms[i].Platform = platforms.Normalize(p.Platform)
		}
		return ps, nil
	}

	p := platforms.DefaultSpec()
	if imgConfig, ok := meta[ExporterImageConfigKey]; ok {
		var img ocispecs.Image
		err := json.Unmarshal(imgConfig, &img)
		if err != nil {
			return Platforms{}, err
		}

		if img.OS != "" && img.Architecture != "" {
			p = ocispecs.Platform{
				Architecture: img.Architecture,
				OS:           img.OS,
				OSVersion:    img.OSVersion,
				OSFeatures:   img.OSFeatures,
				Variant:      img.Variant,
			}
		} else if img.OS != "" || img.Architecture != "" {
			return Platforms{}, errors.Errorf("invalid image config: os and architecture must be specified together")
		}
	}
	p = platforms.Normalize(p)
	pk := platforms.FormatAll(p)
	ps := Platforms{
		Platforms: []Platform{{ID: pk, Platform: p}},
	}
	return ps, nil
}

func ParseKey(meta map[string][]byte, key string, p *Platform) []byte {
	if p != nil {
		if v, ok := meta[fmt.Sprintf("%s/%s", key, p.ID)]; ok {
			return v
		}
	}
	if v, ok := meta[key]; ok {
		return v
	}
	return nil
}
