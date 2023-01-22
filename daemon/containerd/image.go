package containerd

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	cplatforms "github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	containertypes "github.com/docker/docker/api/types/container"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/platforms"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

var truncatedID = regexp.MustCompile(`^([a-f0-9]{4,64})$`)

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(ctx context.Context, refOrID string, options imagetype.GetImageOpts) (*image.Image, error) {
	desc, err := i.resolveDescriptor(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	platform := platforms.AllPlatformsWithPreference(cplatforms.Default())
	if options.Platform != nil {
		platform = cplatforms.OnlyStrict(*options.Platform)
	}

	cs := i.client.ContentStore()
	conf, err := containerdimages.Config(ctx, cs, desc, platform)
	if err != nil {
		return nil, err
	}

	imageConfigBytes, err := content.ReadBlob(ctx, cs, conf)
	if err != nil {
		return nil, err
	}

	var ociimage ocispec.Image
	if err := json.Unmarshal(imageConfigBytes, &ociimage); err != nil {
		return nil, err
	}

	rootfs := image.NewRootFS()
	for _, id := range ociimage.RootFS.DiffIDs {
		rootfs.Append(layer.DiffID(id))
	}
	exposedPorts := make(nat.PortSet, len(ociimage.Config.ExposedPorts))
	for k, v := range ociimage.Config.ExposedPorts {
		exposedPorts[nat.Port(k)] = v
	}

	img := image.NewImage(image.ID(desc.Digest))
	img.V1Image = image.V1Image{
		ID:           string(desc.Digest),
		OS:           ociimage.OS,
		Architecture: ociimage.Architecture,
		Config: &containertypes.Config{
			Entrypoint:   ociimage.Config.Entrypoint,
			Env:          ociimage.Config.Env,
			Cmd:          ociimage.Config.Cmd,
			User:         ociimage.Config.User,
			WorkingDir:   ociimage.Config.WorkingDir,
			ExposedPorts: exposedPorts,
			Volumes:      ociimage.Config.Volumes,
			Labels:       ociimage.Config.Labels,
			StopSignal:   ociimage.Config.StopSignal,
		},
	}

	img.RootFS = rootfs

	if options.Details {
		lastUpdated := time.Unix(0, 0)
		size, err := i.size(ctx, desc, platform)
		if err != nil {
			return nil, err
		}

		tagged, err := i.client.ImageService().List(ctx, "target.digest=="+desc.Digest.String())
		if err != nil {
			return nil, err
		}
		tags := make([]reference.Named, 0, len(tagged))
		for _, i := range tagged {
			if i.UpdatedAt.After(lastUpdated) {
				lastUpdated = i.UpdatedAt
			}
			name, err := reference.ParseNamed(i.Name)
			if err != nil {
				return nil, err
			}
			tags = append(tags, name)
		}

		img.Details = &image.Details{
			References:  tags,
			Size:        size,
			Metadata:    nil,
			Driver:      i.snapshotter,
			LastUpdated: lastUpdated,
		}
	}

	return img, nil
}

// size returns the total size of the image's packed resources.
func (i *ImageService) size(ctx context.Context, desc ocispec.Descriptor, platform cplatforms.MatchComparer) (int64, error) {
	var size int64

	cs := i.client.ContentStore()
	handler := containerdimages.LimitManifests(containerdimages.ChildrenHandler(cs), platform, 1)

	var wh containerdimages.HandlerFunc = func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		children, err := handler(ctx, desc)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				return nil, err
			}
		}

		atomic.AddInt64(&size, desc.Size)

		return children, nil
	}

	l := semaphore.NewWeighted(3)
	if err := containerdimages.Dispatch(ctx, wh, l, desc); err != nil {
		return 0, err
	}

	return size, nil
}

// resolveDescriptor searches for a descriptor based on the given
// reference or identifier. Returns the descriptor of
// the image, which could be a manifest list, manifest, or config.
func (i *ImageService) resolveDescriptor(ctx context.Context, refOrID string) (ocispec.Descriptor, error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return ocispec.Descriptor{}, errdefs.InvalidParameter(err)
	}

	is := i.client.ImageService()

	digested, ok := parsed.(reference.Digested)
	if ok {
		imgs, err := is.List(ctx, "target.digest=="+digested.Digest().String())
		if err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "failed to lookup digest")
		}
		if len(imgs) == 0 {
			return ocispec.Descriptor{}, images.ErrImageDoesNotExist{Ref: parsed}
		}

		return imgs[0].Target, nil
	}

	ref := reference.TagNameOnly(parsed.(reference.Named)).String()

	// If the identifier could be a short ID, attempt to match
	if truncatedID.MatchString(refOrID) {
		filters := []string{
			fmt.Sprintf("name==%q", ref), // Or it could just look like one.
			"target.digest~=" + strconv.Quote(fmt.Sprintf(`^sha256:%s[0-9a-fA-F]{%d}$`, regexp.QuoteMeta(refOrID), 64-len(refOrID))),
		}
		imgs, err := is.List(ctx, filters...)
		if err != nil {
			return ocispec.Descriptor{}, err
		}

		if len(imgs) == 0 {
			return ocispec.Descriptor{}, images.ErrImageDoesNotExist{Ref: parsed}
		}
		if len(imgs) > 1 {
			digests := map[digest.Digest]struct{}{}
			for _, img := range imgs {
				if img.Name == ref {
					return img.Target, nil
				}
				digests[img.Target.Digest] = struct{}{}
			}

			if len(digests) > 1 {
				return ocispec.Descriptor{}, errdefs.NotFound(errors.New("ambiguous reference"))
			}
		}

		return imgs[0].Target, nil
	}

	img, err := is.Get(ctx, ref)
	if err != nil {
		// TODO(containerd): error translation can use common function
		if !cerrdefs.IsNotFound(err) {
			return ocispec.Descriptor{}, err
		}
		return ocispec.Descriptor{}, images.ErrImageDoesNotExist{Ref: parsed}
	}

	return img.Target, nil
}
