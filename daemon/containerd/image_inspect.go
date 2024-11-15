// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

package containerd

import (
	"context"
	"sync/atomic"
	"time"

	containerdimages "github.com/containerd/containerd/images"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/storage"
	"github.com/docker/docker/internal/sliceutil"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/semaphore"
)

func (i *ImageService) ImageInspect(ctx context.Context, refOrID string, _ backend.ImageInspectOpts) (*imagetypes.InspectResponse, error) {
	img, err := i.GetImage(ctx, refOrID, backend.GetImageOpts{})
	if err != nil {
		return nil, err
	}

	lastUpdated := time.Unix(0, 0)

	tagged, err := i.images.List(ctx, "target.digest=="+img.ImageID())
	if err != nil {
		return nil, err
	}

	// This could happen only if the image was deleted after the GetImage call above.
	if len(tagged) == 0 {
		return nil, errInconsistentData
	}

	platform := matchAllWithPreference(platforms.Default())

	size, err := i.size(ctx, tagged[0].Target, platform)
	if err != nil {
		return nil, err
	}
	imgDgst := tagged[0].Target.Digest

	repoTags := make([]string, 0, len(tagged))
	repoDigests := make([]string, 0, len(tagged))
	for _, i := range tagged {
		if i.UpdatedAt.After(lastUpdated) {
			lastUpdated = i.UpdatedAt
		}
		if isDanglingImage(i) {
			if len(tagged) > 1 {
				// This is unexpected - dangling image should be deleted
				// as soon as another image with the same target is created.
				// Log a warning, but don't error out the whole operation.
				log.G(ctx).WithField("refs", tagged).Warn("multiple images have the same target, but one of them is still dangling")
			}
			continue
		}

		name, err := reference.ParseNamed(i.Name)
		if err != nil {
			log.G(ctx).WithField("name", name).WithError(err).Error("failed to parse image name as reference")
			// Include the malformed name in RepoTags to be consistent with `docker image ls`.
			repoTags = append(repoTags, i.Name)
			continue
		}

		repoTags = append(repoTags, reference.FamiliarString(name))
		if _, ok := name.(reference.Digested); ok {
			repoDigests = append(repoDigests, reference.FamiliarString(name))
			// Image name is a digested reference already, so no need to create a digested reference.
			continue
		}

		digested, err := reference.WithDigest(reference.TrimNamed(name), imgDgst)
		if err != nil {
			// This could only happen if digest is invalid, but considering that
			// we get it from the Descriptor it's highly unlikely.
			// Log error just in case.
			log.G(ctx).WithError(err).Error("failed to create digested reference")
			continue
		}
		repoDigests = append(repoDigests, reference.FamiliarString(digested))
	}

	var comment string
	if len(comment) == 0 && len(img.History) > 0 {
		comment = img.History[len(img.History)-1].Comment
	}

	var created string
	if img.Created != nil {
		created = img.Created.Format(time.RFC3339Nano)
	}

	var layers []string
	for _, layer := range img.RootFS.DiffIDs {
		layers = append(layers, layer.String())
	}

	return &imagetypes.InspectResponse{
		ID:            img.ImageID(),
		RepoTags:      repoTags,
		RepoDigests:   sliceutil.Dedup(repoDigests),
		Parent:        img.Parent.String(),
		Comment:       comment,
		Created:       created,
		DockerVersion: "",
		Author:        img.Author,
		Config:        img.Config,
		Architecture:  img.Architecture,
		Variant:       img.Variant,
		Os:            img.OS,
		OsVersion:     img.OSVersion,
		Size:          size,
		GraphDriver: storage.DriverData{
			Name: i.snapshotter,
			Data: nil,
		},
		RootFS: imagetypes.RootFS{
			Type:   img.RootFS.Type,
			Layers: layers,
		},
		Metadata: imagetypes.Metadata{
			LastTagTime: lastUpdated,
		},
	}, nil
}

// size returns the total size of the image's packed resources.
func (i *ImageService) size(ctx context.Context, desc ocispec.Descriptor, platform platforms.MatchComparer) (int64, error) {
	var size atomic.Int64

	cs := i.content
	handler := containerdimages.LimitManifests(containerdimages.ChildrenHandler(cs), platform, 1)

	var wh containerdimages.HandlerFunc = func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
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
	if err := containerdimages.Dispatch(ctx, wh, l, desc); err != nil {
		return 0, err
	}

	return size.Load(), nil
}
