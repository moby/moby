package image

import (
	"fmt"
	"runtime"
	"strings"
)

func archMatches(arch string) bool {
	// Special case x86_64 as an alias for amd64
	return arch == runtime.GOARCH || (arch == "x86_64" && runtime.GOARCH == "amd64")
}

// ValidateOSCompatibility validates that an image with the given properties can run on this machine.
func ValidateOSCompatibility(os string, arch string, osVersion string, osFeatures []string) error {
	if os != "" && os != runtime.GOOS {
		return fmt.Errorf("image is for OS %s, expected %s", os, runtime.GOOS)
	}
	if arch != "" && !archMatches(arch) {
		return fmt.Errorf("image is for architecture %s, expected %s", arch, runtime.GOARCH)
	}
	if osVersion != "" {
		thisOSVersion := getOSVersion()
		if thisOSVersion != osVersion {
			return fmt.Errorf("image is for OS version '%s', expected '%s'", osVersion, thisOSVersion)
		}
	}
	var missing []string
	for _, f := range osFeatures {
		if !hasOSFeature(f) {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("image requires missing OS features: %s", strings.Join(missing, ", "))
	}
	return nil
}
