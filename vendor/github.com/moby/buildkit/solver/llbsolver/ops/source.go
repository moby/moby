package ops

import (
	"context"
	"sync"

	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
)

const sourceCacheType = "buildkit.source.v0"

type sourceOp struct {
	mu  sync.Mutex
	op  *pb.Op_Source
	sm  *source.Manager
	src source.SourceInstance
	w   worker.Worker
}

func NewSourceOp(_ solver.Vertex, op *pb.Op_Source, sm *source.Manager, w worker.Worker) (solver.Op, error) {
	return &sourceOp{
		op: op,
		sm: sm,
		w:  w,
	}, nil
}

func (s *sourceOp) instance(ctx context.Context) (source.SourceInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.src != nil {
		return s.src, nil
	}
	id, err := source.FromLLB(s.op)
	if err != nil {
		return nil, err
	}
	src, err := s.sm.Resolve(ctx, id)
	if err != nil {
		return nil, err
	}
	s.src = src
	return s.src, nil
}

func (s *sourceOp) CacheMap(ctx context.Context, index int) (*solver.CacheMap, bool, error) {
	src, err := s.instance(ctx)
	if err != nil {
		return nil, false, err
	}
	k, done, err := src.CacheKey(ctx, index)
	if err != nil {
		return nil, false, err
	}

	return &solver.CacheMap{
		// TODO: add os/arch
		Digest: digest.FromBytes([]byte(sourceCacheType + ":" + k)),
	}, done, nil
}

func (s *sourceOp) Exec(ctx context.Context, _ []solver.Result) (outputs []solver.Result, err error) {
	src, err := s.instance(ctx)
	if err != nil {
		return nil, err
	}
	ref, err := src.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return []solver.Result{worker.NewWorkerRefResult(ref, s.w)}, nil
}
