package control

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	contentapi "github.com/containerd/containerd/api/services/content/v1"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/services/content/contentserver"
	"github.com/docker/distribution/reference"
	"github.com/mitchellh/hashstructure/v2"
	controlapi "github.com/moby/buildkit/api/services/control"
	apitypes "github.com/moby/buildkit/api/types"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/cmd/buildkitd/config"
	controlgateway "github.com/moby/buildkit/control/gateway"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/util/epoch"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/attestations"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/grpchijack"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/llbsolver/proc"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/util/throttle"
	"github.com/moby/buildkit/util/tracing/transform"
	"github.com/moby/buildkit/version"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
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
	HistoryDB                 *bbolt.DB
	LeaseManager              leases.Manager
	ContentStore              content.Store
	HistoryConfig             *config.HistoryConfig
}

type Controller struct { // TODO: ControlService
	// buildCount needs to be 64bit aligned
	buildCount       int64
	opt              Opt
	solver           *llbsolver.Solver
	history          *llbsolver.HistoryQueue
	cache            solver.CacheManager
	gatewayForwarder *controlgateway.GatewayForwarder
	throttledGC      func()
	gcmu             sync.Mutex
	*tracev1.UnimplementedTraceServiceServer
}

func NewController(opt Opt) (*Controller, error) {
	cache := solver.NewCacheManager(context.TODO(), "local", opt.CacheKeyStorage, worker.NewCacheResultStorage(opt.WorkerController))

	gatewayForwarder := controlgateway.NewGatewayForwarder()

	hq := llbsolver.NewHistoryQueue(llbsolver.HistoryQueueOpt{
		DB:           opt.HistoryDB,
		LeaseManager: opt.LeaseManager,
		ContentStore: opt.ContentStore,
		CleanConfig:  opt.HistoryConfig,
	})

	s, err := llbsolver.New(llbsolver.Opt{
		WorkerController: opt.WorkerController,
		Frontends:        opt.Frontends,
		CacheManager:     cache,
		CacheResolvers:   opt.ResolveCacheImporterFuncs,
		GatewayForwarder: gatewayForwarder,
		SessionManager:   opt.SessionManager,
		Entitlements:     opt.Entitlements,
		HistoryQueue:     hq,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create solver")
	}

	c := &Controller{
		opt:              opt,
		solver:           s,
		history:          hq,
		cache:            cache,
		gatewayForwarder: gatewayForwarder,
	}
	c.throttledGC = throttle.After(time.Minute, c.gc)

	defer func() {
		time.AfterFunc(time.Second, c.throttledGC)
	}()

	return c, nil
}

func (c *Controller) Close() error {
	return c.opt.WorkerController.Close()
}

func (c *Controller) Register(server *grpc.Server) {
	controlapi.RegisterControlServer(server, c)
	c.gatewayForwarder.Register(server)
	tracev1.RegisterTraceServiceServer(server, c)

	store := &roContentStore{c.opt.ContentStore}
	contentapi.RegisterContentServer(server, contentserver.New(store))
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

func (c *Controller) ListenBuildHistory(req *controlapi.BuildHistoryRequest, srv controlapi.Control_ListenBuildHistoryServer) error {
	return c.history.Listen(srv.Context(), req, func(h *controlapi.BuildHistoryEvent) error {
		if err := srv.Send(h); err != nil {
			return err
		}
		return nil
	})
}

func (c *Controller) UpdateBuildHistory(ctx context.Context, req *controlapi.UpdateBuildHistoryRequest) (*controlapi.UpdateBuildHistoryResponse, error) {
	if !req.Delete {
		err := c.history.UpdateRef(ctx, req.Ref, func(r *controlapi.BuildHistoryRecord) error {
			if req.Pinned == r.Pinned {
				return nil
			}
			r.Pinned = req.Pinned
			return nil
		})
		return &controlapi.UpdateBuildHistoryResponse{}, err
	}

	err := c.history.Delete(ctx, req.Ref)
	return &controlapi.UpdateBuildHistoryResponse{}, err
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

	// if SOURCE_DATE_EPOCH is set, enable it for the exporter
	if v, ok := epoch.ParseBuildArgs(req.FrontendAttrs); ok {
		if _, ok := req.ExporterAttrs[epoch.KeySourceDateEpoch]; !ok {
			if req.ExporterAttrs == nil {
				req.ExporterAttrs = make(map[string]string)
			}
			req.ExporterAttrs[epoch.KeySourceDateEpoch] = v
		}
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

	if c, err := findDuplicateCacheOptions(req.Cache.Exports); err != nil {
		return nil, err
	} else if c != nil {
		types := []string{}
		for _, c := range c {
			types = append(types, c.Type)
		}
		return nil, errors.Errorf("duplicate cache exports %s", types)
	}
	var cacheExporters []llbsolver.RemoteCacheExporter
	for _, e := range req.Cache.Exports {
		cacheExporterFunc, ok := c.opt.ResolveCacheExporterFuncs[e.Type]
		if !ok {
			return nil, errors.Errorf("unknown cache exporter: %q", e.Type)
		}
		var exp llbsolver.RemoteCacheExporter
		exp.Exporter, err = cacheExporterFunc(ctx, session.NewGroup(req.Session), e.Attrs)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to configure %v cache exporter", e.Type)
		}
		if exportMode, supported := parseCacheExportMode(e.Attrs["mode"]); !supported {
			bklog.G(ctx).Debugf("skipping invalid cache export mode: %s", e.Attrs["mode"])
		} else {
			exp.CacheExportMode = exportMode
		}
		cacheExporters = append(cacheExporters, exp)
	}

	var cacheImports []frontend.CacheOptionsEntry
	for _, im := range req.Cache.Imports {
		cacheImports = append(cacheImports, frontend.CacheOptionsEntry{
			Type:  im.Type,
			Attrs: im.Attrs,
		})
	}

	attests, err := attestations.Parse(req.FrontendAttrs)
	if err != nil {
		return nil, err
	}

	var procs []llbsolver.Processor

	if attrs, ok := attests["sbom"]; ok {
		src := attrs["generator"]
		if src == "" {
			return nil, errors.Errorf("sbom generator cannot be empty")
		}
		ref, err := reference.ParseNormalizedNamed(src)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse sbom generator %s", src)
		}
		ref = reference.TagNameOnly(ref)
		procs = append(procs, proc.SBOMProcessor(ref.String()))
	}

	if attrs, ok := attests["provenance"]; ok {
		procs = append(procs, proc.ProvenanceProcessor(attrs))
	}

	resp, err := c.solver.Solve(ctx, req.Ref, req.Session, frontend.SolveRequest{
		Frontend:       req.Frontend,
		Definition:     req.Definition,
		FrontendOpt:    req.FrontendAttrs,
		FrontendInputs: req.FrontendInputs,
		CacheImports:   cacheImports,
	}, llbsolver.ExporterRequest{
		Exporter:       expi,
		CacheExporters: cacheExporters,
	}, req.Entitlements, procs, req.Internal, req.SourcePolicy)
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
			for _, sr := range ss.Marshal() {
				if err := stream.SendMsg(sr); err != nil {
					return err
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
			ID:              w.ID(),
			Labels:          w.Labels(),
			Platforms:       pb.PlatformsFromSpec(w.Platforms(true)),
			GCPolicy:        toPBGCPolicy(w.GCPolicy()),
			BuildkitVersion: toPBBuildkitVersion(w.BuildkitVersion()),
		})
	}
	return resp, nil
}

