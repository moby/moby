package containerd

import (
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// platformsWithPreferenceMatcher is a platform matcher that matches any of the
// platforms in the platformList, but orders platforms to match the preferred matcher
// first. If the platformList is empty, it matches all platforms.
// It implements the platforms.MatchComparer interface.
type platformsWithPreferenceMatcher struct {
	platformList []ocispec.Platform
	preferred    platforms.MatchComparer
}

func matchAnyWithPreference(preferred platforms.MatchComparer, platformList []ocispec.Platform) platformsWithPreferenceMatcher {
	return platformsWithPreferenceMatcher{
		platformList: platformList,
		preferred:    preferred,
	}
}

func (c platformsWithPreferenceMatcher) Match(p ocispec.Platform) bool {
	if len(c.platformList) == 0 {
		return true
	}
	return platforms.Any(c.platformList...).Match(p)
}

func (c platformsWithPreferenceMatcher) Less(p1, p2 ocispec.Platform) bool {
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

// matchRequestedOrDefault returns a platform match comparer that matches the given platform
// using the given match comparer. If no platform is given, matches any platform with
// preference for the host platform.
func (i *ImageService) matchRequestedOrDefault(
	fpm matchComparerProvider, // function to create a platform matcher if platform is not nil
	platform *ocispec.Platform, // input platform, nil if not specified
) platformMatcherWithRequestedPlatform {
	var inner platforms.MatchComparer
	if platform == nil {
		inner = matchAnyWithPreference(i.hostPlatformMatcher(), nil)
	} else {
		inner = fpm(*platform)
	}

	return platformMatcherWithRequestedPlatform{
		Requested:     platform,
		MatchComparer: inner,
	}
}

// hostPlatformMatcher returns a platform match comparer that matches the host platform.
func (i *ImageService) hostPlatformMatcher() platforms.MatchComparer {
	// Allow to override the host platform for testing purposes.
	if i.defaultPlatformOverride != nil {
		return i.defaultPlatformOverride
	}
	return platforms.Default()
}
