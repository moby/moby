package containerd

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
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
	desc, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	platform := platforms.AllPlatformsWithPreference(cplatforms.Default())
	if options.Platform != nil {
		platform = cplatforms.OnlyStrict(*options.Platform)
	}

	cs := i.client.ContentStore()

	var presentImages []ocispec.Image
	err = i.walkImageManifests(ctx, desc, func(img *ImageManifest) error {
		conf, err := img.Config(ctx)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				log.G(ctx).WithFields(log.Fields{
					"manifestDescriptor": img.Target(),
				}).Debug("manifest was present, but accessing its config failed, ignoring")
				return nil
			}
			return errdefs.System(fmt.Errorf("failed to get config descriptor: %w", err))
		}

		var ociimage ocispec.Image
		if err := readConfig(ctx, cs, conf, &ociimage); err != nil {
			if cerrdefs.IsNotFound(err) {
				log.G(ctx).WithFields(log.Fields{
					"manifestDescriptor": img.Target(),
					"configDescriptor":   conf,
				}).Debug("manifest present, but its config is missing, ignoring")
				return nil
			}
			return errdefs.System(fmt.Errorf("failed to read config of the manifest %v: %w", img.Target().Digest, err))
		}
		presentImages = append(presentImages, ociimage)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(presentImages) == 0 {
		ref, _ := reference.ParseAnyReference(refOrID)
		return nil, images.ErrImageDoesNotExist{Ref: ref}
	}

	sort.SliceStable(presentImages, func(i, j int) bool {
		return platform.Less(presentImages[i].Platform, presentImages[j].Platform)
	})
	ociimage := presentImages[0]

	rootfs := image.NewRootFS()
	for _, id := range ociimage.RootFS.DiffIDs {
		rootfs.Append(layer.DiffID(id))
	}
	exposedPorts := make(nat.PortSet, len(ociimage.Config.ExposedPorts))
	for k, v := range ociimage.Config.ExposedPorts {
		exposedPorts[nat.Port(k)] = v
	}

	img := image.NewImage(image.ID(desc.Target.Digest))
	img.V1Image = image.V1Image{
		ID:           string(desc.Target.Digest),
		OS:           ociimage.OS,
		Architecture: ociimage.Architecture,
		Variant:      ociimage.Variant,
		Created:      ociimage.Created,
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
	img.History = ociimage.History

	if options.Details {
		lastUpdated := time.Unix(0, 0)
		size, err := i.size(ctx, desc.Target, platform)
		if err != nil {
			return nil, err
		}

		tagged, err := i.client.ImageService().List(ctx, "target.digest=="+desc.Target.Digest.String())
		if err != nil {
			return nil, err
		}

		// Usually each image will result in 2 references (named and digested).
		refs := make([]reference.Named, 0, len(tagged)*2)
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
				// This is inconsistent with `docker image ls` which will
				// still include the malformed name in RepoTags.
				log.G(ctx).WithField("name", name).WithError(err).Error("failed to parse image name as reference")
				continue
			}
			refs = append(refs, name)

			if _, ok := name.(reference.Digested); ok {
				// Image name already contains a digest, so no need to create a digested reference.
				continue
			}

			digested, err := reference.WithDigest(reference.TrimNamed(name), desc.Target.Digest)
			if err != nil {
				// This could only happen if digest is invalid, but considering that
				// we get it from the Descriptor it's highly unlikely.
				// Log error just in case.
				log.G(ctx).WithError(err).Error("failed to create digested reference")
				continue
			}
			refs = append(refs, digested)
		}

		img.Details = &image.Details{
			References:  refs,
			Size:        size,
			Metadata:    nil,
			Driver:      i.snapshotter,
			LastUpdated: lastUpdated,
		}
	}

	return img, nil
}

func (i *ImageService) GetImageManifest(ctx context.Context, refOrID string, options imagetype.GetImageOpts) (*ocispec.Descriptor, error) {
	cs := i.client.ContentStore()

	desc, err := i.resolveDescriptor(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	if containerdimages.IsManifestType(desc.MediaType) {
		return &desc, nil
	}

	if containerdimages.IsIndexType(desc.MediaType) {
		platform := platforms.AllPlatformsWithPreference(cplatforms.Default())
		if options.Platform != nil {
			platform = cplatforms.Only(*options.Platform)
		}

		childManifests, err := containerdimages.LimitManifests(containerdimages.ChildrenHandler(cs), platform, 1)(ctx, desc)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				return nil, errdefs.NotFound(err)
			}
			return nil, errdefs.System(err)
		}

		// len(childManifests) == 1 since we requested 1 and if none
		// were found LimitManifests would have thrown an error
		if !containerdimages.IsManifestType(childManifests[0].MediaType) {
			return nil, errdefs.NotFound(fmt.Errorf("manifest has incorrect mediatype: %s", childManifests[0].MediaType))
		}

		return &childManifests[0], nil
	}

	return nil, errdefs.NotFound(errors.New("failed to find manifest"))
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
	img, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return img.Target, nil
}

func (i *ImageService) resolveImage(ctx context.Context, refOrID string) (containerdimages.Image, error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return containerdimages.Image{}, errdefs.InvalidParameter(err)
	}

	is := i.client.ImageService()

	digested, ok := parsed.(reference.Digested)
	if ok {
		imgs, err := is.List(ctx, "target.digest=="+digested.Digest().String())
		if err != nil {
			return containerdimages.Image{}, errors.Wrap(err, "failed to lookup digest")
		}
		if len(imgs) == 0 {
			return containerdimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
		}

		// If reference is both Named and Digested, make sure we don't match
		// images with a different repository even if digest matches.
		// For example, busybox@sha256:abcdef..., shouldn't match asdf@sha256:abcdef...
		if parsedNamed, ok := parsed.(reference.Named); ok {
			for _, img := range imgs {
				imgNamed, err := reference.ParseNormalizedNamed(img.Name)
				if err != nil {
					log.G(ctx).WithError(err).WithField("image", img.Name).Warn("image with invalid name encountered")
					continue
				}

				if parsedNamed.Name() == imgNamed.Name() {
					return img, nil
				}
			}
			return containerdimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
		}

		return imgs[0], nil
	}

	ref := reference.TagNameOnly(parsed.(reference.Named)).String()
	img, err := is.Get(ctx, ref)
	if err == nil {
		return img, nil
	} else {
		// TODO(containerd): error translation can use common function
		if !cerrdefs.IsNotFound(err) {
			return containerdimages.Image{}, err
		}
	}

	// If the identifier could be a short ID, attempt to match
	if truncatedID.MatchString(refOrID) {
		filters := []string{
			fmt.Sprintf("name==%q", ref), // Or it could just look like one.
			"target.digest~=" + strconv.Quote(fmt.Sprintf(`^sha256:%s[0-9a-fA-F]{%d}$`, regexp.QuoteMeta(refOrID), 64-len(refOrID))),
		}
		imgs, err := is.List(ctx, filters...)
		if err != nil {
			return containerdimages.Image{}, err
		}

		if len(imgs) == 0 {
			return containerdimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
		}
		if len(imgs) > 1 {
			digests := map[digest.Digest]struct{}{}
			for _, img := range imgs {
				if img.Name == ref {
					return img, nil
				}
				digests[img.Target.Digest] = struct{}{}
			}

			if len(digests) > 1 {
				return containerdimages.Image{}, errdefs.NotFound(errors.New("ambiguous reference"))
			}
		}

		return imgs[0], nil
	}

	return containerdimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
}
