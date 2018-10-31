package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"sync"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/digestset"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/builder"
	buildcache "github.com/docker/docker/image/cache"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type cachedImage struct {
	id     digest.Digest
	parent digest.Digest

	// Mutable values
	m          sync.Mutex
	references []reference.Named
	children   []digest.Digest
}

type cache struct {
	m sync.RWMutex
	// idCache maps Docker identifiers
	idCache map[digest.Digest]*cachedImage
	// dCache maps target digests to images
	tCache map[digest.Digest]*cachedImage
	ids    *digestset.Set
}

func (c *cache) byID(id digest.Digest) *cachedImage {
	c.m.RLock()
	img, ok := c.idCache[id]
	c.m.RUnlock()
	if !ok {
		return nil
	}
	return img
}

func (c *cache) byTarget(target digest.Digest) *cachedImage {
	c.m.RLock()
	img, ok := c.tCache[target]
	c.m.RUnlock()
	if !ok {
		return nil
	}
	return img
}

// LoadCache loads the image cache by scanning containerd images
// and listening to containerd events
// This process can be backgrounded and will hold hold the cache
// lock until fully populated.
func (i *ImageService) LoadCache(ctx context.Context) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	_, err = i.loadNSCache(ctx, namespace)
	return err
}

func (i *ImageService) loadNSCache(ctx context.Context, namespace string) (*cache, error) {
	i.cacheL.Lock()
	defer i.cacheL.Unlock()

	c := &cache{
		idCache: map[digest.Digest]*cachedImage{},
		tCache:  map[digest.Digest]*cachedImage{},
		ids:     digestset.NewSet(),
	}

	is := i.client.ImageService()

	// TODO(containerd): This must use some streaming approach
	imgs, err := is.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, img := range imgs {
		ref, err := reference.Parse(img.Name)
		if err != nil {
			log.G(ctx).WithError(err).WithField("name", img.Name).Debug("skipping invalid image name")
			continue
		}

		named, hasName := ref.(reference.Named)

		ci := c.tCache[img.Target.Digest]
		if ci == nil {
			var id digest.Digest
			if !hasName {
				digested, ok := ref.(reference.Digested)
				if ok {
					id = digested.Digest()
				}
			}
			if img.Target.MediaType == images.MediaTypeDockerSchema2Config || img.Target.MediaType == ocispec.MediaTypeImageConfig {
				id = img.Target.Digest

			}
			if id == "" {
				idstr, ok := img.Labels[LabelImageID]
				if !ok {
					cs := i.client.ContentStore()
					// TODO(containerd): resolve architecture from context
					platform := platforms.Default()
					desc, err := images.Config(ctx, cs, img.Target, platform)
					if err != nil {
						log.G(ctx).WithError(err).WithField("name", img.Name).Debug("TODO: no label")
						continue
					}
					id = desc.Digest
				} else {
					id, err = digest.Parse(idstr)
					if err != nil {
						log.G(ctx).WithError(err).WithField("name", img.Name).Debug("skipping invalid image id label")
						continue
					}
				}
			}

			ci = c.idCache[id]
			if ci == nil {
				ci = &cachedImage{
					id: id,
				}
				if s := img.Labels[LabelImageParent]; s != "" {
					pid, err := digest.Parse(s)
					if err != nil {
						log.G(ctx).WithError(err).WithField("name", img.Name).Debug("skipping invalid parent label")
					} else {
						ci.parent = pid
					}
				}

				c.idCache[id] = ci
			}
			c.tCache[img.Target.Digest] = ci
		}
		if hasName {
			ci.addReference(named)
		}
	}
	i.cache[namespace] = c

	return c, nil
}

func (ci *cachedImage) addReference(named reference.Named) {
	var (
		i int
		s = named.String()
	)

	// Add references, add in sorted place
	for ; i < len(ci.references); i++ {
		if rs := ci.references[i].String(); s < rs {
			ci.references = append(ci.references, nil)
			copy(ci.references[i+1:], ci.references[i:])
			ci.references[i] = named
			break
		} else if rs == s {
			break
		}
	}
	if i == len(ci.references) {
		ci.references = append(ci.references, named)
	}
}

func (i *ImageService) getCache(ctx context.Context) (c *cache, err error) {
	namespace, ok := namespaces.Namespace(ctx)
	if !ok {
		// Default namespace
		// TODO(containerd): define default in service
		namespace = ""
	}
	i.cacheL.RLock()
	c, ok = i.cache[namespace]
	i.cacheL.RUnlock()
	if !ok {
		c, err = i.loadNSCache(ctx, namespace)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

// MakeImageCache creates a stateful image cache for build.
func (i *ImageService) MakeImageCache(sourceRefs []string) builder.ImageCache {
	if len(sourceRefs) == 0 {
		return buildcache.NewLocal(i.imageStore)
	}

	cache := buildcache.New(i.imageStore)

	for _, ref := range sourceRefs {
		img, err := i.GetImage(ref)
		if err != nil {
			logrus.Warnf("Could not look up %s for cache resolution, skipping: %+v", ref, err)
			continue
		}
		cache.Populate(img)
	}

	return cache
}
