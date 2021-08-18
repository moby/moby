package cache

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/winlayers"
	digest "github.com/opencontainers/go-digest"
	imagespecidentity "github.com/opencontainers/image-spec/identity"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var g flightcontrol.Group

const containerdUncompressed = "containerd.io/uncompressed"

var ErrNoBlobs = errors.Errorf("no blobs for snapshot")

// computeBlobChain ensures every ref in a parent chain has an associated blob in the content store. If
// a blob is missing and createIfNeeded is true, then the blob will be created, otherwise ErrNoBlobs will
// be returned. Caller must hold a lease when calling this function.
// If forceCompression is specified but the blob of compressionType doesn't exist, this function creates it.
func (sr *immutableRef) computeBlobChain(ctx context.Context, createIfNeeded bool, compressionType compression.Type, forceCompression bool, s session.Group) error {
	if _, ok := leases.FromContext(ctx); !ok {
		return errors.Errorf("missing lease requirement for computeBlobChain")
	}

	if err := sr.finalizeLocked(ctx); err != nil {
		return err
	}

	if isTypeWindows(sr) {
		ctx = winlayers.UseWindowsLayerMode(ctx)
	}

	return computeBlobChain(ctx, sr, createIfNeeded, compressionType, forceCompression, s)
}

func computeBlobChain(ctx context.Context, sr *immutableRef, createIfNeeded bool, compressionType compression.Type, forceCompression bool, s session.Group) error {
	eg, ctx := errgroup.WithContext(ctx)
	if sr.parent != nil {
		eg.Go(func() error {
			return computeBlobChain(ctx, sr.parent, createIfNeeded, compressionType, forceCompression, s)
		})
	}

	eg.Go(func() error {
		v, err := g.Do(ctx, fmt.Sprintf("%s-%t", sr.ID(), createIfNeeded), func(ctx context.Context) (interface{}, error) {
			if getBlob(sr.md) != "" {
				return sr.ociDesc()
			}
			if !createIfNeeded {
				return nil, errors.WithStack(ErrNoBlobs)
			}

			var mediaType string
			switch compressionType {
			case compression.Uncompressed:
				mediaType = ocispecs.MediaTypeImageLayer
			case compression.Gzip:
				mediaType = ocispecs.MediaTypeImageLayerGzip
			default:
				return nil, errors.Errorf("unknown layer compression type: %q", compressionType)
			}

			var lower []mount.Mount
			if sr.parent != nil {
				m, err := sr.parent.Mount(ctx, true, s)
				if err != nil {
					return nil, err
				}
				var release func() error
				lower, release, err = m.Mount()
				if err != nil {
					return nil, err
				}
				if release != nil {
					defer release()
				}
			}
			m, err := sr.Mount(ctx, true, s)
			if err != nil {
				return nil, err
			}
			upper, release, err := m.Mount()
			if err != nil {
				return nil, err
			}
			if release != nil {
				defer release()
			}
			desc, err := sr.cm.Differ.Compare(ctx, lower, upper,
				diff.WithMediaType(mediaType),
				diff.WithReference(sr.ID()),
			)
			if err != nil {
				return nil, err
			}

			if desc.Annotations == nil {
				desc.Annotations = map[string]string{}
			}

			info, err := sr.cm.ContentStore.Info(ctx, desc.Digest)
			if err != nil {
				return nil, err
			}

			if diffID, ok := info.Labels[containerdUncompressed]; ok {
				desc.Annotations[containerdUncompressed] = diffID
			} else if mediaType == ocispecs.MediaTypeImageLayer {
				desc.Annotations[containerdUncompressed] = desc.Digest.String()
			} else {
				return nil, errors.Errorf("unknown layer compression type")
			}

			if err := sr.setBlob(ctx, desc); err != nil {
				return nil, err
			}

			return desc, nil
		})
		if err != nil {
			return err
		}
		descr, ok := v.(ocispecs.Descriptor)
		if !ok {
			return fmt.Errorf("invalid descriptor returned by differ while computing blob for %s", sr.ID())
		}

		if forceCompression {
			if err := ensureCompression(ctx, sr, descr, compressionType, s); err != nil {
				return err
			}
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}
	return sr.setChains(ctx)
}

// setBlob associates a blob with the cache record.
// A lease must be held for the blob when calling this function
// Caller should call Info() for knowing what current values are actually set
func (sr *immutableRef) setBlob(ctx context.Context, desc ocispecs.Descriptor) error {
	if _, ok := leases.FromContext(ctx); !ok {
		return errors.Errorf("missing lease requirement for setBlob")
	}

	diffID, err := diffIDFromDescriptor(desc)
	if err != nil {
		return err
	}
	if _, err := sr.cm.ContentStore.Info(ctx, desc.Digest); err != nil {
		return err
	}

	compressionType := compression.FromMediaType(desc.MediaType)
	if compressionType == compression.UnknownCompression {
		return errors.Errorf("unhandled layer media type: %q", desc.MediaType)
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	if getBlob(sr.md) != "" {
		return nil
	}

	if err := sr.finalize(ctx); err != nil {
		return err
	}

	if err := sr.cm.LeaseManager.AddResource(ctx, leases.Lease{ID: sr.ID()}, leases.Resource{
		ID:   desc.Digest.String(),
		Type: "content",
	}); err != nil {
		return err
	}

	queueDiffID(sr.md, diffID.String())
	queueBlob(sr.md, desc.Digest.String())
	queueMediaType(sr.md, desc.MediaType)
	queueBlobSize(sr.md, desc.Size)
	if err := sr.md.Commit(); err != nil {
		return err
	}

	if err := sr.addCompressionBlob(ctx, desc.Digest, compressionType); err != nil {
		return err
	}
	return nil
}

func (sr *immutableRef) setChains(ctx context.Context) error {
	if _, ok := leases.FromContext(ctx); !ok {
		return errors.Errorf("missing lease requirement for setChains")
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	if getChainID(sr.md) != "" {
		return nil
	}

	var chainIDs []digest.Digest
	var blobChainIDs []digest.Digest
	if sr.parent != nil {
		chainIDs = append(chainIDs, digest.Digest(getChainID(sr.parent.md)))
		blobChainIDs = append(blobChainIDs, digest.Digest(getBlobChainID(sr.parent.md)))
	}
	diffID := digest.Digest(getDiffID(sr.md))
	chainIDs = append(chainIDs, diffID)
	blobChainIDs = append(blobChainIDs, imagespecidentity.ChainID([]digest.Digest{digest.Digest(getBlob(sr.md)), diffID}))

	chainID := imagespecidentity.ChainID(chainIDs)
	blobChainID := imagespecidentity.ChainID(blobChainIDs)

	queueChainID(sr.md, chainID.String())
	queueBlobChainID(sr.md, blobChainID.String())
	if err := sr.md.Commit(); err != nil {
		return err
	}
	return nil
}

func isTypeWindows(sr *immutableRef) bool {
	if GetLayerType(sr) == "windows" {
		return true
	}
	if parent := sr.parent; parent != nil {
		return isTypeWindows(parent)
	}
	return false
}

// ensureCompression ensures the specified ref has the blob of the specified compression Type.
func ensureCompression(ctx context.Context, ref *immutableRef, desc ocispecs.Descriptor, compressionType compression.Type, s session.Group) error {
	_, err := g.Do(ctx, fmt.Sprintf("%s-%d", desc.Digest, compressionType), func(ctx context.Context) (interface{}, error) {
		// Resolve converters
		layerConvertFunc, _, err := getConverters(desc, compressionType)
		if err != nil {
			return nil, err
		} else if layerConvertFunc == nil {
			return nil, nil // no need to convert
		}

		// First, lookup local content store
		if _, err := ref.getCompressionBlob(ctx, compressionType); err == nil {
			return nil, nil // found the compression variant. no need to convert.
		}

		// Convert layer compression type
		if err := (lazyRefProvider{
			ref:     ref,
			desc:    desc,
			dh:      ref.descHandlers[desc.Digest],
			session: s,
		}).Unlazy(ctx); err != nil {
			return nil, err
		}
		newDesc, err := layerConvertFunc(ctx, ref.cm.ContentStore, desc)
		if err != nil {
			return nil, err
		}

		// Start to track converted layer
		if err := ref.addCompressionBlob(ctx, newDesc.Digest, compressionType); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}
