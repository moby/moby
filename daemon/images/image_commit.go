package images // import "github.com/docker/docker/daemon/images"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/ioutils"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// CommitImage creates a new image from a commit config
func (i *ImageService) CommitImage(ctx context.Context, c backend.CommitConfig) (ocispec.Descriptor, error) {
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

	if c.ParentImageID == "" {
		img.RootFS.Type = "layers"
	} else {
		cache.m.RLock()
		pci, ok := cache.idCache[digest.Digest(c.ParentImageID)]
		cache.m.RUnlock()

		if !ok {
			return ocispec.Descriptor{}, errors.Wrap(errdefs.ErrNotFound, "parent not found")
		}

		b, err := content.ReadBlob(ctx, i.client.ContentStore(), pci.config)
		if err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "unable to read config")
		}

		if err := json.Unmarshal(b, &img); err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "failed to unmarshal config")
		}
	}

	// TODO(containerd): get from container metadata
	layerStore, err := i.getLayerStoreByOS(c.ContainerOS)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	rwTar, err := exportContainerRw(layerStore, c.ContainerID, c.ContainerMountLabel)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer func() {
		if rwTar != nil {
			rwTar.Close()
		}
	}()

	// TODO(containerd): Tee compressed output to content store
	// for generation of the manifest.
	l, err := layerStore.Register(rwTar, layer.ChainID(identity.ChainID(img.RootFS.DiffIDs)))
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	created := time.Now().UTC()
	diffID := l.DiffID()

	img.Created = &created

	isEmptyLayer := layer.IsEmpty(diffID)
	if !isEmptyLayer {
		img.RootFS.DiffIDs = append(img.RootFS.DiffIDs, digest.Digest(diffID))
	}
	img.History = append(img.History, ocispec.History{
		Author:     c.Author,
		Created:    &created,
		CreatedBy:  strings.Join(c.ContainerConfig.Cmd, " "),
		Comment:    c.Comment,
		EmptyLayer: isEmptyLayer,
	})

	img.DockerVersion = dockerversion.Version
	img.Author = c.Author
	img.Comment = c.Comment
	if img.OS == "" {
		img.OS = c.ContainerOS
	}
	img.Container = c.ContainerID
	img.Config = c.Config
	img.ContainerConfig = *c.ContainerConfig

	config, err := json.Marshal(img)
	if err != nil {
		layer.ReleaseAndLog(layerStore, l)
		return ocispec.Descriptor{}, errors.Wrap(err, "failed to marshal committed image")
	}

	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageConfig,
		Digest:    digest.FromBytes(config),
		Size:      int64(len(config)),
	}

	labels := map[string]string{
		fmt.Sprintf("%s%s", LabelLayerPrefix, layerStore.DriverName()): l.ChainID().String(),
	}

	if c.ParentImageID != "" {
		labels[LabelImageParent] = c.ParentImageID
	}

	opts := []content.Opt{content.WithLabels(labels)}

	ref := fmt.Sprintf("config-%s-%s", desc.Digest.Algorithm().String(), desc.Digest.Encoded())
	if err := content.WriteBlob(ctx, i.client.ContentStore(), ref, bytes.NewReader(config), desc, opts...); err != nil {
		layer.ReleaseAndLog(layerStore, l)
		return ocispec.Descriptor{}, errors.Wrap(err, "unable to store config")
	}

	// Create a dangling image
	_, err = i.client.ImageService().Create(ctx, images.Image{
		// TODO(containerd): Add a name component here
		Name:      desc.Digest.String(),
		Target:    desc,
		CreatedAt: created,
		UpdatedAt: created,
		Labels: map[string]string{
			// TODO(containerd): name can be used to determine this
			LabelImageDangling: desc.Digest.String(),
		},
	})
	if err != nil {
		layer.ReleaseAndLog(layerStore, l)
		return ocispec.Descriptor{}, errors.Wrap(err, "unable to store image")
	}

	cache.m.Lock()
	layerKey := digest.Digest(l.ChainID())
	if _, ok := cache.layers[layerStore.DriverName()][layerKey]; !ok {
		cache.layers[layerStore.DriverName()][layerKey] = l
	} else {
		// Image already retained, don't hold onto layer
		defer layer.ReleaseAndLog(layerStore, l)
	}

	// TODO(containerd): remove this, no longer used
	if _, ok := cache.idCache[desc.Digest]; !ok {
		ci := &cachedImage{
			config: desc,
			parent: digest.Digest(c.ParentImageID),
		}
		cache.idCache[desc.Digest] = ci

		// TODO(containerd): Refer to manifest here
		cache.tCache[desc.Digest] = ci

		if ci.parent != "" {
			pci, ok := cache.idCache[ci.parent]
			if ok {
				pci.m.Lock()
				pci.children = append(pci.children, desc.Digest)
				pci.m.Unlock()
			}
		}
	}

	cache.m.Unlock()

	return desc, nil
}

func exportContainerRw(layerStore layer.Store, id, mountLabel string) (arch io.ReadCloser, err error) {
	rwlayer, err := layerStore.GetRWLayer(id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			layerStore.ReleaseRWLayer(rwlayer)
		}
	}()

	// TODO: this mount call is not necessary as we assume that TarStream() should
	// mount the layer if needed. But the Diff() function for windows requests that
	// the layer should be mounted when calling it. So we reserve this mount call
	// until windows driver can implement Diff() interface correctly.
	_, err = rwlayer.Mount(mountLabel)
	if err != nil {
		return nil, err
	}

	archive, err := rwlayer.TarStream()
	if err != nil {
		rwlayer.Unmount()
		return nil, err
	}
	return ioutils.NewReadCloserWrapper(archive, func() error {
			archive.Close()
			err = rwlayer.Unmount()
			layerStore.ReleaseRWLayer(rwlayer)
			return err
		}),
		nil
}

// CommitBuildStep is used by the builder to create an image for each step in
// the build.
//
// This method is different from CreateImageFromContainer:
//   * it doesn't attempt to validate container state
//   * it doesn't send a commit action to metrics
//   * it doesn't log a container commit event
//
// This is a temporary shim. Should be removed when builder stops using commit.
func (i *ImageService) CommitBuildStep(ctx context.Context, c backend.CommitConfig) (image.ID, error) {
	container := i.containers.Get(c.ContainerID)
	if container == nil {
		// TODO: use typed error
		return "", errors.Errorf("container not found: %s", c.ContainerID)
	}
	c.ContainerMountLabel = container.MountLabel
	c.ContainerOS = container.OS
	c.ParentImageID = string(container.ImageID)
	desc, err := i.CommitImage(ctx, c)
	if err != nil {
		return "", err
	}
	return image.ID(desc.Digest.String()), nil
}
