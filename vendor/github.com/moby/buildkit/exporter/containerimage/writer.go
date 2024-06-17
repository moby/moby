package containerimage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/platforms"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/attestation"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/exporter/util/epoch"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/result"
	attestationTypes "github.com/moby/buildkit/util/attestation"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/converter"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/purl"
	"github.com/moby/buildkit/util/system"
	"github.com/moby/buildkit/util/tracing"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/package-url/packageurl-go"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

type WriterOpt struct {
	Snapshotter  snapshot.Snapshotter
	ContentStore content.Store
	Applier      diff.Applier
	Differ       diff.Comparer
}

func NewImageWriter(opt WriterOpt) (*ImageWriter, error) {
	return &ImageWriter{opt: opt}, nil
}

type ImageWriter struct {
	opt WriterOpt
}

func (ic *ImageWriter) Commit(ctx context.Context, inp *exporter.Source, sessionID string, inlineCache exptypes.InlineCache, opts *ImageCommitOpts) (*ocispecs.Descriptor, error) {
	if _, ok := inp.Metadata[exptypes.ExporterPlatformsKey]; len(inp.Refs) > 0 && !ok {
		return nil, errors.Errorf("unable to export multiple refs, missing platforms mapping")
	}

	isMap := len(inp.Refs) > 0

	ps, err := exptypes.ParsePlatforms(inp.Metadata)
	if err != nil {
		return nil, err
	}

	if !isMap {
		// enable index if we need to include attestations
		for _, p := range ps.Platforms {
			if atts, ok := inp.Attestations[p.ID]; ok {
				if !opts.ForceInlineAttestations {
					// if we don't need force inline attestations (for oci
					// exporter), filter them out
					atts = attestation.Filter(atts, nil, map[string][]byte{
						result.AttestationInlineOnlyKey: []byte(strconv.FormatBool(true)),
					})
				}
				if len(atts) > 0 {
					isMap = true
					break
				}
			}
		}
	}
	if opts.Epoch == nil {
		if tm, ok, err := epoch.ParseSource(inp); err != nil {
			return nil, err
		} else if ok {
			opts.Epoch = tm
		}
	}

	for pk, a := range opts.Annotations {
		if pk != "" {
			if _, ok := inp.FindRef(pk); !ok {
				return nil, errors.Errorf("invalid annotation: no platform %s found in source", pk)
			}
		}
		if len(a.Index)+len(a.IndexDescriptor)+len(a.ManifestDescriptor) > 0 {
			opts.EnableOCITypes(ctx, "annotations")
		}
	}

	if !isMap {
		if len(ps.Platforms) > 1 {
			return nil, errors.Errorf("cannot export multiple platforms without multi-platform enabled")
		}

		var ref cache.ImmutableRef
		var p *exptypes.Platform
		if len(ps.Platforms) > 0 {
			p = &ps.Platforms[0]
			if r, ok := inp.FindRef(p.ID); ok {
				ref = r
			}
		} else {
			ref = inp.Ref
		}
		config := exptypes.ParseKey(inp.Metadata, exptypes.ExporterImageConfigKey, p)
		baseImgConfig := exptypes.ParseKey(inp.Metadata, exptypes.ExporterImageBaseConfigKey, p)
		var baseImg *dockerspec.DockerOCIImage
		if len(baseImgConfig) > 0 {
			var baseImgX dockerspec.DockerOCIImage
			if err := json.Unmarshal(baseImgConfig, &baseImgX); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal base image config")
			}
			baseImg = &baseImgX
		}

		remotes, err := ic.exportLayers(ctx, opts.RefCfg, session.NewGroup(sessionID), ref)
		if err != nil {
			return nil, err
		}
		remote := &remotes[0]
		if opts.RewriteTimestamp {
			remote, err = ic.rewriteRemoteWithEpoch(ctx, opts, remote, baseImg)
			if err != nil {
				return nil, err
			}
		}

		annotations := opts.Annotations.Platform(nil)
		if len(annotations.Index) > 0 || len(annotations.IndexDescriptor) > 0 {
			return nil, errors.Errorf("index annotations not supported for single platform export")
		}

		var inlineCacheEntry *exptypes.InlineCacheEntry
		if inlineCache != nil {
			inlineCacheResult, err := inlineCache(ctx)
			if err != nil {
				return nil, err
			}
			if inlineCacheResult != nil {
				if p != nil {
					inlineCacheEntry, _ = inlineCacheResult.FindRef(p.ID)
				} else {
					inlineCacheEntry = inlineCacheResult.Ref
				}
			}
		}

		mfstDesc, configDesc, err := ic.commitDistributionManifest(ctx, opts, ref, config, remote, annotations, inlineCacheEntry, opts.Epoch, session.NewGroup(sessionID), baseImg)
		if err != nil {
			return nil, err
		}
		if mfstDesc.Annotations == nil {
			mfstDesc.Annotations = make(map[string]string)
		}
		if len(ps.Platforms) == 1 {
			mfstDesc.Platform = &ps.Platforms[0].Platform
		}
		mfstDesc.Annotations[exptypes.ExporterConfigDigestKey] = configDesc.Digest.String()

		return mfstDesc, nil
	}

	if len(inp.Attestations) > 0 {
		opts.EnableOCITypes(ctx, "attestations")
	}

	refs := make([]cache.ImmutableRef, 0, len(inp.Refs))
	remotesMap := make(map[string]int, len(inp.Refs))
	for _, p := range ps.Platforms {
		r, ok := inp.FindRef(p.ID)
		if !ok {
			return nil, errors.Errorf("failed to find ref for ID %s", p.ID)
		}
		remotesMap[p.ID] = len(refs)
		refs = append(refs, r)
	}

	remotes, err := ic.exportLayers(ctx, opts.RefCfg, session.NewGroup(sessionID), refs...)
	if err != nil {
		return nil, err
	}

	var inlineCacheResult *result.Result[*exptypes.InlineCacheEntry]
	if inlineCache != nil {
		inlineCacheResult, err = inlineCache(ctx)
		if err != nil {
			return nil, err
		}
	}

	idx := ocispecs.Index{
		MediaType:   ocispecs.MediaTypeImageIndex,
		Annotations: opts.Annotations.Platform(nil).Index,
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
	}

	if !opts.OCITypes {
		idx.MediaType = images.MediaTypeDockerSchema2ManifestList
	}

	labels := map[string]string{}

	var attestationManifests []ocispecs.Descriptor

	for i, p := range ps.Platforms {
		r, ok := inp.FindRef(p.ID)
		if !ok {
			return nil, errors.Errorf("failed to find ref for ID %s", p.ID)
		}
		config := exptypes.ParseKey(inp.Metadata, exptypes.ExporterImageConfigKey, &p)
		baseImgConfig := exptypes.ParseKey(inp.Metadata, exptypes.ExporterImageBaseConfigKey, &p)
		var baseImg *dockerspec.DockerOCIImage
		if len(baseImgConfig) > 0 {
			var baseImgX dockerspec.DockerOCIImage
			if err := json.Unmarshal(baseImgConfig, &baseImgX); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal base image config")
			}
			baseImg = &baseImgX
		}

		remote := &remotes[remotesMap[p.ID]]
		if remote == nil {
			remote = &solver.Remote{
				Provider: ic.opt.ContentStore,
			}
		}
		if opts.RewriteTimestamp {
			remote, err = ic.rewriteRemoteWithEpoch(ctx, opts, remote, baseImg)
			if err != nil {
				return nil, err
			}
		}

		var inlineCacheEntry *exptypes.InlineCacheEntry
		if inlineCacheResult != nil {
			inlineCacheEntry, _ = inlineCacheResult.FindRef(p.ID)
		}

		desc, _, err := ic.commitDistributionManifest(ctx, opts, r, config, remote, opts.Annotations.Platform(&p.Platform), inlineCacheEntry, opts.Epoch, session.NewGroup(sessionID), baseImg)
		if err != nil {
			return nil, err
		}
		dp := p.Platform
		desc.Platform = &dp
		idx.Manifests = append(idx.Manifests, *desc)

		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i)] = desc.Digest.String()

		if attestations, ok := inp.Attestations[p.ID]; ok {
			attestations, err := attestation.Unbundle(ctx, session.NewGroup(sessionID), attestations)
			if err != nil {
				return nil, err
			}

			eg, ctx2 := errgroup.WithContext(ctx)
			for i, att := range attestations {
				i, att := i, att
				eg.Go(func() error {
					att, err := supplementSBOM(ctx2, session.NewGroup(sessionID), r, remote, att)
					if err != nil {
						return err
					}
					attestations[i] = att
					return nil
				})
			}
			if err := eg.Wait(); err != nil {
				return nil, err
			}

			var defaultSubjects []intoto.Subject
			for _, name := range strings.Split(opts.ImageName, ",") {
				if name == "" {
					continue
				}
				pl, err := purl.RefToPURL(packageurl.TypeDocker, name, &p.Platform)
				if err != nil {
					return nil, err
				}
				defaultSubjects = append(defaultSubjects, intoto.Subject{
					Name:   pl,
					Digest: result.ToDigestMap(desc.Digest),
				})
			}
			stmts, err := attestation.MakeInTotoStatements(ctx, session.NewGroup(sessionID), attestations, defaultSubjects)
			if err != nil {
				return nil, err
			}

			desc, err := ic.commitAttestationsManifest(ctx, opts, desc.Digest.String(), stmts)
			if err != nil {
				return nil, err
			}
			desc.Platform = &intotoPlatform
			attestationManifests = append(attestationManifests, *desc)
		}
	}

	for i, mfst := range attestationManifests {
		idx.Manifests = append(idx.Manifests, mfst)
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", len(ps.Platforms)+i)] = mfst.Digest.String()
	}

	idxBytes, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal index")
	}

	idxDigest := digest.FromBytes(idxBytes)
	idxDesc := ocispecs.Descriptor{
		Digest:      idxDigest,
		Size:        int64(len(idxBytes)),
		MediaType:   idx.MediaType,
		Annotations: opts.Annotations.Platform(nil).IndexDescriptor,
	}
	idxDone := progress.OneOff(ctx, "exporting manifest list "+idxDigest.String())

	if err := content.WriteBlob(ctx, ic.opt.ContentStore, idxDigest.String(), bytes.NewReader(idxBytes), idxDesc, content.WithLabels(labels)); err != nil {
		return nil, idxDone(errors.Wrapf(err, "error writing manifest list blob %s", idxDigest))
	}
	idxDone(nil)

	return &idxDesc, nil
}

