package containerd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	c8dimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/rootfs"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	dimage "github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	registrypkg "github.com/docker/docker/registry"
	imagespec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	// Digest of the image which was the base image of the committed container.
	imageLabelClassicBuilderParent = "org.mobyproject.image.parent"

	// "1" means that the image was created directly from the "FROM scratch".
	imageLabelClassicBuilderFromScratch = "org.mobyproject.image.fromscratch"

	// digest of the ContainerConfig stored in the content store.
	imageLabelClassicBuilderContainerConfig = "org.mobyproject.image.containerconfig"
)

const (
	// gc.ref label that associates the ContainerConfig content blob with the
	// corresponding Config content.
	contentLabelGcRefContainerConfig = "containerd.io/gc.ref.content.moby/container.config"

	// Digest of the image this ContainerConfig blobs describes.
	// Only ContainerConfig content should be labelled with it.
	contentLabelClassicBuilderImage = "org.mobyproject.content.image"
)

// GetImageAndReleasableLayer returns an image and releaseable layer for a
// reference or ID. Every call to GetImageAndReleasableLayer MUST call
// releasableLayer.Release() to prevent leaking of layers.
func (i *ImageService) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error) {
	if refOrID == "" { // FROM scratch
		if runtime.GOOS == "windows" {
			return nil, nil, fmt.Errorf(`"FROM scratch" is not supported on Windows`)
		}
		if opts.Platform != nil {
			if err := dimage.CheckOS(opts.Platform.OS); err != nil {
				return nil, nil, err
			}
		}
		return nil, &rolayer{
			c:           i.client,
			snapshotter: i.snapshotter,
		}, nil
	}

	if opts.PullOption != backend.PullOptionForcePull {
		// TODO(laurazard): same as below
		img, err := i.GetImage(ctx, refOrID, backend.GetImageOpts{Platform: opts.Platform})
		if err != nil && opts.PullOption == backend.PullOptionNoPull {
			return nil, nil, err
		}
		imgDesc, err := i.resolveDescriptor(ctx, refOrID)
		if err != nil && !errdefs.IsNotFound(err) {
			return nil, nil, err
		}
		if img != nil {
			if err := dimage.CheckOS(img.OperatingSystem()); err != nil {
				return nil, nil, err
			}

			roLayer, err := newROLayerForImage(ctx, &imgDesc, i, opts.Platform)
			if err != nil {
				return nil, nil, err
			}

			return img, roLayer, nil
		}
	}

	ctx, release, err := i.withLease(ctx, true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create lease for commit: %w", err)
	}
	defer release()

	// TODO(laurazard): do we really need a new method here to pull the image?
	imgDesc, err := i.pullForBuilder(ctx, refOrID, opts.AuthConfig, opts.Output, opts.Platform)
	if err != nil {
		return nil, nil, err
	}

	// TODO(laurazard): pullForBuilder should return whatever we
	// need here instead of having to go and get it again
	img, err := i.GetImage(ctx, refOrID, backend.GetImageOpts{
		Platform: opts.Platform,
	})
	if err != nil {
		return nil, nil, err
	}

	roLayer, err := newROLayerForImage(ctx, imgDesc, i, opts.Platform)
	if err != nil {
		return nil, nil, err
	}

	return img, roLayer, nil
}

func (i *ImageService) pullForBuilder(ctx context.Context, name string, authConfigs map[string]registry.AuthConfig, output io.Writer, platform *ocispec.Platform) (*ocispec.Descriptor, error) {
	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, err
	}

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

	if err := i.PullImage(ctx, reference.TagNameOnly(ref), platform, nil, pullRegistryAuth, output); err != nil {
		return nil, err
	}

	img, err := i.GetImage(ctx, name, backend.GetImageOpts{Platform: platform})
	if err != nil {
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
			}
		} else {
			return nil, err
		}
	}

	if err := dimage.CheckOS(img.OperatingSystem()); err != nil {
		return nil, err
	}

	imgDesc, err := i.resolveDescriptor(ctx, name)
	if err != nil {
		return nil, err
	}

	return &imgDesc, err
}

