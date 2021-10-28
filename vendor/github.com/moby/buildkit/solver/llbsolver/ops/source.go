package ops

import (
	"context"
	"strings"
	"sync"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"golang.org/x/sync/semaphore"
)

const sourceCacheType = "buildkit.source.v0"

type sourceOp struct {
	mu          sync.Mutex
	op          *pb.Op_Source
	platform    *pb.Platform
	sm          *source.Manager
	src         source.SourceInstance
	sessM       *session.Manager
	w           worker.Worker
	vtx         solver.Vertex
	parallelism *semaphore.Weighted
}

func NewSourceOp(vtx solver.Vertex, op *pb.Op_Source, platform *pb.Platform, sm *source.Manager, parallelism *semaphore.Weighted, sessM *session.Manager, w worker.Worker) (solver.Op, error) {
	if err := llbsolver.ValidateOp(&pb.Op{Op: op}); err != nil {
		return nil, err
	}
	return &sourceOp{
		op:          op,
		sm:          sm,
		w:           w,
		sessM:       sessM,
		platform:    platform,
		vtx:         vtx,
		parallelism: parallelism,
	}, nil
}

func (s *sourceOp) instance(ctx context.Context) (source.SourceInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.src != nil {
		return s.src, nil
	}
	id, err := source.FromLLB(s.op, s.platform)
	if err != nil {
		return nil, err
	}
	src, err := s.sm.Resolve(ctx, id, s.sessM, s.vtx)
	if err != nil {
		return nil, err
	}
	s.src = src
	return s.src, nil
}

func (s *sourceOp) CacheMap(ctx context.Context, g session.Group, index int) (*solver.CacheMap, bool, error) {
	src, err := s.instance(ctx)
	if err != nil {
		return nil, false, err
	}

	k, pin, cacheOpts, done, err := src.CacheKey(ctx, g, index)
	if err != nil {
		return nil, false, err
	}

	dgst := digest.FromBytes([]byte(sourceCacheType + ":" + k))
	if strings.HasPrefix(k, "session:") {
		dgst = digest.Digest("random:" + strings.TrimPrefix(dgst.String(), dgst.Algorithm().String()+":"))
	}

	var buildInfo map[string]string
	if !strings.HasPrefix(s.op.Source.GetIdentifier(), "local://") {
		buildInfo = map[string]string{s.op.Source.GetIdentifier(): pin}
	}

	return &solver.CacheMap{
		// TODO: add os/arch
		Digest:    dgst,
		Opts:      cacheOpts,
		BuildInfo: buildInfo,
	}, done, nil
}

func (s *sourceOp) Exec(ctx context.Context, g session.Group, _ []solver.Result) (outputs []solver.Result, err error) {
	src, err := s.instance(ctx)
	if err != nil {
		return nil, err
	}
	ref, err := src.Snapshot(ctx, g)
	if err != nil {
		return nil, err
	}
	return []solver.Result{worker.NewWorkerRefResult(ref, s.w)}, nil
}

func (s *sourceOp) Acquire(ctx context.Context) (solver.ReleaseFunc, error) {
	if s.parallelism == nil {
		return func() {}, nil
	}
	err := s.parallelism.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	return func() {
		s.parallelism.Release(1)
	}, nil
}