func (c *Controller) Info(ctx context.Context, r *controlapi.InfoRequest) (*controlapi.InfoResponse, error) {
	return &controlapi.InfoResponse{
		BuildkitVersion: &apitypes.BuildkitVersion{
			Package:  version.Package,
			Version:  version.Version,
			Revision: version.Revision,
		},
	}, nil
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

func toPBBuildkitVersion(in client.BuildkitVersion) *apitypes.BuildkitVersion {
	return &apitypes.BuildkitVersion{
		Package:  in.Package,
		Version:  in.Version,
		Revision: in.Revision,
	}
}

func findDuplicateCacheOptions(cacheOpts []*controlapi.CacheOptionsEntry) ([]*controlapi.CacheOptionsEntry, error) {
	seen := map[string]*controlapi.CacheOptionsEntry{}
	duplicate := map[string]struct{}{}
	for _, opt := range cacheOpts {
		k, err := cacheOptKey(*opt)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[k]; ok {
			duplicate[k] = struct{}{}
		}
		seen[k] = opt
	}

	var duplicates []*controlapi.CacheOptionsEntry
	for k := range duplicate {
		duplicates = append(duplicates, seen[k])
	}
	return duplicates, nil
}

func cacheOptKey(opt controlapi.CacheOptionsEntry) (string, error) {
	if opt.Type == "registry" && opt.Attrs["ref"] != "" {
		return opt.Attrs["ref"], nil
	}
	var rawOpt = struct {
		Type  string
		Attrs map[string]string
	}{
		Type:  opt.Type,
		Attrs: opt.Attrs,
	}
	hash, err := hashstructure.Hash(rawOpt, hashstructure.FormatV2, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(opt.Type, ":", hash), nil
}

type roContentStore struct {
	content.Store
}

func (cs *roContentStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return nil, errors.Errorf("read-only content store")
}

func (cs *roContentStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return errors.Errorf("read-only content store")
}

func (cs *roContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return content.Info{}, errors.Errorf("read-only content store")
}

func (cs *roContentStore) Abort(ctx context.Context, ref string) error {
	return errors.Errorf("read-only content store")
}
