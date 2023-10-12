package containerd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/rootfs"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/internal/compatcontext"
	registrypkg "github.com/docker/docker/registry"

	// "github.com/docker/docker/api/types/container"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/log"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/errdefs"
	dimage "github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const imageLabelClassicBuilderParent = "org.mobyproject.image.parent"

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
		img, err := i.GetImage(ctx, refOrID, image.GetImageOpts{Platform: opts.Platform})
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

	ctx, _, err := i.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(1*time.Hour))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create lease for commit: %w", err)
	}

	// TODO(laurazard): do we really need a new method here to pull the image?
	imgDesc, err := i.pullForBuilder(ctx, refOrID, opts.AuthConfig, opts.Output, opts.Platform)
	if err != nil {
		return nil, nil, err
	}

	// TODO(laurazard): pullForBuilder should return whatever we
	// need here instead of having to go and get it again
	img, err := i.GetImage(ctx, refOrID, imagetypes.GetImageOpts{
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

	img, err := i.GetImage(ctx, name, imagetypes.GetImageOpts{Platform: platform})
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

	confDesc, err := containerdimages.Config(ctx, i.client.ContentStore(), *imgDesc, platMatcher)
	if err != nil {
		return nil, err
	}

	diffIDs, err := containerdimages.RootFS(ctx, i.client.ContentStore(), confDesc)
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
		leases.WithExpiration(time.Hour*24),
		leases.WithLabels(map[string]string{
			"org.mobyproject.lease.classicbuilder": "true",
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
		log.G(context.TODO()).WithError(err).WithField("root", rw.root).Error("failed to unmount ROLayer")
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
		parentImageManifest, err := containerdimages.Manifest(ctx, i.client.ContentStore(), parentDesc, platforms.Default())
		if err != nil {
			return nil, err
		}

		layers = parentImageManifest.Layers
		parentDigest = parentDesc.Digest
	}

	cs := i.client.ContentStore()

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
			MediaType: containerdimages.MediaTypeDockerSchema2LayerGzip,
			Digest:    layerDigest,
			Size:      info.Size,
		})
	}

	// necessary to prevent the contents from being GC'd
	// between writing them here and creating an image
	ctx, release, err := i.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(1*time.Hour))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := release(compatcontext.WithoutCancel(ctx)); err != nil {
			log.G(ctx).WithError(err).Warn("failed to release lease created for create")
		}
	}()

	commitManifestDesc, err := writeContentsForImage(ctx, i.snapshotter, i.client.ContentStore(), ociImgToCreate, layers)
	if err != nil {
		return nil, err
	}

	// image create
	img := containerdimages.Image{
		Name:      danglingImageName(commitManifestDesc.Digest),
		Target:    commitManifestDesc,
		CreatedAt: time.Now(),
		Labels: map[string]string{
			imageLabelClassicBuilderParent: parentDigest.String(),
		},
	}

	createdImage, err := i.client.ImageService().Update(ctx, img)
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			return nil, err
		}

		if createdImage, err = i.client.ImageService().Create(ctx, img); err != nil {
			return nil, fmt.Errorf("failed to create new image: %w", err)
		}
	}

	if err := i.unpackImage(ctx, i.StorageDriver(), img, commitManifestDesc); err != nil {
		return nil, err
	}

	newImage := dimage.Clone(imgToCreate, dimage.ID(createdImage.Target.Digest))
	return newImage, nil
}
