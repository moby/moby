package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim/osversion"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/pkg/system"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ distribution.Describable = &layerDescriptor{}

func (ld *layerDescriptor) Descriptor() distribution.Descriptor {
	if ld.src.MediaType == schema2.MediaTypeForeignLayer && len(ld.src.URLs) > 0 {
		return ld.src
	}
	return distribution.Descriptor{}
}

func (ld *layerDescriptor) open(ctx context.Context) (distribution.ReadSeekCloser, error) {
	blobs := ld.repo.Blobs(ctx)
	rsc, err := blobs.Open(ctx, ld.digest)

	if len(ld.src.URLs) == 0 {
		return rsc, err
	}

	// We're done if the registry has this blob.
	if err == nil {
		// Seek does an HTTP GET.  If it succeeds, the blob really is accessible.
		if _, err = rsc.Seek(0, io.SeekStart); err == nil {
			return rsc, nil
		}
		rsc.Close()
	}

	// Find the first URL that results in a 200 result code.
	for _, url := range ld.src.URLs {
		log.G(ctx).Debugf("Pulling %v from foreign URL %v", ld.digest, url)
		rsc = transport.NewHTTPReadSeeker(http.DefaultClient, url, nil)

		// Seek does an HTTP GET.  If it succeeds, the blob really is accessible.
		_, err = rsc.Seek(0, io.SeekStart)
		if err == nil {
			break
		}
		log.G(ctx).Debugf("Download for %v failed: %v", ld.digest, err)
		rsc.Close()
		rsc = nil
	}
	return rsc, err
}

func filterManifests(manifests []manifestlist.ManifestDescriptor, p ocispec.Platform) []manifestlist.ManifestDescriptor {
	version := osversion.Get()
	osVersion := fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.Build)
	log.G(context.TODO()).Debugf("will prefer Windows entries with version %s", osVersion)

	var matches []manifestlist.ManifestDescriptor
	foundWindowsMatch := false
	for _, manifestDescriptor := range manifests {
		if (manifestDescriptor.Platform.Architecture == runtime.GOARCH) &&
			((p.OS != "" && manifestDescriptor.Platform.OS == p.OS) || // Explicit user request for an OS we know we support
				(p.OS == "" && system.IsOSSupported(manifestDescriptor.Platform.OS))) { // No user requested OS, but one we can support
			if strings.EqualFold("windows", manifestDescriptor.Platform.OS) {
				if err := checkImageCompatibility("windows", manifestDescriptor.Platform.OSVersion); err != nil {
					continue
				}
				foundWindowsMatch = true
			}
			matches = append(matches, manifestDescriptor)
			log.G(context.TODO()).Debugf("found match %s/%s %s with media type %s, digest %s", manifestDescriptor.Platform.OS, runtime.GOARCH, manifestDescriptor.Platform.OSVersion, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
		} else {
			log.G(context.TODO()).Debugf("ignoring %s/%s %s with media type %s, digest %s", manifestDescriptor.Platform.OS, manifestDescriptor.Platform.Architecture, manifestDescriptor.Platform.OSVersion, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
		}
	}
	if foundWindowsMatch {
		sort.Stable(manifestsByVersion{osVersion, matches})
	}
	return matches
}

func versionMatch(actual, expected string) bool {
	// Check whether the version matches up to the build, ignoring UBR
	return strings.HasPrefix(actual, expected+".")
}

type manifestsByVersion struct {
	version string
	list    []manifestlist.ManifestDescriptor
}

func (mbv manifestsByVersion) Less(i, j int) bool {
	// TODO: Split version by parts and compare
	// TODO: Prefer versions which have a greater version number
	// Move compatible versions to the top, with no other ordering changes
	return (strings.EqualFold("windows", mbv.list[i].Platform.OS) && !strings.EqualFold("windows", mbv.list[j].Platform.OS)) ||
		(versionMatch(mbv.list[i].Platform.OSVersion, mbv.version) && !versionMatch(mbv.list[j].Platform.OSVersion, mbv.version))
}

func (mbv manifestsByVersion) Len() int {
	return len(mbv.list)
}

func (mbv manifestsByVersion) Swap(i, j int) {
	mbv.list[i], mbv.list[j] = mbv.list[j], mbv.list[i]
}

// checkImageCompatibility blocks pulling incompatible images based on a later OS build
// Fixes https://github.com/moby/moby/issues/36184.
func checkImageCompatibility(imageOS, imageOSVersion string) error {
	if imageOS == "windows" {
		hostOSV := osversion.Get()
		splitImageOSVersion := strings.Split(imageOSVersion, ".") // eg 10.0.16299.nnnn
		if len(splitImageOSVersion) >= 3 {
			if imageOSBuild, err := strconv.Atoi(splitImageOSVersion[2]); err == nil {
				if imageOSBuild > int(hostOSV.Build) {
					errMsg := fmt.Sprintf("a Windows version %s.%s.%s-based image is incompatible with a %s host", splitImageOSVersion[0], splitImageOSVersion[1], splitImageOSVersion[2], hostOSV.ToString())
					log.G(context.TODO()).Debugf(errMsg)
					return errors.New(errMsg)
				}
			}
		}
	}
	return nil
}

func formatPlatform(platform ocispec.Platform) string {
	if platform.OS == "" {
		platform = platforms.DefaultSpec()
	}
	return fmt.Sprintf("%s %s", platforms.Format(platform), osversion.Get().ToString())
}
