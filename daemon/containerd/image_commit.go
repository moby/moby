package containerd

import (
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
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/cleanup"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/image"
	imagespec "github.com/docker/docker/image/spec/specs-go/v1"
	"github.com/docker/docker/internal/compatcontext"
	"github.com/docker/docker/pkg/archive"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
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

	diffLayerDesc, diffID, err := i.createDiff(ctx, cc.ContainerID, sn, cs, differ)
	if err != nil {
		return "", fmt.Errorf("failed to export layer: %w", err)
	}
	imageConfig := generateCommitImageConfig(parentImage, diffID, cc)

	layers := parentManifest.Layers
	if diffLayerDesc != nil {
		rootfsID := identity.ChainID(imageConfig.RootFS.DiffIDs).String()

		if err := i.applyDiffLayer(ctx, rootfsID, cc.ContainerID, sn, differ, *diffLayerDesc); err != nil {
			return "", fmt.Errorf("failed to apply diff: %w", err)
		}

		layers = append(layers, *diffLayerDesc)
	}

	return i.createImageOCI(ctx, imageConfig, digest.Digest(cc.ParentImageID), layers, *cc.ContainerConfig)
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

// createDiff creates a layer diff into containerd's content store.
// If the diff is empty it returns nil empty digest and no error.
func (i *ImageService) createDiff(ctx context.Context, name string, sn snapshots.Snapshotter, cs content.Store, comparer diff.Comparer) (*ocispec.Descriptor, digest.Digest, error) {
	info, err := sn.Stat(ctx, name)
	if err != nil {
		return nil, "", err
	}

	var upper []mount.Mount
	if !i.idMapping.Empty() {
		// The rootfs of the container is remapped if an id mapping exists, we
		// need to "unremap" it before committing the snapshot
		rootPair := i.idMapping.RootPair()
		usernsID := fmt.Sprintf("%s-%d-%d-%s", name, rootPair.UID, rootPair.GID, uniquePart())
		remappedID := usernsID + remapSuffix
		baseName := name

		if info.Kind == snapshots.KindActive {
			source, err := sn.Mounts(ctx, name)
			if err != nil {
				return nil, "", err
			}

			// No need to use parent since the whole snapshot is copied.
			// Using parent would require doing diff/apply while starting
			// from empty can just copy the whole snapshot.
			// TODO: Optimize this for overlay mounts, can use parent
			// and just copy upper directories without mounting
			upper, err = sn.Prepare(ctx, remappedID, "")
			if err != nil {
				return nil, "", err
			}

			if err := i.copyAndUnremapRootFS(ctx, upper, source); err != nil {
				return nil, "", err
			}
		} else {
			upper, err = sn.Prepare(ctx, remappedID, baseName)
			if err != nil {
				return nil, "", err
			}

			if err := i.unremapRootFS(ctx, upper); err != nil {
				return nil, "", err
			}
		}
	} else {
		if info.Kind == snapshots.KindActive {
			upper, err = sn.Mounts(ctx, name)
			if err != nil {
				return nil, "", err
			}
		} else {
			upperKey := fmt.Sprintf("%s-view-%s", name, uniquePart())
			upper, err = sn.View(ctx, upperKey, name)
			if err != nil {
				return nil, "", err
			}
			defer cleanup.Do(ctx, func(ctx context.Context) {
				sn.Remove(ctx, upperKey)
			})
		}
	}

	lowerKey := fmt.Sprintf("%s-parent-view-%s", info.Parent, uniquePart())
	lower, err := sn.View(ctx, lowerKey, info.Parent)
	if err != nil {
		return nil, "", err
	}
	defer cleanup.Do(ctx, func(ctx context.Context) {
		sn.Remove(ctx, lowerKey)
	})

	newDesc, err := comparer.Compare(ctx, lower, upper)
	if err != nil {
		return nil, "", errors.Wrap(err, "CreateDiff")
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

	cinfo, err := cs.Info(ctx, newDesc.Digest)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get content info: %w", err)
	}

	diffIDStr, ok := cinfo.Labels["containerd.io/uncompressed"]
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
		Size:      cinfo.Size,
	}, diffID, nil
}

// applyDiffLayer will apply diff layer content created by createDiff into the snapshotter.
func (i *ImageService) applyDiffLayer(ctx context.Context, name string, containerID string, sn snapshots.Snapshotter, differ diff.Applier, diffDesc ocispec.Descriptor) (retErr error) {
	// Let containerd know that this snapshot is only for diff-applying.
	key := snapshots.UnpackKeyPrefix + "-" + uniquePart() + "-" + name

	info, err := sn.Stat(ctx, containerID)
	if err != nil {
		return err
	}

	mounts, err := sn.Prepare(ctx, key, info.Parent)
	if err != nil {
		return fmt.Errorf("failed to prepare snapshot: %w", err)
	}

	defer func() {
		if retErr != nil {
			// NOTE: the snapshotter should be held by lease. Even
			// if the cleanup fails, the containerd gc can delete it.
			if err := sn.Remove(ctx, key); err != nil {
				log.G(ctx).Warnf("failed to cleanup aborted apply %s: %s", key, err)
			}
		}
	}()

	if _, err = differ.Apply(ctx, diffDesc, mounts); err != nil {
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
