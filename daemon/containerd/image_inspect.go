package containerd

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/labels"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	imagetypes "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/storage"
	"github.com/moby/moby/v2/daemon/internal/builder-next/exporter"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/moby/moby/v2/internal/sliceutil"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/semaphore"
)

func (i *ImageService) ImageInspect(ctx context.Context, refOrID string, opts imagebackend.ImageInspectOpts) (*imagebackend.InspectData, error) {
	requestedPlatform := opts.Platform

	c8dImg, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	target := c8dImg.Target
	tagged, err := i.images.List(ctx, "target.digest=="+target.Digest.String())
	if err != nil {
		return nil, err
	}

	// This could happen only if the image was deleted after the resolveImage call above.
	if len(tagged) == 0 {
		return nil, errInconsistentData
	}

	lastUpdated := time.Unix(0, 0)
	for _, i := range tagged {
		if i.UpdatedAt.After(lastUpdated) {
			lastUpdated = i.UpdatedAt
		}
	}

	platform := i.matchRequestedOrDefault(platforms.OnlyStrict, requestedPlatform)
	size, err := i.size(ctx, target, platform)
	if err != nil {
		return nil, err
	}

	multi, err := i.multiPlatformSummary(ctx, c8dImg, platform)
	if err != nil {
		return nil, err
	}

	if multi.Best == nil && requestedPlatform != nil {
		return nil, &errPlatformNotFound{
			imageRef: refOrID,
			wanted:   *requestedPlatform,
		}
	}

	parent, err := i.getImageLabelByDigest(ctx, target.Digest, imageLabelClassicBuilderParent)
	if err != nil {
		log.G(ctx).WithError(err).Warn("failed to determine Parent property")
	}

	var manifests []imagetypes.ManifestSummary
	if opts.Manifests {
		manifests = multi.Manifests
	}

	repoTags, repoDigests := collectRepoTagsAndDigests(ctx, tagged)

	if requestedPlatform != nil {
		target = multi.Best.Target()
	}

	identity, err := i.imageIdentity(ctx, target.Digest)
	if err != nil {
		log.G(ctx).WithError(err).Warn("failed to determine Identity property")
	}

	resp := &imagebackend.InspectData{
		InspectResponse: imagetypes.InspectResponse{
			ID:          target.Digest.String(),
			RepoTags:    repoTags,
			Descriptor:  &target,
			RepoDigests: repoDigests,
			Size:        size,
			Manifests:   manifests,
			Metadata: imagetypes.Metadata{
				LastTagTime: lastUpdated,
			},
			Identity: identity,
		},
		Parent: parent, // field is deprecated with the legacy builder, but returned by the API if present.

		// GraphDriver is omitted in API v1.52 unless using a graphdriver.
		GraphDriverLegacy: &storage.DriverData{Name: i.snapshotter},
	}

	var img dockerspec.DockerOCIImage
	if multi.Best != nil {
		if err := multi.Best.ReadConfig(ctx, &img); err != nil && !cerrdefs.IsNotFound(err) {
			return nil, fmt.Errorf("failed to read image config: %w", err)
		}
	}

	// Copy the config
	imgConfig := img.Config
	resp.Config = &imgConfig

	resp.Author = img.Author
	resp.Architecture = img.Architecture
	resp.Variant = img.Variant
	resp.Os = img.OS
	resp.OsVersion = img.OSVersion

	if len(img.History) > 0 {
		resp.Comment = img.History[len(img.History)-1].Comment
	}

	if img.Created != nil {
		resp.Created = img.Created.Format(time.RFC3339Nano)
	}

	resp.RootFS = imagetypes.RootFS{
		Type: img.RootFS.Type,
	}
	for _, layer := range img.RootFS.DiffIDs {
		resp.RootFS.Layers = append(resp.RootFS.Layers, layer.String())
	}

	return resp, nil
}

