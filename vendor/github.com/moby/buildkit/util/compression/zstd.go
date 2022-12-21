package compression

import (
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/klauspost/compress/zstd"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func (c zstdType) Compress(ctx context.Context, comp Config) (compressorFunc Compressor, finalize Finalizer) {
	return func(dest io.Writer, _ string) (io.WriteCloser, error) {
		return zstdWriter(comp)(dest)
	}, nil
}

func (c zstdType) Decompress(ctx context.Context, cs content.Store, desc ocispecs.Descriptor) (io.ReadCloser, error) {
	return decompress(ctx, cs, desc)
}

func (c zstdType) NeedsConversion(ctx context.Context, cs content.Store, desc ocispecs.Descriptor) (bool, error) {
	if !images.IsLayerType(desc.MediaType) {
		return false, nil
	}
	ct, err := FromMediaType(desc.MediaType)
	if err != nil {
		return false, err
	}
	if ct == Zstd {
		return false, nil
	}
	return true, nil
}

func (c zstdType) NeedsComputeDiffBySelf() bool {
	return true
}

func (c zstdType) OnlySupportOCITypes() bool {
	return false
}

func (c zstdType) NeedsForceCompression() bool {
	return false
}

func (c zstdType) MediaType() string {
	return mediaTypeImageLayerZstd
}

func (c zstdType) String() string {
	return "zstd"
}

func zstdWriter(comp Config) func(io.Writer) (io.WriteCloser, error) {
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
