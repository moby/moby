package containerd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/moby/buildkit/util/attestation"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (i *ImageService) ImageConvert(ctx context.Context, src string, dsts []reference.NamedTagged, opts imagetypes.ConvertOptions) error {
	log.G(ctx).Debugf("converting: %+v", opts)

	srcImg, err := i.resolveImage(ctx, src)
	if err != nil {
		return err
	}

	if opts.OnlyAvailablePlatforms && len(opts.Platforms) > 0 {
		return errdefs.InvalidParameter(errors.New("specifying both explicit platform list and only-available-platforms is not allowed"))
	}

	srcMediaType := srcImg.Target.MediaType
	if !images.IsIndexType(srcMediaType) {
		return errdefs.InvalidParameter(errors.New("cannot convert non-index image"))
	}

	oldIndex, info, err := readIndex(ctx, i.content, srcImg.Target)
	if err != nil {
		return err
	}

	newImg := srcImg
	newManifests, err := i.convertManifests(ctx, srcImg, opts)
	if err != nil {
		return err
	}
	if len(newManifests) == 0 {
		return errdefs.InvalidParameter(errors.New("refusing to create an empty image"))
	}

	newImg.Target = newManifests[0]
	if len(newManifests) > 1 {
		newIndex := oldIndex
		newIndex.Manifests = newManifests
		t, err := storeJson(ctx, i.content, newIndex.MediaType, newIndex, info.Labels)
		if err != nil {
			return errdefs.System(fmt.Errorf("failed to write modified image target: %w", err))
		}
		newImg.Target = t
	}

	newImg.CreatedAt = time.Now()
	newImg.UpdatedAt = newImg.CreatedAt

	for _, dst := range dsts {
		newImg.Name = dst.String()

		if err := i.forceCreateImage(ctx, newImg); err != nil {
			return err
		}
	}
	return nil
}

var errConvertNoop = errors.New("no conversion performed")

func (i *ImageService) convertManifests(ctx context.Context, srcImg images.Image, opts imagetypes.ConvertOptions) ([]ocispec.Descriptor, error) {
	changed := false
	pm := platforms.All
	if len(opts.Platforms) > 0 {
		pm = platforms.Any(opts.Platforms...)
	}

	var newManifests []ocispec.Descriptor
	walker := i.walkReachableImageManifests
	if opts.OnlyAvailablePlatforms {
		walker = i.walkImageManifests
		changed = true
	}

	// Key: Manifest digest, Value: OCI descriptor of the attestation
	manifestToAttestationDesc := map[string]ocispec.Descriptor{}

	// Collect attestation descriptors
	if !opts.NoAttestations {
		err := walker(ctx, srcImg, func(im *ImageManifest) error {
			if im.IsAttestation() {
				desc := im.Target()
				typ := desc.Annotations[attestation.DockerAnnotationReferenceType]
				if typ != attestation.DockerAnnotationReferenceTypeDefault {
					log.G(ctx).WithFields(log.Fields{
						"digest": desc.Digest,
						"type":   typ,
					}).Debug("skipping attestation manifest with unknown type")
					return images.ErrSkipDesc
				}

				mfstDgst := im.Target().Annotations[attestation.DockerAnnotationReferenceDigest]
				manifestToAttestationDesc[mfstDgst] = desc
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	err := walker(ctx, srcImg, func(m *ImageManifest) error {
		if m.IsAttestation() {
			return images.ErrSkipDesc
		} else {
			mtarget := m.Target()
			mplatform, err := m.ImagePlatform(ctx)
			if err != nil {
				return err
			}
			if !pm.Match(mplatform) {
				changed = true
				log.G(ctx).WithFields(log.Fields{
					"platform": mplatform,
					"digest":   mtarget.Digest,
				}).Debugf("skipping manifest %s due to platform mismatch", mtarget.Digest)
				return images.ErrSkipDesc
			}

			newManifests = append(newManifests, mtarget)
			log.G(ctx).WithFields(log.Fields{
				"platform": mplatform,
				"digest":   mtarget.Digest,
			}).Debug("add platform-specific image manifest")

			attestation, hasAttestation := manifestToAttestationDesc[mtarget.Digest.String()]
			if hasAttestation {
				newManifests = append(newManifests, attestation)
				log.G(ctx).WithFields(log.Fields{
					"platform":    mplatform,
					"manifest":    mtarget.Digest,
					"attestation": attestation.Digest,
				}).Debug("add attestation")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if !changed {
		return newManifests, errConvertNoop
	}

	return newManifests, nil
}

func readIndex(ctx context.Context, store content.InfoReaderProvider, desc ocispec.Descriptor) (ocispec.Index, content.Info, error) {
	info, err := store.Info(ctx, desc.Digest)
	if err != nil {
		return ocispec.Index{}, content.Info{}, err
	}

	p, err := content.ReadBlob(ctx, store, desc)
	if err != nil {
		return ocispec.Index{}, info, err
	}

	var idx ocispec.Index
	if err := json.Unmarshal(p, &idx); err != nil {
		return ocispec.Index{}, info, err
	}

	return idx, info, nil
}
