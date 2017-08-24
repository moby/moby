package system

import (
	"runtime"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// SupportedPlatforms is a helper function which returns an array of
// supported platforms. This is so that an API client can query the daemon
// via a _ping to determine whether a request is valid or not.
func SupportedPlatforms() []*specs.Platform {
	var platforms []*specs.Platform

	platforms = append(platforms, &specs.Platform{
		Architecture: runtime.GOARCH,
		OS:           runtime.GOOS,
	})

	if LCOWSupported() {
		platforms = append(platforms, &specs.Platform{
			Architecture: runtime.GOARCH,
			OS:           "linux",
		})
	}

	return platforms
}