func newROLayerForImage(ctx context.Context, imgDesc *ocispec.Descriptor, i *ImageService, platform *ocispec.Platform) (builder.ROLayer, error) {
	if imgDesc == nil {
		return nil, fmt.Errorf("can't make an RO layer for a nil image :'(")
	}

	platMatcher := platforms.Default()
	if platform != nil {
		platMatcher = platforms.Only(*platform)
	}

	confDesc, err := c8dimages.Config(ctx, i.content, *imgDesc, platMatcher)
	if err != nil {
		return nil, err
	}

	diffIDs, err := c8dimages.RootFS(ctx, i.content, confDesc)
	if err != nil {
		return nil, err
	}

	// TODO(vvoland): Check if image is unpacked, and unpack it if it's not.
	imageSnapshotID := identity.ChainID(diffIDs).String()

	snapshotter := i.StorageDriver()
	_, lease, err := createLease(ctx, i.client.LeasesService())
	if err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to lease image snapshot %s: %w", imageSnapshotID, err))
	}

	return &rolayer{
		key:                imageSnapshotID,
		c:                  i.client,
		snapshotter:        snapshotter,
		diffID:             "", // Image RO layer doesn't have a diff.
		contentStoreDigest: "",
		lease:              &lease,
	}, nil
}

func createLease(ctx context.Context, lm leases.Manager) (context.Context, leases.Lease, error) {
	lease, err := lm.Create(ctx,
		leases.WithExpiration(leaseExpireDuration),
		leases.WithLabels(map[string]string{
			pruneLeaseLabel: "true",
		}),
	)
	if err != nil {
		return nil, leases.Lease{}, fmt.Errorf("failed to create a lease for snapshot: %w", err)
	}

	return leases.WithLease(ctx, lease.ID), lease, nil
}

type rolayer struct {
	key                string
	c                  *containerd.Client
	snapshotter        string
	diffID             layer.DiffID
	contentStoreDigest digest.Digest
	lease              *leases.Lease
}

func (rl *rolayer) ContentStoreDigest() digest.Digest {
	return rl.contentStoreDigest
}

func (rl *rolayer) DiffID() layer.DiffID {
	if rl.diffID == "" {
		return layer.DigestSHA256EmptyTar
	}
	return rl.diffID
}

func (rl *rolayer) Release() error {
	if rl.lease != nil {
		lm := rl.c.LeasesService()
		err := lm.Delete(context.TODO(), *rl.lease)
		if err != nil {
			return err
		}
		rl.lease = nil
	}
	return nil
}

// NewRWLayer creates a new read-write layer for the builder
func (rl *rolayer) NewRWLayer() (_ builder.RWLayer, outErr error) {
	snapshotter := rl.c.SnapshotService(rl.snapshotter)

	key := stringid.GenerateRandomID()

	ctx, lease, err := createLease(context.TODO(), rl.c.LeasesService())
	if err != nil {
		return nil, err
	}
	defer func() {
		if outErr != nil {
			if err := rl.c.LeasesService().Delete(ctx, lease); err != nil {
				log.G(ctx).WithError(err).Warn("failed to remove lease after NewRWLayer error")
			}
		}
	}()

	mounts, err := snapshotter.Prepare(ctx, key, rl.key)
	if err != nil {
		return nil, err
	}

	root, err := os.MkdirTemp(os.TempDir(), "rootfs-mount")
	if err != nil {
		return nil, err
	}
	if err := mount.All(mounts, root); err != nil {
		return nil, err
	}

	return &rwlayer{
		key:         key,
		parent:      rl.key,
		c:           rl.c,
		snapshotter: rl.snapshotter,
		root:        root,
		lease:       &lease,
	}, nil
}

type rwlayer struct {
	key         string
	parent      string
	c           *containerd.Client
	snapshotter string
	root        string
	lease       *leases.Lease
}

func (rw *rwlayer) Root() string {
	return rw.root
}

