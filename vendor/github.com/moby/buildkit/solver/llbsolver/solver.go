package llbsolver

import (
	"context"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client"
	controlgateway "github.com/moby/buildkit/control/gateway"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const keyEntitlements = "llb.entitlements"

type ExporterRequest struct {
	Exporter        exporter.ExporterInstance
	CacheExporter   remotecache.Exporter
	CacheExportMode solver.CacheExportMode
}

// ResolveWorkerFunc returns default worker for the temporary default non-distributed use cases
type ResolveWorkerFunc func() (worker.Worker, error)

type Solver struct {
	workerController     *worker.Controller
	solver               *solver.Solver
	resolveWorker        ResolveWorkerFunc
	frontends            map[string]frontend.Frontend
	resolveCacheImporter remotecache.ResolveCacheImporterFunc
	platforms            []specs.Platform
	gatewayForwarder     *controlgateway.GatewayForwarder
}

func New(wc *worker.Controller, f map[string]frontend.Frontend, cache solver.CacheManager, resolveCI remotecache.ResolveCacheImporterFunc, gatewayForwarder *controlgateway.GatewayForwarder) (*Solver, error) {
	s := &Solver{
		workerController:     wc,
		resolveWorker:        defaultResolver(wc),
		frontends:            f,
		resolveCacheImporter: resolveCI,
		gatewayForwarder:     gatewayForwarder,
	}

	// executing is currently only allowed on default worker
	w, err := wc.GetDefault()
	if err != nil {
		return nil, err
	}
	s.platforms = w.Platforms()

	s.solver = solver.NewSolver(solver.SolverOpt{
		ResolveOpFunc: s.resolver(),
		DefaultCache:  cache,
	})
	return s, nil
}

func (s *Solver) resolver() solver.ResolveOpFunc {
	return func(v solver.Vertex, b solver.Builder) (solver.Op, error) {
		w, err := s.resolveWorker()
		if err != nil {
			return nil, err
		}
		return w.ResolveOp(v, s.Bridge(b))
	}
}

func (s *Solver) Bridge(b solver.Builder) frontend.FrontendLLBBridge {
	return &llbBridge{
		builder:              b,
		frontends:            s.frontends,
		resolveWorker:        s.resolveWorker,
		resolveCacheImporter: s.resolveCacheImporter,
		cms:                  map[string]solver.CacheManager{},
		platforms:            s.platforms,
	}
}

func (s *Solver) Solve(ctx context.Context, id string, req frontend.SolveRequest, exp ExporterRequest, ent []entitlements.Entitlement) (*client.SolveResponse, error) {
	j, err := s.solver.NewJob(id)
	if err != nil {
		return nil, err
	}

	defer j.Discard()

	set, err := entitlements.WhiteList(ent, supportedEntitlements())
	if err != nil {
		return nil, err
	}
	j.SetValue(keyEntitlements, set)

	j.SessionID = session.FromContext(ctx)

	var res *frontend.Result
	if s.gatewayForwarder != nil && req.Definition == nil && req.Frontend == "" {
		fwd := gateway.NewBridgeForwarder(ctx, s.Bridge(j), s.workerController)
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
		res, err = s.Bridge(j).Solve(ctx, req)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		res.EachRef(func(ref solver.CachedResult) error {
			go ref.Release(context.TODO())
			return nil
		})
	}()

	var exporterResponse map[string]string
	if exp := exp.Exporter; exp != nil {
		inp := exporter.Source{
			Metadata: res.Metadata,
		}
		if inp.Metadata == nil {
			inp.Metadata = make(map[string][]byte)
		}
		if res := res.Ref; res != nil {
			workerRef, ok := res.Sys().(*worker.WorkerRef)
			if !ok {
				return nil, errors.Errorf("invalid reference: %T", res.Sys())
			}
			inp.Ref = workerRef.ImmutableRef
		}
		if res.Refs != nil {
			m := make(map[string]cache.ImmutableRef, len(res.Refs))
			for k, res := range res.Refs {
				if res == nil {
					m[k] = nil
				} else {
					workerRef, ok := res.Sys().(*worker.WorkerRef)
					if !ok {
						return nil, errors.Errorf("invalid reference: %T", res.Sys())
					}
					m[k] = workerRef.ImmutableRef
				}
			}
			inp.Refs = m
		}

		if err := inVertexContext(j.Context(ctx), exp.Name(), func(ctx context.Context) error {
			exporterResponse, err = exp.Export(ctx, inp)
			return err
		}); err != nil {
			return nil, err
		}
	}

	if e := exp.CacheExporter; e != nil {
		if err := inVertexContext(j.Context(ctx), "exporting cache", func(ctx context.Context) error {
			prepareDone := oneOffProgress(ctx, "preparing build cache for export")
			if err := res.EachRef(func(res solver.CachedResult) error {
				// all keys have same export chain so exporting others is not needed
				_, err := res.CacheKeys()[0].Exporter.ExportTo(ctx, e, solver.CacheExportOpt{
					Convert: workerRefConverter,
					Mode:    exp.CacheExportMode,
				})
				return err
			}); err != nil {
				return prepareDone(err)
			}
			prepareDone(nil)
			return e.Finalize(ctx)
		}); err != nil {
			return nil, err
		}
	}

	return &client.SolveResponse{
		ExporterResponse: exporterResponse,
	}, nil
}

func (s *Solver) Status(ctx context.Context, id string, statusChan chan *client.SolveStatus) error {
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

func oneOffProgress(ctx context.Context, id string) func(err error) error {
	pw, _, _ := progress.FromContext(ctx)
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

func inVertexContext(ctx context.Context, name string, f func(ctx context.Context) error) error {
	v := client.Vertex{
		Digest: digest.FromBytes([]byte(identity.NewID())),
		Name:   name,
	}
	pw, _, ctx := progress.FromContext(ctx, progress.WithMetadata("vertex", v.Digest))
	notifyStarted(ctx, &v, false)
	defer pw.Close()
	err := f(ctx)
	notifyCompleted(ctx, &v, err, false)
	return err
}

func notifyStarted(ctx context.Context, v *client.Vertex, cached bool) {
	pw, _, _ := progress.FromContext(ctx)
	defer pw.Close()
	now := time.Now()
	v.Started = &now
	v.Completed = nil
	v.Cached = cached
	pw.Write(v.Digest.String(), *v)
}

func notifyCompleted(ctx context.Context, v *client.Vertex, err error, cached bool) {
	pw, _, _ := progress.FromContext(ctx)
	defer pw.Close()
	now := time.Now()
	if v.Started == nil {
		v.Started = &now
	}
	v.Completed = &now
	v.Cached = cached
	if err != nil {
		v.Error = err.Error()
	}
	pw.Write(v.Digest.String(), *v)
}

var AllowNetworkHostUnstable = false // TODO: enable in constructor

func supportedEntitlements() []entitlements.Entitlement {
	out := []entitlements.Entitlement{} // nil means no filter
	if AllowNetworkHostUnstable {
		out = append(out, entitlements.EntitlementNetworkHost)
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