func (ic *ImageWriter) exportLayers(ctx context.Context, refCfg cacheconfig.RefConfig, s session.Group, refs ...cache.ImmutableRef) ([]solver.Remote, error) {
	attr := []attribute.KeyValue{
		attribute.String("exportLayers.compressionType", refCfg.Compression.Type.String()),
		attribute.Bool("exportLayers.forceCompression", refCfg.Compression.Force),
	}
	if refCfg.Compression.Level != nil {
		attr = append(attr, attribute.Int("exportLayers.compressionLevel", *refCfg.Compression.Level))
	}
	span, ctx := tracing.StartSpan(ctx, "export layers", trace.WithAttributes(attr...))

	eg, ctx := errgroup.WithContext(ctx)
	layersDone := progress.OneOff(ctx, "exporting layers")

	out := make([]solver.Remote, len(refs))

	for i, ref := range refs {
		func(i int, ref cache.ImmutableRef) {
			if ref == nil {
				return
			}
			eg.Go(func() error {
				remotes, err := ref.GetRemotes(ctx, true, refCfg, false, s)
				if err != nil {
					return err
				}
				remote := remotes[0]
				out[i] = *remote
				return nil
			})
		}(i, ref)
	}

	err := layersDone(eg.Wait())
	tracing.FinishWithError(span, err)
	return out, err
}

