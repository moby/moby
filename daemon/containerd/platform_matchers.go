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

type matchComparerProvider func(ocispec.Platform) platforms.MatchComparer

func (i *ImageService) matchRequestedOrDefault(
	fpm matchComparerProvider, // function to create a platform matcher if platform is not nil
	platform *ocispec.Platform, // input platform, nil if not specified
) platforms.MatchComparer {
	if platform == nil {
		return matchAllWithPreference(i.hostPlatformMatcher())
	}
	return fpm(*platform)
}

func (i *ImageService) hostPlatformMatcher() platforms.MatchComparer {
	// Allow to override the host platform for testing purposes.
	if i.defaultPlatformOverride != nil {
		return i.defaultPlatformOverride
	}
	return platforms.Default()
}
