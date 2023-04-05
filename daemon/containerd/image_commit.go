package containerd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/api/types/backend"
	containerapi "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
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

	imageManifestBytes, err := content.ReadBlob(ctx, cs, *container.ImageManifest)
	if err != nil {
		return "", err
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(imageManifestBytes, &manifest); err != nil {
		return "", err
	}

	imageConfigBytes, err := content.ReadBlob(ctx, cs, manifest.Config)
	if err != nil {
		return "", err
	}
	var ociimage ocispec.Image
	if err := json.Unmarshal(imageConfigBytes, &ociimage); err != nil {
		return "", err
	}

	var (
		differ = i.client.DiffService()
		sn     = i.client.SnapshotService(container.Driver)
	)

	// Don't gc me and clean the dirty data after 1 hour!
	ctx, done, err := i.client.WithLease(ctx, leases.WithRandomID(), leases.WithExpiration(1*time.Hour))
	if err != nil {
		return "", fmt.Errorf("failed to create lease for commit: %w", err)
	}
	defer done(ctx)

	diffLayerDesc, diffID, err := createDiff(ctx, cc.ContainerID, sn, cs, differ)
	if err != nil {
		return "", fmt.Errorf("failed to export layer: %w", err)
	}

	imageConfig, err := generateCommitImageConfig(ctx, container.Config, ociimage, diffID, cc)
	if err != nil {
		return "", fmt.Errorf("failed to generate commit image config: %w", err)
	}

	rootfsID := identity.ChainID(imageConfig.RootFS.DiffIDs).String()
	if err := applyDiffLayer(ctx, rootfsID, ociimage, sn, differ, diffLayerDesc); err != nil {
		return "", fmt.Errorf("failed to apply diff: %w", err)
	}

	layers := append(manifest.Layers, diffLayerDesc)
	commitManifestDesc, err := writeContentsForImage(ctx, container.Driver, cs, imageConfig, layers)
	if err != nil {
		return "", err
	}

	// image create
	img := images.Image{
		Name:      danglingImageName(commitManifestDesc.Digest),
		Target:    commitManifestDesc,
		CreatedAt: time.Now(),
	}

	if _, err := i.client.ImageService().Update(ctx, img); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return "", err
		}

		if _, err := i.client.ImageService().Create(ctx, img); err != nil {
			return "", fmt.Errorf("failed to create new image: %w", err)
		}
	}
	return image.ID(img.Target.Digest), nil
}

// generateCommitImageConfig returns commit oci image config based on the container's image.
func generateCommitImageConfig(ctx context.Context, container *containerapi.Config, baseConfig ocispec.Image, diffID digest.Digest, opts backend.CommitConfig) (ocispec.Image, error) {
	if opts.Config.Cmd != nil {
		baseConfig.Config.Cmd = opts.Config.Cmd
	}
	if opts.Config.Entrypoint != nil {
		baseConfig.Config.Entrypoint = opts.Config.Entrypoint
	}
	if opts.Author == "" {
		opts.Author = baseConfig.Author
	}

	createdTime := time.Now()
	arch := baseConfig.Architecture
	if arch == "" {
		arch = runtime.GOARCH
		logrus.Warnf("assuming arch=%q", arch)
	}
	os := baseConfig.OS
	if os == "" {
		os = runtime.GOOS
		logrus.Warnf("assuming os=%q", os)
	}
	logrus.Debugf("generateCommitImageConfig(): arch=%q, os=%q", arch, os)
	return ocispec.Image{
		Architecture: arch,
		OS:           os,
		Created:      &createdTime,
		Author:       opts.Author,
		Config:       baseConfig.Config,
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: append(baseConfig.RootFS.DiffIDs, diffID),
		},
		History: append(baseConfig.History, ocispec.History{
			Created:    &createdTime,
			CreatedBy:  "", // FIXME(ndeloof) ?
			Author:     opts.Author,
			Comment:    opts.Comment,
			EmptyLayer: diffID == "",
		}),
	}, nil
}

// writeContentsForImage will commit oci image config and manifest into containerd's content store.
func writeContentsForImage(ctx context.Context, snName string, cs content.Store, newConfig ocispec.Image, layers []ocispec.Descriptor) (ocispec.Descriptor, error) {
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
func createDiff(ctx context.Context, name string, sn snapshots.Snapshotter, cs content.Store, comparer diff.Comparer) (ocispec.Descriptor, digest.Digest, error) {
	newDesc, err := rootfs.CreateDiff(ctx, name, sn, comparer)
	if err != nil {
		return ocispec.Descriptor{}, "", err
	}

	info, err := cs.Info(ctx, newDesc.Digest)
	if err != nil {
		return ocispec.Descriptor{}, "", err
	}

	diffIDStr, ok := info.Labels["containerd.io/uncompressed"]
	if !ok {
		return ocispec.Descriptor{}, "", fmt.Errorf("invalid differ response with no diffID")
	}

	diffID, err := digest.Parse(diffIDStr)
	if err != nil {
		return ocispec.Descriptor{}, "", err
	}

	return ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Digest:    newDesc.Digest,
		Size:      info.Size,
	}, diffID, nil
}

// applyDiffLayer will apply diff layer content created by createDiff into the snapshotter.
func applyDiffLayer(ctx context.Context, name string, baseImg ocispec.Image, sn snapshots.Snapshotter, differ diff.Applier, diffDesc ocispec.Descriptor) (retErr error) {
	var (
		key    = uniquePart() + "-" + name
		parent = identity.ChainID(baseImg.RootFS.DiffIDs).String()
	)

	mount, err := sn.Prepare(ctx, key, parent)
	if err != nil {
		return err
	}

	defer func() {
		if retErr != nil {
			// NOTE: the snapshotter should be hold by lease. Even
			// if the cleanup fails, the containerd gc can delete it.
			if err := sn.Remove(ctx, key); err != nil {
				logrus.Warnf("failed to cleanup aborted apply %s: %s", key, err)
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
	return "", errdefs.NotImplemented(errors.New("not implemented"))
}