func (rw *rwlayer) Commit() (_ builder.ROLayer, outErr error) {
	snapshotter := rw.c.SnapshotService(rw.snapshotter)

	key := stringid.GenerateRandomID()

	lm := rw.c.LeasesService()
	ctx, lease, err := createLease(context.TODO(), lm)
	if err != nil {
		return nil, err
	}
	defer func() {
		if outErr != nil {
			if err := lm.Delete(ctx, lease); err != nil {
				log.G(ctx).WithError(err).Warn("failed to remove lease after NewRWLayer error")
			}
		}
	}()

	// Unmount the layer, required by the containerd windows snapshotter.
	// The windowsfilter graphdriver does this inside its own Diff method.
	//
	// The only place that calls this in-tree is (b *Builder) exportImage and
	// that is called from the end of (b *Builder) performCopy which has a
	// `defer rwLayer.Release()` pending.
	//
	// After the snapshotter.Commit the source snapshot is deleted anyway and
	// it shouldn't be accessed afterwards.
	if rw.root != "" {
		if err := mount.UnmountAll(rw.root, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.G(ctx).WithError(err).WithField("root", rw.root).Error("failed to unmount RWLayer")
			return nil, err
		}
	}

	err = snapshotter.Commit(ctx, key, rw.key)
	if err != nil && !cerrdefs.IsAlreadyExists(err) {
		return nil, err
	}

	differ := rw.c.DiffService()
	desc, err := rootfs.CreateDiff(ctx, key, snapshotter, differ)
	if err != nil {
		return nil, err
	}
	info, err := rw.c.ContentStore().Info(ctx, desc.Digest)
	if err != nil {
		return nil, err
	}
	diffIDStr, ok := info.Labels["containerd.io/uncompressed"]
	if !ok {
		return nil, fmt.Errorf("invalid differ response with no diffID")
	}
	diffID, err := digest.Parse(diffIDStr)
	if err != nil {
		return nil, err
	}

	return &rolayer{
		key:                key,
		c:                  rw.c,
		snapshotter:        rw.snapshotter,
		diffID:             layer.DiffID(diffID),
		contentStoreDigest: desc.Digest,
		lease:              &lease,
	}, nil
}

func (rw *rwlayer) Release() (outErr error) {
	if rw.root == "" { // nothing to release
		return nil
	}

	if err := mount.UnmountAll(rw.root, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.G(context.TODO()).WithError(err).WithField("root", rw.root).Error("failed to unmount RWLayer")
		return err
	}
	if err := os.Remove(rw.root); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.G(context.TODO()).WithError(err).WithField("dir", rw.root).Error("failed to remove mount temp dir")
		return err
	}
	rw.root = ""

	if rw.lease != nil {
		lm := rw.c.LeasesService()
		err := lm.Delete(context.TODO(), *rw.lease)
		if err != nil {
			log.G(context.TODO()).WithError(err).Warn("failed to delete lease when releasing RWLayer")
		} else {
			rw.lease = nil
		}
	}

	return nil
}

// CreateImage creates a new image by adding a config and ID to the image store.
// This is similar to LoadImage() except that it receives JSON encoded bytes of
// an image instead of a tar archive.
func (i *ImageService) CreateImage(ctx context.Context, config []byte, parent string, layerDigest digest.Digest) (builder.Image, error) {
	imgToCreate, err := dimage.NewFromJSON(config)
	if err != nil {
		return nil, err
	}

	ociImgToCreate := dockerImageToDockerOCIImage(*imgToCreate)

	var layers []ocispec.Descriptor

	var parentDigest digest.Digest
	// if the image has a parent, we need to start with the parents layers descriptors
	if parent != "" {
		parentDesc, err := i.resolveDescriptor(ctx, parent)
		if err != nil {
			return nil, err
		}
		parentImageManifest, err := c8dimages.Manifest(ctx, i.content, parentDesc, platforms.Default())
		if err != nil {
			return nil, err
		}

		layers = parentImageManifest.Layers
		parentDigest = parentDesc.Digest
	}

	cs := i.content

	ra, err := cs.ReaderAt(ctx, ocispec.Descriptor{Digest: layerDigest})
	if err != nil {
		return nil, fmt.Errorf("failed to read diff archive: %w", err)
	}
	defer ra.Close()

	empty, err := archive.IsEmpty(content.NewReader(ra))
	if err != nil {
		return nil, fmt.Errorf("failed to check if archive is empty: %w", err)
	}
	if !empty {
		info, err := cs.Info(ctx, layerDigest)
		if err != nil {
			return nil, err
		}

		layers = append(layers, ocispec.Descriptor{
			MediaType: c8dimages.MediaTypeDockerSchema2LayerGzip,
			Digest:    layerDigest,
			Size:      info.Size,
		})
	}

	createdImageId, err := i.createImageOCI(ctx, ociImgToCreate, parentDigest, layers, imgToCreate.ContainerConfig)
	if err != nil {
		return nil, err
	}

	return dimage.Clone(imgToCreate, createdImageId), nil
}

