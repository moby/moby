package client

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types/versions"
	"github.com/pkg/errors"
)

// fallbackAPIVersion is the version to fallback to if API-version negotiation
// fails. This version is the highest version of the API before API-version
// negotiation was introduced. If negotiation fails (or no API version was
// included in the API response), we assume the API server uses the most
// recent version before negotiation was introduced.
const fallbackAPIVersion = "1.24"

// defaultMinimumAPIVersion is the default minimum API version supported
// by the client. It can be overridden through the "DOCKER_MIN_API_VERSION"
// environment variable. The minimum allowed version is determined
// by [minimumAPIVersion].
const defaultMinimumAPIVersion = fallbackAPIVersion

// minimumAPIVersion represents Minimum REST API version supported by the client.
const minimumAPIVersion = "1.12"

// ValidateMinAPIVersion verifies if the given API version is within the
// range supported by the daemon. It is used to validate a custom minimum
// API version set through [EnvOverrideMinAPIVersion].
func ValidateMinAPIVersion(ver string) error {
	if ver == "" {
		return errors.New(`value is empty`)
	}
	if strings.EqualFold(ver[0:1], "v") {
		return errors.New(`API version must be provided without "v" prefix`)
	}
	if versions.LessThan(ver, minimumAPIVersion) {
		return errors.Errorf(`minimum supported API version is %s: %s`, minimumAPIVersion, ver)
	}
	if versions.GreaterThan(ver, api.DefaultVersion) {
		return errors.Errorf(`maximum supported API version is %s: %s`, api.DefaultVersion, ver)
	}
	return nil
}

// getMinAPIVersion returns the minimum API version provided by the client.
// It defaults to [defaultMinimumAPIVersion], but can be overridden through
// the [EnvOverrideMinAPIVersion] environment variable.
func getMinAPIVersion() (string, error) {
	minVer := defaultMinimumAPIVersion
	if ver := os.Getenv(EnvOverrideMinAPIVersion); ver != "" {
		if err := ValidateMinAPIVersion(ver); err != nil {
			return minVer, fmt.Errorf("invalid %s: %w", EnvOverrideMinAPIVersion, err)
		}
		minVer = ver
	}
	return minVer, nil
}
