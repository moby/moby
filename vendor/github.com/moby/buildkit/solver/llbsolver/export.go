package llbsolver

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/exporter/verifier"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	sessionexporter "github.com/moby/buildkit/session/exporter"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/result"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/tracing"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
)

func (s *Solver) getSessionExporters(ctx context.Context, sessionID string, id int, inp *exporter.Source) ([]exporter.ExporterInstance, error) {
	timeoutCtx, cancel := context.WithCancelCause(ctx)
	timeoutCtx, _ = context.WithTimeoutCause(timeoutCtx, 5*time.Second, errors.WithStack(context.DeadlineExceeded)) //nolint:govet
	defer func() { cancel(errors.WithStack(context.Canceled)) }()

	caller, err := s.sm.Get(timeoutCtx, sessionID, false)
	if err != nil {
		return nil, err
	}

	client := sessionexporter.NewExporterClient(caller.Conn())

	var ids []string
	if err := inp.EachRef(func(ref cache.ImmutableRef) error {
		ids = append(ids, ref.ID())
		return nil
	}); err != nil {
		return nil, err
	}

	res, err := client.FindExporters(ctx, &sessionexporter.FindExportersRequest{
		Metadata: inp.Metadata,
		Refs:     ids,
	})
	if err != nil {
		switch grpcerrors.Code(err) {
		case codes.Unavailable, codes.Unimplemented:
			return nil, nil
		default:
			return nil, err
		}
	}

	w, err := defaultResolver(s.workerController)()
	if err != nil {
		return nil, err
	}

	var out []exporter.ExporterInstance
	for i, req := range res.Exporters {
		exp, err := w.Exporter(req.Type, s.sm)
		if err != nil {
			return nil, err
		}
		expi, err := exp.Resolve(ctx, id+i, req.Attrs)
		if err != nil {
			return nil, err
		}
		out = append(out, expi)
	}
	return out, nil
}