func (i *ImageService) createImageOCI(ctx context.Context, imgToCreate imagespec.DockerOCIImage,
	parentDigest digest.Digest, layers []ocispec.Descriptor,
	containerConfig container.Config,
) (dimage.ID, error) {
	ctx, release, err := i.withLease(ctx, false)
	if err != nil {
		return "", err
	}
	defer release()

	manifestDesc, ccDesc, err := writeContentsForImage(ctx, i.snapshotter, i.content, imgToCreate, layers, containerConfig)
	if err != nil {
		return "", err
	}

	img := c8dimages.Image{
		Name:      danglingImageName(manifestDesc.Digest),
		Target:    manifestDesc,
		CreatedAt: time.Now(),
		Labels: map[string]string{
			imageLabelClassicBuilderParent:          parentDigest.String(),
			imageLabelClassicBuilderContainerConfig: ccDesc.Digest.String(),
		},
	}

	if parentDigest == "" {
		img.Labels[imageLabelClassicBuilderFromScratch] = "1"
	}

	if err := i.createOrReplaceImage(ctx, img); err != nil {
		return "", err
	}

	id := image.ID(img.Target.Digest)
	i.LogImageEvent(id.String(), id.String(), events.ActionCreate)

	if err := i.unpackImage(ctx, i.StorageDriver(), img, manifestDesc); err != nil {
		return "", err
	}

	return id, nil
}

// writeContentsForImage will commit oci image config and manifest into containerd's content store.
func writeContentsForImage(ctx context.Context, snName string, cs content.Store,
	newConfig imagespec.DockerOCIImage, layers []ocispec.Descriptor,
	containerConfig container.Config,
) (
	manifestDesc ocispec.Descriptor,
	containerConfigDesc ocispec.Descriptor,
	_ error,
) {
	newConfigJSON, err := json.Marshal(newConfig)
	if err != nil {
		return ocispec.Descriptor{}, ocispec.Descriptor{}, err
	}

	configDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageConfig,
		Digest:    digest.FromBytes(newConfigJSON),
		Size:      int64(len(newConfigJSON)),
	}

	newMfst := struct {
		MediaType string `json:"mediaType,omitempty"`
		ocispec.Manifest
	}{
		MediaType: ocispec.MediaTypeImageManifest,
		Manifest: ocispec.Manifest{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Config: configDesc,
			Layers: layers,
		},
	}

	newMfstJSON, err := json.MarshalIndent(newMfst, "", "    ")
	if err != nil {
		return ocispec.Descriptor{}, ocispec.Descriptor{}, err
	}

	newMfstDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(newMfstJSON),
		Size:      int64(len(newMfstJSON)),
	}

	// new manifest should reference the layers and config content
	labels := map[string]string{
		"containerd.io/gc.ref.content.0": configDesc.Digest.String(),
	}
	for i, l := range layers {
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i+1)] = l.Digest.String()
	}

	err = content.WriteBlob(ctx, cs, newMfstDesc.Digest.String(), bytes.NewReader(newMfstJSON), newMfstDesc, content.WithLabels(labels))
	if err != nil {
		return ocispec.Descriptor{}, ocispec.Descriptor{}, err
	}

	ccDesc, err := saveContainerConfig(ctx, cs, newMfstDesc.Digest, containerConfig)
	if err != nil {
		return ocispec.Descriptor{}, ocispec.Descriptor{}, err
	}

	// config should reference to snapshotter and container config
	labelOpt := content.WithLabels(map[string]string{
		fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", snName): identity.ChainID(newConfig.RootFS.DiffIDs).String(),
		contentLabelGcRefContainerConfig:                        ccDesc.Digest.String(),
	})
	err = content.WriteBlob(ctx, cs, configDesc.Digest.String(), bytes.NewReader(newConfigJSON), configDesc, labelOpt)
	if err != nil {
		return ocispec.Descriptor{}, ocispec.Descriptor{}, err
	}

	return newMfstDesc, ccDesc, nil
}

// saveContainerConfig serializes the given ContainerConfig into a json and
// stores it in the content store and returns its descriptor.
func saveContainerConfig(ctx context.Context, content content.Ingester, imgID digest.Digest, containerConfig container.Config) (ocispec.Descriptor, error) {
	containerConfigDesc, err := storeJson(ctx, content,
		"application/vnd.docker.container.image.v1+json", containerConfig,
		map[string]string{contentLabelClassicBuilderImage: imgID.String()},
	)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return containerConfigDesc, nil
}
