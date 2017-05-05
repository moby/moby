package dockerfile

import (
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/pkg/errors"
)

type pathCache interface {
	Load(key interface{}) (value interface{}, ok bool)
	Store(key, value interface{})
}

// imageContexts is a helper for stacking up built image rootfs and reusing
// them as contexts
type imageContexts struct {
	list   []*imageMount
	byName map[string]*imageMount
	cache  pathCache
}

func (ic *imageContexts) add(name string, im *imageMount) error {
	if len(name) > 0 {
		if ic.byName == nil {
			ic.byName = make(map[string]*imageMount)
		}
		if _, ok := ic.byName[name]; ok {
			return errors.Errorf("duplicate name %s", name)
		}
		ic.byName[name] = im
	}
	ic.list = append(ic.list, im)
	return nil
}

func (ic *imageContexts) update(imageID string, runConfig *container.Config) {
	ic.list[len(ic.list)-1].update(imageID, runConfig)
}

func (ic *imageContexts) validate(i int) error {
	if i < 0 || i >= len(ic.list)-1 {
		if i == len(ic.list)-1 {
			return errors.New("refers to current build stage")
		}
		return errors.New("index out of bounds")
	}
	return nil
}

func (ic *imageContexts) getMount(indexOrName string) (*imageMount, error) {
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
	return nil, nil
}

func (ic *imageContexts) unmount() (retErr error) {
	for _, iml := range append([][]*imageMount{}, ic.list, ic.implicitMounts) {
		for _, im := range iml {
			if err := im.unmount(); err != nil {
				logrus.Error(err)
				retErr = err
			}
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

// TODO: remove getCache/setCache from imageContexts
func (ic *imageContexts) getCache(id, path string) (interface{}, bool) {
	if ic.cache != nil {
		if id == "" {
			return nil, false
		}
		return ic.cache.Load(id + path)
	}
	return nil, false
}

func (ic *imageContexts) setCache(id, path string, v interface{}) {
	if ic.cache != nil {
		ic.cache.Store(id+path, v)
	}
}

// imageMount is a reference to an image that can be used as a builder.Source
type imageMount struct {
	id        string
	source    builder.Source
	runConfig *container.Config
	layer     builder.ReleaseableLayer
}

func newImageMount(image builder.Image, layer builder.ReleaseableLayer) *imageMount {
	im := &imageMount{layer: layer}
	if image != nil {
		im.update(image.ImageID(), image.RunConfig())
	}
	return im
}

func (im *imageMount) context() (builder.Source, error) {
	if im.source == nil {
		if im.id == "" || im.layer == nil {
			return nil, errors.Errorf("empty context")
		}
		mountPath, err := im.layer.Mount(im.id)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to mount %s", im.id)
		}
		source, err := remotecontext.NewLazyContext(mountPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create lazycontext for %s", mountPath)
		}
		im.source = source
	}
	return im.source, nil
}

func (im *imageMount) unmount() error {
	if im.layer == nil {
		return nil
	}
	if err := im.layer.Release(); err != nil {
		return errors.Wrapf(err, "failed to unmount previous build image %s", im.id)
	}
	return nil
}

func (im *imageMount) update(imageID string, runConfig *container.Config) {
	im.id = imageID
	im.runConfig = runConfig
}

func (im *imageMount) ImageID() string {
	return im.id
}

func (im *imageMount) RunConfig() *container.Config {
	return im.runConfig
}
