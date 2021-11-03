package llb

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/flightcontrol"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type asyncState struct {
	f      func(context.Context, State, *Constraints) (State, error)
	prev   State
	target State
	set    bool
	err    error
	g      flightcontrol.Group
}

func (as *asyncState) Output() Output {
	return as
}

func (as *asyncState) Vertex(ctx context.Context, c *Constraints) Vertex {
	err := as.Do(ctx, c)
	if err != nil {
		return &errVertex{err}
	}
	if as.set {
		out := as.target.Output()
		if out == nil {
			return nil
		}
		return out.Vertex(ctx, c)
	}
	return nil
}

func (as *asyncState) ToInput(ctx context.Context, c *Constraints) (*pb.Input, error) {
	err := as.Do(ctx, c)
	if err != nil {
		return nil, err
	}
	if as.set {
		out := as.target.Output()
		if out == nil {
			return nil, nil
		}
		return out.ToInput(ctx, c)
	}
	return nil, nil
}

func (as *asyncState) Do(ctx context.Context, c *Constraints) error {
	_, err := as.g.Do(ctx, "", func(ctx context.Context) (interface{}, error) {
		if as.set {
			return as.target, as.err
		}
		res, err := as.f(ctx, as.prev, c)
		if err != nil {
			select {
			case <-ctx.Done():
				if errors.Is(err, ctx.Err()) {
					return res, err
				}
			default:
			}
		}
		as.target = res
		as.err = err
		as.set = true
		return res, err
	})
	if err != nil {
		return err
	}
	return as.err
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
