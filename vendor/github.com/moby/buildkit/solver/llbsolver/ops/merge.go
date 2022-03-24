package ops

import (
	"context"
	"encoding/json"

	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

const mergeCacheType = "buildkit.merge.v0"

type mergeOp struct {
	op     *pb.MergeOp
	worker worker.Worker
	vtx    solver.Vertex
}

func NewMergeOp(v solver.Vertex, op *pb.Op_Merge, w worker.Worker) (solver.Op, error) {
	if err := llbsolver.ValidateOp(&pb.Op{Op: op}); err != nil {
		return nil, err
	}
	return &mergeOp{
		op:     op.Merge,
		worker: w,
		vtx:    v,
	}, nil
}

func (m *mergeOp) CacheMap(ctx context.Context, group session.Group, index int) (*solver.CacheMap, bool, error) {
	dt, err := json.Marshal(struct {
		Type  string
		Merge *pb.MergeOp
	}{
		Type:  mergeCacheType,
		Merge: m.op,
	})
	if err != nil {
		return nil, false, err
	}

	cm := &solver.CacheMap{
		Digest: digest.Digest(dt),
		Deps: make([]struct {
			Selector          digest.Digest
			ComputeDigestFunc solver.ResultBasedCacheFunc
			PreprocessFunc    solver.PreprocessFunc
		}, len(m.op.Inputs)),
	}

	return cm, true, nil
}

func (m *mergeOp) Exec(ctx context.Context, g session.Group, inputs []solver.Result) ([]solver.Result, error) {
	refs := make([]cache.ImmutableRef, len(inputs))
	var index int
	for _, inp := range inputs {
		if inp == nil {
			continue
		}
		wref, ok := inp.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, errors.Errorf("invalid reference for merge %T", inp.Sys())
		}
		if wref.ImmutableRef == nil {
			continue
		}
		refs[index] = wref.ImmutableRef
		index++
	}
	refs = refs[:index]

	if len(refs) == 0 {
		return nil, nil
	}

	mergedRef, err := m.worker.CacheManager().Merge(ctx, refs, solver.ProgressControllerFromContext(ctx),
		cache.WithDescription(m.vtx.Name()))
	if err != nil {
		return nil, err
	}

	return []solver.Result{worker.NewWorkerRefResult(mergedRef, m.worker)}, nil
}

func (m *mergeOp) Acquire(ctx context.Context) (release solver.ReleaseFunc, err error) {
	return func() {}, nil
}
