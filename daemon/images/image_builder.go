package images // import "github.com/docker/docker/daemon/images"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/containerd/containerd/images"

	"github.com/containerd/containerd/content"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type roLayer struct {
	released   bool
	layerStore layer.Store
	roLayer    layer.Layer
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
	fs         containerfs.ContainerFS
}

func (l *rwLayer) Root() containerfs.ContainerFS {
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

	if l.fs != nil {
		if err := l.rwLayer.Unmount(); err != nil {
			return errors.Wrap(err, "failed to unmount RWLayer")
		}
		l.fs = nil
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
	layer, err := layerStore.Get(img.RootFS.ChainID())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get layer for image %s", img.ImageID())
	}
	return &roLayer{layerStore: layerStore, roLayer: layer}, nil
}

// TODO: could this use the regular daemon PullImage ?
// TODO(containerd): don't return *image.Image type
func (i *ImageService) pullForBuilder(ctx context.Context, name string, authConfigs map[string]types.AuthConfig, output io.Writer, platform *ocispec.Platform) (*image.Image, error) {
	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, err
	}
	ref = reference.TagNameOnly(ref)

	pullRegistryAuth := &types.AuthConfig{}
	if len(authConfigs) > 0 {
		// The request came with a full auth config, use it
		repoInfo, err := i.registryService.ResolveRepository(ref)
		if err != nil {
			return nil, err
		}

		resolvedConfig := registry.ResolveAuthConfig(authConfigs, repoInfo.Index)
		pullRegistryAuth = &resolvedConfig
	}

	if err := i.pullImageWithReference(ctx, ref, platform, nil, pullRegistryAuth, output); err != nil {
		return nil, err
	}
	return i.getDockerImage(name)
}

// GetImageAndReleasableLayer returns an image and releaseable layer for a reference or ID.
// Every call to GetImageAndReleasableLayer MUST call releasableLayer.Release() to prevent
// leaking of layers.
func (i *ImageService) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error) {
	if refOrID == "" { // ie FROM scratch
		os := runtime.GOOS
		if opts.Platform != nil {
			os = opts.Platform.OS
		}
		if !system.IsOSSupported(os) {
			return nil, nil, system.ErrNotSupportedOperatingSystem
		}
		layer, err := newROLayerForImage(nil, i.layerStores[os])
		return nil, layer, err
	}

	if opts.PullOption != backend.PullOptionForcePull {
		image, err := i.getDockerImage(refOrID)
		if err != nil && opts.PullOption == backend.PullOptionNoPull {
			return nil, nil, err
		}
		// TODO: shouldn't we error out if error is different from "not found" ?
		if image != nil {
			if !system.IsOSSupported(image.OperatingSystem()) {
				return nil, nil, system.ErrNotSupportedOperatingSystem
			}
			layer, err := newROLayerForImage(image, i.layerStores[image.OperatingSystem()])
			return image, layer, err
		}
	}

	image, err := i.pullForBuilder(ctx, refOrID, opts.AuthConfig, opts.Output, opts.Platform)
	if err != nil {
		return nil, nil, err
	}
	if !system.IsOSSupported(image.OperatingSystem()) {
		return nil, nil, system.ErrNotSupportedOperatingSystem
	}
	layer, err := newROLayerForImage(image, i.layerStores[image.OperatingSystem()])
	return image, layer, err
}

// CreateImage creates a new image by adding a config and ID to the image store.
// This is similar to LoadImage() except that it receives JSON encoded bytes of
// an image instead of a tar archive.
func (i *ImageService) CreateImage(ctx context.Context, newImage backend.NewImageConfig, newROLayer builder.ROLayer) (ocispec.Descriptor, error) {
	// TODO(containerd): use containerd's image store

	cache, err := i.getCache(ctx)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	var img struct {
		ocispec.Image

		// Overwrite config for custom Docker fields
		Container       string            `json:"container,omitempty"`
		ContainerConfig container.Config  `json:"container_config,omitempty"`
		Config          *container.Config `json:"config,omitempty"`

		Comment    string   `json:"comment,omitempty"`
		OSVersion  string   `json:"os.version,omitempty"`
		OSFeatures []string `json:"os.features,omitempty"`
		Variant    string   `json:"variant,omitempty"`
		// TODO: Overwrite this with a label from config
		DockerVersion string `json:"docker_version,omitempty"`
	}

	if newImage.ParentImageID == "" {
		img.RootFS.Type = "layers"
	} else {
		desc := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    digest.Digest(newImage.ParentImageID),
		}

		b, err := content.ReadBlob(ctx, i.client.ContentStore(), desc)
		if err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "unable to read config")
		}

		if err := json.Unmarshal(b, &img); err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "failed to unmarshal config")
		}
	}
	created := time.Now().UTC()
	img.Created = &created

	isEmptyLayer := layer.IsEmpty(newROLayer.DiffID())
	if !isEmptyLayer {
		img.RootFS.DiffIDs = append(img.RootFS.DiffIDs, digest.Digest(newROLayer.DiffID()))
	}
	img.History = append(img.History, ocispec.History{
		Author:     newImage.Author,
		Created:    &created,
		CreatedBy:  strings.Join(newImage.ContainerConfig.Cmd, " "),
		EmptyLayer: isEmptyLayer,
	})
	img.Author = newImage.Author
	img.OS = newImage.OS
	img.Config = newImage.Config
	img.ContainerConfig = *newImage.ContainerConfig

	store, err := i.getLayerStore(ocispec.Platform{OS: newImage.OS})
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	config, err := json.Marshal(img)
	if err != nil {
		return ocispec.Descriptor{}, errors.Wrap(err, "failed to marshal committed image")
	}

	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageConfig,
		Digest:    digest.FromBytes(config),
		Size:      int64(len(config)),
	}

	newLayer, ok := newROLayer.(*roLayer)
	if !ok {
		return ocispec.Descriptor{}, errors.Errorf("unexpected image type")
	}

	driver := newLayer.layerStore.DriverName()
	key := fmt.Sprintf("%s%s", LabelLayerPrefix, driver)
	layerID := digest.Digest(newLayer.roLayer.ChainID())
	labels := map[string]string{
		key: layerID.String(),
	}

	if newImage.ParentImageID != "" {
		labels[LabelImageParent] = newImage.ParentImageID
	}

	opts := []content.Opt{content.WithLabels(labels)}

	// write image config data to content store
	ref := fmt.Sprintf("config-%s-%s", desc.Digest.Algorithm().String(), desc.Digest.Encoded())
	if err := content.WriteBlob(ctx, i.client.ContentStore(), ref, bytes.NewReader(config), desc, opts...); err != nil {
		return ocispec.Descriptor{}, errors.Wrap(err, "unable to store config")
	}

	// create a dangling image
	_, err = i.client.ImageService().Create(ctx, images.Image{
		Name:      desc.Digest.String(),
		Target:    desc,
		CreatedAt: created,
		UpdatedAt: created,
	})
	if err != nil {
		return ocispec.Descriptor{}, errors.Wrapf(err, "failed to create image")
	}

	cache.m.Lock()

	if _, ok := cache.layers[driver][layerID]; !ok {
		cache.layers[driver][layerID] = newLayer.roLayer
	} else {
		// Image already retained, don't hold onto layer
		defer layer.ReleaseAndLog(store, newLayer.roLayer)
	}
	cache.m.Unlock()

	return desc, nil
}
