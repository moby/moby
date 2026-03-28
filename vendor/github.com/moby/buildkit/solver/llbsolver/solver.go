package llbsolver

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client"
	controlgateway "github.com/moby/buildkit/control/gateway"
	"github.com/moby/buildkit/executor/resources"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/verifier"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/history"
	"github.com/moby/buildkit/solver/result"
	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type ExporterRequest struct {
	Exporters             []exporter.ExporterInstance
	CacheExporters        []RemoteCacheExporter
	EnableSessionExporter bool
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
	HistoryQueue     *history.Queue
	ResourceMonitor  *resources.Monitor
	ProvenanceEnv    map[string]any
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
	history                   *history.Queue
	sysSampler                *resources.Sampler[*resourcestypes.SysSample]
	provenanceEnv             map[string]any
}

// Processor defines a processing function to be applied after solving, but
// before exporting
type Processor func(ctx context.Context, result *Result, s *Solver, j *solver.Job, usage *resources.SysSampler) (*Result, error)

func New(opt Opt) (*Solver, error) {
	// buildConfig,builderPlatform,platform are not allowd
	forbiddenKeys := map[string]struct{}{
		"buildConfig":     {},
		"builderPlatform": {},
		"platform":        {},
	}
	for k := range opt.ProvenanceEnv {
		if _, ok := forbiddenKeys[k]; ok {
			return nil, errors.Errorf("key %q is builtin and not allowed to be modified in provenance config", k)
		}
	}

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
		provenanceEnv:             opt.ProvenanceEnv,
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

func (s *Solver) Solve(ctx context.Context, id string, sessionID string, req frontend.SolveRequest, exp ExporterRequest, ent []entitlements.Entitlement, post []Processor, internal bool, srcPol *spb.Policy, policySession string) (_ *client.SolveResponse, err error) {
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
	if policySession != "" {
		j.SetValue(keySourcePolicySession, policySession)
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
	// that the object is not garbage collected immediately. This is protected by the individual components,
	// but because creating a lease is not cheap and requires a disk write, we create a single lease here
	// early and let all the exporters, cache export, provenance creation, and finalize callbacks use the
	// same one. The lease must span both artifact creation and the finalize phase (registry push) to
	// prevent GC from collecting blobs before they are pushed.
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

	if exp.EnableSessionExporter {
		exporters, err := s.getSessionExporters(ctx, j.SessionID, len(exp.Exporters), inp)
		if err != nil {
			return nil, err
		}
		exp.Exporters = append(exp.Exporters, exporters...)
	}

	var exporterResponse map[string]string
	var finalizers []exporter.FinalizeFunc
	exporterResponse, finalizers, descrefs, err = s.runExporters(ctx, id, exp.Exporters, inlineCacheExporter, j, cached, inp)
	if err != nil {
		return nil, err
	}

	// Run image finalize and cache export in parallel.
	// Image Export has already created layers in the content store,
	// so cache exporters can see and reuse them.
	eg, egCtx := errgroup.WithContext(ctx)
	for i, finalize := range finalizers {
		if finalize == nil {
			continue
		}
		name := exp.Exporters[i].Name()
		id := exporterVertexID(j.SessionID, i)
		eg.Go(func() error {
			return inBuilderContext(egCtx, j, name, id, func(ctx context.Context, _ solver.JobContext) error {
				return finalize(ctx)
			})
		})
	}
	var cacheExporterResponse map[string]string
	eg.Go(func() error {
		var err error
		cacheExporterResponse, err = runCacheExporters(egCtx, cacheExporters, j, cached, inp)
		return err
	})
	if err := eg.Wait(); err != nil {
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

func (s *Solver) leaseManager() (*leaseutil.Manager, error) {
	w, err := defaultResolver(s.workerController)()
	if err != nil {
		return nil, err
	}
	return w.LeaseManager(), nil
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

func inBuilderContext(ctx context.Context, b solver.Builder, name, id string, f func(ctx context.Context, jobCtx solver.JobContext) error) error {
	if id == "" {
		id = name
	}
	v := client.Vertex{
		Digest: digest.FromBytes([]byte(id)),
		Name:   name,
	}
	return b.InContext(ctx, func(ctx context.Context, jobCtx solver.JobContext) error {
		pw, _, ctx := progress.NewFromContext(ctx, progress.WithMetadata("vertex", v.Digest))
		notifyCompleted := notifyStarted(ctx, &v)
		defer pw.Close()
		err := f(ctx, jobCtx)
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