// rewriteImageLayerWithEpoch rewrites the file timestamps in the layer blob to match the epoch, and returns a new descriptor that points to
// the new blob.
//
// If no conversion is needed, this returns nil without error.
func rewriteImageLayerWithEpoch(ctx context.Context, cs content.Store, desc ocispecs.Descriptor, comp compression.Config, epoch *time.Time, immDiffID digest.Digest) (*ocispecs.Descriptor, error) {
	var immDiffIDs map[digest.Digest]struct{}
	if immDiffID != "" {
		immDiffIDs = map[digest.Digest]struct{}{
			immDiffID: {},
		}
	}
	converterFn, err := converter.NewWithRewriteTimestamp(ctx, cs, desc, comp, epoch, immDiffIDs)
	if err != nil {
		return nil, err
	}
	if converterFn == nil {
		return nil, nil
	}
	return converterFn(ctx, cs, desc)
}

func (ic *ImageWriter) rewriteRemoteWithEpoch(ctx context.Context, opts *ImageCommitOpts, remote *solver.Remote, baseImg *dockerspec.DockerOCIImage) (*solver.Remote, error) {
	if opts.Epoch == nil {
		bklog.G(ctx).Warn("rewrite-timestamp is specified, but no source-date-epoch was found")
		return remote, nil
	}
	remoteDescriptors := remote.Descriptors
	cs := contentutil.NewStoreWithProvider(ic.opt.ContentStore, remote.Provider)
	eg, ctx := errgroup.WithContext(ctx)
	rewriteDone := progress.OneOff(ctx,
		fmt.Sprintf("rewriting layers with source-date-epoch %d (%s)", opts.Epoch.Unix(), opts.Epoch.String()))
	var divergedFromBase bool
	for i, desc := range remoteDescriptors {
		i, desc := i, desc
		// Usually we get non-empty diffID here, but if the content was ingested via a third-party containerd client,
		// diffID here can be empty, and will be computed by the converter.
		diffID := digest.Digest(desc.Annotations[labels.LabelUncompressed])
		if diffID == "" {
			info, err := cs.Info(ctx, desc.Digest)
			if err != nil {
				return nil, err
			}
			diffID = digest.Digest(info.Labels[labels.LabelUncompressed]) // can be still empty
		}
		var immDiffID digest.Digest
		if !divergedFromBase && baseImg != nil && i < len(baseImg.RootFS.DiffIDs) {
			immDiffID = baseImg.RootFS.DiffIDs[i]
			if immDiffID == diffID {
				bklog.G(ctx).WithField("blob", desc).Debugf("Not rewriting to apply epoch (immutable diffID %q)", diffID)
				continue
			}
			divergedFromBase = true
		}
		eg.Go(func() error {
			if rewrittenDesc, err := rewriteImageLayerWithEpoch(ctx, cs, desc, opts.RefCfg.Compression, opts.Epoch, immDiffID); err != nil {
				bklog.G(ctx).WithError(err).Warnf("failed to rewrite layer %d/%d to match source-date-epoch %d (%s)",
					i+1, len(remoteDescriptors), opts.Epoch.Unix(), opts.Epoch.String())
			} else if rewrittenDesc != nil {
				remoteDescriptors[i] = *rewrittenDesc
			}
			return nil
		})
	}
	if err := rewriteDone(eg.Wait()); err != nil {
		return nil, err
	}
	return &solver.Remote{
		Provider:    cs,
		Descriptors: remoteDescriptors,
	}, nil
}

