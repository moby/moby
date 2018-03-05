package distribution // import "github.com/docker/docker/distribution"

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/pkg/system"
	"github.com/sirupsen/logrus"
)

var _ distribution.Describable = &v2LayerDescriptor{}

func (ld *v2LayerDescriptor) Descriptor() distribution.Descriptor {
	if ld.src.MediaType == schema2.MediaTypeForeignLayer && len(ld.src.URLs) > 0 {
		return ld.src
	}
	return distribution.Descriptor{}
}

func (ld *v2LayerDescriptor) open(ctx context.Context) (distribution.ReadSeekCloser, error) {
	blobs := ld.repo.Blobs(ctx)
	rsc, err := blobs.Open(ctx, ld.digest)

	if len(ld.src.URLs) == 0 {
		return rsc, err
	}

	// We're done if the registry has this blob.
	if err == nil {
		// Seek does an HTTP GET.  If it succeeds, the blob really is accessible.
		if _, err = rsc.Seek(0, os.SEEK_SET); err == nil {
			return rsc, nil
		}
		rsc.Close()
	}

	// Find the first URL that results in a 200 result code.
	for _, url := range ld.src.URLs {
		logrus.Debugf("Pulling %v from foreign URL %v", ld.digest, url)
		rsc = transport.NewHTTPReadSeeker(http.DefaultClient, url, nil)

		// Seek does an HTTP GET.  If it succeeds, the blob really is accessible.
		_, err = rsc.Seek(0, os.SEEK_SET)
		if err == nil {
			break
		}
		logrus.Debugf("Download for %v failed: %v", ld.digest, err)
		rsc.Close()
		rsc = nil
	}
	return rsc, err
}

func filterManifests(manifests []manifestlist.ManifestDescriptor, os string) []manifestlist.ManifestDescriptor {
	osVersion := ""
	if os == "windows" {
		version := system.GetOSVersion()
		osVersion = fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.Build)
		logrus.Debugf("will prefer entries with version %s", osVersion)
	}

	var matches []manifestlist.ManifestDescriptor
	for _, manifestDescriptor := range manifests {
		if manifestDescriptor.Platform.Architecture == runtime.GOARCH && manifestDescriptor.Platform.OS == os {
			matches = append(matches, manifestDescriptor)
			logrus.Debugf("found match for %s/%s %s with media type %s, digest %s", os, runtime.GOARCH, manifestDescriptor.Platform.OSVersion, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
		} else {
			logrus.Debugf("ignoring %s/%s %s with media type %s, digest %s", os, runtime.GOARCH, manifestDescriptor.Platform.OSVersion, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
		}
	}
	if os == "windows" {
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
	return versionMatch(mbv.list[i].Platform.OSVersion, mbv.version) && !versionMatch(mbv.list[j].Platform.OSVersion, mbv.version)
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
		hostOSV := system.GetOSVersion()
		splitImageOSVersion := strings.Split(imageOSVersion, ".") // eg 10.0.16299.nnnn
		if len(splitImageOSVersion) >= 3 {
			if imageOSBuild, err := strconv.Atoi(splitImageOSVersion[2]); err == nil {
				if imageOSBuild > int(hostOSV.Build) {
					errMsg := fmt.Sprintf("a Windows version %s.%s.%s-based image is incompatible with a %s host", splitImageOSVersion[0], splitImageOSVersion[1], splitImageOSVersion[2], hostOSV.ToString())
					logrus.Debugf(errMsg)
					return errors.New(errMsg)
				}
			}
		}
	}
	return nil
}
