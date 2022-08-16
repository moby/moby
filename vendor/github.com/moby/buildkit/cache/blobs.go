package cache

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/diff/walking"
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
func (sr *immutableRef) computeBlobChain(ctx context.Context, createIfNeeded bool, comp compression.Config, s session.Group) error {
	if _, ok := leases.FromContext(ctx); !ok {
		return errors.Errorf("missing lease requirement for computeBlobChain")
	}

	if err := sr.Finalize(ctx); err != nil {
		return err
	}

	if isTypeWindows(sr) {
		ctx = winlayers.UseWindowsLayerMode(ctx)
	}

	// filter keeps track of which layers should actually be included in the blob chain.
	// This is required for diff refs, which can include only a single layer from their parent
	// refs rather than every single layer present among their ancestors.
	filter := sr.layerSet()

	return computeBlobChain(ctx, sr, createIfNeeded, comp, s, filter)
}

type compressor func(dest io.Writer, requiredMediaType string) (io.WriteCloser, error)

func computeBlobChain(ctx context.Context, sr *immutableRef, createIfNeeded bool, comp compression.Config, s session.Group, filter map[string]struct{}) error {
	eg, ctx := errgroup.WithContext(ctx)
	switch sr.kind() {
	case Merge:
		for _, parent := range sr.mergeParents {
			parent := parent
			eg.Go(func() error {
				return computeBlobChain(ctx, parent, createIfNeeded, comp, s, filter)
			})
		}
	case Diff:
		if _, ok := filter[sr.ID()]; !ok && sr.diffParents.upper != nil {
			// This diff is just re-using the upper blob, compute that
			eg.Go(func() error {
				return computeBlobChain(ctx, sr.diffParents.upper, createIfNeeded, comp, s, filter)
			})
		}
	case Layer:
		eg.Go(func() error {
			return computeBlobChain(ctx, sr.layerParent, createIfNeeded, comp, s, filter)
		})
	}

	if _, ok := filter[sr.ID()]; ok {
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
				switch comp.Type {
				case compression.Uncompressed:
					mediaType = ocispecs.MediaTypeImageLayer
				case compression.Gzip:
					compressorFunc = func(dest io.Writer, _ string) (io.WriteCloser, error) {
						return gzipWriter(comp)(dest)
					}
					mediaType = ocispecs.MediaTypeImageLayerGzip
				case compression.EStargz:
					compressorFunc, finalize = compressEStargz(comp)
					mediaType = ocispecs.MediaTypeImageLayerGzip
				case compression.Zstd:
					compressorFunc = func(dest io.Writer, _ string) (io.WriteCloser, error) {
						return zstdWriter(comp)(dest)
					}
					mediaType = ocispecs.MediaTypeImageLayer + "+zstd"
				default:
					return nil, errors.Errorf("unknown layer compression type: %q", comp.Type)
				}

				var lowerRef *immutableRef
				switch sr.kind() {
				case Diff:
					lowerRef = sr.diffParents.lower
				case Layer:
					lowerRef = sr.layerParent
				}
				var lower []mount.Mount
				if lowerRef != nil {
					m, err := lowerRef.Mount(ctx, true, s)
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

				var upperRef *immutableRef
				switch sr.kind() {
				case Diff:
					upperRef = sr.diffParents.upper
				default:
					upperRef = sr
				}
				var upper []mount.Mount
				if upperRef != nil {
					m, err := upperRef.Mount(ctx, true, s)
					if err != nil {
						return nil, err
					}
					var release func() error
					upper, release, err = m.Mount()
					if err != nil {
						return nil, err
					}
					if release != nil {
						defer release()
					}
				}

				var desc ocispecs.Descriptor
				var err error

				// Determine differ and error/log handling according to the platform, envvar and the snapshotter.
				var enableOverlay, fallback, logWarnOnErr bool
				if forceOvlStr := os.Getenv("BUILDKIT_DEBUG_FORCE_OVERLAY_DIFF"); forceOvlStr != "" && sr.kind() != Diff {
					enableOverlay, err = strconv.ParseBool(forceOvlStr)
					if err != nil {
						return nil, errors.Wrapf(err, "invalid boolean in BUILDKIT_DEBUG_FORCE_OVERLAY_DIFF")
					}
					fallback = false // prohibit fallback on debug
				} else if !isTypeWindows(sr) {
					enableOverlay, fallback = true, true
					switch sr.cm.Snapshotter.Name() {
					case "overlayfs", "stargz":
						// overlayfs-based snapshotters should support overlay diff except when running an arbitrary diff
						// (in which case lower and upper may differ by more than one layer), so print warn log on unexpected
						// failure.
						logWarnOnErr = sr.kind() != Diff
					case "fuse-overlayfs", "native":
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

				if desc.Digest == "" && !isTypeWindows(sr) && (comp.Type == compression.Zstd || comp.Type == compression.EStargz) {
					// These compression types aren't supported by containerd differ. So try to compute diff on buildkit side.
					// This case can be happen on containerd worker + non-overlayfs snapshotter (e.g. native).
					// See also: https://github.com/containerd/containerd/issues/4263
					desc, err = walking.NewWalkingDiff(sr.cm.ContentStore).Compare(ctx, lower, upper,
						diff.WithMediaType(mediaType),
						diff.WithReference(sr.ID()),
						diff.WithCompressor(compressorFunc),
					)
					if err != nil {
						logrus.WithError(err).Warnf("failed to compute blob by buildkit differ")
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

				if err := sr.setBlob(ctx, desc); err != nil {
					return nil, err
				}
				return nil, nil
			})
			if err != nil {
				return err
			}

			if comp.Force {
				if err := ensureCompression(ctx, sr, comp, s); err != nil {
					return errors.Wrapf(err, "failed to ensure compression type of %q", comp.Type)
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}
	return sr.computeChainMetadata(ctx, filter)
}

// setBlob associates a blob with the cache record.
// A lease must be held for the blob when calling this function
func (sr *immutableRef) setBlob(ctx context.Context, desc ocispecs.Descriptor) (rerr error) {
	if _, ok := leases.FromContext(ctx); !ok {
		return errors.Errorf("missing lease requirement for setBlob")
	}
	defer func() {
		if rerr == nil {
			rerr = sr.linkBlob(ctx, desc)
		}
	}()

	diffID, err := diffIDFromDescriptor(desc)
	if err != nil {
		return err
	}
	if _, err := sr.cm.ContentStore.Info(ctx, desc.Digest); err != nil {
		return err
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
	sr.appendURLs(desc.URLs)
	if err := sr.commitMetadata(); err != nil {
		return err
	}

	return nil
}

func (sr *immutableRef) computeChainMetadata(ctx context.Context, filter map[string]struct{}) error {
	if _, ok := leases.FromContext(ctx); !ok {
		return errors.Errorf("missing lease requirement for computeChainMetadata")
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.getChainID() != "" {
		return nil
	}

	var chainID digest.Digest
	var blobChainID digest.Digest

	switch sr.kind() {
	case BaseLayer:
		if _, ok := filter[sr.ID()]; !ok {
			return nil
		}
		diffID := sr.getDiffID()
		chainID = diffID
		blobChainID = imagespecidentity.ChainID([]digest.Digest{digest.Digest(sr.getBlob()), diffID})
	case Layer:
		if _, ok := filter[sr.ID()]; !ok {
			return nil
		}
		if _, ok := filter[sr.layerParent.ID()]; ok {
			if parentChainID := sr.layerParent.getChainID(); parentChainID != "" {
				chainID = parentChainID
			} else {
				return errors.Errorf("failed to set chain for reference with non-addressable parent %q", sr.layerParent.GetDescription())
			}
			if parentBlobChainID := sr.layerParent.getBlobChainID(); parentBlobChainID != "" {
				blobChainID = parentBlobChainID
			} else {
				return errors.Errorf("failed to set blobchain for reference with non-addressable parent %q", sr.layerParent.GetDescription())
			}
		}
		diffID := digest.Digest(sr.getDiffID())
		chainID = imagespecidentity.ChainID([]digest.Digest{chainID, diffID})
		blobID := imagespecidentity.ChainID([]digest.Digest{digest.Digest(sr.getBlob()), diffID})
		blobChainID = imagespecidentity.ChainID([]digest.Digest{blobChainID, blobID})
	case Merge:
		baseInput := sr.mergeParents[0]
		if _, ok := filter[baseInput.ID()]; !ok {
			// not enough information to compute chain at this time
			return nil
		}
		chainID = baseInput.getChainID()
		blobChainID = baseInput.getBlobChainID()
		for _, mergeParent := range sr.mergeParents[1:] {
			for _, layer := range mergeParent.layerChain() {
				if _, ok := filter[layer.ID()]; !ok {
					// not enough information to compute chain at this time
					return nil
				}
				diffID := digest.Digest(layer.getDiffID())
				chainID = imagespecidentity.ChainID([]digest.Digest{chainID, diffID})
				blobID := imagespecidentity.ChainID([]digest.Digest{digest.Digest(layer.getBlob()), diffID})
				blobChainID = imagespecidentity.ChainID([]digest.Digest{blobChainID, blobID})
			}
		}
	case Diff:
		if _, ok := filter[sr.ID()]; ok {
			// this diff is its own blob
			diffID := sr.getDiffID()
			chainID = diffID
			blobChainID = imagespecidentity.ChainID([]digest.Digest{digest.Digest(sr.getBlob()), diffID})
		} else {
			// re-using upper blob
			chainID = sr.diffParents.upper.getChainID()
			blobChainID = sr.diffParents.upper.getBlobChainID()
		}
	}

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
	switch sr.kind() {
	case Merge:
		for _, p := range sr.mergeParents {
			if isTypeWindows(p) {
				return true
			}
		}
	case Layer:
		return isTypeWindows(sr.layerParent)
	}
	return false
}

// ensureCompression ensures the specified ref has the blob of the specified compression Type.
func ensureCompression(ctx context.Context, ref *immutableRef, comp compression.Config, s session.Group) error {
	_, err := g.Do(ctx, fmt.Sprintf("%s-%d", ref.ID(), comp.Type), func(ctx context.Context) (interface{}, error) {
		desc, err := ref.ociDesc(ctx, ref.descHandlers, true)
		if err != nil {
			return nil, err
		}

		// Resolve converters
		layerConvertFunc, err := getConverter(ctx, ref.cm.ContentStore, desc, comp)
		if err != nil {
			return nil, err
		} else if layerConvertFunc == nil {
			if isLazy, err := ref.isLazy(ctx); err != nil {
				return nil, err
			} else if isLazy {
				// This ref can be used as the specified compressionType. Keep it lazy.
				return nil, nil
			}
			return nil, ref.linkBlob(ctx, desc)
		}

		// First, lookup local content store
		if _, err := ref.getBlobWithCompression(ctx, comp.Type); err == nil {
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
		if err := ref.linkBlob(ctx, *newDesc); err != nil {
			return nil, errors.Wrapf(err, "failed to add compression blob")
		}
		return nil, nil
	})
	return err
}

func gzipWriter(comp compression.Config) func(io.Writer) (io.WriteCloser, error) {
	return func(dest io.Writer) (io.WriteCloser, error) {
		level := gzip.DefaultCompression
		if comp.Level != nil {
			level = *comp.Level
		}
		return gzip.NewWriterLevel(dest, level)
	}
}

func zstdWriter(comp compression.Config) func(io.Writer) (io.WriteCloser, error) {
	return func(dest io.Writer) (io.WriteCloser, error) {
		level := zstd.SpeedDefault
		if comp.Level != nil {
			level = toZstdEncoderLevel(*comp.Level)
		}
		return zstd.NewWriter(dest, zstd.WithEncoderLevel(level))
	}
}

func toZstdEncoderLevel(level int) zstd.EncoderLevel {
	// map zstd compression levels to go-zstd levels
	// once we also have c based implementation move this to helper pkg
	if level < 0 {
		return zstd.SpeedDefault
	} else if level < 3 {
		return zstd.SpeedFastest
	} else if level < 7 {
		return zstd.SpeedDefault
	} else if level < 9 {
		return zstd.SpeedBetterCompression
	}
	return zstd.SpeedBestCompression
}
