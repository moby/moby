package cache

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/converter"
	"github.com/containerd/containerd/images/converter/uncompress"
	"github.com/containerd/containerd/labels"
	"github.com/moby/buildkit/util/compression"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

// getConverters returns converter functions according to the specified compression type.
// If no conversion is needed, this returns nil without error.
func getConverters(desc ocispecs.Descriptor, compressionType compression.Type) (converter.ConvertFunc, func(string) string, error) {
	switch compressionType {
	case compression.Uncompressed:
		if !images.IsLayerType(desc.MediaType) || uncompress.IsUncompressedType(desc.MediaType) {
			// No conversion. No need to return an error here.
			return nil, nil, nil
		}
		return uncompress.LayerConvertFunc, convertMediaTypeToUncompress, nil
	case compression.Gzip:
		if !images.IsLayerType(desc.MediaType) || isGzipCompressedType(desc.MediaType) {
			// No conversion. No need to return an error here.
			return nil, nil, nil
		}
		return gzipLayerConvertFunc, convertMediaTypeToGzip, nil
	default:
		return nil, nil, fmt.Errorf("unknown compression type during conversion: %q", compressionType)
	}
}

func gzipLayerConvertFunc(ctx context.Context, cs content.Store, desc ocispecs.Descriptor) (*ocispecs.Descriptor, error) {
	if !images.IsLayerType(desc.MediaType) || isGzipCompressedType(desc.MediaType) {
		// No conversion. No need to return an error here.
		return nil, nil
	}

	// prepare the source and destination
	info, err := cs.Info(ctx, desc.Digest)
	if err != nil {
		return nil, err
	}
	labelz := info.Labels
	if labelz == nil {
		labelz = make(map[string]string)
	}
	ra, err := cs.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer ra.Close()
	ref := fmt.Sprintf("convert-gzip-from-%s", desc.Digest)
	w, err := cs.Writer(ctx, content.WithRef(ref))
	if err != nil {
		return nil, err
	}
	defer w.Close()
	if err := w.Truncate(0); err != nil { // Old written data possibly remains
		return nil, err
	}
	zw := gzip.NewWriter(w)
	defer zw.Close()

	// convert this layer
	diffID := digest.Canonical.Digester()
	if _, err := io.Copy(zw, io.TeeReader(io.NewSectionReader(ra, 0, ra.Size()), diffID.Hash())); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil { // Flush the writer
		return nil, err
	}
	labelz[labels.LabelUncompressed] = diffID.Digest().String() // update diffID label
	if err = w.Commit(ctx, 0, "", content.WithLabels(labelz)); err != nil && !errdefs.IsAlreadyExists(err) {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	info, err = cs.Info(ctx, w.Digest())
	if err != nil {
		return nil, err
	}

	newDesc := desc
	newDesc.MediaType = convertMediaTypeToGzip(newDesc.MediaType)
	newDesc.Digest = info.Digest
	newDesc.Size = info.Size
	return &newDesc, nil
}

func isGzipCompressedType(mt string) bool {
	switch mt {
	case
		images.MediaTypeDockerSchema2LayerGzip,
		images.MediaTypeDockerSchema2LayerForeignGzip,
		ocispecs.MediaTypeImageLayerGzip,
		ocispecs.MediaTypeImageLayerNonDistributableGzip:
		return true
	default:
		return false
	}
}

func convertMediaTypeToUncompress(mt string) string {
	switch mt {
	case images.MediaTypeDockerSchema2LayerGzip:
		return images.MediaTypeDockerSchema2Layer
	case images.MediaTypeDockerSchema2LayerForeignGzip:
		return images.MediaTypeDockerSchema2LayerForeign
	case ocispecs.MediaTypeImageLayerGzip:
		return ocispecs.MediaTypeImageLayer
	case ocispecs.MediaTypeImageLayerNonDistributableGzip:
		return ocispecs.MediaTypeImageLayerNonDistributable
	default:
		return mt
	}
}

func convertMediaTypeToGzip(mt string) string {
	if uncompress.IsUncompressedType(mt) {
		if images.IsDockerType(mt) {
			mt += ".gzip"
		} else {
			mt += "+gzip"
		}
		return mt
	}
	return mt
}
