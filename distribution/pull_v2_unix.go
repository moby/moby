//go:build !windows
// +build !windows

package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"sort"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

func (ld *layerDescriptor) open(ctx context.Context) (distribution.ReadSeekCloser, error) {
	blobs := ld.repo.Blobs(ctx)
	return blobs.Open(ctx, ld.digest)
}

func filterManifests(manifests []manifestlist.ManifestDescriptor, p ocispec.Platform) []manifestlist.ManifestDescriptor {
	p = platforms.Normalize(withDefault(p))
	m := platforms.Only(p)
	var matches []manifestlist.ManifestDescriptor
	for _, desc := range manifests {
		descP := toOCIPlatform(desc.Platform)
		if descP == nil || m.Match(*descP) {
			matches = append(matches, desc)
			if descP != nil {
				logrus.Debugf("found match for %s with media type %s, digest %s", platforms.Format(p), desc.MediaType, desc.Digest.String())
			}
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		p1 := toOCIPlatform(matches[i].Platform)
		if p1 == nil {
			return false
		}
		p2 := toOCIPlatform(matches[j].Platform)
		if p2 == nil {
			return true
		}
		return m.Less(*p1, *p2)
	})

	return matches
}

// checkImageCompatibility is a Windows-specific function. No-op on Linux
func checkImageCompatibility(imageOS, imageOSVersion string) error {
	return nil
}

func withDefault(p ocispec.Platform) ocispec.Platform {
	def := maximumSpec()
	if p.OS == "" {
		p.OS = def.OS
	}
	if p.Architecture == "" {
		p.Architecture = def.Architecture
		p.Variant = def.Variant
	}
	return p
}

func formatPlatform(platform ocispec.Platform) string {
	if platform.OS == "" {
		platform = platforms.DefaultSpec()
	}
	return platforms.Format(platform)
}
