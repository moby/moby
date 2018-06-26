// +build !windows

package distribution // import "github.com/docker/docker/distribution"

import (
	"context"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

func (ld *v2LayerDescriptor) open(ctx context.Context) (distribution.ReadSeekCloser, error) {
	blobs := ld.repo.Blobs(ctx)
	return blobs.Open(ctx, ld.digest)
}

func filterManifests(manifests []manifestlist.ManifestDescriptor, p specs.Platform) []manifestlist.ManifestDescriptor {
	p = withDefault(p)
	var matches []manifestlist.ManifestDescriptor
	for _, desc := range manifests {
		if compareNormalized(toOCIPlatform(desc.Platform), p) {
			matches = append(matches, desc)
			logrus.Debugf("found match for %s with media type %s, digest %s", platforms.Format(p), desc.MediaType, desc.Digest.String())
		}
	}

	// deprecated: backwards compatibility with older versions that didn't compare variant
	if len(matches) == 0 && p.Architecture == "arm" {
		p = normalize(p)
		for _, desc := range manifests {
			if desc.Platform.OS == p.OS && desc.Platform.Architecture == p.Architecture {
				matches = append(matches, desc)
				logrus.Debugf("found deprecated partial match for %s with media type %s, digest %s", platforms.Format(p), desc.MediaType, desc.Digest.String())
			}
		}
	}

	return matches
}

// checkImageCompatibility is a Windows-specific function. No-op on Linux
func checkImageCompatibility(imageOS, imageOSVersion string) error {
	return nil
}

func withDefault(p specs.Platform) specs.Platform {
	def := platforms.DefaultSpec()
	if p.OS == "" {
		p.OS = def.OS
	}
	if p.Architecture == "" {
		p.Architecture = def.Architecture
		p.Variant = def.Variant
	}
	return p
}

func compareNormalized(p1, p2 specs.Platform) bool {
	// remove after https://github.com/containerd/containerd/pull/2414
	return p1.OS == p2.OS &&
		p1.Architecture == p2.Architecture &&
		p1.Variant == p2.Variant
}

func normalize(p specs.Platform) specs.Platform {
	p = platforms.Normalize(p)
	// remove after https://github.com/containerd/containerd/pull/2414
	if p.Architecture == "arm" {
		if p.Variant == "" {
			p.Variant = "v7"
		}
	}
	if p.Architecture == "arm64" {
		if p.Variant == "" {
			p.Variant = "v8"
		}
	}
	return p
}
