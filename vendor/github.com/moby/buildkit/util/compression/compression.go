package compression

import (
	"bytes"
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/stargz-snapshotter/estargz"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Type represents compression type for blob data.
type Type int

const (
	// Uncompressed indicates no compression.
	Uncompressed Type = iota

	// Gzip is used for blob data.
	Gzip

	// EStargz is used for estargz data.
	EStargz

	// Zstd is used for Zstandard data.
	Zstd

	// UnknownCompression means not supported yet.
	UnknownCompression Type = -1
)

type Config struct {
	Type  Type
	Force bool
	Level *int
}

func New(t Type) Config {
	return Config{
		Type: t,
	}
}

func (c Config) SetForce(v bool) Config {
	c.Force = v
	return c
}

func (c Config) SetLevel(l int) Config {
	c.Level = &l
	return c
}

const (
	mediaTypeDockerSchema2LayerZstd = images.MediaTypeDockerSchema2Layer + ".zstd"
	mediaTypeImageLayerZstd         = ocispecs.MediaTypeImageLayer + "+zstd" // unreleased image-spec#790
)

var Default = Gzip

func Parse(t string) Type {
	switch t {
	case "uncompressed":
		return Uncompressed
	case "gzip":
		return Gzip
	case "estargz":
		return EStargz
	case "zstd":
		return Zstd
	default:
		return UnknownCompression
	}
}

func (ct Type) String() string {
	switch ct {
	case Uncompressed:
		return "uncompressed"
	case Gzip:
		return "gzip"
	case EStargz:
		return "estargz"
	case Zstd:
		return "zstd"
	default:
		return "unknown"
	}
}

func (ct Type) DefaultMediaType() string {
	switch ct {
	case Uncompressed:
		return ocispecs.MediaTypeImageLayer
	case Gzip, EStargz:
		return ocispecs.MediaTypeImageLayerGzip
	case Zstd:
		return mediaTypeImageLayerZstd
	default:
		return ocispecs.MediaTypeImageLayer + "+unknown"
	}
}

func (ct Type) IsMediaType(mt string) bool {
	mt, ok := toOCILayerType[mt]
	if !ok {
		return false
	}
	return mt == ct.DefaultMediaType()
}

func FromMediaType(mediaType string) Type {
	switch toOCILayerType[mediaType] {
	case ocispecs.MediaTypeImageLayer, ocispecs.MediaTypeImageLayerNonDistributable:
		return Uncompressed
	case ocispecs.MediaTypeImageLayerGzip, ocispecs.MediaTypeImageLayerNonDistributableGzip:
		return Gzip
	case mediaTypeImageLayerZstd, ocispecs.MediaTypeImageLayerNonDistributableZstd:
		return Zstd
	default:
		return UnknownCompression
	}
}

// DetectLayerMediaType returns media type from existing blob data.
func DetectLayerMediaType(ctx context.Context, cs content.Store, id digest.Digest, oci bool) (string, error) {
	ra, err := cs.ReaderAt(ctx, ocispecs.Descriptor{Digest: id})
	if err != nil {
		return "", err
	}
	defer ra.Close()

	ct, err := detectCompressionType(io.NewSectionReader(ra, 0, ra.Size()))
	if err != nil {
		return "", err
	}

	switch ct {
	case Uncompressed:
		if oci {
			return ocispecs.MediaTypeImageLayer, nil
		}
		return images.MediaTypeDockerSchema2Layer, nil
	case Gzip, EStargz:
		if oci {
			return ocispecs.MediaTypeImageLayerGzip, nil
		}
		return images.MediaTypeDockerSchema2LayerGzip, nil

	default:
		return "", errors.Errorf("failed to detect layer %v compression type", id)
	}
}

// detectCompressionType detects compression type from real blob data.
func detectCompressionType(cr *io.SectionReader) (Type, error) {
	var buf [10]byte
	var n int
	var err error

	if n, err = cr.Read(buf[:]); err != nil && err != io.EOF {
		// Note: we'll ignore any io.EOF error because there are some
		// odd cases where the layer.tar file will be empty (zero bytes)
		// and we'll just treat it as a non-compressed stream and that
		// means just create an empty layer.
		//
		// See issue docker/docker#18170
		return UnknownCompression, err
	}

	if _, _, err := estargz.OpenFooter(cr); err == nil {
		return EStargz, nil
	}

	for c, m := range map[Type][]byte{
		Gzip: {0x1F, 0x8B, 0x08},
		Zstd: {0x28, 0xB5, 0x2F, 0xFD},
	} {
		if n < len(m) {
			continue
		}
		if bytes.Equal(m, buf[:len(m)]) {
			return c, nil
		}
	}

	return Uncompressed, nil
}

var toDockerLayerType = map[string]string{
	ocispecs.MediaTypeImageLayer:                     images.MediaTypeDockerSchema2Layer,
	images.MediaTypeDockerSchema2Layer:               images.MediaTypeDockerSchema2Layer,
	ocispecs.MediaTypeImageLayerGzip:                 images.MediaTypeDockerSchema2LayerGzip,
	images.MediaTypeDockerSchema2LayerGzip:           images.MediaTypeDockerSchema2LayerGzip,
	images.MediaTypeDockerSchema2LayerForeign:        images.MediaTypeDockerSchema2LayerForeign,
	images.MediaTypeDockerSchema2LayerForeignGzip:    images.MediaTypeDockerSchema2LayerForeignGzip,
	ocispecs.MediaTypeImageLayerNonDistributable:     images.MediaTypeDockerSchema2LayerForeign,
	ocispecs.MediaTypeImageLayerNonDistributableGzip: images.MediaTypeDockerSchema2LayerForeignGzip,
	mediaTypeImageLayerZstd:                          mediaTypeDockerSchema2LayerZstd,
	mediaTypeDockerSchema2LayerZstd:                  mediaTypeDockerSchema2LayerZstd,
}

var toOCILayerType = map[string]string{
	ocispecs.MediaTypeImageLayer:                     ocispecs.MediaTypeImageLayer,
	ocispecs.MediaTypeImageLayerNonDistributable:     ocispecs.MediaTypeImageLayerNonDistributable,
	ocispecs.MediaTypeImageLayerNonDistributableGzip: ocispecs.MediaTypeImageLayerNonDistributableGzip,
	ocispecs.MediaTypeImageLayerNonDistributableZstd: ocispecs.MediaTypeImageLayerNonDistributableZstd,
	images.MediaTypeDockerSchema2Layer:               ocispecs.MediaTypeImageLayer,
	ocispecs.MediaTypeImageLayerGzip:                 ocispecs.MediaTypeImageLayerGzip,
	images.MediaTypeDockerSchema2LayerGzip:           ocispecs.MediaTypeImageLayerGzip,
	images.MediaTypeDockerSchema2LayerForeign:        ocispecs.MediaTypeImageLayerNonDistributable,
	images.MediaTypeDockerSchema2LayerForeignGzip:    ocispecs.MediaTypeImageLayerNonDistributableGzip,
	mediaTypeImageLayerZstd:                          mediaTypeImageLayerZstd,
	mediaTypeDockerSchema2LayerZstd:                  mediaTypeImageLayerZstd,
}

func convertLayerMediaType(mediaType string, oci bool) string {
	var converted string
	if oci {
		converted = toOCILayerType[mediaType]
	} else {
		converted = toDockerLayerType[mediaType]
	}
	if converted == "" {
		logrus.Warnf("unhandled conversion for mediatype %q", mediaType)
		return mediaType
	}
	return converted
}

func ConvertAllLayerMediaTypes(oci bool, descs ...ocispecs.Descriptor) []ocispecs.Descriptor {
	var converted []ocispecs.Descriptor
	for _, desc := range descs {
		desc.MediaType = convertLayerMediaType(desc.MediaType, oci)
		converted = append(converted, desc)
	}
	return converted
}
