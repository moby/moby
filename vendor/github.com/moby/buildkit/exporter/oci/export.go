package oci

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	archiveexporter "github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/leases"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
)

type ExporterVariant string

const (
	keyImageName        = "name"
	keyLayerCompression = "compression"
	VariantOCI          = "oci"
	VariantDocker       = "docker"
	ociTypes            = "oci-mediatypes"
	keyForceCompression = "force-compression"
	keyCompressionLevel = "compression-level"
	keyBuildInfo        = "buildinfo"
	keyBuildInfoAttrs   = "buildinfo-attrs"
	// preferNondistLayersKey is an exporter option which can be used to mark a layer as non-distributable if the layer reference was
	// already found to use a non-distributable media type.
	// When this option is not set, the exporter will change the media type of the layer to a distributable one.
	preferNondistLayersKey = "prefer-nondist-layers"
)

type Opt struct {
	SessionManager *session.Manager
	ImageWriter    *containerimage.ImageWriter
	Variant        ExporterVariant
	LeaseManager   leases.Manager
}

type imageExporter struct {
	opt Opt
}

func New(opt Opt) (exporter.Exporter, error) {
	im := &imageExporter{opt: opt}
	return im, nil
}

func (e *imageExporter) Resolve(ctx context.Context, opt map[string]string) (exporter.ExporterInstance, error) {
	var ot *bool
	i := &imageExporterInstance{
		imageExporter:    e,
		layerCompression: compression.Default,
		buildInfo:        true,
	}
	var esgz bool
	for k, v := range opt {
		switch k {
		case keyImageName:
			i.name = v
		case keyLayerCompression:
			switch v {
			case "gzip":
				i.layerCompression = compression.Gzip
			case "estargz":
				i.layerCompression = compression.EStargz
				esgz = true
			case "zstd":
				i.layerCompression = compression.Zstd
			case "uncompressed":
				i.layerCompression = compression.Uncompressed
			default:
				return nil, errors.Errorf("unsupported layer compression type: %v", v)
			}
		case keyForceCompression:
			if v == "" {
				i.forceCompression = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value %v specified for %s", v, k)
			}
			i.forceCompression = b
		case keyCompressionLevel:
			ii, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "non-int value %s specified for %s", v, k)
			}
			v := int(ii)
			i.compressionLevel = &v
		case ociTypes:
			ot = new(bool)
			if v == "" {
				*ot = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			*ot = b
		case keyBuildInfo:
			if v == "" {
				i.buildInfo = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.buildInfo = b
		case keyBuildInfoAttrs:
			if v == "" {
				i.buildInfoAttrs = false
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.buildInfoAttrs = b
		case preferNondistLayersKey:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.preferNonDist = b
		default:
			if i.meta == nil {
				i.meta = make(map[string][]byte)
			}
			i.meta[k] = []byte(v)
		}
	}
	if ot == nil {
		i.ociTypes = e.opt.Variant == VariantOCI
	} else {
		i.ociTypes = *ot
	}
	if esgz && !i.ociTypes {
		logrus.Warn("forcibly turning on oci-mediatype mode for estargz")
		i.ociTypes = true
	}
	return i, nil
}

type imageExporterInstance struct {
	*imageExporter
	meta             map[string][]byte
	name             string
	ociTypes         bool
	layerCompression compression.Type
	forceCompression bool
	compressionLevel *int
	buildInfo        bool
	buildInfoAttrs   bool
	preferNonDist    bool
}

func (e *imageExporterInstance) Name() string {
	return "exporting to oci image format"
}

func (e *imageExporterInstance) Config() exporter.Config {
	return exporter.Config{
		Compression: e.compression(),
	}
}

func (e *imageExporterInstance) compression() compression.Config {
	c := compression.New(e.layerCompression).SetForce(e.forceCompression)
	if e.compressionLevel != nil {
		c = c.SetLevel(*e.compressionLevel)
	}
	return c
}

func (e *imageExporterInstance) refCfg() cacheconfig.RefConfig {
	return cacheconfig.RefConfig{
		Compression:            e.compression(),
		PreferNonDistributable: e.preferNonDist,
	}
}

func (e *imageExporterInstance) Export(ctx context.Context, src exporter.Source, sessionID string) (map[string]string, error) {
	if e.opt.Variant == VariantDocker && len(src.Refs) > 0 {
		return nil, errors.Errorf("docker exporter does not currently support exporting manifest lists")
	}

	if src.Metadata == nil {
		src.Metadata = make(map[string][]byte)
	}
	for k, v := range e.meta {
		src.Metadata[k] = v
	}

	ctx, done, err := leaseutil.WithLease(ctx, e.opt.LeaseManager, leaseutil.MakeTemporary)
	if err != nil {
		return nil, err
	}
	defer done(context.TODO())

	desc, err := e.opt.ImageWriter.Commit(ctx, src, e.ociTypes, e.refCfg(), e.buildInfo, e.buildInfoAttrs, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		e.opt.ImageWriter.ContentStore().Delete(context.TODO(), desc.Digest)
	}()

	if desc.Annotations == nil {
		desc.Annotations = map[string]string{}
	}
	desc.Annotations[ocispecs.AnnotationCreated] = time.Now().UTC().Format(time.RFC3339)

	resp := make(map[string]string)

	resp[exptypes.ExporterImageDigestKey] = desc.Digest.String()
	if v, ok := desc.Annotations[exptypes.ExporterConfigDigestKey]; ok {
		resp[exptypes.ExporterImageConfigDigestKey] = v
		delete(desc.Annotations, exptypes.ExporterConfigDigestKey)
	}

	dtdesc, err := json.Marshal(desc)
	if err != nil {
		return nil, err
	}
	resp[exptypes.ExporterImageDescriptorKey] = base64.StdEncoding.EncodeToString(dtdesc)

	if n, ok := src.Metadata["image.name"]; e.name == "*" && ok {
		e.name = string(n)
	}

	names, err := normalizedNames(e.name)
	if err != nil {
		return nil, err
	}

	if len(names) != 0 {
		resp["image.name"] = strings.Join(names, ",")
	}

	expOpts := []archiveexporter.ExportOpt{archiveexporter.WithManifest(*desc, names...)}
	switch e.opt.Variant {
	case VariantOCI:
		expOpts = append(expOpts, archiveexporter.WithAllPlatforms(), archiveexporter.WithSkipDockerManifest())
	case VariantDocker:
	default:
		return nil, errors.Errorf("invalid variant %q", e.opt.Variant)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	caller, err := e.opt.SessionManager.Get(timeoutCtx, sessionID, false)
	if err != nil {
		return nil, err
	}

	w, err := filesync.CopyFileWriter(ctx, resp, caller)
	if err != nil {
		return nil, err
	}

	mprovider := contentutil.NewMultiProvider(e.opt.ImageWriter.ContentStore())
	if src.Ref != nil {
		remotes, err := src.Ref.GetRemotes(ctx, false, e.refCfg(), false, session.NewGroup(sessionID))
		if err != nil {
			return nil, err
		}
		remote := remotes[0]
		// unlazy before tar export as the tar writer does not handle
		// layer blobs in parallel (whereas unlazy does)
		if unlazier, ok := remote.Provider.(cache.Unlazier); ok {
			if err := unlazier.Unlazy(ctx); err != nil {
				return nil, err
			}
		}
		for _, desc := range remote.Descriptors {
			mprovider.Add(desc.Digest, remote.Provider)
		}
	}
	if len(src.Refs) > 0 {
		for _, r := range src.Refs {
			remotes, err := r.GetRemotes(ctx, false, e.refCfg(), false, session.NewGroup(sessionID))
			if err != nil {
				return nil, err
			}
			remote := remotes[0]
			if unlazier, ok := remote.Provider.(cache.Unlazier); ok {
				if err := unlazier.Unlazy(ctx); err != nil {
					return nil, err
				}
			}
			for _, desc := range remote.Descriptors {
				mprovider.Add(desc.Digest, remote.Provider)
			}
		}
	}

	report := oneOffProgress(ctx, "sending tarball")
	if err := archiveexporter.Export(ctx, mprovider, w, expOpts...); err != nil {
		w.Close()
		if grpcerrors.Code(err) == codes.AlreadyExists {
			return resp, report(nil)
		}
		return nil, report(err)
	}
	err = w.Close()
	if grpcerrors.Code(err) == codes.AlreadyExists {
		return resp, report(nil)
	}
	return resp, report(err)
}

func oneOffProgress(ctx context.Context, id string) func(err error) error {
	pw, _, _ := progress.NewFromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
	}
	pw.Write(id, st)
	return func(err error) error {
		// TODO: set error on status
		now := time.Now()
		st.Completed = &now
		pw.Write(id, st)
		pw.Close()
		return err
	}
}

func normalizedNames(name string) ([]string, error) {
	if name == "" {
		return nil, nil
	}
	names := strings.Split(name, ",")
	var tagNames = make([]string, len(names))
	for i, name := range names {
		parsed, err := reference.ParseNormalizedNamed(name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", name)
		}
		tagNames[i] = reference.TagNameOnly(parsed).String()
	}
	return tagNames, nil
}
