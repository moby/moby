/*
 * Copyright (c) 2023. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type Feature string
type Features map[Feature]struct{}

const envNydusDisableTar2Rafs string = "NYDUS_DISABLE_TAR2RAFS"

const (
	// The option `--type tar-rafs` enables converting OCI tar blob
	// stream into nydus blob directly, the tar2rafs eliminates the
	// need to decompress it to a local directory first, thus greatly
	// accelerating the pack process.
	FeatureTar2Rafs Feature = "--type tar-rafs"
	// The option `--batch-size` enables merging multiple small chunks
	// into a big batch chunk, which can reduce the the size of the image
	// and accelerate the runtime file loading.
	FeatureBatchSize Feature = "--batch-size"
	// The option `--encrypt` enables converting directories, tar files
	// or OCI images into encrypted nydus blob.
	FeatureEncrypt Feature = "--encrypt"
)

var requiredFeatures Features
var detectedFeatures Features
var detectFeaturesOnce sync.Once
var disableTar2Rafs = os.Getenv(envNydusDisableTar2Rafs) != ""

func NewFeatures(items ...Feature) Features {
	features := Features{}
	features.Add(items...)
	return features
}

func (features *Features) Add(items ...Feature) {
	for _, item := range items {
		(*features)[item] = struct{}{}
	}
}

func (features *Features) Remove(items ...Feature) {
	for _, item := range items {
		delete(*features, item)
	}
}

func (features *Features) Contains(feature Feature) bool {
	_, ok := (*features)[feature]
	return ok
}

func (features *Features) Equals(other Features) bool {
	if len(*features) != len(other) {
		return false
	}

	for f := range *features {
		if !other.Contains(f) {
			return false
		}
	}

	return true
}

// GetHelp returns the help message of `nydus-image create`.
func GetHelp(builder string) []byte {
	cmd := exec.CommandContext(context.Background(), builder, "create", "-h")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return output
}

// detectFeature returns true if the feature is detected in the help message.
func detectFeature(msg []byte, feature Feature) bool {
	if feature == "" {
		return false
	}

	if strings.Contains(string(msg), string(feature)) {
		return true
	}

	if parts := strings.Split(string(feature), " "); len(parts) == 2 {
		// Check each part of the feature.
		// e.g., "--type tar-rafs" -> ["--type", "tar-rafs"]
		if strings.Contains(string(msg), parts[0]) && strings.Contains(string(msg), parts[1]) {
			return true
		}
	}

	return false
}

// DetectFeatures returns supported feature list from required feature list.
// The supported feature list is detected from the help message of `nydus-image create`.
func DetectFeatures(builder string, required Features, getHelp func(string) []byte) (Features, error) {
	detectFeaturesOnce.Do(func() {
		requiredFeatures = required
		detectedFeatures = Features{}

		helpMsg := getHelp(builder)

		for feature := range required {
			// The feature is supported by current version of nydus-image.
			supported := detectFeature(helpMsg, feature)
			if supported {
				// It is an experimental feature, so we still provide an env
				// variable to allow users to disable it.
				if feature == FeatureTar2Rafs && disableTar2Rafs {
					logrus.Warnf("the feature '%s' is disabled by env '%s'", FeatureTar2Rafs, envNydusDisableTar2Rafs)
					continue
				}
				detectedFeatures.Add(feature)
			} else {
				logrus.Warnf("the feature '%s' is ignored, it requires higher version of nydus-image", feature)
			}
		}
	})

	// Return Error if required features changed in different calls.
	if !requiredFeatures.Equals(required) {
		return nil, fmt.Errorf("features changed: %v -> %v", requiredFeatures, required)
	}

	return detectedFeatures, nil
}
