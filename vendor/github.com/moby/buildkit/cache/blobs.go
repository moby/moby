package cache

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/klauspost/compress/zstd"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/winlayers"
	digest "github.com/opencontainers/go-digest"
	imagespecidentity "github.com/opencontainers/image-spec/identity"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

	if err := sr.Finalize(ctx); err != nil {
		return err
	}

	if isTypeWindows(sr) {
		ctx = winlayers.UseWindowsLayerMode(ctx)
	}

	return computeBlobChain(ctx, sr, createIfNeeded, compressionType, forceCompression, s)
}

type compressor func(dest io.Writer, requiredMediaType string) (io.WriteCloser, error)

func computeBlobChain(ctx context.Context, sr *immutableRef, createIfNeeded bool, compressionType compression.Type, forceCompression bool, s session.Group) error {
	eg, ctx := errgroup.WithContext(ctx)
	if sr.parent != nil {
		eg.Go(func() error {
			return computeBlobChain(ctx, sr.parent, createIfNeeded, compressionType, forceCompression, s)
		})
	}

	eg.Go(func() error {
		_, err := g.Do(ctx, fmt.Sprintf("%s-%t", sr.ID(), createIfNeeded), func(ctx context.Context) (interface{}, error) {
			if sr.getBlob() != "" {
				return nil, nil
			}
			if !createIfNeeded {
				return nil, errors.WithStack(ErrNoBlobs)
			}

			var mediaType string
			var compressorFunc compressor
			var finalize func(context.Context, content.Store) (map[string]string, error)
			switch compressionType {
			case compression.Uncompressed:
				mediaType = ocispecs.MediaTypeImageLayer
			case compression.Gzip:
				mediaType = ocispecs.MediaTypeImageLayerGzip
			case compression.EStargz:
				compressorFunc, finalize = compressEStargz()
				mediaType = ocispecs.MediaTypeImageLayerGzip
			case compression.Zstd:
				compressorFunc = zstdWriter
				mediaType = ocispecs.MediaTypeImageLayer + "+zstd"
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
			var desc ocispecs.Descriptor

			// Determine differ and error/log handling according to the platform, envvar and the snapshotter.
			var enableOverlay, fallback, logWarnOnErr bool
			if forceOvlStr := os.Getenv("BUILDKIT_DEBUG_FORCE_OVERLAY_DIFF"); forceOvlStr != "" {
				enableOverlay, err = strconv.ParseBool(forceOvlStr)
				if err != nil {
					return nil, errors.Wrapf(err, "invalid boolean in BUILDKIT_DEBUG_FORCE_OVERLAY_DIFF")
				}
				fallback = false // prohibit fallback on debug
			} else if !isTypeWindows(sr) {
				enableOverlay, fallback = true, true
				switch sr.cm.ManagerOpt.Snapshotter.Name() {
				case "overlayfs", "stargz":
					// overlayfs-based snapshotters should support overlay diff. so print warn log on failure.
					logWarnOnErr = true
				case "fuse-overlayfs":
					// not supported with fuse-overlayfs snapshotter which doesn't provide overlayfs mounts.
					// TODO: add support for fuse-overlayfs
					enableOverlay = false
				}
			}
			if enableOverlay {
				computed, ok, err := sr.tryComputeOverlayBlob(ctx, lower, upper, mediaType, sr.ID(), compressorFunc)
				if !ok || err != nil {
					if !fallback {
						if !ok {
							return nil, errors.Errorf("overlay mounts not detected (lower=%+v,upper=%+v)", lower, upper)
						}
						if err != nil {
							return nil, errors.Wrapf(err, "failed to compute overlay diff")
						}
					}
					if logWarnOnErr {
						logrus.Warnf("failed to compute blob by overlay differ (ok=%v): %v", ok, err)
					}
				}
				if ok {
					desc = computed
				}
			}

			if desc.Digest == "" {
				desc, err = sr.cm.Differ.Compare(ctx, lower, upper,
					diff.WithMediaType(mediaType),
					diff.WithReference(sr.ID()),
					diff.WithCompressor(compressorFunc),
				)
				if err != nil {
					return nil, err
				}
			}

			if desc.Annotations == nil {
				desc.Annotations = map[string]string{}
			}
			if finalize != nil {
				a, err := finalize(ctx, sr.cm.ContentStore)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to finalize compression")
				}
				for k, v := range a {
					desc.Annotations[k] = v
				}
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

			if err := sr.setBlob(ctx, compressionType, desc); err != nil {
				return nil, err
			}

			return nil, nil
		})
		if err != nil {
			return err
		}
		if forceCompression {
			if err := ensureCompression(ctx, sr, compressionType, s); err != nil {
				return errors.Wrapf(err, "failed to ensure compression type of %q", compressionType)
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
func (sr *immutableRef) setBlob(ctx context.Context, compressionType compression.Type, desc ocispecs.Descriptor) error {
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

	if compressionType == compression.UnknownCompression {
		return errors.Errorf("unhandled layer media type: %q", desc.MediaType)
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.getBlob() != "" {
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

	sr.queueDiffID(diffID)
	sr.queueBlob(desc.Digest)
	sr.queueMediaType(desc.MediaType)
	sr.queueBlobSize(desc.Size)
	if err := sr.commitMetadata(); err != nil {
		return err
	}

	if err := sr.addCompressionBlob(ctx, desc, compressionType); err != nil {
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

	if sr.getChainID() != "" {
		return nil
	}

	var chainIDs []digest.Digest
	var blobChainIDs []digest.Digest
	if sr.parent != nil {
		chainIDs = append(chainIDs, digest.Digest(sr.parent.getChainID()))
		blobChainIDs = append(blobChainIDs, digest.Digest(sr.parent.getBlobChainID()))
	}
	diffID := digest.Digest(sr.getDiffID())
	chainIDs = append(chainIDs, diffID)
	blobChainIDs = append(blobChainIDs, imagespecidentity.ChainID([]digest.Digest{digest.Digest(sr.getBlob()), diffID}))

	chainID := imagespecidentity.ChainID(chainIDs)
	blobChainID := imagespecidentity.ChainID(blobChainIDs)

	sr.queueChainID(chainID)
	sr.queueBlobChainID(blobChainID)
	if err := sr.commitMetadata(); err != nil {
		return err
	}
	return nil
}

func isTypeWindows(sr *immutableRef) bool {
	if sr.GetLayerType() == "windows" {
		return true
	}
	if parent := sr.parent; parent != nil {
		return isTypeWindows(parent)
	}
	return false
}

// ensureCompression ensures the specified ref has the blob of the specified compression Type.
func ensureCompression(ctx context.Context, ref *immutableRef, compressionType compression.Type, s session.Group) error {
	_, err := g.Do(ctx, fmt.Sprintf("%s-%d", ref.ID(), compressionType), func(ctx context.Context) (interface{}, error) {
		desc, err := ref.ociDesc(ctx, ref.descHandlers)
		if err != nil {
			return nil, err
		}

		// Resolve converters
		layerConvertFunc, err := getConverter(ctx, ref.cm.ContentStore, desc, compressionType)
		if err != nil {
			return nil, err
		} else if layerConvertFunc == nil {
			if isLazy, err := ref.isLazy(ctx); err != nil {
				return nil, err
			} else if isLazy {
				// This ref can be used as the specified compressionType. Keep it lazy.
				return nil, nil
			}
			return nil, ref.addCompressionBlob(ctx, desc, compressionType)
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
			return nil, errors.Wrapf(err, "failed to convert")
		}

		// Start to track converted layer
		if err := ref.addCompressionBlob(ctx, *newDesc, compressionType); err != nil {
			return nil, errors.Wrapf(err, "failed to add compression blob")
		}
		return nil, nil
	})
	return err
}

func zstdWriter(dest io.Writer, requiredMediaType string) (io.WriteCloser, error) {
	return zstd.NewWriter(dest)
}
