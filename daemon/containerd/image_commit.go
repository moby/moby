package containerd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/image"
	imagespec "github.com/docker/docker/image/spec/specs-go/v1"
	"github.com/docker/docker/internal/compatcontext"
	"github.com/docker/docker/pkg/archive"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

/*
This code is based on `commit` support in nerdctl, under Apache License
https://github.com/containerd/nerdctl/blob/master/pkg/imgutil/commit/commit.go
with adaptations to match the Moby data model and services.
*/

// CommitImage creates a new image from a commit config.
func (i *ImageService) CommitImage(ctx context.Context, cc backend.CommitConfig) (image.ID, error) {
	container := i.containers.Get(cc.ContainerID)
	cs := i.client.ContentStore()

	var parentManifest ocispec.Manifest
	var parentImage imagespec.DockerOCIImage

	// ImageManifest can be nil when committing an image with base FROM scratch
	if container.ImageManifest != nil {
		imageManifestBytes, err := content.ReadBlob(ctx, cs, *container.ImageManifest)
		if err != nil {
			return "", err
		}

		if err := json.Unmarshal(imageManifestBytes, &parentManifest); err != nil {
			return "", err
		}

		imageConfigBytes, err := content.ReadBlob(ctx, cs, parentManifest.Config)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(imageConfigBytes, &parentImage); err != nil {
			return "", err
		}
	}

	var (
		differ = i.client.DiffService()
		sn     = i.client.SnapshotService(container.Driver)
	)

	// Don't gc me and clean the dirty data after 1 hour!
	ctx, release, err := i.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(1*time.Hour))
	if err != nil {
		return "", fmt.Errorf("failed to create lease for commit: %w", err)
	}
	defer func() {
		if err := release(compatcontext.WithoutCancel(ctx)); err != nil {
			log.G(ctx).WithError(err).Warn("failed to release lease created for commit")
		}
	}()

	diffLayerDesc, diffID, err := createDiff(ctx, cc.ContainerID, sn, cs, differ)
	if err != nil {
		return "", fmt.Errorf("failed to export layer: %w", err)
	}
	imageConfig := generateCommitImageConfig(parentImage, diffID, cc)

	layers := parentManifest.Layers
	if diffLayerDesc != nil {
		rootfsID := identity.ChainID(imageConfig.RootFS.DiffIDs).String()

		if err := applyDiffLayer(ctx, rootfsID, parentImage, sn, differ, *diffLayerDesc); err != nil {
			return "", fmt.Errorf("failed to apply diff: %w", err)
		}

		layers = append(layers, *diffLayerDesc)
	}

	commitManifestDesc, err := writeContentsForImage(ctx, container.Driver, cs, imageConfig, layers)
	if err != nil {
		return "", err
	}

	// image create
	img := images.Image{
		Name:      danglingImageName(commitManifestDesc.Digest),
		Target:    commitManifestDesc,
		CreatedAt: time.Now(),
		Labels: map[string]string{
			imageLabelClassicBuilderParent: cc.ParentImageID,
		},
	}

	if _, err := i.client.ImageService().Update(ctx, img); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return "", err
		}

		if _, err := i.client.ImageService().Create(ctx, img); err != nil {
			return "", fmt.Errorf("failed to create new image: %w", err)
		}
	}
	id := image.ID(img.Target.Digest)

	c8dImg, err := i.NewImageManifest(ctx, img, commitManifestDesc)
	if err != nil {
		return id, err
	}
	if err := c8dImg.Unpack(ctx, container.Driver); err != nil && !cerrdefs.IsAlreadyExists(err) {
		return id, fmt.Errorf("failed to unpack image: %w", err)
	}

	return id, nil
}

// generateCommitImageConfig generates an OCI Image config based on the
// container's image and the CommitConfig options.
func generateCommitImageConfig(baseConfig imagespec.DockerOCIImage, diffID digest.Digest, opts backend.CommitConfig) imagespec.DockerOCIImage {
	if opts.Author == "" {
		opts.Author = baseConfig.Author
	}

	createdTime := time.Now()
	arch := baseConfig.Architecture
	if arch == "" {
		arch = runtime.GOARCH
		log.G(context.TODO()).Warnf("assuming arch=%q", arch)
	}
	os := baseConfig.OS
	if os == "" {
		os = runtime.GOOS
		log.G(context.TODO()).Warnf("assuming os=%q", os)
	}
	log.G(context.TODO()).Debugf("generateCommitImageConfig(): arch=%q, os=%q", arch, os)

	diffIds := baseConfig.RootFS.DiffIDs
	if diffID != "" {
		diffIds = append(diffIds, diffID)
	}

	return imagespec.DockerOCIImage{
		Image: ocispec.Image{
			Platform: ocispec.Platform{
				Architecture: arch,
				OS:           os,
			},
			Created: &createdTime,
			Author:  opts.Author,
			RootFS: ocispec.RootFS{
				Type:    "layers",
				DiffIDs: diffIds,
			},
			History: append(baseConfig.History, ocispec.History{
				Created:    &createdTime,
				CreatedBy:  strings.Join(opts.ContainerConfig.Cmd, " "),
				Author:     opts.Author,
				Comment:    opts.Comment,
				EmptyLayer: diffID == "",
			}),
		},
		Config: containerConfigToDockerOCIImageConfig(opts.Config),
	}
}

