package control

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moby/buildkit/util/bklog"

	controlapi "github.com/moby/buildkit/api/services/control"
	apitypes "github.com/moby/buildkit/api/types"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client"
	controlgateway "github.com/moby/buildkit/control/gateway"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/grpchijack"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/util/throttle"
	"github.com/moby/buildkit/util/tracing/transform"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Opt struct {
	SessionManager            *session.Manager
	WorkerController          *worker.Controller
	Frontends                 map[string]frontend.Frontend
	CacheKeyStorage           solver.CacheKeyStorage
	ResolveCacheExporterFuncs map[string]remotecache.ResolveCacheExporterFunc
	ResolveCacheImporterFuncs map[string]remotecache.ResolveCacheImporterFunc
	Entitlements              []string
	TraceCollector            sdktrace.SpanExporter
}

type Controller struct { // TODO: ControlService
	// buildCount needs to be 64bit aligned
	buildCount       int64
	opt              Opt
	solver           *llbsolver.Solver
	cache            solver.CacheManager
	gatewayForwarder *controlgateway.GatewayForwarder
	throttledGC      func()
	gcmu             sync.Mutex
	*tracev1.UnimplementedTraceServiceServer
}

func NewController(opt Opt) (*Controller, error) {
	cache := solver.NewCacheManager(context.TODO(), "local", opt.CacheKeyStorage, worker.NewCacheResultStorage(opt.WorkerController))

	gatewayForwarder := controlgateway.NewGatewayForwarder()

	solver, err := llbsolver.New(opt.WorkerController, opt.Frontends, cache, opt.ResolveCacheImporterFuncs, gatewayForwarder, opt.SessionManager, opt.Entitlements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create solver")
	}

	c := &Controller{
		opt:              opt,
		solver:           solver,
		cache:            cache,
		gatewayForwarder: gatewayForwarder,
	}
	c.throttledGC = throttle.After(time.Minute, c.gc)

	defer func() {
		time.AfterFunc(time.Second, c.throttledGC)
	}()

	return c, nil
}

func (c *Controller) Register(server *grpc.Server) error {
	controlapi.RegisterControlServer(server, c)
	c.gatewayForwarder.Register(server)
	tracev1.RegisterTraceServiceServer(server, c)
	return nil
}

func (c *Controller) DiskUsage(ctx context.Context, r *controlapi.DiskUsageRequest) (*controlapi.DiskUsageResponse, error) {
	resp := &controlapi.DiskUsageResponse{}
	workers, err := c.opt.WorkerController.List()
	if err != nil {
		return nil, err
	}
	for _, w := range workers {
		du, err := w.DiskUsage(ctx, client.DiskUsageInfo{
			Filter: r.Filter,
		})
		if err != nil {
			return nil, err
		}

		for _, r := range du {
			resp.Record = append(resp.Record, &controlapi.UsageRecord{
				// TODO: add worker info
				ID:          r.ID,
				Mutable:     r.Mutable,
				InUse:       r.InUse,
				Size_:       r.Size,
				Parents:     r.Parents,
				UsageCount:  int64(r.UsageCount),
				Description: r.Description,
				CreatedAt:   r.CreatedAt,
				LastUsedAt:  r.LastUsedAt,
				RecordType:  string(r.RecordType),
				Shared:      r.Shared,
			})
		}
	}
	return resp, nil
}

func (c *Controller) Prune(req *controlapi.PruneRequest, stream controlapi.Control_PruneServer) error {
	if atomic.LoadInt64(&c.buildCount) == 0 {
		imageutil.CancelCacheLeases()
	}

	ch := make(chan client.UsageInfo)

	eg, ctx := errgroup.WithContext(stream.Context())
	workers, err := c.opt.WorkerController.List()
	if err != nil {
		return errors.Wrap(err, "failed to list workers for prune")
	}

	didPrune := false
	defer func() {
		if didPrune {
			if c, ok := c.cache.(interface {
				ReleaseUnreferenced() error
			}); ok {
				if err := c.ReleaseUnreferenced(); err != nil {
					bklog.G(ctx).Errorf("failed to release cache metadata: %+v", err)
				}
			}
		}
	}()

	for _, w := range workers {
		func(w worker.Worker) {
			eg.Go(func() error {
				return w.Prune(ctx, ch, client.PruneInfo{
					Filter:       req.Filter,
					All:          req.All,
					KeepDuration: time.Duration(req.KeepDuration),
					KeepBytes:    req.KeepBytes,
				})
			})
		}(w)
	}

	eg2, _ := errgroup.WithContext(stream.Context())

	eg2.Go(func() error {
		defer close(ch)
		return eg.Wait()
	})

	eg2.Go(func() error {
		for r := range ch {
			didPrune = true
			if err := stream.Send(&controlapi.UsageRecord{
				// TODO: add worker info
				ID:          r.ID,
				Mutable:     r.Mutable,
				InUse:       r.InUse,
				Size_:       r.Size,
				Parents:     r.Parents,
				UsageCount:  int64(r.UsageCount),
				Description: r.Description,
				CreatedAt:   r.CreatedAt,
				LastUsedAt:  r.LastUsedAt,
				RecordType:  string(r.RecordType),
				Shared:      r.Shared,
			}); err != nil {
				return err
			}
		}
		return nil
	})

	return eg2.Wait()
}