func (i *ImageService) imageIdentity(ctx context.Context, dgst digest.Digest) (*imagetypes.ImageIdentity, error) {
	info, err := i.content.Info(ctx, dgst)
	if err != nil {
		return nil, err
	}
	identity := &imagetypes.ImageIdentity{}

	seenRepos := make(map[string]struct{})

	for k, v := range info.Labels {
		if ref, ok := strings.CutPrefix(k, exporter.BuildRefLabel); ok {
			var val exporter.BuildRefLabelValue
			if err := json.Unmarshal([]byte(v), &val); err == nil {
				var createdAt time.Time
				if val.CreatedAt != nil {
					createdAt = *val.CreatedAt
				}
				identity.Build = append(identity.Build, imagetypes.ImageBuildIdentity{
					Ref:       ref,
					CreatedAt: createdAt,
				})
			}
		}
		if registry, ok := strings.CutPrefix(k, labels.LabelDistributionSource+"."); ok {
			for repo := range strings.SplitSeq(v, ",") {
				ref, err := reference.ParseNormalizedNamed(registry + "/" + repo)
				if err != nil {
					log.G(ctx).WithError(err).Error("failed to parse image name as reference")
					continue
				}
				name := ref.Name()
				if _, ok := seenRepos[name]; ok {
					continue
				}
				seenRepos[name] = struct{}{}
				identity.Pull = append(identity.Pull, imagetypes.ImagePullIdentity{
					Repository: name,
				})
			}
		}
	}

	// return nil if there is no identity information
	if len(identity.Build) == 0 && len(identity.Pull) == 0 && len(identity.Signature) == 0 {
		return nil, nil
	}

	slices.SortFunc(identity.Build, func(a, b imagetypes.ImageBuildIdentity) int {
		return cmp.Compare(a.Ref, b.Ref)
	})

	return identity, nil
}

func collectRepoTagsAndDigests(ctx context.Context, tagged []c8dimages.Image) (repoTags []string, repoDigests []string) {
	repoTags = make([]string, 0, len(tagged))
	repoDigests = make([]string, 0, len(tagged))
	for _, img := range tagged {
		if isDanglingImage(img) {
			if len(tagged) > 1 {
				// This is unexpected - dangling image should be deleted
				// as soon as another image with the same target is created.
				// Log a warning, but don't error out the whole operation.
				log.G(ctx).WithField("refs", tagged).Warn("multiple images have the same target, but one of them is still dangling")
			}
			continue
		}

		name, err := reference.ParseNamed(img.Name)
		if err != nil {
			log.G(ctx).WithField("name", name).WithError(err).Error("failed to parse image name as reference")
			// Include the malformed name in RepoTags to be consistent with `docker image ls`.
			repoTags = append(repoTags, img.Name)
			continue
		}

		repoTags = append(repoTags, reference.FamiliarString(name))
		if _, ok := name.(reference.Digested); ok {
			repoDigests = append(repoDigests, reference.FamiliarString(name))
			// Image name is a digested reference already, so no need to create a digested reference.
			continue
		}

		digested, err := reference.WithDigest(reference.TrimNamed(name), img.Target.Digest)
		if err != nil {
			// This could only happen if digest is invalid, but considering that
			// we get it from the Descriptor it's highly unlikely.
			// Log error just in case.
			log.G(ctx).WithError(err).Error("failed to create digested reference")
			continue
		}
		repoDigests = append(repoDigests, reference.FamiliarString(digested))
	}
	return sliceutil.Dedup(repoTags), sliceutil.Dedup(repoDigests)
}

// size returns the total size of the image's packed resources.
func (i *ImageService) size(ctx context.Context, desc ocispec.Descriptor, platform platforms.MatchComparer) (int64, error) {
	var size atomic.Int64

	cs := i.content
	handler := c8dimages.LimitManifests(c8dimages.ChildrenHandler(cs), platform, 1)

	var wh c8dimages.HandlerFunc = func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		children, err := handler(ctx, desc)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				return nil, err
			}
		}

		size.Add(desc.Size)

		return children, nil
	}

	l := semaphore.NewWeighted(3)
	if err := c8dimages.Dispatch(ctx, wh, l, desc); err != nil {
		return 0, err
	}

	return size.Load(), nil
}
