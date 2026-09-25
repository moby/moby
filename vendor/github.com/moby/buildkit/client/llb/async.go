package llb

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/flightcontrol"
	digest "github.com/opencontainers/go-digest"
)

type asyncState struct {
	f    func(context.Context, State, *Constraints) (State, error)
	prev State
	g    flightcontrol.CachedGroup[State]
}

func (as *asyncState) Output() Output {
	return as
}

func (as *asyncState) Vertex(ctx context.Context, c *Constraints) Vertex {
	target, err := as.Do(ctx, c)
	if err != nil {
		return &errVertex{err}
	}
	out := target.Output()
	if out == nil {
		return nil
	}
	return out.Vertex(ctx, c)
}

func (as *asyncState) ToInput(ctx context.Context, c *Constraints) (*pb.Input, error) {
	target, err := as.Do(ctx, c)
	if err != nil {
		return nil, err
	}
	out := target.Output()
	if out == nil {
		return nil, nil
	}
	return out.ToInput(ctx, c)
}

func (as *asyncState) Do(ctx context.Context, c *Constraints) (State, error) {
	return as.g.Do(ctx, "", func(ctx context.Context) (State, error) {
		return as.f(ctx, as.prev, c)
	})
}

type errVertex struct {
	err error
}

func (v *errVertex) Validate(context.Context, *Constraints) error {
	return v.err
}
func (v *errVertex) Marshal(context.Context, *Constraints) (digest.Digest, []byte, *pb.OpMetadata, []*SourceLocation, error) {
	return "", nil, nil, nil, v.err
}
func (v *errVertex) Output() Output {
	return nil
}
func (v *errVertex) Inputs() []Output {
	return nil
}

var _ Vertex = &errVertex{}
