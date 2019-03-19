package daemon

import (
	"github.com/containerd/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type osMatcher string

func (os osMatcher) Match(p ocispec.Platform) bool {
	return p.OS == string(os)
}

func matchOS(p ocispec.Platform) platforms.Matcher {
	return osMatcher(p.OS)
}
