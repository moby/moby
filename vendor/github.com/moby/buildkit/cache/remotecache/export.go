package remotecache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	v1 "github.com/moby/buildkit/cache/remotecache/v1"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/progress/logs"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ResolveCacheExporterFunc func(ctx context.Context, g session.Group, attrs map[string]string) (Exporter, error)

type Exporter interface {
	solver.CacheExporterTarget
	// Name uniquely identifies the exporter
	Name() string
	// Finalize finalizes and return metadata that are returned to the client
	// e.g. ExporterResponseManifestDesc
	Finalize(ctx context.Context) (map[string]string, error)
	Config() Config
}

type Config struct {
	Compression compression.Config
}

type CacheType int

const (
	// ExportResponseManifestDesc is a key for the map returned from Exporter.Finalize.
	// The map value is a JSON string of an OCI desciptor of a manifest.
	ExporterResponseManifestDesc = "cache.manifest"
)

const (
	NotSet CacheType = iota
	ManifestList
	ImageManifest
)

func (data CacheType) String() string {
	switch data {
	case ManifestList:
		return "Manifest List"
	case ImageManifest:
		return "Image Manifest"
	default:
		return "Not Set"
	}
}

func NewExporter(ingester content.Ingester, ref string, oci bool, imageManifest bool, compressionConfig compression.Config) Exporter {
	cc := v1.NewCacheChains()
	return &contentCacheExporter{CacheExporterTarget: cc, chains: cc, ingester: ingester, oci: oci, imageManifest: imageManifest, ref: ref, comp: compressionConfig}
}

type ExportableCache struct {
	// This cache describes two distinct styles of exportable cache, one is an Index (or Manifest List) of blobs,
	// or as an artifact using the OCI image manifest format.
	ExportedManifest ocispecs.Manifest
	ExportedIndex    ocispecs.Index
	CacheType        CacheType
	OCI              bool
}

func NewExportableCache(oci bool, imageManifest bool) (*ExportableCache, error) {
	var mediaType string

	if imageManifest {
		mediaType = ocispecs.MediaTypeImageManifest
		if !oci {
			return nil, errors.Errorf("invalid configuration for remote cache")
		}
	} else {
		if oci {
			mediaType = ocispecs.MediaTypeImageIndex
		} else {
			mediaType = images.MediaTypeDockerSchema2ManifestList
		}
	}

	cacheType := ManifestList
	if imageManifest {
		cacheType = ImageManifest
	}

	schemaVersion := specs.Versioned{SchemaVersion: 2}
	switch cacheType {
	case ManifestList:
		return &ExportableCache{ExportedIndex: ocispecs.Index{
			MediaType: mediaType,
			Versioned: schemaVersion,
		},
			CacheType: cacheType,
			OCI:       oci,
		}, nil
	case ImageManifest:
		return &ExportableCache{ExportedManifest: ocispecs.Manifest{
			MediaType: mediaType,
			Versioned: schemaVersion,
		},
			CacheType: cacheType,
			OCI:       oci,
		}, nil
	default:
		return nil, errors.Errorf("exportable cache type not set")
	}
}

func (ec *ExportableCache) MediaType() string {
	if ec.CacheType == ManifestList {
		return ec.ExportedIndex.MediaType
	}
	return ec.ExportedManifest.MediaType
}

func (ec *ExportableCache) AddCacheBlob(blob ocispecs.Descriptor) {
	if ec.CacheType == ManifestList {
		ec.ExportedIndex.Manifests = append(ec.ExportedIndex.Manifests, blob)
	} else {
		ec.ExportedManifest.Layers = append(ec.ExportedManifest.Layers, blob)
	}
}

func (ec *ExportableCache) FinalizeCache(ctx context.Context) {
	if ec.CacheType == ManifestList {
		ec.ExportedIndex.Manifests = compression.ConvertAllLayerMediaTypes(ctx, ec.OCI, ec.ExportedIndex.Manifests...)
	} else {
		ec.ExportedManifest.Layers = compression.ConvertAllLayerMediaTypes(ctx, ec.OCI, ec.ExportedManifest.Layers...)
	}
}

