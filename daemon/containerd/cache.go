package containerd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/cache"
	"github.com/docker/docker/internal/multierror"
	"github.com/docker/docker/layer"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, sourceRefs []string) (builder.ImageCache, error) {
	return cache.New(ctx, cacheAdaptor{i}, sourceRefs)
}

type cacheAdaptor struct {
	is *ImageService
}

func (c cacheAdaptor) Get(id image.ID) (*image.Image, error) {
	ctx := context.TODO()
	ref := id.String()

	outImg, err := c.is.GetImage(ctx, id.String(), backend.GetImageOpts{})
	if err != nil {
		return nil, fmt.Errorf("GetImage: %w", err)
	}

	c8dImg, err := c.is.resolveImage(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolveImage: %w", err)
	}

	var errFound = errors.New("success")
	err = c.is.walkImageManifests(ctx, c8dImg, func(img *ImageManifest) error {
		desc, err := img.Config(ctx)
		if err != nil {
			log.G(ctx).WithFields(log.Fields{
				"image": img,
				"error": err,
			}).Warn("failed to get config descriptor for image")
			return nil
		}

		info, err := c.is.content.Info(ctx, desc.Digest)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				log.G(ctx).WithFields(log.Fields{
					"image": img,
					"desc":  desc,
					"error": err,
				}).Warn("failed to get info of image config")
			}
			return nil
		}

		if dgstStr, ok := info.Labels[contentLabelGcRefContainerConfig]; ok {
			dgst, err := digest.Parse(dgstStr)
			if err != nil {
				log.G(ctx).WithFields(log.Fields{
					"label":   contentLabelClassicBuilderImage,
					"value":   dgstStr,
					"content": desc.Digest,
					"error":   err,
				}).Warn("invalid digest in label")
				return nil
			}

			configDesc := ocispec.Descriptor{
				Digest: dgst,
			}

			var config container.Config
			if err := readConfig(ctx, c.is.content, configDesc, &config); err != nil {
				if !errdefs.IsNotFound(err) {
					log.G(ctx).WithFields(log.Fields{
						"configDigest": dgst,
						"error":        err,
					}).Warn("failed to read container config")
				}
				return nil
			}

			outImg.ContainerConfig = config

			// We already have the config we looked for, so return an error to
			// stop walking the image further. This error will be ignored.
			return errFound
		}
		return nil
	})
	if err != nil && err != errFound {
		return nil, err
	}

	return outImg, nil
}

func (c cacheAdaptor) GetByRef(ctx context.Context, refOrId string) (*image.Image, error) {
	return c.is.GetImage(ctx, refOrId, backend.GetImageOpts{})
}

func (c cacheAdaptor) SetParent(target, parent image.ID) error {
	ctx := context.TODO()
	_, imgs, err := c.is.resolveAllReferences(ctx, target.String())
	if err != nil {
		return fmt.Errorf("failed to list images with digest %q", target)
	}

	var errs []error
	is := c.is.images
	for _, img := range imgs {
		if img.Labels == nil {
			img.Labels = make(map[string]string)
		}
		img.Labels[imageLabelClassicBuilderParent] = parent.String()
		if _, err := is.Update(ctx, img, "labels."+imageLabelClassicBuilderParent); err != nil {
			errs = append(errs, fmt.Errorf("failed to update parent label on image %v: %w", img, err))
		}
	}

	return multierror.Join(errs...)
}

func (c cacheAdaptor) GetParent(target image.ID) (image.ID, error) {
	ctx := context.TODO()
	value, err := c.is.getImageLabelByDigest(ctx, target.Digest(), imageLabelClassicBuilderParent)
	if err != nil {
		return "", fmt.Errorf("failed to read parent image: %w", err)
	}

	dgst, err := digest.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid parent value: %q", value)
	}

	return image.ID(dgst), nil
}

func (c cacheAdaptor) Create(parent *image.Image, target image.Image, extraLayer layer.DiffID) (image.ID, error) {
	ctx := context.TODO()
	data, err := json.Marshal(target)
	if err != nil {
		return "", fmt.Errorf("failed to marshal image config: %w", err)
	}

	var layerDigest digest.Digest
	if extraLayer != "" {
		info, err := findContentByUncompressedDigest(ctx, c.is.client.ContentStore(), digest.Digest(extraLayer))
		if err != nil {
			return "", fmt.Errorf("failed to find content for diff ID %q: %w", extraLayer, err)
		}
		layerDigest = info.Digest
	}

	var parentRef string
	if parent != nil {
		parentRef = parent.ID().String()
	}
	img, err := c.is.CreateImage(ctx, data, parentRef, layerDigest)
	if err != nil {
		return "", fmt.Errorf("failed to created cached image: %w", err)
	}

	return image.ID(img.ImageID()), nil
}

func (c cacheAdaptor) IsBuiltLocally(target image.ID) (bool, error) {
	ctx := context.TODO()
	value, err := c.is.getImageLabelByDigest(ctx, target.Digest(), imageLabelClassicBuilderContainerConfig)
	if err != nil {
		return false, fmt.Errorf("failed to read container config label: %w", err)
	}
	return value != "", nil
}

func (c cacheAdaptor) Children(id image.ID) []image.ID {
	ctx := context.TODO()

	if id.String() == "" {
		imgs, err := c.is.getImagesWithLabel(ctx, imageLabelClassicBuilderFromScratch, "1")
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to get from scratch images")
			return nil
		}
		return imgs
	}

	imgs, err := c.is.Children(ctx, id)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to get image children")
		return nil
	}

	return imgs
}

func findContentByUncompressedDigest(ctx context.Context, cs content.Manager, uncompressed digest.Digest) (content.Info, error) {
	var out content.Info

	errStopWalk := errors.New("success")
	err := cs.Walk(ctx, func(i content.Info) error {
		out = i
		return errStopWalk
	}, `labels."containerd.io/uncompressed"==`+uncompressed.String())

	if err != nil && err != errStopWalk {
		return out, err
	}
	if out.Digest == "" {
		return out, errdefs.NotFound(errors.New("no content matches this uncompressed digest"))
	}
	return out, nil
}
