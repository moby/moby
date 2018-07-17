package remotecache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	v1 "github.com/moby/buildkit/cache/remotecache/v1"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/progress"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ResolveCacheExporterFunc func(ctx context.Context, typ, target string) (Exporter, error)

func oneOffProgress(ctx context.Context, id string) func(err error) error {
	pw, _, _ := progress.FromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
	}
	pw.Write(id, st)
	return func(err error) error {
		now := time.Now()
		st.Completed = &now
		pw.Write(id, st)
		pw.Close()
		return err
	}
}

type Exporter interface {
	solver.CacheExporterTarget
	Finalize(ctx context.Context) error
}

type contentCacheExporter struct {
	solver.CacheExporterTarget
	chains   *v1.CacheChains
	ingester content.Ingester
}

func NewExporter(ingester content.Ingester) Exporter {
	cc := v1.NewCacheChains()
	return &contentCacheExporter{CacheExporterTarget: cc, chains: cc, ingester: ingester}
}

func (ce *contentCacheExporter) Finalize(ctx context.Context) error {
	return export(ctx, ce.ingester, ce.chains)
}

func export(ctx context.Context, ingester content.Ingester, cc *v1.CacheChains) error {
	config, descs, err := cc.Marshal()
	if err != nil {
		return err
	}

	// own type because oci type can't be pushed and docker type doesn't have annotations
	type manifestList struct {
		specs.Versioned

		MediaType string `json:"mediaType,omitempty"`

		// Manifests references platform specific manifests.
		Manifests []ocispec.Descriptor `json:"manifests"`
	}

	var mfst manifestList
	mfst.SchemaVersion = 2
	mfst.MediaType = images.MediaTypeDockerSchema2ManifestList

	for _, l := range config.Layers {
		dgstPair, ok := descs[l.Blob]
		if !ok {
			return errors.Errorf("missing blob %s", l.Blob)
		}
		layerDone := oneOffProgress(ctx, fmt.Sprintf("writing layer %s", l.Blob))
		if err := contentutil.Copy(ctx, ingester, dgstPair.Provider, dgstPair.Descriptor); err != nil {
			return layerDone(errors.Wrap(err, "error writing layer blob"))
		}
		layerDone(nil)
		mfst.Manifests = append(mfst.Manifests, dgstPair.Descriptor)
	}

	dt, err := json.Marshal(config)
	if err != nil {
		return err
	}
	dgst := digest.FromBytes(dt)
	desc := ocispec.Descriptor{
		Digest:    dgst,
		Size:      int64(len(dt)),
		MediaType: v1.CacheConfigMediaTypeV0,
	}
	configDone := oneOffProgress(ctx, fmt.Sprintf("writing config %s", dgst))
	if err := content.WriteBlob(ctx, ingester, dgst.String(), bytes.NewReader(dt), desc); err != nil {
		return configDone(errors.Wrap(err, "error writing config blob"))
	}
	configDone(nil)

	mfst.Manifests = append(mfst.Manifests, desc)

	dt, err = json.Marshal(mfst)
	if err != nil {
		return errors.Wrap(err, "failed to marshal manifest")
	}
	dgst = digest.FromBytes(dt)

	desc = ocispec.Descriptor{
		Digest:    dgst,
		Size:      int64(len(dt)),
		MediaType: mfst.MediaType,
	}
	mfstDone := oneOffProgress(ctx, fmt.Sprintf("writing manifest %s", dgst))
	if err := content.WriteBlob(ctx, ingester, dgst.String(), bytes.NewReader(dt), desc); err != nil {
		return mfstDone(errors.Wrap(err, "error writing manifest blob"))
	}
	mfstDone(nil)
	return nil
}
