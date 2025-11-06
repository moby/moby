package llbsolver

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
)

type SourcePolicyEvaluator interface {
	Evaluate(ctx context.Context, op *pb.Op) (bool, error)
}