func (ic *ImageWriter) commitDistributionManifest(ctx context.Context, opts *ImageCommitOpts, ref cache.ImmutableRef, config []byte, remote *solver.Remote, annotations *Annotations, inlineCache *exptypes.InlineCacheEntry, epoch *time.Time, sg session.Group, baseImg *dockerspec.DockerOCIImage) (*ocispecs.Descriptor, *ocispecs.Descriptor, error) {
	if len(config) == 0 {
		var err error
		config, err = defaultImageConfig()
		if err != nil {
			return nil, nil, err
		}
	}

	history, err := parseHistoryFromConfig(config)
	if err != nil {
		return nil, nil, err
	}

	remote, history, err = patchImageLayers(ctx, remote, history, ref, opts, sg)
	if err != nil {
		return nil, nil, err
	}

	config, err = patchImageConfig(config, remote.Descriptors, history, inlineCache, epoch, baseImg)
	if err != nil {
		return nil, nil, err
	}

	var (
		configDigest = digest.FromBytes(config)
		manifestType = ocispecs.MediaTypeImageManifest
		configType   = ocispecs.MediaTypeImageConfig
	)

	// Use docker media types for older Docker versions and registries
	if !opts.OCITypes {
		manifestType = images.MediaTypeDockerSchema2Manifest
		configType = images.MediaTypeDockerSchema2Config
	}

	mfst := ocispecs.Manifest{
		MediaType:   manifestType,
		Annotations: annotations.Manifest,
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Config: ocispecs.Descriptor{
			Digest:    configDigest,
			Size:      int64(len(config)),
			MediaType: configType,
		},
	}

	labels := map[string]string{
		"containerd.io/gc.ref.content.0": configDigest.String(),
	}

	for i, desc := range remote.Descriptors {
		desc.Annotations = RemoveInternalLayerAnnotations(desc.Annotations, opts.OCITypes)
		mfst.Layers = append(mfst.Layers, desc)
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i+1)] = desc.Digest.String()
	}

	mfstJSON, err := json.MarshalIndent(mfst, "", "  ")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal manifest")
	}

	mfstDigest := digest.FromBytes(mfstJSON)
	mfstDesc := ocispecs.Descriptor{
		Digest: mfstDigest,
		Size:   int64(len(mfstJSON)),
	}
	mfstDone := progress.OneOff(ctx, "exporting manifest "+mfstDigest.String())

	if err := content.WriteBlob(ctx, ic.opt.ContentStore, mfstDigest.String(), bytes.NewReader(mfstJSON), mfstDesc, content.WithLabels((labels))); err != nil {
		return nil, nil, mfstDone(errors.Wrapf(err, "error writing manifest blob %s", mfstDigest))
	}
	mfstDone(nil)

	configDesc := ocispecs.Descriptor{
		Digest:    configDigest,
		Size:      int64(len(config)),
		MediaType: configType,
	}
	configDone := progress.OneOff(ctx, "exporting config "+configDigest.String())

	if err := content.WriteBlob(ctx, ic.opt.ContentStore, configDigest.String(), bytes.NewReader(config), configDesc); err != nil {
		return nil, nil, configDone(errors.Wrap(err, "error writing config blob"))
	}
	configDone(nil)

	return &ocispecs.Descriptor{
		Annotations: annotations.ManifestDescriptor,
		Digest:      mfstDigest,
		Size:        int64(len(mfstJSON)),
		MediaType:   manifestType,
	}, &configDesc, nil
}

