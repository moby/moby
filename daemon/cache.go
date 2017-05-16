package daemon

import (
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image/cache"
)

// MakeImageCache creates a stateful image cache.
func (daemon *Daemon) MakeImageCache(sourceRefs []string) builder.ImageCache {
	if len(sourceRefs) == 0 {
		// TODO @jhowardmsft LCOW. For now, assume it is the OS of the host
		return cache.NewLocal(daemon.stores[runtime.GOOS].imageStore)
	}

	// TODO @jhowardmsft LCOW. For now, assume it is the OS of the host
	cache := cache.New(daemon.stores[runtime.GOOS].imageStore)

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