func (ec *ExportableCache) SetConfig(config ocispecs.Descriptor) {
	if ec.CacheType == ManifestList {
		ec.ExportedIndex.Manifests = append(ec.ExportedIndex.Manifests, config)
	} else {
		ec.ExportedManifest.Config = config
	}
}

func (ec *ExportableCache) MarshalJSON() ([]byte, error) {
	if ec.CacheType == ManifestList {
		return json.Marshal(ec.ExportedIndex)
	}
	return json.Marshal(ec.ExportedManifest)
}

type contentCacheExporter struct {
	solver.CacheExporterTarget
	chains        *v1.CacheChains
	ingester      content.Ingester
	oci           bool
	imageManifest bool
	ref           string
	comp          compression.Config
}

func (ce *contentCacheExporter) Name() string {
	return "exporting content cache"
}

func (ce *contentCacheExporter) Config() Config {
	return Config{
		Compression: ce.comp,
	}
}

func (ce *contentCacheExporter) Finalize(ctx context.Context) (map[string]string, error) {
	res := make(map[string]string)
	config, descs, err := ce.chains.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	if len(config.Layers) == 0 {
		bklog.G(ctx).Warn("failed to match any cache with layers")
		return nil, progress.OneOff(ctx, "skipping cache export for empty result")(nil)
	}

	cache, err := NewExportableCache(ce.oci, ce.imageManifest)
	if err != nil {
		return nil, err
	}

	for _, l := range config.Layers {
		dgstPair, ok := descs[l.Blob]
		if !ok {
			return nil, errors.Errorf("missing blob %s", l.Blob)
		}
		layerDone := progress.OneOff(ctx, fmt.Sprintf("writing layer %s", l.Blob))
		if err := contentutil.Copy(ctx, ce.ingester, dgstPair.Provider, dgstPair.Descriptor, ce.ref, logs.LoggerFromContext(ctx)); err != nil {
			return nil, layerDone(errors.Wrap(err, "error writing layer blob"))
		}
		layerDone(nil)
		cache.AddCacheBlob(dgstPair.Descriptor)
	}

	cache.FinalizeCache(ctx)

	dt, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	dgst := digest.FromBytes(dt)
	desc := ocispecs.Descriptor{
		Digest:    dgst,
		Size:      int64(len(dt)),
		MediaType: v1.CacheConfigMediaTypeV0,
	}
	configDone := progress.OneOff(ctx, fmt.Sprintf("writing config %s", dgst))
	if err := content.WriteBlob(ctx, ce.ingester, dgst.String(), bytes.NewReader(dt), desc); err != nil {
		return nil, configDone(errors.Wrap(err, "error writing config blob"))
	}
	configDone(nil)

	cache.SetConfig(desc)

	dt, err = cache.MarshalJSON()
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal manifest")
	}
	dgst = digest.FromBytes(dt)

	desc = ocispecs.Descriptor{
		Digest:    dgst,
		Size:      int64(len(dt)),
		MediaType: cache.MediaType(),
	}

	mfstLog := fmt.Sprintf("writing cache manifest %s", dgst)
	if ce.imageManifest {
		mfstLog = fmt.Sprintf("writing cache image manifest %s", dgst)
	}
	mfstDone := progress.OneOff(ctx, mfstLog)
	if err := content.WriteBlob(ctx, ce.ingester, dgst.String(), bytes.NewReader(dt), desc); err != nil {
		return nil, mfstDone(errors.Wrap(err, "error writing manifest blob"))
	}
	descJSON, err := json.Marshal(desc)
	if err != nil {
		return nil, err
	}
	res[ExporterResponseManifestDesc] = string(descJSON)
	mfstDone(nil)

	return res, nil
}
