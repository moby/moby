package llbsolver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client"
	controlgateway "github.com/moby/buildkit/control/gateway"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/provenance"
	"github.com/moby/buildkit/solver/result"
	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/attestation"
	"github.com/moby/buildkit/util/buildinfo"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/tracing/detect"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	keyEntitlements = "llb.entitlements"
	keySourcePolicy = "llb.sourcepolicy"
)

type ExporterRequest struct {
	Exporter       exporter.ExporterInstance
	CacheExporters []RemoteCacheExporter
}

type RemoteCacheExporter struct {
	remotecache.Exporter
	solver.CacheExportMode
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
}

// Processor defines a processing function to be applied after solving, but
// before exporting
type Processor func(ctx context.Context, result *Result, s *Solver, j *solver.Job) (*Result, error)

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

	s.solver = solver.NewSolver(solver.SolverOpt{
		ResolveOpFunc: s.resolver(),
		DefaultCache:  opt.CacheManager,
	})
	return s, nil
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

func (s *Solver) recordBuildHistory(ctx context.Context, id string, req frontend.SolveRequest, j *solver.Job) (func(*Result, exporter.DescriptorReference, error) error, error) {
	var stopTrace func() []tracetest.SpanStub

	if s := trace.SpanFromContext(ctx); s.SpanContext().IsValid() {
		if exp, err := detect.Exporter(); err == nil {
			if rec, ok := exp.(*detect.TraceRecorder); ok {
				stopTrace = rec.Record(s.SpanContext().TraceID())
			}
		}
	}

	st := time.Now()
	rec := &controlapi.BuildHistoryRecord{
		Ref:           id,
		Frontend:      req.Frontend,
		FrontendAttrs: req.FrontendOpt,
		CreatedAt:     &st,
	}
	if err := s.history.Update(ctx, &controlapi.BuildHistoryEvent{
		Type:   controlapi.BuildHistoryEventType_STARTED,
		Record: rec,
	}); err != nil {
		return nil, err
	}

	return func(res *Result, descref exporter.DescriptorReference, err error) error {
		en := time.Now()
		rec.CompletedAt = &en

		j.CloseProgress()

		if res != nil && len(res.Metadata) > 0 {
			rec.ExporterResponse = map[string]string{}
			for k, v := range res.Metadata {
				rec.ExporterResponse[k] = string(v)
			}
		}

		var mu sync.Mutex
		ch := make(chan *client.SolveStatus)
		eg, ctx2 := errgroup.WithContext(ctx)
		var releasers []func()

		attrs := map[string]string{
			"mode": "max",
		}

		makeProvenance := func(res solver.ResultProxy, cap *provenance.Capture) (*controlapi.Descriptor, func(), error) {
			prc, err := NewProvenanceCreator(ctx2, cap, res, attrs, j)
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
			w, err := s.history.OpenBlobWriter(ctx, attestation.MediaTypeDockerSchema2AttestationType)
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
				Digest:    desc.Digest,
				Size_:     desc.Size,
				MediaType: desc.MediaType,
				Annotations: map[string]string{
					"in-toto.io/predicate-type": slsa.PredicateSLSAProvenance,
				},
			}, release, nil
		}

		if res != nil {
			if res.Ref != nil {
				eg.Go(func() error {
					desc, release, err := makeProvenance(res.Ref, res.Provenance.Ref)
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
				k, r := k, r
				cp := res.Provenance.Refs[k]
				eg.Go(func() error {
					desc, release, err := makeProvenance(r, cp)
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
			desc, releaseStatus, err := s.history.ImportStatus(ctx2, ch)
			if err != nil {
				return err
			}
			mu.Lock()
			releasers = append(releasers, releaseStatus)
			rec.Logs = &controlapi.Descriptor{
				Digest:    desc.Digest,
				Size_:     desc.Size,
				MediaType: desc.MediaType,
			}
			mu.Unlock()
			return nil
		})
		eg.Go(func() error {
			return j.Status(ctx2, ch)
		})

		if descref != nil {
			eg.Go(func() error {
				mu.Lock()
				if rec.Result == nil {
					rec.Result = &controlapi.BuildResultInfo{}
				}
				desc := descref.Descriptor()
				rec.Result.Result = &controlapi.Descriptor{
					Digest:      desc.Digest,
					Size_:       desc.Size,
					MediaType:   desc.MediaType,
					Annotations: desc.Annotations,
				}
				mu.Unlock()
				return nil
			})
		}

		if err1 := eg.Wait(); err == nil {
			err = err1
		}

		defer func() {
			for _, f := range releasers {
				f()
			}
		}()

		if err != nil {
			st, ok := grpcerrors.AsGRPCStatus(grpcerrors.ToGRPC(err))
			if !ok {
				st = status.New(codes.Unknown, err.Error())
			}
			rec.Error = grpcerrors.ToRPCStatus(st.Proto())
		}
		if err1 := s.history.Update(ctx, &controlapi.BuildHistoryEvent{
			Type:   controlapi.BuildHistoryEventType_COMPLETE,
			Record: rec,
		}); err1 != nil {
			if err == nil {
				err = err1
			}
		}

		if err != nil {
			return err
		}

		if stopTrace == nil {
			logrus.Warn("no trace recorder found, skipping")
			return err
		}
		go func() {
			time.Sleep(3 * time.Second)
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
						Digest:    desc.Digest,
						MediaType: desc.MediaType,
						Size_:     desc.Size,
					}
					return nil
				}); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				logrus.Errorf("failed to save trace for %s: %+v", id, err)
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

	var res *frontend.Result
	var resProv *Result
	var descref exporter.DescriptorReference

	var releasers []func()
	defer func() {
		for _, f := range releasers {
			f()
		}
		if descref != nil {
			descref.Release()
		}
	}()

	if internal {
		defer j.CloseProgress()
	} else {
		rec, err1 := s.recordBuildHistory(ctx, id, req, j)
		if err != nil {
			defer j.CloseProgress()
			return nil, err1
		}
		defer func() {
			err = rec(resProv, descref, err)
		}()
	}

	set, err := entitlements.WhiteList(ent, supportedEntitlements(s.entitlements))
	if err != nil {
		return nil, err
	}
	j.SetValue(keyEntitlements, set)

	if srcPol != nil {
		j.SetValue(keySourcePolicy, *srcPol)
	}

	j.SessionID = sessionID

	br := s.bridge(j)
	if s.gatewayForwarder != nil && req.Definition == nil && req.Frontend == "" {
		fwd := gateway.NewBridgeForwarder(ctx, br, s.workerController, req.FrontendInputs, sessionID, s.sm)
		defer fwd.Discard()
		if err := s.gatewayForwarder.RegisterBuild(ctx, id, fwd); err != nil {
			return nil, err
		}
		defer s.gatewayForwarder.UnregisterBuild(ctx, id)

		var err error
		select {
		case <-fwd.Done():
			res, err = fwd.Result()
		case <-ctx.Done():
			err = ctx.Err()
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
		res2, err := post(ctx, resProv, s, j)
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

	cacheExporters, inlineCacheExporter := splitCacheExporters(exp.CacheExporters)

	var exporterResponse map[string]string
	if e := exp.Exporter; e != nil {
		meta, err := runInlineCacheExporter(ctx, e, inlineCacheExporter, j, cached)
		if err != nil {
			return nil, err
		}
		for k, v := range meta {
			inp.AddMeta(k, v)
		}

		if err := inBuilderContext(ctx, j, e.Name(), j.SessionID+"-export", func(ctx context.Context, _ session.Group) error {
			exporterResponse, descref, err = e.Export(ctx, inp, j.SessionID)
			return err
		}); err != nil {
			return nil, err
		}
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
		if strings.HasPrefix(k, exptypes.ExporterBuildInfo) {
			exporterResponse[k] = base64.StdEncoding.EncodeToString(v)
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

func runCacheExporters(ctx context.Context, exporters []RemoteCacheExporter, j *solver.Job, cached *result.Result[solver.CachedResult], inp *result.Result[cache.ImmutableRef]) (map[string]string, error) {
	eg, ctx := errgroup.WithContext(ctx)
	g := session.NewGroup(j.SessionID)
	var cacheExporterResponse map[string]string
	resps := make([]map[string]string, len(exporters))
	for i, exp := range exporters {
		func(exp RemoteCacheExporter, i int) {
			eg.Go(func() (err error) {
				id := fmt.Sprint(j.SessionID, "-cache-", i)
				return inBuilderContext(ctx, j, exp.Exporter.Name(), id, func(ctx context.Context, _ session.Group) error {
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
					prepareDone(nil)
					resps[i], err = exp.Finalize(ctx)
					if err != nil {
						return err
					}
					return nil
				})
			})
		}(exp, i)
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	for _, resp := range resps {
		for k, v := range resp {
			if cacheExporterResponse == nil {
				cacheExporterResponse = make(map[string]string)
			}
			cacheExporterResponse[k] = v
		}
	}
	return cacheExporterResponse, nil
}

func runInlineCacheExporter(ctx context.Context, e exporter.ExporterInstance, inlineExporter *RemoteCacheExporter, j *solver.Job, cached *result.Result[solver.CachedResult]) (map[string][]byte, error) {
	meta := map[string][]byte{}
	if inlineExporter == nil {
		return nil, nil
	}
	if err := inBuilderContext(ctx, j, "preparing layers for inline cache", j.SessionID+"-cache-inline", func(ctx context.Context, _ session.Group) error {
		if res := cached.Ref; res != nil {
			dtic, err := inlineCache(ctx, inlineExporter.Exporter, res, e.Config().Compression(), session.NewGroup(j.SessionID))
			if err != nil {
				return err
			}
			if dtic != nil {
				meta[exptypes.ExporterInlineCache] = dtic
			}
		}
		for k, res := range cached.Refs {
			dtic, err := inlineCache(ctx, inlineExporter.Exporter, res, e.Config().Compression(), session.NewGroup(j.SessionID))
			if err != nil {
				return err
			}
			if dtic != nil {
				meta[fmt.Sprintf("%s/%s", exptypes.ExporterInlineCache, k)] = dtic
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return meta, nil
}

func splitCacheExporters(exporters []RemoteCacheExporter) (rest []RemoteCacheExporter, inline *RemoteCacheExporter) {
	rest = make([]RemoteCacheExporter, 0, len(exporters))
	for i, exp := range exporters {
		if _, ok := asInlineCache(exp.Exporter); ok {
			inline = &exporters[i]
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
		if err := buildinfo.AddMetadata(res.Metadata, exptypes.ExporterBuildInfo, cp); err != nil {
			return nil, err
		}
	}

	if len(res.Refs) != 0 {
		out.Provenance.Refs = make(map[string]*provenance.Capture, len(res.Refs))
	}
	for k, ref := range res.Refs {
		cp, err := getProvenance(ref, reqs.refs[k].bridge, k, reqs)
		if err != nil {
			return nil, err
		}
		out.Provenance.Refs[k] = cp
		if res.Metadata == nil {
			res.Metadata = map[string][]byte{}
		}
		if err := buildinfo.AddMetadata(res.Metadata, fmt.Sprintf("%s/%s", exptypes.ExporterBuildInfo, k), cp); err != nil {
			return nil, err
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
		return nil, errors.Errorf("missing provenance for %s", ref.ID())
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
	ExportForLayers(context.Context, []digest.Digest) ([]byte, error)
}

func asInlineCache(e remotecache.Exporter) (inlineCacheExporter, bool) {
	ie, ok := e.(inlineCacheExporter)
	return ie, ok
}

func inlineCache(ctx context.Context, e remotecache.Exporter, res solver.CachedResult, compressionopt compression.Config, g session.Group) ([]byte, error) {
	ie, ok := asInlineCache(e)
	if !ok {
		return nil, nil
	}
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
	if _, err := res.CacheKeys()[0].Exporter.ExportTo(ctx, e, solver.CacheExportOpt{
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
		notifyCompleted := notifyStarted(ctx, &v, false)
		defer pw.Close()
		err := f(ctx, g)
		notifyCompleted(err, false)
		return err
	})
}

func notifyStarted(ctx context.Context, v *client.Vertex, cached bool) func(err error, cached bool) {
	pw, _, _ := progress.NewFromContext(ctx)
	start := time.Now()
	v.Started = &start
	v.Completed = nil
	v.Cached = cached
	id := identity.NewID()
	pw.Write(id, *v)
	return func(err error, cached bool) {
		defer pw.Close()
		stop := time.Now()
		v.Completed = &stop
		v.Cached = cached
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
	set := make(map[spb.Rule]struct{}, 0)
	err := b.EachValue(context.TODO(), keySourcePolicy, func(v interface{}) error {
		x, ok := v.(spb.Policy)
		if !ok {
			return errors.Errorf("invalid source policy %T", v)
		}
		for _, f := range x.Rules {
			set[*f] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var srcPol *spb.Policy
	if len(set) > 0 {
		srcPol = &spb.Policy{}
		for k := range set {
			k := k
			srcPol.Rules = append(srcPol.Rules, &k)
		}
	}
	return srcPol, nil
}
