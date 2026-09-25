package llb

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

type DiffOp struct {
	cache       MarshalCache
	lower       Output
	upper       Output
	output      Output
	constraints Constraints
}

func NewDiff(lower, upper State, c Constraints) *DiffOp {
	addCap(&c, pb.CapDiffOp)
	op := &DiffOp{
		lower:       lower.Output(),
		upper:       upper.Output(),
		constraints: c,
	}
	op.output = &output{vertex: op}
	return op
}

func (m *DiffOp) Validate(ctx context.Context, constraints *Constraints) error {
	return nil
}

func (m *DiffOp) Marshal(ctx context.Context, constraints *Constraints) (digest.Digest, []byte, *pb.OpMetadata, []*SourceLocation, error) {
	cache := m.cache.Acquire()
	defer cache.Release()

	if dgst, dt, md, srcs, err := cache.Load(constraints); err == nil {
		return dgst, dt, md, srcs, nil
	}
	if err := m.Validate(ctx, constraints); err != nil {
		return "", nil, nil, nil, err
	}

	proto, md := MarshalConstraints(constraints, &m.constraints)
	proto.Platform = nil // diff op is not platform specific

	op := &pb.DiffOp{}

	op.Lower = &pb.LowerDiffInput{Input: int64(len(proto.Inputs))}
	if m.lower == nil {
		op.Lower.Input = int64(pb.Empty)
	} else {
		pbLowerInput, err := m.lower.ToInput(ctx, constraints)
		if err != nil {
			return "", nil, nil, nil, err
		}
		proto.Inputs = append(proto.Inputs, pbLowerInput)
	}

	op.Upper = &pb.UpperDiffInput{Input: int64(len(proto.Inputs))}
	if m.upper == nil {
		op.Upper.Input = int64(pb.Empty)
	} else {
		pbUpperInput, err := m.upper.ToInput(ctx, constraints)
		if err != nil {
			return "", nil, nil, nil, err
		}
		proto.Inputs = append(proto.Inputs, pbUpperInput)
	}

	proto.Op = &pb.Op_Diff{Diff: op}

	dt, err := deterministicMarshal(proto)
	if err != nil {
		return "", nil, nil, nil, err
	}

	return cache.Store(dt, md, m.constraints.SourceLocations, constraints)
}

func (m *DiffOp) Output() Output {
	return m.output
}

func (m *DiffOp) Inputs() (out []Output) {
	if m.lower != nil {
		out = append(out, m.lower)
	}
	if m.upper != nil {
		out = append(out, m.upper)
	}
	return out
}

// Diff returns a state that represents the diff of the lower and upper states.
// The returned State is useful for use with [Merge] where you can merge the lower state with the diff.
func Diff(lower, upper State, opts ...ConstraintsOpt) State {
	if lower.Output() == nil {
		if upper.Output() == nil {
			// diff of scratch and scratch is scratch
			return Scratch()
		}
		// diff of scratch and upper is just upper
		return upper
	}

	var c Constraints
	for _, o := range opts {
		o.SetConstraintsOption(&c)
	}
	return lower.WithOutput(NewDiff(lower, upper, c).Output())
}
