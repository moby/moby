package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/containerd/containerd/platforms"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	registrypkg "github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type roLayer struct {
	released   bool
	layerStore layer.Store
	roLayer    layer.Layer
}

func (l *roLayer) ContentStoreDigest() digest.Digest {
	return ""
}

func (l *roLayer) DiffID() layer.DiffID {
	if l.roLayer == nil {
		return layer.DigestSHA256EmptyTar
	}
	return l.roLayer.DiffID()
}

func (l *roLayer) Release() error {
	if l.released {
		return nil
	}
	if l.roLayer != nil {
		metadata, err := l.layerStore.Release(l.roLayer)
		layer.LogReleaseMetadata(metadata)
		if err != nil {
			return errors.Wrap(err, "failed to release ROLayer")
		}
	}
	l.roLayer = nil
	l.released = true
	return nil
}

func (l *roLayer) NewRWLayer() (builder.RWLayer, error) {
	var chainID layer.ChainID
	if l.roLayer != nil {
		chainID = l.roLayer.ChainID()
	}

	mountID := stringid.GenerateRandomID()
	newLayer, err := l.layerStore.CreateRWLayer(mountID, chainID, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create rwlayer")
	}

	rwLayer := &rwLayer{layerStore: l.layerStore, rwLayer: newLayer}

	fs, err := newLayer.Mount("")
	if err != nil {
		rwLayer.Release()
		return nil, err
	}

	rwLayer.fs = fs

	return rwLayer, nil
}

type rwLayer struct {
	released   bool
	layerStore layer.Store
	rwLayer    layer.RWLayer
	fs         string
}

func (l *rwLayer) Root() string {
	return l.fs
}

func (l *rwLayer) Commit() (builder.ROLayer, error) {
	stream, err := l.rwLayer.TarStream()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var chainID layer.ChainID
	if parent := l.rwLayer.Parent(); parent != nil {
		chainID = parent.ChainID()
	}

	newLayer, err := l.layerStore.Register(stream, chainID)
	if err != nil {
		return nil, err
	}
	// TODO: An optimization would be to handle empty layers before returning
	return &roLayer{layerStore: l.layerStore, roLayer: newLayer}, nil
}

func (l *rwLayer) Release() error {
	if l.released {
		return nil
	}

	if l.fs != "" {
		if err := l.rwLayer.Unmount(); err != nil {
			return errors.Wrap(err, "failed to unmount RWLayer")
		}
		l.fs = ""
	}

	metadata, err := l.layerStore.ReleaseRWLayer(l.rwLayer)
	layer.LogReleaseMetadata(metadata)
	if err != nil {
		return errors.Wrap(err, "failed to release RWLayer")
	}
	l.released = true
	return nil
}

func newROLayerForImage(img *image.Image, layerStore layer.Store) (builder.ROLayer, error) {
	if img == nil || img.RootFS.ChainID() == "" {
		return &roLayer{layerStore: layerStore}, nil
	}
	// Hold a reference to the image layer so that it can't be removed before
	// it is released
	lyr, err := layerStore.Get(img.RootFS.ChainID())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get layer for image %s", img.ImageID())
	}
	return &roLayer{layerStore: layerStore, roLayer: lyr}, nil
}

// TODO: could this use the regular daemon PullImage ?
func (i *ImageService) pullForBuilder(ctx context.Context, name string, authConfigs map[string]registry.AuthConfig, output io.Writer, platform *ocispec.Platform) (*image.Image, error) {
	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, err
	}
	ref = reference.TagNameOnly(ref)

	pullRegistryAuth := &registry.AuthConfig{}
	if len(authConfigs) > 0 {
		// The request came with a full auth config, use it
		repoInfo, err := i.registryService.ResolveRepository(ref)
		if err != nil {
			return nil, err
		}

		resolvedConfig := registrypkg.ResolveAuthConfig(authConfigs, repoInfo.Index)
		pullRegistryAuth = &resolvedConfig
	}

	if err := i.pullImageWithReference(ctx, ref, platform, nil, pullRegistryAuth, output); err != nil {
		return nil, err
	}

	img, err := i.GetImage(ctx, name, backend.GetImageOpts{Platform: platform})
	if errdefs.IsNotFound(err) && img != nil && platform != nil {
		imgPlat := ocispec.Platform{
			OS:           img.OS,
			Architecture: img.BaseImgArch(),
			Variant:      img.BaseImgVariant(),
		}

		p := *platform
		if !platforms.Only(p).Match(imgPlat) {
			po := streamformatter.NewJSONProgressOutput(output, false)
			progress.Messagef(po, "", `
WARNING: Pulled image with specified platform (%s), but the resulting image's configured platform (%s) does not match.
This is most likely caused by a bug in the build system that created the fetched image (%s).
Please notify the image author to correct the configuration.`,
				platforms.Format(p), platforms.Format(imgPlat), name,
			)
			log.G(ctx).WithError(err).WithField("image", name).Warn("Ignoring error about platform mismatch where the manifest list points to an image whose configuration does not match the platform in the manifest.")
			err = nil
		}
	}
	return img, err
}

// GetImageAndReleasableLayer returns an image and releaseable layer for a reference or ID.
// Every call to GetImageAndReleasableLayer MUST call releasableLayer.Release() to prevent
// leaking of layers.
func (i *ImageService) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error) {
	if refOrID == "" { // FROM scratch
		if runtime.GOOS == "windows" {
			return nil, nil, fmt.Errorf(`"FROM scratch" is not supported on Windows`)
		}
		if opts.Platform != nil {
			if err := image.CheckOS(opts.Platform.OS); err != nil {
				return nil, nil, err
			}
		}
		lyr, err := newROLayerForImage(nil, i.layerStore)
		return nil, lyr, err
	}

	if opts.PullOption != backend.PullOptionForcePull {
		img, err := i.GetImage(ctx, refOrID, backend.GetImageOpts{Platform: opts.Platform})
		if err != nil && opts.PullOption == backend.PullOptionNoPull {
			return nil, nil, err
		}
		if err != nil && !errdefs.IsNotFound(err) {
			return nil, nil, err
		}
		if img != nil {
			if err := image.CheckOS(img.OperatingSystem()); err != nil {
				return nil, nil, err
			}
			lyr, err := newROLayerForImage(img, i.layerStore)
			return img, lyr, err
		}
	}

	img, err := i.pullForBuilder(ctx, refOrID, opts.AuthConfig, opts.Output, opts.Platform)
	if err != nil {
		return nil, nil, err
	}
	if err := image.CheckOS(img.OperatingSystem()); err != nil {
		return nil, nil, err
	}
	lyr, err := newROLayerForImage(img, i.layerStore)
	return img, lyr, err
}

// CreateImage creates a new image by adding a config and ID to the image store.
// This is similar to LoadImage() except that it receives JSON encoded bytes of
// an image instead of a tar archive.
func (i *ImageService) CreateImage(ctx context.Context, config []byte, parent string, _ digest.Digest) (builder.Image, error) {
	id, err := i.imageStore.Create(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create image")
	}

	if parent != "" {
		if err := i.imageStore.SetParent(id, image.ID(parent)); err != nil {
			return nil, errors.Wrapf(err, "failed to set parent %s", parent)
		}
	}
	if err := i.imageStore.SetBuiltLocally(id); err != nil {
		return nil, errors.Wrapf(err, "failed to mark image %s as built locally", id)
	}

	return i.imageStore.Get(id)
}
