// +build !windows

package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"runtime"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/sirupsen/logrus"
)

func (ld *v2LayerDescriptor) open(ctx context.Context) (distribution.ReadSeekCloser, error) {
	blobs := ld.repo.Blobs(ctx)
	return blobs.Open(ctx, ld.digest)
}

func filterManifests(manifests []manifestlist.ManifestDescriptor, os string) []manifestlist.ManifestDescriptor {
	var matches []manifestlist.ManifestDescriptor
	arch := runtime.GOARCH
	if arch == "arm" || arch == "arm64" {
		for _, manifestDescriptor := range manifests {
			if manifestDescriptor.Platform.Architecture == arch && manifestDescriptor.Platform.OS == os && manifestDescriptor.Platform.Variant == GOARM {
				matches = append(matches, manifestDescriptor)

				logrus.Debugf("found match for %s/%s/%s with media type %s, digest %s", os, arch, GOARM, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
			}
		}
		if matches == nil { //failure mode
			order := make(chan string)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go getOrderOfCompatibility(ctx, arch, GOARM, order)
			for {
				o, ok := <-order
				if ok {
					for _, manifestDescriptor := range manifests {
						if manifestDescriptor.Platform.Architecture == arch && manifestDescriptor.Platform.OS == os && manifestDescriptor.Platform.Variant == o {
							matches = append(matches, manifestDescriptor)

							logrus.Debugf("found compatible match for %s/%s/%s with media type %s, digest %s", os, arch, o, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
						}
					}
					if matches != nil {
						cancel()
						break
					}
				} else {
					break
				}
			}
		}
	} else {
		for _, manifestDescriptor := range manifests {
			if manifestDescriptor.Platform.Architecture == arch && manifestDescriptor.Platform.OS == os {
				matches = append(matches, manifestDescriptor)

				logrus.Debugf("found match for %s/%s with media type %s, digest %s", os, arch, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
			}
		}
	}
	return matches
}

// checkImageCompatibility is a Windows-specific function. No-op on Linux
func checkImageCompatibility(imageOS, imageOSVersion string) error {
	return nil
}