func runCacheExporters(ctx context.Context, exporters []RemoteCacheExporter, j *solver.Job, cached *result.Result[solver.CachedResult], inp *result.Result[cache.ImmutableRef]) (map[string]string, error) {
	eg, ctx := errgroup.WithContext(ctx)
	g := session.NewGroup(j.SessionID)
	resps := make([]map[string]string, len(exporters))
	for i, exp := range exporters {
		id := fmt.Sprint(j.SessionID, "-cache-", i)
		eg.Go(func() (err error) {
			err = inBuilderContext(ctx, j, exp.Name(), id, func(ctx context.Context, _ solver.JobContext) error {
				prepareDone := progress.OneOff(ctx, "preparing build cache for export")
				if err := result.EachRef(cached, inp, func(res solver.CachedResult, ref cache.ImmutableRef) error {
					ctx := withDescHandlerCacheOpts(ctx, ref)

					// Configure compression
					compressionConfig := exp.Config().Compression

					// all keys have same export chain so exporting others is not needed
					_, err = res.CacheKeys()[0].Exporter.ExportTo(ctx, exp, solver.CacheExportOpt{
						ResolveRemotes: workerRefResolver(cacheconfig.RefConfig{Compression: compressionConfig}, false, g),
						Mode:           exp.CacheExportMode,
						Session:        g,
						CompressionOpt: &compressionConfig,
					})
					return err
				}); err != nil {
					return prepareDone(err)
				}
				prepareDone(nil)
				finalizeDone := progress.OneOff(ctx, "sending cache export")
				resps[i], err = exp.Finalize(ctx)
				return finalizeDone(err)
			})
			if exp.IgnoreError {
				err = nil
			}
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	var cacheExporterResponse map[string]string
	for _, resp := range resps {
		if cacheExporterResponse == nil {
			cacheExporterResponse = make(map[string]string)
		}
		maps.Copy(cacheExporterResponse, resp)
	}
	return cacheExporterResponse, nil
}

func runInlineCacheExporter(ctx context.Context, e exporter.ExporterInstance, inlineExporter inlineCacheExporter, j *solver.Job, cached *result.Result[solver.CachedResult]) (*result.Result[*exptypes.InlineCacheEntry], error) {
	if inlineExporter == nil {
		return nil, nil
	}

	done := progress.OneOff(ctx, "preparing layers for inline cache")
	res, err := result.ConvertResult(cached, func(res solver.CachedResult) (*exptypes.InlineCacheEntry, error) {
		dtic, err := inlineCache(ctx, inlineExporter, res, e.Config().Compression(), session.NewGroup(j.SessionID))
		if err != nil {
			return nil, err
		}
		if dtic == nil {
			return nil, nil
		}
		return &exptypes.InlineCacheEntry{Data: dtic}, nil
	})
	return res, done(err)
}

func exporterVertexID(sessionID string, exporterIndex int) string {
	return fmt.Sprint(sessionID, "-export-", exporterIndex)
}

func (s *Solver) runExporters(ctx context.Context, ref string, exporters []exporter.ExporterInstance, inlineCacheExporter inlineCacheExporter, job *solver.Job, cached *result.Result[solver.CachedResult], inp *exporter.Source) (exporterResponse map[string]string, finalizers []exporter.FinalizeFunc, descrefs []exporter.DescriptorReference, err error) {
	warnings, err := verifier.CheckInvalidPlatforms(ctx, inp)
	if err != nil {
		return nil, nil, nil, err
	}

	eg, ctx := errgroup.WithContext(ctx)
	resps := make([]map[string]string, len(exporters))
	finalizeFuncs := make([]exporter.FinalizeFunc, len(exporters))
	descs := make([]exporter.DescriptorReference, len(exporters))
	var inlineCacheMu sync.Mutex
	for i, exp := range exporters {
		id := exporterVertexID(job.SessionID, i)
		eg.Go(func() error {
			return inBuilderContext(ctx, job, exp.Name(), id, func(ctx context.Context, _ solver.JobContext) error {
				span, ctx := tracing.StartSpan(ctx, exp.Name())
				defer span.End()

				if i == 0 && len(warnings) > 0 {
					pw, _, _ := progress.NewFromContext(ctx)
					for _, w := range warnings {
						pw.Write(identity.NewID(), w)
					}
					if err := pw.Close(); err != nil {
						return err
					}
				}
				inlineCache := exptypes.InlineCache(func(ctx context.Context) (*result.Result[*exptypes.InlineCacheEntry], error) {
					inlineCacheMu.Lock() // ensure only one inline cache exporter runs at a time
					defer inlineCacheMu.Unlock()
					return runInlineCacheExporter(ctx, exp, inlineCacheExporter, job, cached)
				})

				resp, finalize, desc, expErr := exp.Export(ctx, inp, exporter.ExportBuildInfo{
					Ref:         ref,
					SessionID:   job.SessionID,
					InlineCache: inlineCache,
				})
				resps[i], finalizeFuncs[i], descs[i] = resp, finalize, desc
				if expErr != nil {
					return expErr
				}
				return nil
			})
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, nil, nil, err
	}

	if len(exporters) == 0 && len(warnings) > 0 {
		err := inBuilderContext(ctx, job, "Verifying build result", identity.NewID(), func(ctx context.Context, _ solver.JobContext) error {
			pw, _, _ := progress.NewFromContext(ctx)
			for _, w := range warnings {
				pw.Write(identity.NewID(), w)
			}
			return pw.Close()
		})
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// TODO: separate these out, and return multiple exporter responses to the
	// client
	for _, resp := range resps {
		for k, v := range resp {
			if exporterResponse == nil {
				exporterResponse = make(map[string]string)
			}
			exporterResponse[k] = v
		}
	}

	return exporterResponse, finalizeFuncs, descs, nil
}

func splitCacheExporters(exporters []RemoteCacheExporter) (rest []RemoteCacheExporter, inline inlineCacheExporter) {
	rest = make([]RemoteCacheExporter, 0, len(exporters))
	for _, exp := range exporters {
		if ic, ok := asInlineCache(exp.Exporter); ok {
			inline = ic
			continue
		}
		rest = append(rest, exp)
	}
	return rest, inline
}

type inlineCacheExporter interface {
	solver.CacheExporterTarget
	ExportForLayers(context.Context, []digest.Digest) ([]byte, error)
}

func asInlineCache(e remotecache.Exporter) (inlineCacheExporter, bool) {
	ie, ok := e.(inlineCacheExporter)
	return ie, ok
}

func inlineCache(ctx context.Context, ie inlineCacheExporter, res solver.CachedResult, compressionopt compression.Config, g session.Group) ([]byte, error) {
	workerRef, ok := res.Sys().(*worker.WorkerRef)
	if !ok {
		return nil, errors.Errorf("invalid reference: %T", res.Sys())
	}

	remotes, err := workerRef.GetRemotes(ctx, true, cacheconfig.RefConfig{Compression: compressionopt}, false, g)
	if err != nil || len(remotes) == 0 {
		return nil, nil
	}
	remote := remotes[0]

	digests := make([]digest.Digest, 0, len(remote.Descriptors))
	for _, desc := range remote.Descriptors {
		digests = append(digests, desc.Digest)
	}

	ctx = withDescHandlerCacheOpts(ctx, workerRef.ImmutableRef)
	refCfg := cacheconfig.RefConfig{Compression: compressionopt}
	if _, err := res.CacheKeys()[0].Exporter.ExportTo(ctx, ie, solver.CacheExportOpt{
		ResolveRemotes: workerRefResolver(refCfg, true, g), // load as many compression blobs as possible
		Mode:           solver.CacheExportModeMin,
		Session:        g,
		CompressionOpt: &compressionopt, // cache possible compression variants
	}); err != nil {
		return nil, err
	}
	return ie.ExportForLayers(ctx, digests)
}

func withDescHandlerCacheOpts(ctx context.Context, ref cache.ImmutableRef) context.Context {
	return solver.WithCacheOptGetter(ctx, func(includeAncestors bool, keys ...any) map[any]any {
		vals := make(map[any]any)
		for _, k := range keys {
			if key, ok := k.(cache.DescHandlerKey); ok {
				if handler := ref.DescHandler(digest.Digest(key)); handler != nil {
					vals[k] = handler
				}
			}
		}
		return vals
	})
}
