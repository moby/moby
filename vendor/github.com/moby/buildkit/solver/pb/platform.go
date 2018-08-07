package pb

import (
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func (p *Platform) Spec() specs.Platform {
	return specs.Platform{
		OS:           p.OS,
		Architecture: p.Architecture,
		Variant:      p.Variant,
		OSVersion:    p.OSVersion,
		OSFeatures:   p.OSFeatures,
	}
}

func PlatformFromSpec(p specs.Platform) Platform {
	return Platform{
		OS:           p.OS,
		Architecture: p.Architecture,
		Variant:      p.Variant,
		OSVersion:    p.OSVersion,
		OSFeatures:   p.OSFeatures,
	}
}

func ToSpecPlatforms(p []Platform) []specs.Platform {
	out := make([]specs.Platform, 0, len(p))
	for _, pp := range p {
		out = append(out, pp.Spec())
	}
	return out
}

func PlatformsFromSpec(p []specs.Platform) []Platform {
	out := make([]Platform, 0, len(p))
	for _, pp := range p {
		out = append(out, PlatformFromSpec(pp))
	}
	return out
}
