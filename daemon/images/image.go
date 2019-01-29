package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	LabelImageID     = "docker.io/image.id"
	LabelImageParent = "docker.io/image.parent"
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

func (i *ImageService) GetImage(ctx context.Context, refOrID string) (ocispec.Descriptor, error) {
	ci, err := i.getCachedRef(ctx, refOrID)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return ci.config, nil
}

// GetImage returns an image corresponding to the image referred to by refOrID.
// Deprecated: Use (i *ImageService).GetImage instead.
func (i *ImageService) getDockerImage(refOrID string) (*image.Image, error) {
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

// TODO(containerd): remove or replace this function to return local type
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

func (i *ImageService) getReferences(ctx context.Context, imageID digest.Digest) ([]reference.Named, error) {
	c, err := i.getCache(ctx)
	if err != nil {
		return nil, err
	}
	img := c.byID(imageID)
	if img == nil {
		return nil, errdefs.NotFound(errors.New("no image with given id"))
	}

	return img.references, nil
}

func (i *ImageService) getCachedRef(ctx context.Context, ref string) (*cachedImage, error) {
	img, err := i.getImageByRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	return img.cached, nil
}

type imageLink struct {
	name   reference.Named
	target *ocispec.Descriptor
	cached *cachedImage
}

func (i *ImageService) getImageByRef(ctx context.Context, ref string) (imageLink, error) {
	parsed, err := reference.ParseAnyReference(ref)
	if err != nil {
		return imageLink{}, err
	}

	c, err := i.getCache(ctx)
	if err != nil {
		return imageLink{}, err
	}

	c.m.RLock()
	defer c.m.RUnlock()

	namedRef, ok := parsed.(reference.Named)
	if !ok {
		digested, ok := parsed.(reference.Digested)
		if !ok {
			return imageLink{}, errdefs.InvalidParameter(errors.New("bad reference"))
		}

		ci, ok := c.idCache[digested.Digest()]
		if !ok {
			return imageLink{}, errdefs.NotFound(errors.New("id not found"))
		}
		return imageLink{
			cached: ci,
		}, nil
	}

	img, err := i.client.ImageService().Get(ctx, namedRef.String())
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			return imageLink{}, err
		}
		dgst, err := c.ids.Lookup(ref)
		if err != nil {
			return imageLink{}, errdefs.NotFound(errors.New("reference not found"))
		}
		ci, ok := c.idCache[dgst]
		if !ok {
			return imageLink{}, errdefs.NotFound(errors.New("id not found"))
		}
		return imageLink{
			cached: ci,
		}, nil
	}
	ci, ok := c.tCache[img.Target.Digest]
	if !ok {
		// TODO(containerd): Update cache and return
		return imageLink{}, errdefs.NotFound(errors.New("id not found"))
	}

	return imageLink{
		name:   namedRef,
		target: &img.Target,
		cached: ci,
	}, nil
}

func (i *ImageService) updateCache(ctx context.Context, img imageLink) error {
	c, err := i.getCache(ctx)
	if err != nil {
		return err
	}

	img.cached.m.Lock()
	img.cached.addReference(img.name)
	img.cached.m.Unlock()

	var parent *cachedImage

	c.m.Lock()
	if _, ok := c.tCache[img.target.Digest]; !ok {
		c.tCache[img.target.Digest] = img.cached
	}
	if _, ok := c.idCache[img.cached.config.Digest]; !ok {
		c.idCache[img.cached.config.Digest] = img.cached
		c.ids.Add(img.cached.config.Digest)
	}
	if img.cached.parent != "" {
		parent = c.idCache[img.cached.parent]
	}
	c.m.Unlock()

	if parent != nil {
		parent.m.Lock()
		parent.addChild(img.cached.config.Digest)
		parent.m.Unlock()
	}

	return nil
}
