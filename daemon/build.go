package daemon

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/registry"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"io"
)

type releaseableLayer struct {
	layerStore layer.Store
	roLayer    layer.Layer
	rwLayer    layer.RWLayer
}

func (rl *releaseableLayer) Mount() (string, error) {
	if rl.roLayer == nil {
		return "", errors.New("can not mount an image with no root FS")
	}
	var err error
	mountID := stringid.GenerateRandomID()
	rl.rwLayer, err = rl.layerStore.CreateRWLayer(mountID, rl.roLayer.ChainID(), nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create rwlayer")
	}

	return rl.rwLayer.Mount("")
}

func (rl *releaseableLayer) Release() error {
	rl.releaseRWLayer()
	return rl.releaseROLayer()
}

func (rl *releaseableLayer) releaseRWLayer() error {
	if rl.rwLayer == nil {
		return nil
	}
	metadata, err := rl.layerStore.ReleaseRWLayer(rl.rwLayer)
	layer.LogReleaseMetadata(metadata)
	if err != nil {
		logrus.Errorf("Failed to release RWLayer: %s", err)
	}
	return err
}

func (rl *releaseableLayer) releaseROLayer() error {
	if rl.roLayer == nil {
		return nil
	}
	metadata, err := rl.layerStore.Release(rl.roLayer)
	layer.LogReleaseMetadata(metadata)
	return err
}

func newReleasableLayerForImage(img *image.Image, layerStore layer.Store) (builder.ReleaseableLayer, error) {
	if img.RootFS.ChainID() == "" {
		return nil, nil
	}
	// Hold a reference to the image layer so that it can't be removed before
	// it is released
	roLayer, err := layerStore.Get(img.RootFS.ChainID())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get layer for image %s", img.ImageID())
	}
	return &releaseableLayer{layerStore: layerStore, roLayer: roLayer}, nil
}

// TODO: could this use the regular daemon PullImage ?
func (daemon *Daemon) pullForBuilder(ctx context.Context, name string, authConfigs map[string]types.AuthConfig, output io.Writer) (*image.Image, error) {
	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, err
	}
	ref = reference.TagNameOnly(ref)

	pullRegistryAuth := &types.AuthConfig{}
	if len(authConfigs) > 0 {
		// The request came with a full auth config, use it
		repoInfo, err := daemon.RegistryService.ResolveRepository(ref)
		if err != nil {
			return nil, err
		}

		resolvedConfig := registry.ResolveAuthConfig(authConfigs, repoInfo.Index)
		pullRegistryAuth = &resolvedConfig
	}

	if err := daemon.pullImageWithReference(ctx, ref, nil, pullRegistryAuth, output); err != nil {
		return nil, err
	}
	return daemon.GetImage(name)
}

// GetImageAndReleasableLayer returns an image and releaseable layer for a reference or ID.
// Every call to GetImageAndReleasableLayer MUST call releasableLayer.Release() to prevent
// leaking of layers.
func (daemon *Daemon) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ReleaseableLayer, error) {
	if !opts.ForcePull {
		image, _ := daemon.GetImage(refOrID)
		// TODO: shouldn't we error out if error is different from "not found" ?
		if image != nil {
			layer, err := newReleasableLayerForImage(image, daemon.layerStore)
			return image, layer, err
		}
	}

	image, err := daemon.pullForBuilder(ctx, refOrID, opts.AuthConfig, opts.Output)
	if err != nil {
		return nil, nil, err
	}
	layer, err := newReleasableLayerForImage(image, daemon.layerStore)
	return image, layer, err
}
