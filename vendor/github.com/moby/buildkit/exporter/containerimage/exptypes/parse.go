package exptypes

import (
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/platforms"
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
		}
	}
	p = platforms.Normalize(p)
	pk := platforms.Format(p)
	ps := Platforms{
		Platforms: []Platform{{ID: pk, Platform: p}},
	}
	return ps, nil
}

func ParseKey(meta map[string][]byte, key string, p Platform) []byte {
	if v, ok := meta[fmt.Sprintf("%s/%s", key, p.ID)]; ok {
		return v
	} else if v, ok := meta[key]; ok {
		return v
	}
	return nil
}
