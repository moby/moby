package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ErrImageDoesNotExist is error returned when no image can be found for a reference.
type ErrImageDoesNotExist struct {
	ref reference.Reference
}

func (e ErrImageDoesNotExist) Error() string {
	ref := e.ref
	if named, ok := ref.(reference.Named); ok {
		ref = reference.TagNameOnly(named)
	}
	return fmt.Sprintf("No such image: %s", reference.FamiliarString(ref))
}

// NotFound implements the NotFound interface
func (e ErrImageDoesNotExist) NotFound() {}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(refOrID string) (*image.Image, error) {
	ref, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	var target ocispec.Descriptor
	cs := i.client.ContentStore()
	references := []ocispec.Descriptor{}

	namedRef, ok := ref.(reference.Named)
	if !ok {
		digested, ok := ref.(reference.Digested)
		if !ok {
			return nil, ErrImageDoesNotExist{ref}
		}

		target.Digest = digested.Digest()

	} else {
		img, err := i.client.ImageService().Get(context.TODO(), namedRef.String())
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return nil, errors.Wrapf(err, "unable to get image: %q", namedRef.String())
			}
			// TODO: If not found here, get all hashes of config and search for best match
			// Search based on ID
			//if id, err := i.imageStore.Search(refOrID); err == nil {
			//	img, err := i.imageStore.Get(id)
			//	if err != nil {
			//		return nil, ErrImageDoesNotExist{ref}
			//	}
			//	return img, nil
			//}
			return nil, ErrImageDoesNotExist{ref}
		} else {
			// TODO: Choose correct platform
			d, err := images.Config(context.TODO(), cs, img.Target, platforms.Default())
			if err != nil {
				if errdefs.IsNotFound(err) {
					return nil, ErrImageDoesNotExist{ref}
				}
				return nil, errors.Wrap(err, "unable to resolve image")
			}
			target = d
			references = append(references, img.Target)
		}
	}

	// TODO(containerd): Move the reference setting and resolution
	img, err := i.getImage(context.TODO(), target)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, ErrImageDoesNotExist{ref}
		}
		return nil, err
	}
	img.References = references

	return img, nil
}

func (i *ImageService) getImage(ctx context.Context, target ocispec.Descriptor) (*image.Image, error) {
	cs := i.client.ContentStore()

	// TODO(containerd): If not config, resolve
	b, err := content.ReadBlob(ctx, cs, target)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read target blob")
	}

	var img ocispec.Image
	if err := json.Unmarshal(b, &img); err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal image config")
	}

	// TODO(containerd): read labels from blob to get parent and Docker calculated size
	return &image.Image{
		Config: target,
		Image:  &img,
	}, nil
}