func (c *Controller) Export(ctx context.Context, req *tracev1.ExportTraceServiceRequest) (*tracev1.ExportTraceServiceResponse, error) {
	if c.opt.TraceCollector == nil {
		return nil, status.Errorf(codes.Unavailable, "trace collector not configured")
	}
	err := c.opt.TraceCollector.ExportSpans(ctx, transform.Spans(req.GetResourceSpans()))
	if err != nil {
		return nil, err
	}
	return &tracev1.ExportTraceServiceResponse{}, nil
}

func translateLegacySolveRequest(req *controlapi.SolveRequest) error {
	// translates ExportRef and ExportAttrs to new Exports (v0.4.0)
	if legacyExportRef := req.Cache.ExportRefDeprecated; legacyExportRef != "" {
		ex := &controlapi.CacheOptionsEntry{
			Type:  "registry",
			Attrs: req.Cache.ExportAttrsDeprecated,
		}
		if ex.Attrs == nil {
			ex.Attrs = make(map[string]string)
		}
		ex.Attrs["ref"] = legacyExportRef
		// FIXME(AkihiroSuda): skip append if already exists
		req.Cache.Exports = append(req.Cache.Exports, ex)
		req.Cache.ExportRefDeprecated = ""
		req.Cache.ExportAttrsDeprecated = nil
	}
	// translates ImportRefs to new Imports (v0.4.0)
	for _, legacyImportRef := range req.Cache.ImportRefsDeprecated {
		im := &controlapi.CacheOptionsEntry{
			Type:  "registry",
			Attrs: map[string]string{"ref": legacyImportRef},
		}
		// FIXME(AkihiroSuda): skip append if already exists
		req.Cache.Imports = append(req.Cache.Imports, im)
	}
	req.Cache.ImportRefsDeprecated = nil
	return nil
}

func (c *Controller) Solve(ctx context.Context, req *controlapi.SolveRequest) (*controlapi.SolveResponse, error) {
	atomic.AddInt64(&c.buildCount, 1)
	defer atomic.AddInt64(&c.buildCount, -1)

	// This method registers job ID in solver.Solve. Make sure there are no blocking calls before that might delay this.

	if err := translateLegacySolveRequest(req); err != nil {
		return nil, err
	}

	defer func() {
		time.AfterFunc(time.Second, c.throttledGC)
	}()

	var expi exporter.ExporterInstance
	// TODO: multiworker
	// This is actually tricky, as the exporter should come from the worker that has the returned reference. We may need to delay this so that the solver loads this.
	w, err := c.opt.WorkerController.GetDefault()
	if err != nil {
		return nil, err
	}
	if req.Exporter != "" {
		exp, err := w.Exporter(req.Exporter, c.opt.SessionManager)
		if err != nil {
			return nil, err
		}
		expi, err = exp.Resolve(ctx, req.ExporterAttrs)
		if err != nil {
			return nil, err
		}
	}

	var (
		cacheExporter   remotecache.Exporter
		cacheExportMode solver.CacheExportMode
		cacheImports    []frontend.CacheOptionsEntry
	)
	if len(req.Cache.Exports) > 1 {
		// TODO(AkihiroSuda): this should be fairly easy
		return nil, errors.New("specifying multiple cache exports is not supported currently")
	}

	if len(req.Cache.Exports) == 1 {
		e := req.Cache.Exports[0]
		cacheExporterFunc, ok := c.opt.ResolveCacheExporterFuncs[e.Type]
		if !ok {
			return nil, errors.Errorf("unknown cache exporter: %q", e.Type)
		}
		cacheExporter, err = cacheExporterFunc(ctx, session.NewGroup(req.Session), e.Attrs)
		if err != nil {
			return nil, err
		}
		if exportMode, supported := parseCacheExportMode(e.Attrs["mode"]); !supported {
			bklog.G(ctx).Debugf("skipping invalid cache export mode: %s", e.Attrs["mode"])
		} else {
			cacheExportMode = exportMode
		}
	}
	for _, im := range req.Cache.Imports {
		cacheImports = append(cacheImports, frontend.CacheOptionsEntry{
			Type:  im.Type,
			Attrs: im.Attrs,
		})
	}

	resp, err := c.solver.Solve(ctx, req.Ref, req.Session, frontend.SolveRequest{
		Frontend:       req.Frontend,
		Definition:     req.Definition,
		FrontendOpt:    req.FrontendAttrs,
		FrontendInputs: req.FrontendInputs,
		CacheImports:   cacheImports,
	}, llbsolver.ExporterRequest{
		Exporter:        expi,
		CacheExporter:   cacheExporter,
		CacheExportMode: cacheExportMode,
	}, req.Entitlements)
	if err != nil {
		return nil, err
	}
	return &controlapi.SolveResponse{
		ExporterResponse: resp.ExporterResponse,
	}, nil
}

