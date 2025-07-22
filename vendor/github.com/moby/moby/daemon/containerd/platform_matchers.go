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

// platformMatcherWithRequestedPlatform is a platform matcher that also
// contains the platform that was requested by the user in the context
// in which the matcher was created.
type platformMatcherWithRequestedPlatform struct {
	Requested *ocispec.Platform

	platforms.MatchComparer
}

type matchComparerProvider func(ocispec.Platform) platforms.MatchComparer

func (i *ImageService) matchRequestedOrDefault(
	fpm matchComparerProvider, // function to create a platform matcher if platform is not nil
	platform *ocispec.Platform, // input platform, nil if not specified
) platformMatcherWithRequestedPlatform {
	var inner platforms.MatchComparer
	if platform == nil {
		inner = matchAllWithPreference(i.hostPlatformMatcher())
	} else {
		inner = fpm(*platform)
	}

	return platformMatcherWithRequestedPlatform{
		Requested:     platform,
		MatchComparer: inner,
	}
}

func (i *ImageService) hostPlatformMatcher() platforms.MatchComparer {
	// Allow to override the host platform for testing purposes.
	if i.defaultPlatformOverride != nil {
		return i.defaultPlatformOverride
	}
	return platforms.Default()
}
