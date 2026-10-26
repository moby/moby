package platform

import (
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/tonistiigi/go-archvariant"
)

// MaximumSpec returns the host platform with the highest CPU variant
// supported by the host.
func MaximumSpec() ocispec.Platform {
	p := platforms.DefaultSpec()
	switch p.Architecture {
	case "amd64":
		p.Variant = archvariant.AMD64Variant()
	case "arm64":
		if v := arm64Variant(); v != "" {
			p.Variant = v
		}
	}
	return p
}
