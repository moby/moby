package distribution

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim/osversion"
	"github.com/containerd/log"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/docker/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func filterManifests(manifests []manifestlist.ManifestDescriptor, p ocispec.Platform) []manifestlist.ManifestDescriptor {
	version := osversion.Get()
	osVersion := fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.Build)
	log.G(context.TODO()).Debugf("will prefer Windows entries with version %s", osVersion)

	var matches []manifestlist.ManifestDescriptor
	foundWindowsMatch := false
	for _, manifestDescriptor := range manifests {
		skip := func() {
			log.G(context.TODO()).Debugf("ignoring %s/%s %s with media type %s, digest %s", manifestDescriptor.Platform.OS, manifestDescriptor.Platform.Architecture, manifestDescriptor.Platform.OSVersion, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
		}
		// TODO(thaJeztah): should we also check for the user-provided architecture (if any)?
		if manifestDescriptor.Platform.Architecture != runtime.GOARCH {
			skip()
			continue
		}
		os := manifestDescriptor.Platform.OS
		if p.OS != "" {
			// Explicit user request for an OS
			os = p.OS
		}
		if err := image.CheckOS(os); err != nil {
			skip()
			continue
		}
		// TODO(thaJeztah): should we also take the user-provided platform into account (if any)?
		if strings.EqualFold("windows", manifestDescriptor.Platform.OS) {
			if err := checkImageCompatibility("windows", manifestDescriptor.Platform.OSVersion); err != nil {
				skip()
				continue
			}
			foundWindowsMatch = true
		}
		matches = append(matches, manifestDescriptor)
		log.G(context.TODO()).Debugf("found match %s/%s %s with media type %s, digest %s", manifestDescriptor.Platform.OS, runtime.GOARCH, manifestDescriptor.Platform.OSVersion, manifestDescriptor.MediaType, manifestDescriptor.Digest.String())
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
					log.G(context.TODO()).Debug(errMsg)
					return errors.New(errMsg)
				}
			}
		}
	}
	return nil
}
