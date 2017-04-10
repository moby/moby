package dockerfile

import (
	"strconv"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/pkg/errors"
)

// imageContexts is a helper for stacking up built image rootfs and reusing
// them as contexts
type imageContexts struct {
	b           *Builder
	list        []*imageMount
	byName      map[string]*imageMount
	cache       *pathCache
	currentName string
}

func (ic *imageContexts) new(name string, increment bool) (*imageMount, error) {
	im := &imageMount{ic: ic}
	if len(name) > 0 {
		if ic.byName == nil {
			ic.byName = make(map[string]*imageMount)
		}
		if _, ok := ic.byName[name]; ok {
			return nil, errors.Errorf("duplicate name %s", name)
		}
		ic.byName[name] = im
	}
	if increment {
		ic.list = append(ic.list, im)
	}
	ic.currentName = name
	return im, nil
}

func (ic *imageContexts) update(imageID string, runConfig *container.Config) {
	ic.list[len(ic.list)-1].id = imageID
	ic.list[len(ic.list)-1].runConfig = runConfig
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

func (ic *imageContexts) get(indexOrName string) (*imageMount, error) {
	index, err := strconv.Atoi(indexOrName)
	if err == nil {
		if err := ic.validate(index); err != nil {
			return nil, err
		}
		return ic.list[index], nil
	}
	if im, ok := ic.byName[strings.ToLower(indexOrName)]; ok {
		return im, nil
	}
	im, err := mountByRef(ic.b, indexOrName)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid from flag value %s", indexOrName)
	}
	return im, nil
}

func (ic *imageContexts) unmount() (retErr error) {
	for _, im := range ic.list {
		if err := im.unmount(); err != nil {
			logrus.Error(err)
			retErr = err
		}
	}
	for _, im := range ic.byName {
		if err := im.unmount(); err != nil {
			logrus.Error(err)
			retErr = err
		}
	}
	return
}

func (ic *imageContexts) isCurrentTarget(target string) bool {
	if target == "" {
		return false
	}
	return strings.EqualFold(ic.currentName, target)
}

func (ic *imageContexts) getCache(id, path string) (interface{}, bool) {
	if ic.cache != nil {
		if id == "" {
			return nil, false
		}
		return ic.cache.get(id + path)
	}
	return nil, false
}

func (ic *imageContexts) setCache(id, path string, v interface{}) {
	if ic.cache != nil {
		ic.cache.set(id+path, v)
	}
}

// imageMount is a reference for getting access to a buildcontext that is backed
// by an existing image
type imageMount struct {
	id        string
	ctx       builder.Context
	release   func() error
	ic        *imageContexts
	runConfig *container.Config
}

func (im *imageMount) context() (builder.Context, error) {
	if im.ctx == nil {
		if im.id == "" {
			return nil, errors.Errorf("could not copy from empty context")
		}
		p, release, err := im.ic.b.docker.MountImage(im.id)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to mount %s", im.id)
		}
		ctx, err := remotecontext.NewLazyContext(p)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create lazycontext for %s", p)
		}
		im.release = release
		im.ctx = ctx
	}
	return im.ctx, nil
}

func (im *imageMount) unmount() error {
	if im.release != nil {
		if err := im.release(); err != nil {
			return errors.Wrapf(err, "failed to unmount previous build image %s", im.id)
		}
		im.release = nil
	}
	return nil
}

func (im *imageMount) ImageID() string {
	return im.id
}
func (im *imageMount) RunConfig() *container.Config {
	return im.runConfig
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