func (c *Controller) Status(req *controlapi.StatusRequest, stream controlapi.Control_StatusServer) error {
	ch := make(chan *client.SolveStatus, 8)

	eg, ctx := errgroup.WithContext(stream.Context())
	eg.Go(func() error {
		return c.solver.Status(ctx, req.Ref, ch)
	})

	eg.Go(func() error {
		for {
			ss, ok := <-ch
			if !ok {
				return nil
			}
			logSize := 0
			for {
				retry := false
				sr := controlapi.StatusResponse{}
				for _, v := range ss.Vertexes {
					sr.Vertexes = append(sr.Vertexes, &controlapi.Vertex{
						Digest:        v.Digest,
						Inputs:        v.Inputs,
						Name:          v.Name,
						Started:       v.Started,
						Completed:     v.Completed,
						Error:         v.Error,
						Cached:        v.Cached,
						ProgressGroup: v.ProgressGroup,
					})
				}
				for _, v := range ss.Statuses {
					sr.Statuses = append(sr.Statuses, &controlapi.VertexStatus{
						ID:        v.ID,
						Vertex:    v.Vertex,
						Name:      v.Name,
						Current:   v.Current,
						Total:     v.Total,
						Timestamp: v.Timestamp,
						Started:   v.Started,
						Completed: v.Completed,
					})
				}
				for i, v := range ss.Logs {
					sr.Logs = append(sr.Logs, &controlapi.VertexLog{
						Vertex:    v.Vertex,
						Stream:    int64(v.Stream),
						Msg:       v.Data,
						Timestamp: v.Timestamp,
					})
					logSize += len(v.Data) + emptyLogVertexSize
					// avoid logs growing big and split apart if they do
					if logSize > 1024*1024 {
						ss.Vertexes = nil
						ss.Statuses = nil
						ss.Logs = ss.Logs[i+1:]
						retry = true
						break
					}
				}
				for _, v := range ss.Warnings {
					sr.Warnings = append(sr.Warnings, &controlapi.VertexWarning{
						Vertex: v.Vertex,
						Level:  int64(v.Level),
						Short:  v.Short,
						Detail: v.Detail,
						Info:   v.SourceInfo,
						Ranges: v.Range,
						Url:    v.URL,
					})
				}
				if err := stream.SendMsg(&sr); err != nil {
					return err
				}
				if !retry {
					break
				}
			}
		}
	})

	return eg.Wait()
}

func (c *Controller) Session(stream controlapi.Control_SessionServer) error {
	bklog.G(stream.Context()).Debugf("session started")

	conn, closeCh, opts := grpchijack.Hijack(stream)
	defer conn.Close()

	ctx, cancel := context.WithCancel(stream.Context())
	go func() {
		<-closeCh
		cancel()
	}()

	err := c.opt.SessionManager.HandleConn(ctx, conn, opts)
	bklog.G(ctx).Debugf("session finished: %v", err)
	return err
}

func (c *Controller) ListWorkers(ctx context.Context, r *controlapi.ListWorkersRequest) (*controlapi.ListWorkersResponse, error) {
	resp := &controlapi.ListWorkersResponse{}
	workers, err := c.opt.WorkerController.List(r.Filter...)
	if err != nil {
		return nil, err
	}
	for _, w := range workers {
		resp.Record = append(resp.Record, &apitypes.WorkerRecord{
			ID:        w.ID(),
			Labels:    w.Labels(),
			Platforms: pb.PlatformsFromSpec(w.Platforms(true)),
			GCPolicy:  toPBGCPolicy(w.GCPolicy()),
		})
	}
	return resp, nil
}

func (c *Controller) gc() {
	c.gcmu.Lock()
	defer c.gcmu.Unlock()

	workers, err := c.opt.WorkerController.List()
	if err != nil {
		return
	}

	eg, ctx := errgroup.WithContext(context.TODO())

	var size int64
	ch := make(chan client.UsageInfo)
	done := make(chan struct{})
	go func() {
		for ui := range ch {
			size += ui.Size
		}
		close(done)
	}()

	for _, w := range workers {
		func(w worker.Worker) {
			eg.Go(func() error {
				if policy := w.GCPolicy(); len(policy) > 0 {
					return w.Prune(ctx, ch, policy...)
				}
				return nil
			})
		}(w)
	}

	err = eg.Wait()
	close(ch)
	if err != nil {
		bklog.G(ctx).Errorf("gc error: %+v", err)
	}
	<-done
	if size > 0 {
		bklog.G(ctx).Debugf("gc cleaned up %d bytes", size)
	}
}

func parseCacheExportMode(mode string) (solver.CacheExportMode, bool) {
	switch mode {
	case "min":
		return solver.CacheExportModeMin, true
	case "max":
		return solver.CacheExportModeMax, true
	}
	return solver.CacheExportModeMin, false
}

func toPBGCPolicy(in []client.PruneInfo) []*apitypes.GCPolicy {
	policy := make([]*apitypes.GCPolicy, 0, len(in))
	for _, p := range in {
		policy = append(policy, &apitypes.GCPolicy{
			All:          p.All,
			KeepBytes:    p.KeepBytes,
			KeepDuration: int64(p.KeepDuration),
			Filters:      p.Filter,
		})
	}
	return policy
}
