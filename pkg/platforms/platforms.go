package platforms

import (
	cplatforms "github.com/containerd/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type allPlatformsWithPreferenceMatcher struct {
	preferred cplatforms.MatchComparer
}

// AllPlatformsWithPreference will return a platform matcher that matches all
// platforms but will order platforms matching the preferred matcher first.
func AllPlatformsWithPreference(preferred cplatforms.MatchComparer) cplatforms.MatchComparer {
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
