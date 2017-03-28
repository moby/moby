package daemon

import (
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
	rwLayer layer.RWLayer
	release func(layer.RWLayer) error
	mount   func() (layer.RWLayer, error)
}

func (rl *releaseableLayer) Release() error {
	if rl.rwLayer == nil {
		return nil
	}
	rl.rwLayer.Unmount()
	return rl.release(rl.rwLayer)
}

func (rl *releaseableLayer) Mount() (string, error) {
	var err error
	// daemon.layerStore.CreateRWLayer(mountID, img.RootFS.ChainID(), nil)
	rl.rwLayer, err = rl.mount()
	if err != nil {
		return "", errors.Wrap(err, "failed to create rwlayer")
	}

	mountPath, err := rl.rwLayer.Mount("")
	if err != nil {
		releaseErr := rl.release(rl.rwLayer)
		if releaseErr != nil {
			err = errors.Wrapf(err, "failed to release rwlayer: %s", releaseErr.Error())
		}
		return "", errors.Wrap(err, "failed to mount rwlayer")
	}
	return mountPath, err
}

func (daemon *Daemon) getReleasableLayerForImage(img *image.Image) (*releaseableLayer, error) {
	mountFunc := func() (layer.RWLayer, error) {
		mountID := stringid.GenerateRandomID()
		return daemon.layerStore.CreateRWLayer(mountID, img.RootFS.ChainID(), nil)
	}

	releaseFunc := func(rwLayer layer.RWLayer) error {
		metadata, err := daemon.layerStore.ReleaseRWLayer(rwLayer)
		layer.LogReleaseMetadata(metadata)
		return err
	}

	return &releaseableLayer{mount: mountFunc, release: releaseFunc}, nil
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
		// The request came with a full auth config file, we prefer to use that
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

// GetImageAndLayer returns an image and releaseable layer for a reference or ID
func (daemon *Daemon) GetImageAndLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ReleaseableLayer, error) {
	if !opts.ForcePull {
		image, _ := daemon.GetImage(refOrID)
		// TODO: shouldn't we error out if error is different from "not found" ?
		if image != nil {
			layer, err := daemon.getReleasableLayerForImage(image)
			return image, layer, err
		}
	}

	image, err := daemon.pullForBuilder(ctx, refOrID, opts.AuthConfig, opts.Output)
	if err != nil {
		return nil, nil, err
	}
	layer, err := daemon.getReleasableLayerForImage(image)
	return image, layer, err
}