// writeContentsForImage will commit oci image config and manifest into containerd's content store.
func writeContentsForImage(ctx context.Context, snName string, cs content.Store, newConfig imagespec.DockerOCIImage, layers []ocispec.Descriptor) (ocispec.Descriptor, error) {
	newConfigJSON, err := json.Marshal(newConfig)
	if err != nil {
		return ocispec.Descriptor{}, err
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
		return ocispec.Descriptor{}, err
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
		return ocispec.Descriptor{}, err
	}

	// config should reference to snapshotter
	labelOpt := content.WithLabels(map[string]string{
		fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", snName): identity.ChainID(newConfig.RootFS.DiffIDs).String(),
	})
	err = content.WriteBlob(ctx, cs, configDesc.Digest.String(), bytes.NewReader(newConfigJSON), configDesc, labelOpt)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return newMfstDesc, nil
}

// createDiff creates a layer diff into containerd's content store.
// If the diff is empty it returns nil empty digest and no error.
func createDiff(ctx context.Context, name string, sn snapshots.Snapshotter, cs content.Store, comparer diff.Comparer) (*ocispec.Descriptor, digest.Digest, error) {
	newDesc, err := rootfs.CreateDiff(ctx, name, sn, comparer)
	if err != nil {
		return nil, "", err
	}

	ra, err := cs.ReaderAt(ctx, newDesc)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read diff archive: %w", err)
	}
	defer ra.Close()

	empty, err := archive.IsEmpty(content.NewReader(ra))
	if err != nil {
		return nil, "", fmt.Errorf("failed to check if archive is empty: %w", err)
	}
	if empty {
		return nil, "", nil
	}

	info, err := cs.Info(ctx, newDesc.Digest)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get content info: %w", err)
	}

	diffIDStr, ok := info.Labels["containerd.io/uncompressed"]
	if !ok {
		return nil, "", fmt.Errorf("invalid differ response with no diffID")
	}

	diffID, err := digest.Parse(diffIDStr)
	if err != nil {
		return nil, "", err
	}

	return &ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Digest:    newDesc.Digest,
		Size:      info.Size,
	}, diffID, nil
}

// applyDiffLayer will apply diff layer content created by createDiff into the snapshotter.
func applyDiffLayer(ctx context.Context, name string, baseImg imagespec.DockerOCIImage, sn snapshots.Snapshotter, differ diff.Applier, diffDesc ocispec.Descriptor) (retErr error) {
	var (
		key    = uniquePart() + "-" + name
		parent = identity.ChainID(baseImg.RootFS.DiffIDs).String()
	)

	mount, err := sn.Prepare(ctx, key, parent)
	if err != nil {
		return fmt.Errorf("failed to prepare snapshot: %w", err)
	}

	defer func() {
		if retErr != nil {
			// NOTE: the snapshotter should be hold by lease. Even
			// if the cleanup fails, the containerd gc can delete it.
			if err := sn.Remove(ctx, key); err != nil {
				log.G(ctx).Warnf("failed to cleanup aborted apply %s: %s", key, err)
			}
		}
	}()

	if _, err = differ.Apply(ctx, diffDesc, mount); err != nil {
		return err
	}

	if err = sn.Commit(ctx, name, key); err != nil {
		if cerrdefs.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

// copied from github.com/containerd/containerd/rootfs/apply.go
func uniquePart() string {
	t := time.Now()
	var b [3]byte
	// Ignore read failures, just decreases uniqueness
	rand.Read(b[:])
	return fmt.Sprintf("%d-%s", t.Nanosecond(), base64.URLEncoding.EncodeToString(b[:]))
}

// CommitBuildStep is used by the builder to create an image for each step in
// the build.
//
// This method is different from CreateImageFromContainer:
//   - it doesn't attempt to validate container state
//   - it doesn't send a commit action to metrics
//   - it doesn't log a container commit event
//
// This is a temporary shim. Should be removed when builder stops using commit.
func (i *ImageService) CommitBuildStep(ctx context.Context, c backend.CommitConfig) (image.ID, error) {
	ctr := i.containers.Get(c.ContainerID)
	if ctr == nil {
		// TODO: use typed error
		return "", fmt.Errorf("container not found: %s", c.ContainerID)
	}
	c.ContainerMountLabel = ctr.MountLabel
	c.ContainerOS = ctr.OS
	c.ParentImageID = string(ctr.ImageID)
	return i.CommitImage(ctx, c)
}
