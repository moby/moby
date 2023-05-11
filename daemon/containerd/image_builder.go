package containerd

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/containerd/containerd"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/rootfs"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	registrypkg "github.com/docker/docker/registry"

	// "github.com/docker/docker/api/types/container"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/errdefs"
	dimage "github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/system"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

// GetImageAndReleasableLayer returns an image and releaseable layer for a
// reference or ID. Every call to GetImageAndReleasableLayer MUST call
// releasableLayer.Release() to prevent leaking of layers.
func (i *ImageService) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error) {
	if refOrID == "" { // from SCRATCH
		os := runtime.GOOS
		if runtime.GOOS == "windows" {
			os = "linux"
		}
		if opts.Platform != nil {
			os = opts.Platform.OS
		}
		if !system.IsOSSupported(os) {
			return nil, nil, system.ErrNotSupportedOperatingSystem
		}
		return nil, &rolayer{
			key:         "",
			c:           i.client,
			snapshotter: i.snapshotter,
			diffID:      "",
			root:        "",
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
			if !system.IsOSSupported(img.OperatingSystem()) {
				return nil, nil, system.ErrNotSupportedOperatingSystem
			}

			layer, err := newROLayerForImage(ctx, &imgDesc, i, opts, refOrID, opts.Platform)
			if err != nil {
				return nil, nil, err
			}

			return img, layer, nil
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

	layer, err := newROLayerForImage(ctx, imgDesc, i, opts, refOrID, opts.Platform)
	if err != nil {
		return nil, nil, err
	}

	return img, layer, nil
}

func (i *ImageService) pullForBuilder(ctx context.Context, name string, authConfigs map[string]registry.AuthConfig, output io.Writer, platform *ocispec.Platform) (*ocispec.Descriptor, error) {
	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, err
	}
	taggedRef := reference.TagNameOnly(ref)

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

	if err := i.PullImage(ctx, ref.Name(), taggedRef.(reference.NamedTagged).Tag(), platform, nil, pullRegistryAuth, output); err != nil {
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
				logrus.WithError(err).WithField("image", name).Warn("Ignoring error about platform mismatch where the manifest list points to an image whose configuration does not match the platform in the manifest.")
			}
		} else {
			return nil, err
		}
	}

	if !system.IsOSSupported(img.OperatingSystem()) {
		return nil, system.ErrNotSupportedOperatingSystem
	}

	imgDesc, err := i.resolveDescriptor(ctx, name)
	if err != nil {
		return nil, err
	}

	return &imgDesc, err
}

func newROLayerForImage(ctx context.Context, imgDesc *ocispec.Descriptor, i *ImageService, opts backend.GetImageAndLayerOptions, refOrID string, platform *ocispec.Platform) (builder.ROLayer, error) {
	if imgDesc == nil {
		return nil, fmt.Errorf("can't make an RO layer for a nil image :'(")
	}

	platMatcher := platforms.Default()
	if platform != nil {
		platMatcher = platforms.Only(*platform)
	}

	// this needs it's own context + lease so that it doesn't get cleaned before we're ready
	confDesc, err := containerdimages.Config(ctx, i.client.ContentStore(), *imgDesc, platMatcher)
	if err != nil {
		return nil, err
	}

	diffIDs, err := containerdimages.RootFS(ctx, i.client.ContentStore(), confDesc)
	if err != nil {
		return nil, err
	}
	parent := identity.ChainID(diffIDs).String()

	s := i.client.SnapshotService(i.snapshotter)
	key := stringid.GenerateRandomID()
	ctx, _, err = i.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(1*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to create lease for commit: %w", err)
	}
	mounts, err := s.View(ctx, key, parent)
	if err != nil {
		return nil, err
	}

	tempMountLocation := os.TempDir()
	root, err := os.MkdirTemp(tempMountLocation, "rootfs-mount")
	if err != nil {
		return nil, err
	}

	if err := mount.All(mounts, root); err != nil {
		return nil, err
	}

	return &rolayer{
		key:                key,
		c:                  i.client,
		snapshotter:        i.snapshotter,
		diffID:             digest.Digest(parent),
		root:               root,
		contentStoreDigest: "",
	}, nil
}

type rolayer struct {
	key                string
	c                  *containerd.Client
	snapshotter        string
	diffID             digest.Digest
	root               string
	contentStoreDigest digest.Digest
}

func (rl *rolayer) ContentStoreDigest() digest.Digest {
	return rl.contentStoreDigest
}

func (rl *rolayer) DiffID() layer.DiffID {
	if rl.diffID == "" {
		return layer.DigestSHA256EmptyTar
	}
	return layer.DiffID(rl.diffID)
}

func (rl *rolayer) Release() error {
	snapshotter := rl.c.SnapshotService(rl.snapshotter)
	err := snapshotter.Remove(context.TODO(), rl.key)
	if err != nil && !cerrdefs.IsNotFound(err) {
		return err
	}

	if rl.root == "" { // nothing to release
		return nil
	}
	if err := mount.UnmountAll(rl.root, 0); err != nil {
		logrus.WithError(err).WithField("root", rl.root).Error("failed to unmount ROLayer")
		return err
	}
	if err := os.Remove(rl.root); err != nil {
		logrus.WithError(err).WithField("dir", rl.root).Error("failed to remove mount temp dir")
		return err
	}
	rl.root = ""
	return nil
}

