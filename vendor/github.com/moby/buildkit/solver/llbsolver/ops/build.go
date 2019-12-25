package ops

import (
	"context"
	"encoding/json"
	"os"

	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const buildCacheType = "buildkit.build.v0"

type buildOp struct {
	op *pb.BuildOp
	b  frontend.FrontendLLBBridge
	v  solver.Vertex
}

func NewBuildOp(v solver.Vertex, op *pb.Op_Build, b frontend.FrontendLLBBridge, _ worker.Worker) (solver.Op, error) {
	if err := llbsolver.ValidateOp(&pb.Op{Op: op}); err != nil {
		return nil, err
	}
	return &buildOp{
		op: op.Build,
		b:  b,
		v:  v,
	}, nil
}

func (b *buildOp) CacheMap(ctx context.Context, index int) (*solver.CacheMap, bool, error) {
	dt, err := json.Marshal(struct {
		Type string
		Exec *pb.BuildOp
	}{
		Type: buildCacheType,
		Exec: b.op,
	})
	if err != nil {
		return nil, false, err
	}

	return &solver.CacheMap{
		Digest: digest.FromBytes(dt),
		Deps: make([]struct {
			Selector          digest.Digest
			ComputeDigestFunc solver.ResultBasedCacheFunc
		}, len(b.v.Inputs())),
	}, true, nil
}

func (b *buildOp) Exec(ctx context.Context, inputs []solver.Result) (outputs []solver.Result, retErr error) {
	if b.op.Builder != pb.LLBBuilder {
		return nil, errors.Errorf("only LLB builder is currently allowed")
	}

	builderInputs := b.op.Inputs
	llbDef, ok := builderInputs[pb.LLBDefinitionInput]
	if !ok {
		return nil, errors.Errorf("no llb definition input %s found", pb.LLBDefinitionInput)
	}

	i := int(llbDef.Input)
	if i >= len(inputs) {
		return nil, errors.Errorf("invalid index %v", i) // TODO: this should be validated before
	}
	inp := inputs[i]

	ref, ok := inp.Sys().(*worker.WorkerRef)
	if !ok {
		return nil, errors.Errorf("invalid reference for build %T", inp.Sys())
	}

	mount, err := ref.ImmutableRef.Mount(ctx, true)
	if err != nil {
		return nil, err
	}

	lm := snapshot.LocalMounter(mount)

	root, err := lm.Mount()
	if err != nil {
		return nil, err
	}

	defer func() {
		if retErr != nil && lm != nil {
			lm.Unmount()
		}
	}()

	fn := pb.LLBDefaultDefinitionFile
	if override, ok := b.op.Attrs[pb.AttrLLBDefinitionFilename]; ok {
		fn = override
	}

	newfn, err := fs.RootPath(root, fn)
	if err != nil {
		return nil, errors.Wrapf(err, "working dir %s points to invalid target", fn)
	}

	f, err := os.Open(newfn)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", newfn)
	}

	def, err := llb.ReadFrom(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	lm.Unmount()
	lm = nil

	newRes, err := b.b.Solve(ctx, frontend.SolveRequest{
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	for _, r := range newRes.Refs {
		r.Release(context.TODO())
	}

	return []solver.Result{newRes.Ref}, err
}
