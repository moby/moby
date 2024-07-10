package oci

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	archiveexporter "github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/leases"
	"github.com/distribution/reference"
	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/session"
	sessioncontent "github.com/moby/buildkit/session/content"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
)

type ExporterVariant string

const (
	VariantOCI    = client.ExporterOCI
	VariantDocker = client.ExporterDocker
)

const (
	keyTar = "tar"
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

func (e *imageExporter) Resolve(ctx context.Context, id int, opt map[string]string) (exporter.ExporterInstance, error) {
	i := &imageExporterInstance{
		imageExporter: e,
		id:            id,
		attrs:         opt,
		tar:           true,
		opts: containerimage.ImageCommitOpts{
			RefCfg: cacheconfig.RefConfig{
				Compression: compression.New(compression.Default),
			},
			OCITypes: e.opt.Variant == VariantOCI,
		},
	}

	opt, err := i.opts.Load(ctx, opt)
	if err != nil {
		return nil, err
	}

	for k, v := range opt {
		switch k {
		case keyTar:
			if v == "" {
				i.tar = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.tar = b
		default:
			if i.meta == nil {
				i.meta = make(map[string][]byte)
			}
			i.meta[k] = []byte(v)
		}
	}
	return i, nil
}

type imageExporterInstance struct {
	*imageExporter
	id    int
	attrs map[string]string

	opts containerimage.ImageCommitOpts
	tar  bool
	meta map[string][]byte
}

func (e *imageExporterInstance) ID() int {
	return e.id
}

func (e *imageExporterInstance) Name() string {
	return fmt.Sprintf("exporting to %s image format", e.opt.Variant)
}

func (e *imageExporterInstance) Type() string {
	return string(e.opt.Variant)
}

func (e *imageExporterInstance) Attrs() map[string]string {
	return e.attrs
}

func (e *imageExporterInstance) Config() *exporter.Config {
	return exporter.NewConfigWithCompression(e.opts.RefCfg.Compression)
}

func (e *imageExporterInstance) Export(ctx context.Context, src *exporter.Source, inlineCache exptypes.InlineCache, sessionID string) (_ map[string]string, descref exporter.DescriptorReference, err error) {
	if e.opt.Variant == VariantDocker && len(src.Refs) > 0 {
		return nil, nil, errors.Errorf("docker exporter does not currently support exporting manifest lists")
	}

	src = src.Clone()
	if src.Metadata == nil {
		src.Metadata = make(map[string][]byte)
	}
	maps.Copy(src.Metadata, e.meta)

	opts := e.opts
	as, _, err := containerimage.ParseAnnotations(src.Metadata)
	if err != nil {
		return nil, nil, err
	}
	opts.Annotations = opts.Annotations.Merge(as)

	ctx, done, err := leaseutil.WithLease(ctx, e.opt.LeaseManager, leaseutil.MakeTemporary)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if descref == nil {
			done(context.WithoutCancel(ctx))
		}
	}()

	desc, err := e.opt.ImageWriter.Commit(ctx, src, sessionID, inlineCache, &opts)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err == nil {
			descref = containerimage.NewDescriptorReference(*desc, done)
		}
	}()

	if desc.Annotations == nil {
		desc.Annotations = map[string]string{}
	}
	if _, ok := desc.Annotations[ocispecs.AnnotationCreated]; !ok {
		tm := time.Now()
		if opts.Epoch != nil {
			tm = *opts.Epoch
		}
		desc.Annotations[ocispecs.AnnotationCreated] = tm.UTC().Format(time.RFC3339)
	}

	resp := make(map[string]string)

	resp[exptypes.ExporterImageDigestKey] = desc.Digest.String()
	if v, ok := desc.Annotations[exptypes.ExporterConfigDigestKey]; ok {
		resp[exptypes.ExporterImageConfigDigestKey] = v
		delete(desc.Annotations, exptypes.ExporterConfigDigestKey)
	}

	dtdesc, err := json.Marshal(desc)
	if err != nil {
		return nil, nil, err
	}
	resp[exptypes.ExporterImageDescriptorKey] = base64.StdEncoding.EncodeToString(dtdesc)

	if n, ok := src.Metadata["image.name"]; e.opts.ImageName == "*" && ok {
		e.opts.ImageName = string(n)
	}

	names, err := normalizedNames(e.opts.ImageName)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, errors.Errorf("invalid variant %q", e.opt.Variant)
	}

	timeoutCtx, cancel := context.WithCancelCause(ctx)
	timeoutCtx, _ = context.WithTimeoutCause(timeoutCtx, 5*time.Second, errors.WithStack(context.DeadlineExceeded))
	defer cancel(errors.WithStack(context.Canceled))

	caller, err := e.opt.SessionManager.Get(timeoutCtx, sessionID, false)
	if err != nil {
		return nil, nil, err
	}

	var refs []cache.ImmutableRef
	if src.Ref != nil {
		refs = append(refs, src.Ref)
	}
	for _, ref := range src.Refs {
		refs = append(refs, ref)
	}
	eg, egCtx := errgroup.WithContext(ctx)
	mprovider := contentutil.NewMultiProvider(e.opt.ImageWriter.ContentStore())
	for _, ref := range refs {
		ref := ref
		eg.Go(func() error {
			remotes, err := ref.GetRemotes(egCtx, false, e.opts.RefCfg, false, session.NewGroup(sessionID))
			if err != nil {
				return err
			}
			remote := remotes[0]
			if unlazier, ok := remote.Provider.(cache.Unlazier); ok {
				if err := unlazier.Unlazy(egCtx); err != nil {
					return err
				}
			}
			for _, desc := range remote.Descriptors {
				mprovider.Add(desc.Digest, remote.Provider)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}

	if e.tar {
		w, err := filesync.CopyFileWriter(ctx, resp, e.id, caller)
		if err != nil {
			return nil, nil, err
		}

		report := progress.OneOff(ctx, "sending tarball")
		if err := archiveexporter.Export(ctx, mprovider, w, expOpts...); err != nil {
			w.Close()
			if grpcerrors.Code(err) == codes.AlreadyExists {
				return resp, nil, report(nil)
			}
			return nil, nil, report(err)
		}
		err = w.Close()
		if grpcerrors.Code(err) == codes.AlreadyExists {
			return resp, nil, report(nil)
		}
		if err != nil {
			return nil, nil, report(err)
		}
		report(nil)
	} else {
		store := sessioncontent.NewCallerStore(caller, "export")
		if err != nil {
			return nil, nil, err
		}
		err := contentutil.CopyChain(ctx, store, mprovider, *desc)
		if err != nil {
			return nil, nil, err
		}
	}

	return resp, nil, nil
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