// NewRWLayer creates a new read-write layer for the builder
func (rl *rolayer) NewRWLayer() (builder.RWLayer, error) {
	snapshotter := rl.c.SnapshotService(rl.snapshotter)

	// we need this here for the prepared snapshots or
	// we'll have racy behaviour where sometimes they
	// will get GC'd before we commit/use them
	ctx, _, err := rl.c.WithLease(context.TODO(), leases.WithRandomID(), leases.WithExpiration(1*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to create lease for commit: %w", err)
	}

	key := stringid.GenerateRandomID()
	mounts, err := snapshotter.Prepare(ctx, key, rl.diffID.String())
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
	}, nil
}

type rwlayer struct {
	key         string
	parent      string
	c           *containerd.Client
	snapshotter string
	root        string
}

func (rw *rwlayer) Root() string {
	return rw.root
}

func (rw *rwlayer) Commit() (builder.ROLayer, error) {
	// we need this here for the prepared snapshots or
	// we'll have racy behaviour where sometimes they
	// will get GC'd before we commit/use them
	ctx, _, err := rw.c.WithLease(context.TODO(), leases.WithRandomID(), leases.WithExpiration(1*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to create lease for commit: %w", err)
	}
	snapshotter := rw.c.SnapshotService(rw.snapshotter)

	key := stringid.GenerateRandomID()
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
		diffID:             diffID,
		root:               "",
		contentStoreDigest: desc.Digest,
	}, nil
}

func (rw *rwlayer) Release() error {
	snapshotter := rw.c.SnapshotService(rw.snapshotter)
	err := snapshotter.Remove(context.TODO(), rw.key)
	if err != nil && !cerrdefs.IsNotFound(err) {
		return err
	}

	if rw.root == "" { // nothing to release
		return nil
	}
	if err := mount.UnmountAll(rw.root, 0); err != nil {
		logrus.WithError(err).WithField("root", rw.root).Error("failed to unmount ROLayer")
		return err
	}
	if err := os.Remove(rw.root); err != nil {
		logrus.WithError(err).WithField("dir", rw.root).Error("failed to remove mount temp dir")
		return err
	}
	rw.root = ""
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

	rootfs := ocispec.RootFS{
		Type:    imgToCreate.RootFS.Type,
		DiffIDs: []digest.Digest{},
	}
	for _, diffId := range imgToCreate.RootFS.DiffIDs {
		rootfs.DiffIDs = append(rootfs.DiffIDs, digest.Digest(diffId))
	}
	exposedPorts := make(map[string]struct{}, len(imgToCreate.Config.ExposedPorts))
	for k, v := range imgToCreate.Config.ExposedPorts {
		exposedPorts[string(k)] = v
	}

	var ociHistory []ocispec.History
	for _, history := range imgToCreate.History {
		created := history.Created
		ociHistory = append(ociHistory, ocispec.History{
			Created:    &created,
			CreatedBy:  history.CreatedBy,
			Author:     history.Author,
			Comment:    history.Comment,
			EmptyLayer: history.EmptyLayer,
		})
	}

	// make an ocispec.Image from the docker/image.Image
	ociImgToCreate := ocispec.Image{
		Created:      &imgToCreate.Created,
		Author:       imgToCreate.Author,
		Architecture: imgToCreate.Architecture,
		Variant:      imgToCreate.Variant,
		OS:           imgToCreate.OS,
		OSVersion:    imgToCreate.OSVersion,
		OSFeatures:   imgToCreate.OSFeatures,
		Config: ocispec.ImageConfig{
			User:         imgToCreate.Config.User,
			ExposedPorts: exposedPorts,
			Env:          imgToCreate.Config.Env,
			Entrypoint:   imgToCreate.Config.Entrypoint,
			Cmd:          imgToCreate.Config.Cmd,
			Volumes:      imgToCreate.Config.Volumes,
			WorkingDir:   imgToCreate.Config.WorkingDir,
			Labels:       imgToCreate.Config.Labels,
			StopSignal:   imgToCreate.Config.StopSignal,
		},
		RootFS:  rootfs,
		History: ociHistory,
	}

	var layers []ocispec.Descriptor
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
	}

	// get the info for the new layers
	info, err := i.client.ContentStore().Info(ctx, layerDigest)
	if err != nil {
		return nil, err
	}

	// append the new layer descriptor
	layers = append(layers,
		ocispec.Descriptor{
			MediaType: containerdimages.MediaTypeDockerSchema2LayerGzip,
			Digest:    layerDigest,
			Size:      info.Size,
		},
	)

	// necessary to prevent the contents from being GC'd
	// between writing them here and creating an image
	ctx, done, err := i.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(1*time.Hour))
	if err != nil {
		return nil, err
	}
	defer done(ctx)

	commitManifestDesc, err := writeContentsForImage(ctx, i.snapshotter, i.client.ContentStore(), ociImgToCreate, layers)
	if err != nil {
		return nil, err
	}

	// image create
	img := containerdimages.Image{
		Name:      danglingImageName(commitManifestDesc.Digest),
		Target:    commitManifestDesc,
		CreatedAt: time.Now(),
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

	if err := i.unpackImage(ctx, createdImage, platforms.DefaultSpec()); err != nil {
		return nil, err
	}

	newImage := dimage.NewImage(dimage.ID(createdImage.Target.Digest))
	newImage.V1Image = imgToCreate.V1Image
	newImage.V1Image.ID = string(createdImage.Target.Digest)
	newImage.History = imgToCreate.History
	return newImage, nil
}
