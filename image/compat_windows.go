package image

import (
	"fmt"

	"github.com/docker/docker/pkg/system"
)

// Windows OS features
const (
	FeatureWin32k = "win32k" // The kernel windowing stack is required
)

func getOSVersion() string {
	v := system.GetOSVersion()
	return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.Build)
}

func hasOSFeature(f string) bool {
	switch f {
	case FeatureWin32k:
		return system.HasWin32KSupport()
	default:
		// Unrecognized feature.
		return false
	}
}
