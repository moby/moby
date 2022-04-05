package imagerefchecker

import (
	"context"

	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/moby/buildkit/cache"
	"github.com/opencontainers/go-digest"
)

// LayerGetter abstracts away the snapshotter
type LayerGetter interface {
	GetLayer(string) (layer.Layer, error)
}

// Opt represents the options needed to create a refchecker
type Opt struct {
	LayerGetter LayerGetter
	ImageStore  image.Store
}

// New creates new image reference checker that can be used to see if a reference
// is being used by any of the images in the image store
func New(opt Opt) cache.ExternalRefCheckerFunc {
	return func() (cache.ExternalRefChecker, error) {
		c := &checker{opt: opt, layers: lchain{}, cache: map[string]bool{}}
		if err := c.init(context.TODO()); err != nil {
			return nil, err
		}
		return c, nil
	}
}

type lchain map[layer.DiffID]lchain

func (c lchain) add(ids []layer.DiffID) {
	if len(ids) == 0 {
		return
	}
	id := ids[0]
	ch, ok := c[id]
	if !ok {
		ch = lchain{}
		c[id] = ch
	}
	ch.add(ids[1:])
}

func (c lchain) has(ids []layer.DiffID) bool {
	if len(ids) == 0 {
		return true
	}
	ch, ok := c[ids[0]]
	return ok && ch.has(ids[1:])
}

type checker struct {
	opt    Opt
	layers lchain
	cache  map[string]bool
}

func (c *checker) Exists(key string, chain []digest.Digest) bool {
	if c.opt.ImageStore == nil {
		return false
	}

	if b, ok := c.cache[key]; ok {
		return b
	}

	l, err := c.opt.LayerGetter.GetLayer(key)
	if err != nil || l == nil {
		c.cache[key] = false
		return false
	}

	ok := c.layers.has(diffIDs(l))
	c.cache[key] = ok
	return ok
}

func (c *checker) init(ctx context.Context) error {
	imgs, err := c.opt.ImageStore.Map(ctx)
	if err != nil {
		return err
	}

	for _, img := range imgs {
		c.layers.add(img.RootFS.DiffIDs)
	}
	return nil
}

func diffIDs(l layer.Layer) []layer.DiffID {
	p := l.Parent()
	if p == nil {
		return []layer.DiffID{l.DiffID()}
	}
	return append(diffIDs(p), l.DiffID())
}
