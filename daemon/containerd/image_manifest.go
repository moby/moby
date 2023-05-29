package containerd

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	containerdimages "github.com/containerd/containerd/images"
	cplatforms "github.com/containerd/containerd/platforms"
	"github.com/docker/docker/errdefs"
	"github.com/moby/buildkit/util/attestation"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// walkImageManifests calls the handler for each locally present manifest in
// the image. The image implements the containerd.Image interface, but all
// operations act on the specific manifest instead of the index.
func (i *ImageService) walkImageManifests(ctx context.Context, img containerdimages.Image, handler func(img *ImageManifest) error) error {
	desc := img.Target

	handleManifest := func(ctx context.Context, d ocispec.Descriptor) error {
		platformImg, err := i.NewImageManifest(ctx, img, d)
		if err != nil {
			return err
		}
		return handler(platformImg)
	}

	if containerdimages.IsIndexType(desc.MediaType) {
		store := i.client.ContentStore()
		return containerdimages.Walk(ctx, presentChildrenHandler(store, containerdimages.HandlerFunc(
			func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
				if containerdimages.IsManifestType(desc.MediaType) {
					return nil, handleManifest(ctx, desc)
				}
				return nil, nil
			})), desc)
	}

	return handleManifest(ctx, desc)
}

type ImageManifest struct {
	containerd.Image

	// Parent of the manifest (index/manifest list)
	RealTarget ocispec.Descriptor

	manifest *ocispec.Manifest
}

func (i *ImageService) NewImageManifest(ctx context.Context, img containerdimages.Image, manifestDesc ocispec.Descriptor) (*ImageManifest, error) {
	if !containerdimages.IsManifestType(manifestDesc.MediaType) {
		return nil, errdefs.InvalidParameter(errors.New("descriptor isn't a manifest"))
	}

	parent := img.Target
	img.Target = manifestDesc

	c8dImg := containerd.NewImageWithPlatform(i.client, img, cplatforms.All)
	return &ImageManifest{
		Image:      c8dImg,
		RealTarget: parent,
	}, nil
}

func (im *ImageManifest) Metadata() containerdimages.Image {
	md := im.Image.Metadata()
	md.Target = im.RealTarget
	return md
}

// IsPseudoImage returns false if the manifest has no layers or any of its layers is a known image layer.
// Some manifests use the image media type for compatibility, even if they are not a real image.
func (im *ImageManifest) IsPseudoImage(ctx context.Context) (bool, error) {
	desc := im.Target()

	// Quick check for buildkit attestation manifests
	// https://github.com/moby/buildkit/blob/v0.11.4/docs/attestations/attestation-storage.md
	// This would have also been caught by the layer check below, but it requires
	// an additional content read and deserialization of Manifest.
	if _, has := desc.Annotations[attestation.DockerAnnotationReferenceType]; has {
		return true, nil
	}

	mfst, err := im.Manifest(ctx)
	if err != nil {
		return true, err
	}
	if len(mfst.Layers) == 0 {
		return false, nil
	}
	for _, l := range mfst.Layers {
		if images.IsLayerType(l.MediaType) {
			return false, nil
		}
	}
	return true, nil
}

func (im *ImageManifest) Manifest(ctx context.Context) (ocispec.Manifest, error) {
	if im.manifest != nil {
		return *im.manifest, nil
	}

	mfst, err := readManifest(ctx, im.ContentStore(), im.Target())
	if err != nil {
		return ocispec.Manifest{}, err
	}

	im.manifest = &mfst
	return mfst, nil
}

func (im *ImageManifest) CheckContentAvailable(ctx context.Context) (bool, error) {
	// The target is already a platform-specific manifest, so no need to match platform.
	pm := cplatforms.All

	available, _, _, missing, err := containerdimages.Check(ctx, im.ContentStore(), im.Target(), pm)
	if err != nil {
		return false, err
	}

	if !available || len(missing) > 0 {
		return false, nil
	}

	return true, nil
}

func readManifest(ctx context.Context, store content.Provider, desc ocispec.Descriptor) (ocispec.Manifest, error) {
	p, err := content.ReadBlob(ctx, store, desc)
	if err != nil {
		return ocispec.Manifest{}, err
	}

	var mfst ocispec.Manifest
	if err := json.Unmarshal(p, &mfst); err != nil {
		return ocispec.Manifest{}, err
	}

	return mfst, nil
}
