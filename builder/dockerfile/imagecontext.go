package dockerfile

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/pkg/errors"
)

// imageContexts is a helper for stacking up built image rootfs and reusing
// them as contexts
type imageContexts struct {
	b      *Builder
	list   []*imageMount
	byName map[string]int
	cache  *pathCache
}

type imageMount struct {
	id      string
	ctx     builder.Context
	release func() error
}

func (ic *imageContexts) new(name string) error {
	if len(name) > 0 {
		if ic.byName == nil {
			ic.byName = make(map[string]int)
		}
		if _, ok := ic.byName[name]; ok {
			return errors.Errorf("duplicate name %s", name)
		}
		ic.byName[name] = len(ic.list)
	}
	ic.list = append(ic.list, &imageMount{})
	return nil
}

func (ic *imageContexts) update(imageID string) {
	ic.list[len(ic.list)-1].id = imageID
}

func (ic *imageContexts) validate(i int) error {
	if i < 0 || i >= len(ic.list)-1 {
		var extraMsg string
		if i == len(ic.list)-1 {
			extraMsg = " refers current build block"
		}
		return errors.Errorf("invalid from flag value %d%s", i, extraMsg)
	}
	return nil
}

func (ic *imageContexts) context(i int) (builder.Context, error) {
	if err := ic.validate(i); err != nil {
		return nil, err
	}
	im := ic.list[i]
	if im.ctx == nil {
		if im.id == "" {
			return nil, errors.Errorf("could not copy from empty context")
		}
		p, release, err := ic.b.docker.MountImage(im.id)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to mount %s", im.id)
		}
		ctx, err := remotecontext.NewLazyContext(p)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create lazycontext for %s", p)
		}
		logrus.Debugf("mounted image: %s %s", im.id, p)
		im.release = release
		im.ctx = ctx
	}
	return im.ctx, nil
}

func (ic *imageContexts) unmount() (retErr error) {
	for _, im := range ic.list {
		if im.release != nil {
			if err := im.release(); err != nil {
				logrus.Error(errors.Wrapf(err, "failed to unmount previous build image"))
				retErr = err
			}
		}
	}
	return
}

func (ic *imageContexts) getCache(i int, path string) (interface{}, bool) {
	if ic.cache != nil {
		im := ic.list[i]
		if im.id == "" {
			return nil, false
		}
		return ic.cache.get(im.id + path)
	}
	return nil, false
}

func (ic *imageContexts) setCache(i int, path string, v interface{}) {
	if ic.cache != nil {
		ic.cache.set(ic.list[i].id+path, v)
	}
}

type pathCache struct {
	mu    sync.Mutex
	items map[string]interface{}
}

func (c *pathCache) set(k string, v interface{}) {
	c.mu.Lock()
	if c.items == nil {
		c.items = make(map[string]interface{})
	}
	c.items[k] = v
	c.mu.Unlock()
}

func (c *pathCache) get(k string) (interface{}, bool) {
	c.mu.Lock()
	if c.items == nil {
		c.mu.Unlock()
		return nil, false
	}
	v, ok := c.items[k]
	c.mu.Unlock()
	return v, ok
}
