package oci

import (
	"runtime"

	"github.com/docker/docker/libcontainerd/windowsoci"
)

// DefaultSpec returns default spec used by docker.
func DefaultSpec() windowsoci.Spec {
	return windowsoci.Spec{
		Version: windowsoci.Version,
		Platform: windowsoci.Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Windows: &windowsoci.Windows{},
	}
}
