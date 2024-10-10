package llbsolver

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client"
	controlgateway "github.com/moby/buildkit/control/gateway"
	"github.com/moby/buildkit/errdefs"
	"github.com/moby/buildkit/executor/resources"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/exporter/verifier"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/attestations"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/provenance"
	"github.com/moby/buildkit/solver/result"
	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/tracing"
	"github.com/moby/buildkit/util/tracing/detect"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	keyEntitlements = "llb.entitlements"
	keySourcePolicy = "llb.sourcepolicy"
)

type ExporterRequest struct {
	Exporters      []exporter.ExporterInstance
	CacheExporters []RemoteCacheExporter
}

type RemoteCacheExporter struct {
	remotecache.Exporter
	solver.CacheExportMode
	IgnoreError bool
}

// ResolveWorkerFunc returns default worker for the temporary default non-distributed use cases
type ResolveWorkerFunc func() (worker.Worker, error)

// Opt defines options for new Solver.
type Opt struct {
	CacheManager     solver.CacheManager
	CacheResolvers   map[string]remotecache.ResolveCacheImporterFunc
	Entitlements     []string
	Frontends        map[string]frontend.Frontend
	GatewayForwarder *controlgateway.GatewayForwarder
	SessionManager   *session.Manager
	WorkerController *worker.Controller
	HistoryQueue     *HistoryQueue
	ResourceMonitor  *resources.Monitor
}

type Solver struct {
	workerController          *worker.Controller
	solver                    *solver.Solver
	resolveWorker             ResolveWorkerFunc
	eachWorker                func(func(worker.Worker) error) error
	frontends                 map[string]frontend.Frontend
	resolveCacheImporterFuncs map[string]remotecache.ResolveCacheImporterFunc
	gatewayForwarder          *controlgateway.GatewayForwarder
	sm                        *session.Manager
	entitlements              []string
	history                   *HistoryQueue
	sysSampler                *resources.Sampler[*resourcestypes.SysSample]
}

// Processor defines a processing function to be applied after solving, but
// before exporting
type Processor func(ctx context.Context, result *Result, s *Solver, j *solver.Job, usage *resources.SysSampler) (*Result, error)

func New(opt Opt) (*Solver, error) {
	s := &Solver{
		workerController:          opt.WorkerController,
		resolveWorker:             defaultResolver(opt.WorkerController),
		eachWorker:                allWorkers(opt.WorkerController),
		frontends:                 opt.Frontends,
		resolveCacheImporterFuncs: opt.CacheResolvers,
		gatewayForwarder:          opt.GatewayForwarder,
		sm:                        opt.SessionManager,
		entitlements:              opt.Entitlements,
		history:                   opt.HistoryQueue,
	}

	sampler, err := resources.NewSysSampler()
	if err != nil {
		return nil, err
	}
	s.sysSampler = sampler

	s.solver = solver.NewSolver(solver.SolverOpt{
		ResolveOpFunc: s.resolver(),
		DefaultCache:  opt.CacheManager,
	})
	return s, nil
}

func (s *Solver) Close() error {
	s.solver.Close()
	if s.sysSampler != nil {
		return s.sysSampler.Close()
	}
	return nil
}

func (s *Solver) resolver() solver.ResolveOpFunc {
	return func(v solver.Vertex, b solver.Builder) (solver.Op, error) {
		w, err := s.resolveWorker()
		if err != nil {
			return nil, err
		}
		return w.ResolveOp(v, s.Bridge(b), s.sm)
	}
}

func (s *Solver) bridge(b solver.Builder) *provenanceBridge {
	return &provenanceBridge{llbBridge: &llbBridge{
		builder:                   b,
		frontends:                 s.frontends,
		resolveWorker:             s.resolveWorker,
		eachWorker:                s.eachWorker,
		resolveCacheImporterFuncs: s.resolveCacheImporterFuncs,
		cms:                       map[string]solver.CacheManager{},
		sm:                        s.sm,
	}}
}

func (s *Solver) Bridge(b solver.Builder) frontend.FrontendLLBBridge {
	return s.bridge(b)
}

