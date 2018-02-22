package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image/cache"
	"github.com/sirupsen/logrus"
)

// MakeImageCache creates a stateful image cache.
func (daemon *Daemon) MakeImageCache(sourceRefs []string) builder.ImageCache {
	if len(sourceRefs) == 0 {
		return cache.NewLocal(daemon.imageStore)
	}

	cache := cache.New(daemon.imageStore)

	for _, ref := range sourceRefs {
		img, err := daemon.GetImage(ref)
		if err != nil {
			logrus.Warnf("Could not look up %s for cache resolution, skipping: %+v", ref, err)
			continue
		}
		cache.Populate(img)
	}

	return cache
}
