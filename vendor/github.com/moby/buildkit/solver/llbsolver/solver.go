package llbsolver

import (
	"context"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/worker"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ExporterRequest struct {
	Exporter        exporter.ExporterInstance
	CacheExporter   *remotecache.RegistryCacheExporter
	CacheExportMode solver.CacheExportMode
}

// ResolveWorkerFunc returns default worker for the temporary default non-distributed use cases
type ResolveWorkerFunc func() (worker.Worker, error)

type Solver struct {
	solver        *solver.Solver
	resolveWorker ResolveWorkerFunc
	frontends     map[string]frontend.Frontend
	ci            *remotecache.CacheImporter
	platforms     []specs.Platform
}

func New(wc *worker.Controller, f map[string]frontend.Frontend, cacheStore solver.CacheKeyStorage, ci *remotecache.CacheImporter) (*Solver, error) {
	s := &Solver{
		resolveWorker: defaultResolver(wc),
		frontends:     f,
		ci:            ci,
	}

	results := newCacheResultStorage(wc)

	cache := solver.NewCacheManager("local", cacheStore, results)

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
		builder:       b,
		frontends:     s.frontends,
		resolveWorker: s.resolveWorker,
		ci:            s.ci,
		cms:           map[string]solver.CacheManager{},
		platforms:     s.platforms,
	}
}

func (s *Solver) Solve(ctx context.Context, id string, req frontend.SolveRequest, exp ExporterRequest) (*client.SolveResponse, error) {
	j, err := s.solver.NewJob(id)
	if err != nil {
		return nil, err
	}

	defer j.Discard()

	j.SessionID = session.FromContext(ctx)

	res, exporterOpt, err := s.Bridge(j).Solve(ctx, req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if res != nil {
			go res.Release(context.TODO())
		}
	}()

	var exporterResponse map[string]string
	if exp := exp.Exporter; exp != nil {
		var immutable cache.ImmutableRef
		if res != nil {
			workerRef, ok := res.Sys().(*worker.WorkerRef)
			if !ok {
				return nil, errors.Errorf("invalid reference: %T", res.Sys())
			}
			immutable = workerRef.ImmutableRef
		}

		if err := j.Call(ctx, exp.Name(), func(ctx context.Context) error {
			exporterResponse, err = exp.Export(ctx, immutable, exporterOpt)
			return err
		}); err != nil {
			return nil, err
		}
	}

	if e := exp.CacheExporter; e != nil {
		if err := j.Call(ctx, "exporting cache", func(ctx context.Context) error {
			prepareDone := oneOffProgress(ctx, "preparing build cache for export")
			if _, err := res.CacheKey().Exporter.ExportTo(ctx, e, solver.CacheExportOpt{
				Convert: workerRefConverter,
				Mode:    exp.CacheExportMode,
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