func (s *Solver) recordBuildHistory(ctx context.Context, id string, req frontend.SolveRequest, exp ExporterRequest, j *solver.Job, usage *resources.SysSampler) (func(context.Context, *Result, []exporter.DescriptorReference, error) error, error) {
	stopTrace, err := detect.Recorder.Record(ctx)
	if err != nil {
		return nil, errdefs.Internal(err)
	}

	rec := &controlapi.BuildHistoryRecord{
		Ref:           id,
		Frontend:      req.Frontend,
		FrontendAttrs: req.FrontendOpt,
		CreatedAt:     timestamppb.Now(),
	}

	for _, e := range exp.Exporters {
		rec.Exporters = append(rec.Exporters, &controlapi.Exporter{
			Type:  e.Type(),
			Attrs: e.Attrs(),
		})
	}

	if err := s.history.Update(ctx, &controlapi.BuildHistoryEvent{
		Type:   controlapi.BuildHistoryEventType_STARTED,
		Record: rec,
	}); err != nil {
		if stopTrace != nil {
			stopTrace()
		}
		return nil, errdefs.Internal(err)
	}

	return func(ctx context.Context, res *Result, descrefs []exporter.DescriptorReference, err error) error {
		rec.CompletedAt = timestamppb.Now()

		span, ctx := tracing.StartSpan(ctx, "create history record")
		defer span.End()

		j.CloseProgress()

		if res != nil && len(res.Metadata) > 0 {
			rec.ExporterResponse = map[string]string{}
			for k, v := range res.Metadata {
				rec.ExporterResponse[k] = string(v)
			}
		}

		ctx, cancel := context.WithCancelCause(ctx)
		ctx, _ = context.WithTimeoutCause(ctx, 300*time.Second, errors.WithStack(context.DeadlineExceeded))
		defer cancel(errors.WithStack(context.Canceled))

		var mu sync.Mutex
		ch := make(chan *client.SolveStatus)
		eg, ctx2 := errgroup.WithContext(ctx)
		var releasers []func()

		attrs := map[string]string{
			"mode":          "max",
			"capture-usage": "true",
		}

		// infer builder-id from user input if available
		if attests, err := attestations.Parse(rec.FrontendAttrs); err == nil {
			if prvAttrs, ok := attests["provenance"]; ok {
				if builderID, ok := prvAttrs["builder-id"]; ok {
					attrs["builder-id"] = builderID
				}
			}
		}

		makeProvenance := func(name string, res solver.ResultProxy, cap *provenance.Capture) (*controlapi.Descriptor, func(), error) {
			span, ctx := tracing.StartSpan(ctx, fmt.Sprintf("create %s history provenance", name))
			defer span.End()

			prc, err := NewProvenanceCreator(ctx2, cap, res, attrs, j, usage)
			if err != nil {
				return nil, nil, err
			}
			pr, err := prc.Predicate()
			if err != nil {
				return nil, nil, err
			}
			dt, err := json.MarshalIndent(pr, "", "  ")
			if err != nil {
				return nil, nil, err
			}
			w, err := s.history.OpenBlobWriter(ctx, intoto.PayloadType)
			if err != nil {
				return nil, nil, err
			}
			defer func() {
				if w != nil {
					w.Discard()
				}
			}()
			if _, err := w.Write(dt); err != nil {
				return nil, nil, err
			}
			desc, release, err := w.Commit(ctx2)
			if err != nil {
				return nil, nil, err
			}
			w = nil
			return &controlapi.Descriptor{
				Digest:    string(desc.Digest),
				Size:      desc.Size,
				MediaType: desc.MediaType,
				Annotations: map[string]string{
					"in-toto.io/predicate-type": slsa02.PredicateSLSAProvenance,
				},
			}, release, nil
		}

		if res != nil {
			if res.Ref != nil {
				eg.Go(func() error {
					desc, release, err := makeProvenance("default", res.Ref, res.Provenance.Ref)
					if err != nil {
						return err
					}

					mu.Lock()
					releasers = append(releasers, release)
					if rec.Result == nil {
						rec.Result = &controlapi.BuildResultInfo{}
					}
					rec.Result.Attestations = append(rec.Result.Attestations, desc)
					mu.Unlock()
					return nil
				})
			}

			for k, r := range res.Refs {
				if r == nil {
					continue
				}
				k, r := k, r
				cp := res.Provenance.Refs[k]
				eg.Go(func() error {
					desc, release, err := makeProvenance(k, r, cp)
					if err != nil {
						return err
					}

					mu.Lock()
					releasers = append(releasers, release)
					if rec.Results == nil {
						rec.Results = make(map[string]*controlapi.BuildResultInfo)
					}
					if rec.Results[k] == nil {
						rec.Results[k] = &controlapi.BuildResultInfo{}
					}
					rec.Results[k].Attestations = append(rec.Results[k].Attestations, desc)
					mu.Unlock()
					return nil
				})
			}
		}

		eg.Go(func() error {
			st, releaseStatus, err := s.history.ImportStatus(ctx2, ch)
			if err != nil {
				return err
			}
			mu.Lock()
			releasers = append(releasers, releaseStatus)
			rec.Logs = &controlapi.Descriptor{
				Digest:    string(st.Descriptor.Digest),
				Size:      st.Descriptor.Size,
				MediaType: st.Descriptor.MediaType,
			}
			rec.NumCachedSteps = int32(st.NumCachedSteps)
			rec.NumCompletedSteps = int32(st.NumCompletedSteps)
			rec.NumTotalSteps = int32(st.NumTotalSteps)
			rec.NumWarnings = int32(st.NumWarnings)
			mu.Unlock()
			return nil
		})
		eg.Go(func() error {
			return j.Status(ctx2, ch)
		})

		setDeprecated := true
		for i, descref := range descrefs {
			i, descref := i, descref
			if descref == nil {
				continue
			}
			deprecate := setDeprecated
			setDeprecated = false
			eg.Go(func() error {
				mu.Lock()
				desc := descref.Descriptor()
				controlDesc := &controlapi.Descriptor{
					Digest:      string(desc.Digest),
					Size:        desc.Size,
					MediaType:   desc.MediaType,
					Annotations: desc.Annotations,
				}
				if rec.Result == nil {
					rec.Result = &controlapi.BuildResultInfo{}
				}
				if rec.Result.Results == nil {
					rec.Result.Results = make(map[int64]*controlapi.Descriptor)
				}
				if deprecate {
					// write the first available descriptor to the deprecated
					// field for legacy clients
					rec.Result.ResultDeprecated = controlDesc
				}
				rec.Result.Results[int64(i)] = controlDesc
				mu.Unlock()
				return nil
			})
		}
		if err1 := eg.Wait(); err == nil {
			// any error from exporting history record is internal
			err = errdefs.Internal(err1)
		}

		defer func() {
			for _, f := range releasers {
				f()
			}
		}()

		if err != nil {
			status, desc, release, err1 := s.history.ImportError(ctx, err)
			if err1 != nil {
				// don't replace the build error with this import error
				bklog.G(ctx).Errorf("failed to import error to build record: %+v", err1)
			} else {
				releasers = append(releasers, release)
			}
			rec.ExternalError = desc
			rec.Error = status
		}

		ready, done := s.history.AcquireFinalizer(rec.Ref)

		if err1 := s.history.Update(ctx, &controlapi.BuildHistoryEvent{
			Type:   controlapi.BuildHistoryEventType_COMPLETE,
			Record: rec,
		}); err1 != nil {
			if err == nil {
				err = errdefs.Internal(err1)
			}
		}

		if stopTrace == nil {
			bklog.G(ctx).Warn("no trace recorder found, skipping")
			done()
			return err
		}
		go func() {
			defer done()

			// if there is no finalizer request then stop tracing after 3 seconds
			select {
			case <-time.After(3 * time.Second):
			case <-ready:
			}
			spans := stopTrace()

			if len(spans) == 0 {
				return
			}

			if err := func() error {
				w, err := s.history.OpenBlobWriter(context.TODO(), "application/vnd.buildkit.otlp.json.v0")
				if err != nil {
					return err
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				for _, sp := range spans {
					if err := enc.Encode(sp); err != nil {
						return err
					}
				}

				desc, release, err := w.Commit(context.TODO())
				if err != nil {
					return err
				}
				defer release()

				if err := s.history.UpdateRef(context.TODO(), id, func(rec *controlapi.BuildHistoryRecord) error {
					rec.Trace = &controlapi.Descriptor{
						Digest:    string(desc.Digest),
						MediaType: desc.MediaType,
						Size:      desc.Size,
					}
					return nil
				}); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				bklog.G(ctx).Errorf("failed to save trace for %s: %+v", id, err)
			}
		}()

		return err
	}, nil
}

func (s *Solver) Solve(ctx context.Context, id string, sessionID string, req frontend.SolveRequest, exp ExporterRequest, ent []entitlements.Entitlement, post []Processor, internal bool, srcPol *spb.Policy) (_ *client.SolveResponse, err error) {
	j, err := s.solver.NewJob(id)
	if err != nil {
		return nil, err
	}

	defer j.Discard()

	var usage *resources.Sub[*resourcestypes.SysSample]
	if s.sysSampler != nil {
		usage = s.sysSampler.Record()
		defer usage.Close(false)
	}

	var res *frontend.Result
	var resProv *Result
	var descrefs []exporter.DescriptorReference

	var releasers []func()
	defer func() {
		for _, f := range releasers {
			f()
		}
		for _, descref := range descrefs {
			if descref != nil {
				descref.Release()
			}
		}
	}()

	if internal {
		defer j.CloseProgress()
	}

	set, err := entitlements.WhiteList(ent, supportedEntitlements(s.entitlements))
	if err != nil {
		return nil, err
	}
	j.SetValue(keyEntitlements, set)

	if srcPol != nil {
		if err := validateSourcePolicy(srcPol); err != nil {
			return nil, err
		}
		j.SetValue(keySourcePolicy, srcPol)
	}

	j.SessionID = sessionID

	br := s.bridge(j)
	var fwd gateway.LLBBridgeForwarder
	if s.gatewayForwarder != nil && req.Definition == nil && req.Frontend == "" {
		fwd = gateway.NewBridgeForwarder(ctx, br, br, s.workerController.Infos(), req.FrontendInputs, sessionID, s.sm)
		defer fwd.Discard()
		// Register build before calling s.recordBuildHistory, because
		// s.recordBuildHistory can block for several seconds on
		// LeaseManager calls, and there is a fixed 3s timeout in
		// GatewayForwarder on build registration.
		if err := s.gatewayForwarder.RegisterBuild(ctx, id, fwd); err != nil {
			return nil, err
		}
		defer s.gatewayForwarder.UnregisterBuild(context.WithoutCancel(ctx), id)
	}

	if !internal {
		rec, err1 := s.recordBuildHistory(ctx, id, req, exp, j, usage)
		if err1 != nil {
			defer j.CloseProgress()
			return nil, err1
		}
		defer func() {
			err = rec(context.WithoutCancel(ctx), resProv, descrefs, err)
		}()
	}

	if fwd != nil {
		var err error
		select {
		case <-fwd.Done():
			res, err = fwd.Result()
		case <-ctx.Done():
			err = context.Cause(ctx)
		}
		if err != nil {
			return nil, err
		}
	} else {
		res, err = br.Solve(ctx, req, sessionID)
		if err != nil {
			return nil, err
		}
	}

	if res == nil {
		res = &frontend.Result{}
	}

	if err := verifier.CaptureFrontendOpts(req.FrontendOpt, res); err != nil {
		return nil, err
	}

	releasers = append(releasers, func() {
		res.EachRef(func(ref solver.ResultProxy) error {
			go ref.Release(context.TODO())
			return nil
		})
	})

	eg, ctx2 := errgroup.WithContext(ctx)
	res.EachRef(func(ref solver.ResultProxy) error {
		eg.Go(func() error {
			_, err := ref.Result(ctx2)
			return err
		})
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	resProv, err = addProvenanceToResult(res, br)
	if err != nil {
		return nil, err
	}

	for _, post := range post {
		res2, err := post(ctx, resProv, s, j, usage)
		if err != nil {
			return nil, err
		}
		resProv = res2
	}
	res = resProv.Result

	cached, err := result.ConvertResult(res, func(res solver.ResultProxy) (solver.CachedResult, error) {
		return res.Result(ctx)
	})
	if err != nil {
		return nil, err
	}
	inp, err := result.ConvertResult(cached, func(res solver.CachedResult) (cache.ImmutableRef, error) {
		workerRef, ok := res.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, errors.Errorf("invalid reference: %T", res.Sys())
		}
		return workerRef.ImmutableRef, nil
	})
	if err != nil {
		return nil, err
	}

	// Functions that create new objects in containerd (eg. content blobs) need to have a lease to ensure
	// that the object is not garbage collected immediately. This is protected by the indivual components,
	// but because creating a lease is not cheap and requires a disk write, we create a single lease here
	// early and let all the exporters, cache export and provenance creation use the same one.
	lm, err := s.leaseManager()
	if err != nil {
		return nil, err
	}
	ctx, done, err := leaseutil.WithLease(ctx, lm, leaseutil.MakeTemporary)
	if err != nil {
		return nil, err
	}
	releasers = append(releasers, func() {
		done(context.WithoutCancel(ctx))
	})

	cacheExporters, inlineCacheExporter := splitCacheExporters(exp.CacheExporters)

	var exporterResponse map[string]string
	exporterResponse, descrefs, err = s.runExporters(ctx, exp.Exporters, inlineCacheExporter, j, cached, inp)
	if err != nil {
		return nil, err
	}

	cacheExporterResponse, err := runCacheExporters(ctx, cacheExporters, j, cached, inp)
	if err != nil {
		return nil, err
	}

	if exporterResponse == nil {
		exporterResponse = make(map[string]string)
	}

	for k, v := range res.Metadata {
		if strings.HasPrefix(k, "frontend.") {
			exporterResponse[k] = string(v)
		}
	}
	for k, v := range cacheExporterResponse {
		if strings.HasPrefix(k, "cache.") {
			exporterResponse[k] = v
		}
	}

	return &client.SolveResponse{
		ExporterResponse: exporterResponse,
	}, nil
}

func validateSourcePolicy(pol *spb.Policy) error {
	for _, r := range pol.Rules {
		if r == nil {
			return errors.New("invalid nil rule in policy")
		}
		if r.Selector == nil {
			return errors.New("invalid nil selector in policy")
		}
		for _, c := range r.Selector.Constraints {
			if c == nil {
				return errors.New("invalid nil constraint in policy")
			}
		}
	}
	return nil
}

func runCacheExporters(ctx context.Context, exporters []RemoteCacheExporter, j *solver.Job, cached *result.Result[solver.CachedResult], inp *result.Result[cache.ImmutableRef]) (map[string]string, error) {
	eg, ctx := errgroup.WithContext(ctx)
	g := session.NewGroup(j.SessionID)
	var cacheExporterResponse map[string]string
	resps := make([]map[string]string, len(exporters))
	for i, exp := range exporters {
		i, exp := i, exp
		eg.Go(func() (err error) {
			id := fmt.Sprint(j.SessionID, "-cache-", i)
			err = inBuilderContext(ctx, j, exp.Exporter.Name(), id, func(ctx context.Context, _ session.Group) error {
				prepareDone := progress.OneOff(ctx, "preparing build cache for export")
				if err := result.EachRef(cached, inp, func(res solver.CachedResult, ref cache.ImmutableRef) error {
					ctx = withDescHandlerCacheOpts(ctx, ref)

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
				resps[i], err = exp.Finalize(ctx)
				return prepareDone(err)
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

	// TODO: separate these out, and return multiple cache exporter responses
	// to the client
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

func (s *Solver) runExporters(ctx context.Context, exporters []exporter.ExporterInstance, inlineCacheExporter inlineCacheExporter, job *solver.Job, cached *result.Result[solver.CachedResult], inp *result.Result[cache.ImmutableRef]) (exporterResponse map[string]string, descrefs []exporter.DescriptorReference, err error) {
	warnings, err := verifier.CheckInvalidPlatforms(ctx, inp)
	if err != nil {
		return nil, nil, err
	}

	eg, ctx := errgroup.WithContext(ctx)
	resps := make([]map[string]string, len(exporters))
	descs := make([]exporter.DescriptorReference, len(exporters))
	for i, exp := range exporters {
		i, exp := i, exp
		eg.Go(func() error {
			id := fmt.Sprint(job.SessionID, "-export-", i)
			return inBuilderContext(ctx, job, exp.Name(), id, func(ctx context.Context, _ session.Group) error {
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
					return runInlineCacheExporter(ctx, exp, inlineCacheExporter, job, cached)
				})

				resps[i], descs[i], err = exp.Export(ctx, inp, inlineCache, job.SessionID)
				if err != nil {
					return err
				}
				return nil
			})
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}

	if len(exporters) == 0 && len(warnings) > 0 {
		err := inBuilderContext(ctx, job, "Verifying build result", identity.NewID(), func(ctx context.Context, _ session.Group) error {
			pw, _, _ := progress.NewFromContext(ctx)
			for _, w := range warnings {
				pw.Write(identity.NewID(), w)
			}
			return pw.Close()
		})
		if err != nil {
			return nil, nil, err
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

	return exporterResponse, descs, nil
}

func (s *Solver) leaseManager() (*leaseutil.Manager, error) {
	w, err := defaultResolver(s.workerController)()
	if err != nil {
		return nil, err
	}
	return w.LeaseManager(), nil
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

func addProvenanceToResult(res *frontend.Result, br *provenanceBridge) (*Result, error) {
	if res == nil {
		return nil, nil
	}
	reqs, err := br.requests(res)
	if err != nil {
		return nil, err
	}
	out := &Result{
		Result:     res,
		Provenance: &provenance.Result{},
	}

	if res.Ref != nil {
		cp, err := getProvenance(res.Ref, reqs.ref.bridge, "", reqs)
		if err != nil {
			return nil, err
		}
		out.Provenance.Ref = cp
		if res.Metadata == nil {
			res.Metadata = map[string][]byte{}
		}
	}

	if len(res.Refs) != 0 {
		out.Provenance.Refs = make(map[string]*provenance.Capture, len(res.Refs))
	}
	for k, ref := range res.Refs {
		if ref == nil {
			continue
		}
		cp, err := getProvenance(ref, reqs.refs[k].bridge, k, reqs)
		if err != nil {
			return nil, err
		}
		out.Provenance.Refs[k] = cp
		if res.Metadata == nil {
			res.Metadata = map[string][]byte{}
		}
	}

	if len(res.Attestations) != 0 {
		out.Provenance.Attestations = make(map[string][]result.Attestation[*provenance.Capture], len(res.Attestations))
	}
	for k, as := range res.Attestations {
		for i, a := range as {
			a2, err := result.ConvertAttestation(&a, func(r solver.ResultProxy) (*provenance.Capture, error) {
				return getProvenance(r, reqs.atts[k][i].bridge, k, reqs)
			})
			if err != nil {
				return nil, err
			}
			out.Provenance.Attestations[k] = append(out.Provenance.Attestations[k], *a2)
		}
	}

	return out, nil
}

func getRefProvenance(ref solver.ResultProxy, br *provenanceBridge) (*provenance.Capture, error) {
	if ref == nil {
		return nil, nil
	}
	p := ref.Provenance()
	if p == nil {
		return nil, nil
	}

	pr, ok := p.(*provenance.Capture)
	if !ok {
		return nil, errors.Errorf("invalid provenance type %T", p)
	}

	if br.req != nil {
		if pr == nil {
			return nil, errors.Errorf("missing provenance for %s", ref.ID())
		}

		pr.Frontend = br.req.Frontend
		pr.Args = provenance.FilterArgs(br.req.FrontendOpt)
		// TODO: should also save some output options like compression

		if len(br.req.FrontendInputs) > 0 {
			pr.IncompleteMaterials = true // not implemented
		}
	}

	return pr, nil
}

func getProvenance(ref solver.ResultProxy, br *provenanceBridge, id string, reqs *resultRequests) (*provenance.Capture, error) {
	pr, err := getRefProvenance(ref, br)
	if err != nil {
		return nil, err
	}
	if pr == nil {
		return nil, nil
	}

	visited := reqs.allRes()
	visited[ref.ID()] = struct{}{}
	// provenance for all the refs not directly in the result needs to be captured as well
	if err := br.eachRef(func(r solver.ResultProxy) error {
		if _, ok := visited[r.ID()]; ok {
			return nil
		}
		visited[r.ID()] = struct{}{}
		pr2, err := getRefProvenance(r, br)
		if err != nil {
			return err
		}
		return pr.Merge(pr2)
	}); err != nil {
		return nil, err
	}

	imgs := br.allImages()
	if id != "" {
		imgs = reqs.filterImagePlatforms(id, imgs)
	}
	for _, img := range imgs {
		pr.AddImage(img)
	}

	if err := pr.OptimizeImageSources(); err != nil {
		return nil, err
	}
	pr.Sort()

	return pr, nil
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
	return solver.WithCacheOptGetter(ctx, func(includeAncestors bool, keys ...interface{}) map[interface{}]interface{} {
		vals := make(map[interface{}]interface{})
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

func (s *Solver) Status(ctx context.Context, id string, statusChan chan *client.SolveStatus) error {
	if err := s.history.Status(ctx, id, statusChan); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			close(statusChan)
			return err
		}
	} else {
		close(statusChan)
		return nil
	}
	j, err := s.solver.Get(id)
	if err != nil {
		close(statusChan)
		return err
	}
	return j.Status(ctx, statusChan)
}

func defaultResolver(wc *worker.Controller) ResolveWorkerFunc {
	return func() (worker.Worker, error) {
		return wc.GetDefault()
	}
}

func allWorkers(wc *worker.Controller) func(func(w worker.Worker) error) error {
	return func(f func(worker.Worker) error) error {
		all, err := wc.List()
		if err != nil {
			return err
		}
		for _, w := range all {
			if err := f(w); err != nil {
				return err
			}
		}
		return nil
	}
}

func inBuilderContext(ctx context.Context, b solver.Builder, name, id string, f func(ctx context.Context, g session.Group) error) error {
	if id == "" {
		id = name
	}
	v := client.Vertex{
		Digest: digest.FromBytes([]byte(id)),
		Name:   name,
	}
	return b.InContext(ctx, func(ctx context.Context, g session.Group) error {
		pw, _, ctx := progress.NewFromContext(ctx, progress.WithMetadata("vertex", v.Digest))
		notifyCompleted := notifyStarted(ctx, &v)
		defer pw.Close()
		err := f(ctx, g)
		notifyCompleted(err)
		return err
	})
}

func notifyStarted(ctx context.Context, v *client.Vertex) func(err error) {
	pw, _, _ := progress.NewFromContext(ctx)
	start := time.Now()
	v.Started = &start
	v.Completed = nil
	id := identity.NewID()
	pw.Write(id, *v)
	return func(err error) {
		defer pw.Close()
		stop := time.Now()
		v.Completed = &stop
		v.Cached = false
		if err != nil {
			v.Error = err.Error()
		}
		pw.Write(id, *v)
	}
}

func supportedEntitlements(ents []string) []entitlements.Entitlement {
	out := []entitlements.Entitlement{} // nil means no filter
	for _, e := range ents {
		if e == string(entitlements.EntitlementNetworkHost) {
			out = append(out, entitlements.EntitlementNetworkHost)
		}
		if e == string(entitlements.EntitlementSecurityInsecure) {
			out = append(out, entitlements.EntitlementSecurityInsecure)
		}
	}
	return out
}

func loadEntitlements(b solver.Builder) (entitlements.Set, error) {
	var ent entitlements.Set = map[entitlements.Entitlement]struct{}{}
	err := b.EachValue(context.TODO(), keyEntitlements, func(v interface{}) error {
		set, ok := v.(entitlements.Set)
		if !ok {
			return errors.Errorf("invalid entitlements %T", v)
		}
		for k := range set {
			ent[k] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ent, nil
}

func loadSourcePolicy(b solver.Builder) (*spb.Policy, error) {
	var srcPol spb.Policy
	err := b.EachValue(context.TODO(), keySourcePolicy, func(v interface{}) error {
		x, ok := v.(*spb.Policy)
		if !ok {
			return errors.Errorf("invalid source policy %T", v)
		}
		for _, f := range x.Rules {
			if f == nil {
				return errors.Errorf("invalid nil policy rule")
			}
			srcPol.Rules = append(srcPol.Rules, f.CloneVT())
		}
		srcPol.Version = x.Version
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &srcPol, nil
}
