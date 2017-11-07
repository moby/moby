package dockerfile

import (
	"runtime"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/remotecontext"
	dockerimage "github.com/docker/docker/image"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type getAndMountFunc func(string, bool) (builder.Image, builder.ReleaseableLayer, error)

// imageSources mounts images and provides a cache for mounted images. It tracks
// all images so they can be unmounted at the end of the build.
type imageSources struct {
	byImageID map[string]*imageMount
	mounts    []*imageMount
	getImage  getAndMountFunc
}

func newImageSources(ctx context.Context, options builderOptions) *imageSources {
	getAndMount := func(idOrRef string, localOnly bool) (builder.Image, builder.ReleaseableLayer, error) {
		pullOption := backend.PullOptionNoPull
		if !localOnly {
			if options.Options.PullParent {
				pullOption = backend.PullOptionForcePull
			} else {
				pullOption = backend.PullOptionPreferLocal
			}
		}
		optionsPlatform := system.ParsePlatform(options.Options.Platform)
		return options.Backend.GetImageAndReleasableLayer(ctx, idOrRef, backend.GetImageAndLayerOptions{
			PullOption: pullOption,
			AuthConfig: options.Options.AuthConfigs,
			Output:     options.ProgressWriter.Output,
			OS:         optionsPlatform.OS,
		})
	}

	return &imageSources{
		byImageID: make(map[string]*imageMount),
		getImage:  getAndMount,
	}
}

func (m *imageSources) Get(idOrRef string, localOnly bool) (*imageMount, error) {
	if im, ok := m.byImageID[idOrRef]; ok {
		return im, nil
	}

	image, layer, err := m.getImage(idOrRef, localOnly)
	if err != nil {
		return nil, err
	}
	im := newImageMount(image, layer)
	m.Add(im)
	return im, nil
}

func (m *imageSources) Unmount() (retErr error) {
	for _, im := range m.mounts {
		if err := im.unmount(); err != nil {
			logrus.Error(err)
			retErr = err
		}
	}
	return
}

func (m *imageSources) Add(im *imageMount) {
	switch im.image {
	case nil:
		// set the OS for scratch images
		os := runtime.GOOS
		// Windows does not support scratch except for LCOW
		if runtime.GOOS == "windows" {
			os = "linux"
		}
		im.image = &dockerimage.Image{V1Image: dockerimage.V1Image{OS: os}}
	default:
		m.byImageID[im.image.ImageID()] = im
	}
	m.mounts = append(m.mounts, im)
}

// imageMount is a reference to an image that can be used as a builder.Source
type imageMount struct {
	image  builder.Image
	source builder.Source
	layer  builder.ReleaseableLayer
}

func newImageMount(image builder.Image, layer builder.ReleaseableLayer) *imageMount {
	im := &imageMount{image: image, layer: layer}
	return im
}

func (im *imageMount) Source() (builder.Source, error) {
	if im.source == nil {
		if im.layer == nil {
			return nil, errors.Errorf("empty context")
		}
		mountPath, err := im.layer.Mount()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to mount %s", im.image.ImageID())
		}
		source, err := remotecontext.NewLazySource(mountPath)
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
		return errors.Wrapf(err, "failed to unmount previous build image %s", im.image.ImageID())
	}
	im.layer = nil
	return nil
}

func (im *imageMount) Image() builder.Image {
	return im.image
}

func (im *imageMount) Layer() builder.ReleaseableLayer {
	return im.layer
}

func (im *imageMount) ImageID() string {
	return im.image.ImageID()
}
