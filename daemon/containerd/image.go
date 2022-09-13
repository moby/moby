package containerd

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference"
	containertypes "github.com/docker/docker/api/types/container"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var shortID = regexp.MustCompile(`^([a-f0-9]{4,64})$`)

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(ctx context.Context, refOrID string, options imagetype.GetImageOpts) (*image.Image, error) {
	containerdImage, img, err := i.getImage(ctx, refOrID, options.Platform)
	if err != nil {
		return nil, err
	}

	if options.Details {
		size, err := containerdImage.Size(ctx)
		if err != nil {
			return nil, err
		}

		tagged, err := i.client.ImageService().List(ctx, fmt.Sprintf("target.digest==%s", containerdImage.Target().Digest.String()))
		if err != nil {
			return nil, err
		}
		tags := make([]reference.Named, 0, len(tagged))
		for _, i := range tagged {
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
			LastUpdated: containerdImage.Metadata().UpdatedAt,
		}
	}
	return img, err
}

func (i *ImageService) getImage(ctx context.Context, refOrID string, platform *ocispec.Platform) (containerd.Image, *image.Image, error) {
	// TODO(rumpl): pass the platform
	ctrdimg, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return nil, nil, err
	}
	containerdImage := containerd.NewImage(i.client, ctrdimg)
	provider := i.client.ContentStore()
	conf, err := ctrdimg.Config(ctx, provider, containerdImage.Platform())
	if err != nil {
		return nil, nil, err
	}

	var ociimage ocispec.Image
	imageConfigBytes, err := content.ReadBlob(ctx, containerdImage.ContentStore(), conf)
	if err != nil {
		return nil, nil, err
	}

	if err := json.Unmarshal(imageConfigBytes, &ociimage); err != nil {
		return nil, nil, err
	}

	fs, err := containerdImage.RootFS(ctx)
	if err != nil {
		return nil, nil, err
	}
	rootfs := image.NewRootFS()
	for _, id := range fs {
		rootfs.Append(layer.DiffID(id))
	}
	exposedPorts := make(nat.PortSet, len(ociimage.Config.ExposedPorts))
	for k, v := range ociimage.Config.ExposedPorts {
		exposedPorts[nat.Port(k)] = v
	}

	img := image.NewImage(image.IDFromDigest(ctrdimg.Target.Digest))
	img.V1Image = image.V1Image{
		ID:           string(ctrdimg.Target.Digest),
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
		},
	}
	img.RootFS = rootfs

	return containerdImage, img, nil
}

// resolveImage searches for an image based on the given
// reference or identifier. Returns the descriptor of
// the image, could be manifest list, manifest, or config.
func (i *ImageService) resolveImage(ctx context.Context, refOrID string) (img containerdimages.Image, err error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return containerdimages.Image{}, errdefs.InvalidParameter(err)
	}

	is := i.client.ImageService()

	namedRef, ok := parsed.(reference.Named)
	if !ok {
		digested, ok := parsed.(reference.Digested)
		if !ok {
			return containerdimages.Image{}, errdefs.InvalidParameter(errors.New("bad reference"))
		}

		imgs, err := is.List(ctx, fmt.Sprintf("target.digest==%s", digested.Digest()))
		if err != nil {
			return containerdimages.Image{}, errors.Wrap(err, "failed to lookup digest")
		}
		if len(imgs) == 0 {
			return containerdimages.Image{}, errdefs.NotFound(errors.New("image not found with digest"))
		}

		return imgs[0], nil
	}

	namedRef = reference.TagNameOnly(namedRef)

	// If the identifier could be a short ID, attempt to match
	if shortID.MatchString(refOrID) {
		ref := namedRef.String()
		filters := []string{
			fmt.Sprintf("name==%q", ref),
			fmt.Sprintf(`target.digest~=/sha256:%s[0-9a-fA-F]{%d}/`, refOrID, 64-len(refOrID)),
		}
		imgs, err := is.List(ctx, filters...)
		if err != nil {
			return containerdimages.Image{}, err
		}

		if len(imgs) == 0 {
			return containerdimages.Image{}, errdefs.NotFound(errors.New("list returned no images"))
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

		if imgs[0].Name != ref {
			namedRef = nil
		}
		return imgs[0], nil
	}
	img, err = is.Get(ctx, namedRef.String())
	if err != nil {
		// TODO(containerd): error translation can use common function
		if !cerrdefs.IsNotFound(err) {
			return containerdimages.Image{}, err
		}
		// Some clients (such as the docker 17.03 CLI used in CI) still
		// perform string-matching to detect that the *image* wasn't found.
		// This requires the error to be prefixed with "No such image:".
		// Without this prefix, the CLI prints the error message, and exits.
		//
		// FIXME(thaJeztah): we need to fix this; if needed, gated by API version (API < X add the prefix)
		return containerdimages.Image{}, errdefs.NotFound(errors.New("No such image: " + refOrID))
	}

	return img, nil
}
