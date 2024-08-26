package containerd

import (
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type allPlatformsWithPreferenceMatcher struct {
	preferred platforms.MatchComparer
}

// matchAllWithPreference will return a platform matcher that matches all
// platforms but will order platforms matching the preferred matcher first.
func matchAllWithPreference(preferred platforms.MatchComparer) platforms.MatchComparer {
	return allPlatformsWithPreferenceMatcher{
		preferred: preferred,
	}
}

func (c allPlatformsWithPreferenceMatcher) Match(_ ocispec.Platform) bool {
	return true
}

func (c allPlatformsWithPreferenceMatcher) Less(p1, p2 ocispec.Platform) bool {
	return c.preferred.Less(p1, p2)
}

func (i *ImageService) hostPlatformMatcher() platforms.MatchComparer {
	// Allow to override the host platform for testing purposes.
	if i.defaultPlatformOverride != nil {
		return i.defaultPlatformOverride
	}
	return platforms.Default()
}
