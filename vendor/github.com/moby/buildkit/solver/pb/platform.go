package pb

import (
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func (p *Platform) Spec() ocispecs.Platform {
	result := ocispecs.Platform{
		OS:           p.OS,
		Architecture: p.Architecture,
		Variant:      p.Variant,
		OSVersion:    p.OSVersion,
	}
	if p.OSFeatures != nil {
		result.OSFeatures = append([]string{}, p.OSFeatures...)
	}
	return result
}

func PlatformFromSpec(p ocispecs.Platform) *Platform {
	result := &Platform{
		OS:           p.OS,
		Architecture: p.Architecture,
		Variant:      p.Variant,
		OSVersion:    p.OSVersion,
	}
	if p.OSFeatures != nil {
		result.OSFeatures = append([]string{}, p.OSFeatures...)
	}
	return result
}

func ToSpecPlatforms(p []*Platform) []ocispecs.Platform {
	out := make([]ocispecs.Platform, 0, len(p))
	for _, pp := range p {
		out = append(out, pp.Spec())
	}
	return out
}

func PlatformsFromSpec(p []ocispecs.Platform) []*Platform {
	out := make([]*Platform, 0, len(p))
	for _, pp := range p {
		out = append(out, PlatformFromSpec(pp))
	}
	return out
}
