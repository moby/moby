package dockerfile2llb

import (
	"context"

	"github.com/moby/buildkit/frontend/subrequests/convertllb"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

func (ds *dispatchState) ConvertLLB(ctx context.Context) (*convertllb.Result, error) {
	def, err := ds.state.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	res := &convertllb.Result{
		Def:      make(map[digest.Digest]*pb.Op, len(def.Def)),
		Metadata: def.Metadata,
		Source:   def.Source,
	}
	for _, dt := range def.Def {
		var op pb.Op
		if err := op.UnmarshalVT(dt); err != nil {
			return nil, err
		}

		dgst := digest.FromBytes(dt)
		res.Def[dgst] = &op
	}
	return res, nil
}
