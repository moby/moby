/*
 * Copyright (c) 2023. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package tool

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

const envNydusDisableTar2Rafs = "NYDUS_DISABLE_TAR2RAFS"

var currentVersion string
var currentVersionDetectOnce sync.Once
var disableTar2Rafs = os.Getenv(envNydusDisableTar2Rafs) != ""

const (
	// The option `--type tar-rafs` enables converting OCI tar blob
	// stream into nydus blob directly, the tar2rafs eliminates the
	// need to decompress it to a local directory first, thus greatly
	// accelerating the pack process.
	FeatureTar2Rafs Feature = "--type tar-rafs"
)

var featureMap = map[Feature]string{
	FeatureTar2Rafs: "v2.2",
}

type Feature string
type Features []Feature

func (features *Features) Contains(feature Feature) bool {
	for _, feat := range *features {
		if feat == feature {
			return true
		}
	}
	return false
}

func (features *Features) Remove(feature Feature) {
	found := -1
	for idx, feat := range *features {
		if feat == feature {
			found = idx
			break
		}
	}
	if found != -1 {
		*features = append((*features)[:found], (*features)[found+1:]...)
	}
}

func detectVersion(msg []byte) string {
	re := regexp.MustCompile(`Version:\s*v*(\d+.\d+.\d+)`)
	matches := re.FindSubmatch(msg)
	if len(matches) > 1 {
		return string(matches[1])
	}
	return ""
}

// DetectFeatures returns supported feature list from required feature list.
func DetectFeatures(builder string, required Features) Features {
	currentVersionDetectOnce.Do(func() {
		if required.Contains(FeatureTar2Rafs) && disableTar2Rafs {
			logrus.Warnf("the feature '%s' is disabled by env '%s'", FeatureTar2Rafs, envNydusDisableTar2Rafs)
		}

		cmd := exec.CommandContext(context.Background(), builder, "--version")
		output, err := cmd.Output()
		if err != nil {
			return
		}

		currentVersion = detectVersion(output)
	})

	if currentVersion == "" {
		return Features{}
	}

	detectedFeatures := Features{}
	for _, feature := range required {
		requiredVersion := featureMap[feature]
		if requiredVersion == "" {
			detectedFeatures = append(detectedFeatures, feature)
			continue
		}

		// The feature is supported by current version
		supported := semver.Compare(requiredVersion, "v"+currentVersion) <= 0
		if supported {
			// It is an experimental feature, so we still provide an env
			// variable to allow users to disable it.
			if feature == FeatureTar2Rafs && disableTar2Rafs {
				continue
			}
			detectedFeatures = append(detectedFeatures, feature)
		}
	}

	return detectedFeatures
}
