//go:build nydus
// +build nydus

package cache

import (
	"compress/gzip"
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/labels"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/compression"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"github.com/containerd/nydus-snapshotter/pkg/converter"
)

func init() {
	additionalAnnotations = append(
		additionalAnnotations,
		converter.LayerAnnotationNydusBlob, converter.LayerAnnotationNydusBootstrap,
	)
}

// MergeNydus does two steps:
// 1. Extracts nydus bootstrap from nydus format (nydus blob + nydus bootstrap) for each layer.
// 2. Merge all nydus bootstraps into a final bootstrap (will as an extra layer).
// The nydus bootstrap size is very small, so the merge operation is fast.
func MergeNydus(ctx context.Context, ref ImmutableRef, comp compression.Config, s session.Group) (*ocispecs.Descriptor, error) {
	iref, ok := ref.(*immutableRef)
	if !ok {
		return nil, errors.Errorf("unsupported ref type %T", ref)
	}
	refs := iref.layerChain()
	if len(refs) == 0 {
		return nil, errors.Errorf("refs can't be empty")
	}

	// Extracts nydus bootstrap from nydus format for each layer.
	var cm *cacheManager
	layers := []converter.Layer{}
	for _, ref := range refs {
		blobDesc, err := getBlobWithCompressionWithRetry(ctx, ref, comp, s)
		if err != nil {
			return nil, errors.Wrapf(err, "get compression blob %q", comp.Type)
		}
		ra, err := ref.cm.ContentStore.ReaderAt(ctx, blobDesc)
		if err != nil {
			return nil, errors.Wrapf(err, "get reader for compression blob %q", comp.Type)
		}
		defer ra.Close()
		if cm == nil {
			cm = ref.cm
		}
		layers = append(layers, converter.Layer{
			Digest:   blobDesc.Digest,
			ReaderAt: ra,
		})
	}

	// Merge all nydus bootstraps into a final nydus bootstrap.
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if _, err := converter.Merge(ctx, layers, pw, converter.MergeOption{
			WithTar: true,
		}); err != nil {
			pw.CloseWithError(errors.Wrapf(err, "merge nydus bootstrap"))
		}
	}()

	// Compress final nydus bootstrap to tar.gz and write into content store.
	cw, err := content.OpenWriter(ctx, cm.ContentStore, content.WithRef("nydus-merge-"+iref.getChainID().String()))
	if err != nil {
		return nil, errors.Wrap(err, "open content store writer")
	}
	defer cw.Close()

	gw := gzip.NewWriter(cw)
	uncompressedDgst := digest.SHA256.Digester()
	compressed := io.MultiWriter(gw, uncompressedDgst.Hash())
	if _, err := io.Copy(compressed, pr); err != nil {
		return nil, errors.Wrapf(err, "copy bootstrap targz into content store")
	}
	if err := gw.Close(); err != nil {
		return nil, errors.Wrap(err, "close gzip writer")
	}

	compressedDgst := cw.Digest()
	if err := cw.Commit(ctx, 0, compressedDgst, content.WithLabels(map[string]string{
		labels.LabelUncompressed: uncompressedDgst.Digest().String(),
	})); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return nil, errors.Wrap(err, "commit to content store")
		}
	}
	if err := cw.Close(); err != nil {
		return nil, errors.Wrap(err, "close content store writer")
	}

	info, err := cm.ContentStore.Info(ctx, compressedDgst)
	if err != nil {
		return nil, errors.Wrap(err, "get info from content store")
	}

	desc := ocispecs.Descriptor{
		Digest:    compressedDgst,
		Size:      info.Size,
		MediaType: ocispecs.MediaTypeImageLayerGzip,
		Annotations: map[string]string{
			labels.LabelUncompressed: uncompressedDgst.Digest().String(),
			// Use this annotation to identify nydus bootstrap layer.
			converter.LayerAnnotationNydusBootstrap: "true",
		},
	}

	return &desc, nil
}