func (ic *ImageWriter) commitAttestationsManifest(ctx context.Context, opts *ImageCommitOpts, target string, statements []intoto.Statement) (*ocispecs.Descriptor, error) {
	var (
		manifestType = ocispecs.MediaTypeImageManifest
		configType   = ocispecs.MediaTypeImageConfig
	)
	if !opts.OCITypes {
		manifestType = images.MediaTypeDockerSchema2Manifest
		configType = images.MediaTypeDockerSchema2Config
	}

	layers := make([]ocispecs.Descriptor, len(statements))
	for i, statement := range statements {
		i, statement := i, statement

		data, err := json.Marshal(statement)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal attestation")
		}
		digest := digest.FromBytes(data)
		desc := ocispecs.Descriptor{
			MediaType: intoto.PayloadType,
			Digest:    digest,
			Size:      int64(len(data)),
			Annotations: map[string]string{
				labels.LabelUncompressed:    digest.String(),
				"in-toto.io/predicate-type": statement.PredicateType,
			},
		}

		if err := content.WriteBlob(ctx, ic.opt.ContentStore, digest.String(), bytes.NewReader(data), desc); err != nil {
			return nil, errors.Wrapf(err, "error writing data blob %s", digest)
		}
		layers[i] = desc
	}

	config, err := attestationsConfig(layers)
	if err != nil {
		return nil, err
	}
	configDigest := digest.FromBytes(config)
	configDesc := ocispecs.Descriptor{
		Digest:    configDigest,
		Size:      int64(len(config)),
		MediaType: configType,
	}

	mfst := ocispecs.Manifest{
		MediaType: manifestType,
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Config: ocispecs.Descriptor{
			Digest:    configDigest,
			Size:      int64(len(config)),
			MediaType: configType,
		},
	}

	labels := map[string]string{
		"containerd.io/gc.ref.content.0": configDigest.String(),
	}
	for i, desc := range layers {
		desc.Annotations = RemoveInternalLayerAnnotations(desc.Annotations, opts.OCITypes)
		mfst.Layers = append(mfst.Layers, desc)
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i+1)] = desc.Digest.String()
	}

	mfstJSON, err := json.MarshalIndent(mfst, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal manifest")
	}

	mfstDigest := digest.FromBytes(mfstJSON)
	mfstDesc := ocispecs.Descriptor{
		Digest: mfstDigest,
		Size:   int64(len(mfstJSON)),
	}

	done := progress.OneOff(ctx, "exporting attestation manifest "+mfstDigest.String())
	if err := content.WriteBlob(ctx, ic.opt.ContentStore, mfstDigest.String(), bytes.NewReader(mfstJSON), mfstDesc, content.WithLabels((labels))); err != nil {
		return nil, done(errors.Wrapf(err, "error writing manifest blob %s", mfstDigest))
	}
	if err := content.WriteBlob(ctx, ic.opt.ContentStore, configDigest.String(), bytes.NewReader(config), configDesc); err != nil {
		return nil, done(errors.Wrap(err, "error writing config blob"))
	}
	done(nil)

	return &ocispecs.Descriptor{
		Digest:    mfstDigest,
		Size:      int64(len(mfstJSON)),
		MediaType: manifestType,
		Annotations: map[string]string{
			attestationTypes.DockerAnnotationReferenceType:   attestationTypes.DockerAnnotationReferenceTypeDefault,
			attestationTypes.DockerAnnotationReferenceDigest: target,
		},
	}, nil
}

