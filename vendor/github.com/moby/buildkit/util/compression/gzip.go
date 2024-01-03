package compression

import (
	"compress/gzip"
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func (c gzipType) Compress(ctx context.Context, comp Config) (compressorFunc Compressor, finalize Finalizer) {
	return func(dest io.Writer, _ string) (io.WriteCloser, error) {
		return gzipWriter(comp)(dest)
	}, nil
}

func (c gzipType) Decompress(ctx context.Context, cs content.Store, desc ocispecs.Descriptor) (io.ReadCloser, error) {
	return decompress(ctx, cs, desc)
}

func (c gzipType) NeedsConversion(ctx context.Context, cs content.Store, desc ocispecs.Descriptor) (bool, error) {
	esgz, err := EStargz.Is(ctx, cs, desc.Digest)
	if err != nil {
		return false, err
	}
	if !images.IsLayerType(desc.MediaType) {
		return false, nil
	}
	ct, err := FromMediaType(desc.MediaType)
	if err != nil {
		return false, err
	}
	if ct == Gzip && !esgz {
		return false, nil
	}
	return true, nil
}

func (c gzipType) NeedsComputeDiffBySelf() bool {
	return false
}

func (c gzipType) OnlySupportOCITypes() bool {
	return false
}

func (c gzipType) NeedsForceCompression() bool {
	return false
}

func (c gzipType) MediaType() string {
	return ocispecs.MediaTypeImageLayerGzip
}

func (c gzipType) String() string {
	return "gzip"
}

func gzipWriter(comp Config) func(io.Writer) (io.WriteCloser, error) {
	return func(dest io.Writer) (io.WriteCloser, error) {
		level := gzip.DefaultCompression
		if comp.Level != nil {
			level = *comp.Level
		}
		return gzip.NewWriterLevel(dest, level)
	}
}
