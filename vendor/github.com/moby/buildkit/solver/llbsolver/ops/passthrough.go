package ops

import (
	"context"
	"encoding/json"

	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/ops/opsutils"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/cachedigest"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const passthroughCacheType = "buildkit.passthrough.v0"

type passthroughOp struct {
	op         *pb.PassthroughOp
	inputCount int
}

func NewPassthroughOp(v solver.Vertex, op *pb.Op_Passthrough) (solver.Op, error) {
	if err := opsutils.Validate(&pb.Op{Op: op}); err != nil {
		return nil, err
	}
	return &passthroughOp{op: op.Passthrough, inputCount: len(v.Inputs())}, nil
}

func (p *passthroughOp) CacheMap(context.Context, solver.JobContext, int) (*solver.CacheMap, bool, error) {
	dt, err := json.Marshal(struct {
		Type        string
		Passthrough *pb.PassthroughOp
	}{
		Type:        passthroughCacheType,
		Passthrough: p.op,
	})
	if err != nil {
		return nil, false, err
	}

	dgst, err := cachedigest.FromBytes(dt, cachedigest.TypeJSON)
	if err != nil {
		return nil, false, err
	}

	cm := &solver.CacheMap{
		Digest: dgst,
		Deps: make([]struct {
			Selector          digest.Digest
			ComputeDigestFunc solver.ResultBasedCacheFunc
			PreprocessFunc    solver.PreprocessFunc
		}, p.inputCount),
	}

	return cm, true, nil
}

func (p *passthroughOp) Exec(ctx context.Context, jobCtx solver.JobContext, inputs []solver.Result) ([]solver.Result, error) {
	outputs := make([]solver.Result, len(p.op.Outputs))
	for i, inputIndex := range p.op.Outputs {
		if inputIndex < 0 || inputIndex >= int64(len(inputs)) {
			return nil, errors.Errorf("invalid passthrough output input index %d", inputIndex)
		}
		inp := inputs[inputIndex]
		if inp != nil {
			outputs[i] = inp.Clone()
		}
	}
	return outputs, nil
}

func (p *passthroughOp) Acquire(ctx context.Context) (solver.ReleaseFunc, error) {
	return func() {}, nil
}