func (ic *ImageWriter) ContentStore() content.Store {
	return ic.opt.ContentStore
}

func (ic *ImageWriter) Snapshotter() snapshot.Snapshotter {
	return ic.opt.Snapshotter
}

func (ic *ImageWriter) Applier() diff.Applier {
	return ic.opt.Applier
}

func defaultImageConfig() ([]byte, error) {
	pl := platforms.Normalize(platforms.DefaultSpec())

	img := ocispecs.Image{}
	img.Architecture = pl.Architecture
	img.OS = pl.OS
	img.OSVersion = pl.OSVersion
	img.OSFeatures = pl.OSFeatures
	img.Variant = pl.Variant
	img.RootFS.Type = "layers"
	img.Config.WorkingDir = "/"
	img.Config.Env = []string{"PATH=" + system.DefaultPathEnv(pl.OS)}
	dt, err := json.Marshal(img)
	return dt, errors.Wrap(err, "failed to create empty image config")
}

func attestationsConfig(layers []ocispecs.Descriptor) ([]byte, error) {
	img := ocispecs.Image{}
	img.Architecture = intotoPlatform.Architecture
	img.OS = intotoPlatform.OS
	img.OSVersion = intotoPlatform.OSVersion
	img.OSFeatures = intotoPlatform.OSFeatures
	img.Variant = intotoPlatform.Variant
	img.RootFS.Type = "layers"
	for _, layer := range layers {
		img.RootFS.DiffIDs = append(img.RootFS.DiffIDs, digest.Digest(layer.Annotations[labels.LabelUncompressed]))
	}
	dt, err := json.Marshal(img)
	return dt, errors.Wrap(err, "failed to create attestations image config")
}

func parseHistoryFromConfig(dt []byte) ([]ocispecs.History, error) {
	var config struct {
		History []ocispecs.History
	}
	if err := json.Unmarshal(dt, &config); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal history from config")
	}
	return config.History, nil
}

func patchImageConfig(dt []byte, descs []ocispecs.Descriptor, history []ocispecs.History, cache *exptypes.InlineCacheEntry, epoch *time.Time, baseImg *dockerspec.DockerOCIImage) ([]byte, error) {
	var img ocispecs.Image
	if err := json.Unmarshal(dt, &img); err != nil {
		return nil, errors.Wrap(err, "invalid image config for export")
	}

	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(dt, &m); err != nil {
		return nil, errors.Wrap(err, "failed to parse image config for patch")
	}

	if m == nil {
		return nil, errors.Errorf("invalid null image config for export")
	}

	if img.OS == "" {
		return nil, errors.Errorf("invalid image config for export: missing os")
	}
	if img.Architecture == "" {
		return nil, errors.Errorf("invalid image config for export: missing architecture")
	}

	var rootFS ocispecs.RootFS
	rootFS.Type = "layers"
	for _, desc := range descs {
		rootFS.DiffIDs = append(rootFS.DiffIDs, digest.Digest(desc.Annotations[labels.LabelUncompressed]))
	}
	dt, err := json.Marshal(rootFS)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal rootfs")
	}
	m["rootfs"] = dt

	if epoch != nil {
		var divergedFromBase bool
		for i, h := range history {
			if !divergedFromBase && baseImg != nil && i < len(baseImg.History) && reflect.DeepEqual(h, baseImg.History[i]) {
				// Retain the timestamp for the base image layers
				// https://github.com/moby/buildkit/issues/4614
				continue
			}
			divergedFromBase = true
			if h.Created == nil || h.Created.After(*epoch) {
				history[i].Created = epoch
			}
		}
	}

	dt, err = json.Marshal(history)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal history")
	}
	m["history"] = dt

	// if epoch is set then clamp creation time
	if v, ok := m["created"]; ok && epoch != nil {
		var tm time.Time
		if err := json.Unmarshal(v, &tm); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal creation time %q", m["created"])
		}
		if tm.After(*epoch) {
			dt, err = json.Marshal(&epoch)
			if err != nil {
				return nil, errors.Wrap(err, "failed to marshal creation time")
			}
			m["created"] = dt
		}
	}

	if _, ok := m["created"]; !ok {
		var tm *time.Time
		for _, h := range history {
			if h.Created != nil {
				tm = h.Created
			}
		}
		dt, err = json.Marshal(&tm)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal creation time")
		}
		m["created"] = dt
	}

	if cache != nil {
		dt, err := json.Marshal(cache.Data)
		if err != nil {
			return nil, err
		}
		m["moby.buildkit.cache.v0"] = dt
	}

	dt, err = json.Marshal(m)
	return dt, errors.Wrap(err, "failed to marshal config after patch")
}

