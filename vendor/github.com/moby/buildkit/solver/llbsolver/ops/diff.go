package ops

import (
	"context"
	"encoding/json"

	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/progress/controller"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

const diffCacheType = "buildkit.diff.v0"

type diffOp struct {
	op     *pb.DiffOp
	worker worker.Worker
	vtx    solver.Vertex
	pg     progress.Controller
}

func NewDiffOp(v solver.Vertex, op *pb.Op_Diff, w worker.Worker) (solver.Op, error) {
	if err := llbsolver.ValidateOp(&pb.Op{Op: op}); err != nil {
		return nil, err
	}
	return &diffOp{
		op:     op.Diff,
		worker: w,
		vtx:    v,
	}, nil
}

func (d *diffOp) CacheMap(ctx context.Context, group session.Group, index int) (*solver.CacheMap, bool, error) {
	dt, err := json.Marshal(struct {
		Type string
		Diff *pb.DiffOp
	}{
		Type: diffCacheType,
		Diff: d.op,
	})
	if err != nil {
		return nil, false, err
	}

	var depCount int
	if d.op.Lower.Input != pb.Empty {
		depCount++
	}
	if d.op.Upper.Input != pb.Empty {
		depCount++
	}

	cm := &solver.CacheMap{
		Digest: digest.Digest(dt),
		Deps: make([]struct {
			Selector          digest.Digest
			ComputeDigestFunc solver.ResultBasedCacheFunc
			PreprocessFunc    solver.PreprocessFunc
		}, depCount),
		Opts: solver.CacheOpts(make(map[interface{}]interface{})),
	}

	d.pg = &controller.Controller{
		WriterFactory: progress.FromContext(ctx),
		Digest:        d.vtx.Digest(),
		Name:          d.vtx.Name(),
		ProgressGroup: d.vtx.Options().ProgressGroup,
	}
	cm.Opts[cache.ProgressKey{}] = d.pg

	return cm, true, nil
}

func (d *diffOp) Exec(ctx context.Context, g session.Group, inputs []solver.Result) ([]solver.Result, error) {
	var curInput int

	var lowerRef cache.ImmutableRef
	if d.op.Lower.Input != pb.Empty {
		if lowerInp := inputs[curInput]; lowerInp != nil {
			wref, ok := lowerInp.Sys().(*worker.WorkerRef)
			if !ok {
				return nil, errors.Errorf("invalid lower reference for diff op %T", lowerInp.Sys())
			}
			lowerRef = wref.ImmutableRef
		} else {
			return nil, errors.New("invalid nil lower input for diff op")
		}
		curInput++
	}

	var upperRef cache.ImmutableRef
	if d.op.Upper.Input != pb.Empty {
		if upperInp := inputs[curInput]; upperInp != nil {
			wref, ok := upperInp.Sys().(*worker.WorkerRef)
			if !ok {
				return nil, errors.Errorf("invalid upper reference for diff op %T", upperInp.Sys())
			}
			upperRef = wref.ImmutableRef
		} else {
			return nil, errors.New("invalid nil upper input for diff op")
		}
	}

	if lowerRef == nil {
		if upperRef == nil {
			// The diff of nothing and nothing is nothing. Just return an empty ref.
			return []solver.Result{worker.NewWorkerRefResult(nil, d.worker)}, nil
		}
		// The diff of nothing and upper is upper. Just return a clone of upper
		return []solver.Result{worker.NewWorkerRefResult(upperRef.Clone(), d.worker)}, nil
	}
	if upperRef != nil && lowerRef.ID() == upperRef.ID() {
		// The diff of a ref and itself is nothing, return an empty ref.
		return []solver.Result{worker.NewWorkerRefResult(nil, d.worker)}, nil
	}

	diffRef, err := d.worker.CacheManager().Diff(ctx, lowerRef, upperRef, d.pg,
		cache.WithDescription(d.vtx.Name()))
	if err != nil {
		return nil, err
	}

	return []solver.Result{worker.NewWorkerRefResult(diffRef, d.worker)}, nil
}

func (d *diffOp) Acquire(ctx context.Context) (release solver.ReleaseFunc, err error) {
	return func() {}, nil
}
