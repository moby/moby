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
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/push"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ExporterOpt struct {
	SessionManager *session.Manager
}

func NewCacheExporter(opt ExporterOpt) *CacheExporter {
	return &CacheExporter{opt: opt}
}

type CacheExporter struct {
	opt ExporterOpt
}

func (ce *CacheExporter) ExporterForTarget(target string) *RegistryCacheExporter {
	cc := v1.NewCacheChains()
	return &RegistryCacheExporter{target: target, CacheExporterTarget: cc, chains: cc, exporter: ce}
}

func (ce *CacheExporter) Finalize(ctx context.Context, cc *v1.CacheChains, target string) error {
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

	allBlobs := map[digest.Digest]struct{}{}
	mp := contentutil.NewMultiProvider(nil)
	for _, l := range config.Layers {
		if _, ok := allBlobs[l.Blob]; ok {
			continue
		}
		dgstPair, ok := descs[l.Blob]
		if !ok {
			return errors.Errorf("missing blob %s", l.Blob)
		}
		allBlobs[l.Blob] = struct{}{}
		mp.Add(l.Blob, dgstPair.Provider)

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
	buf := contentutil.NewBuffer()
	if err := content.WriteBlob(ctx, buf, dgst.String(), bytes.NewReader(dt), desc); err != nil {
		return configDone(errors.Wrap(err, "error writing config blob"))
	}
	configDone(nil)

	mp.Add(dgst, buf)
	mfst.Manifests = append(mfst.Manifests, desc)

	dt, err = json.Marshal(mfst)
	if err != nil {
		return errors.Wrap(err, "failed to marshal manifest")
	}
	dgst = digest.FromBytes(dt)

	buf = contentutil.NewBuffer()
	desc = ocispec.Descriptor{
		Digest: dgst,
		Size:   int64(len(dt)),
	}
	mfstDone := oneOffProgress(ctx, fmt.Sprintf("writing manifest %s", dgst))
	if err := content.WriteBlob(ctx, buf, dgst.String(), bytes.NewReader(dt), desc); err != nil {
		return mfstDone(errors.Wrap(err, "error writing manifest blob"))
	}
	mfstDone(nil)
	mp.Add(dgst, buf)

	return push.Push(ctx, ce.opt.SessionManager, mp, dgst, target, false)
}

type RegistryCacheExporter struct {
	solver.CacheExporterTarget
	chains   *v1.CacheChains
	target   string
	exporter *CacheExporter
}

func (ce *RegistryCacheExporter) Finalize(ctx context.Context) error {
	return ce.exporter.Finalize(ctx, ce.chains, ce.target)
}

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