func normalizeLayersAndHistory(ctx context.Context, remote *solver.Remote, history []ocispecs.History, ref cache.ImmutableRef, oci bool) (*solver.Remote, []ocispecs.History) {
	refMeta := getRefMetadata(ref, len(remote.Descriptors))

	var historyLayers int
	for _, h := range history {
		if !h.EmptyLayer {
			historyLayers++
		}
	}

	if historyLayers > len(remote.Descriptors) {
		// this case shouldn't happen but if it does force set history layers empty
		// from the bottom
		bklog.G(ctx).Warn("invalid image config with unaccounted layers")
		historyCopy := make([]ocispecs.History, 0, len(history))
		var l int
		for _, h := range history {
			if l >= len(remote.Descriptors) {
				h.EmptyLayer = true
			}
			if !h.EmptyLayer {
				l++
			}
			historyCopy = append(historyCopy, h)
		}
		history = historyCopy
	}

	if len(remote.Descriptors) > historyLayers {
		// some history items are missing. add them based on the ref metadata
		for _, md := range refMeta[historyLayers:] {
			history = append(history, ocispecs.History{
				Created:   md.createdAt,
				CreatedBy: md.description,
				Comment:   "buildkit.exporter.image.v0",
			})
		}
	}

	var layerIndex int
	for i, h := range history {
		if !h.EmptyLayer {
			if h.Created == nil {
				h.Created = refMeta[layerIndex].createdAt
			}
			layerIndex++
		}
		history[i] = h
	}

	// Find the first new layer time. Otherwise, the history item for a first
	// metadata command would be the creation time of a base image layer.
	// If there is no such then the last layer with timestamp.
	var created *time.Time
	var noCreatedTime bool
	for _, h := range history {
		if h.Created != nil {
			created = h.Created
			if noCreatedTime {
				break
			}
		} else {
			noCreatedTime = true
		}
	}

	// Fill in created times for all history items to be either the first new
	// layer time or the previous layer.
	noCreatedTime = false
	for i, h := range history {
		if h.Created != nil {
			if noCreatedTime {
				created = h.Created
			}
		} else {
			noCreatedTime = true
			h.Created = created
		}
		history[i] = h
	}

	// convert between oci and docker media types (or vice versa) if needed
	remote.Descriptors = compression.ConvertAllLayerMediaTypes(ctx, oci, remote.Descriptors...)

	return remote, history
}

func RemoveInternalLayerAnnotations(in map[string]string, oci bool) map[string]string {
	if len(in) == 0 || !oci {
		return nil
	}
	m := make(map[string]string, len(in))
	for k, v := range in {
		// oci supports annotations but don't export internal annotations
		switch k {
		case labels.LabelUncompressed, "buildkit/createdat":
			continue
		default:
			if strings.HasPrefix(k, "containerd.io/distribution.source.") {
				continue
			}
			m[k] = v
		}
	}
	return m
}

type refMetadata struct {
	description string
	createdAt   *time.Time
}

func getRefMetadata(ref cache.ImmutableRef, limit int) []refMetadata {
	if ref == nil {
		return make([]refMetadata, limit)
	}

	layerChain := ref.LayerChain()
	defer layerChain.Release(context.TODO())

	if limit < len(layerChain) {
		layerChain = layerChain[len(layerChain)-limit:]
	}

	metas := make([]refMetadata, len(layerChain))
	for i, layer := range layerChain {
		meta := &metas[i]

		if description := layer.GetDescription(); description != "" {
			meta.description = description
		} else {
			meta.description = "created by buildkit" // shouldn't be shown but don't fail build
		}

		createdAt := layer.GetCreatedAt()
		meta.createdAt = &createdAt
	}
	return metas
}
