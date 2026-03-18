package llb

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type MergeOp struct {
	cache       MarshalCache
	inputs      []Output
	output      Output
	constraints Constraints
}

func NewMerge(inputs []State, c Constraints) *MergeOp {
	op := &MergeOp{constraints: c}
	for _, input := range inputs {
		op.inputs = append(op.inputs, input.Output())
	}
	op.output = &output{vertex: op}
	return op
}

func (m *MergeOp) Validate(ctx context.Context, constraints *Constraints) error {
	if len(m.inputs) < 2 {
		return errors.Errorf("merge must have at least 2 inputs")
	}
	return nil
}

func (m *MergeOp) Marshal(ctx context.Context, constraints *Constraints) (digest.Digest, []byte, *pb.OpMetadata, []*SourceLocation, error) {
	cache := m.cache.Acquire()
	defer cache.Release()

	if dgst, dt, md, srcs, err := cache.Load(constraints); err == nil {
		return dgst, dt, md, srcs, nil
	}

	if err := m.Validate(ctx, constraints); err != nil {
		return "", nil, nil, nil, err
	}

	pop, md := MarshalConstraints(constraints, &m.constraints)
	pop.Platform = nil // merge op is not platform specific

	op := &pb.MergeOp{}
	for _, input := range m.inputs {
		op.Inputs = append(op.Inputs, &pb.MergeInput{Input: int64(len(pop.Inputs))})
		pbInput, err := input.ToInput(ctx, constraints)
		if err != nil {
			return "", nil, nil, nil, err
		}
		pop.Inputs = append(pop.Inputs, pbInput)
	}
	pop.Op = &pb.Op_Merge{Merge: op}

	dt, err := deterministicMarshal(pop)
	if err != nil {
		return "", nil, nil, nil, err
	}

	return cache.Store(dt, md, m.constraints.SourceLocations, constraints)
}

func (m *MergeOp) Output() Output {
	return m.output
}

func (m *MergeOp) Inputs() []Output {
	return m.inputs
}

// Merge merges multiple states into a single state. This is useful in
// conjunction with [Diff] to create set of patches which are independent of
// each other to a base state without affecting the cache of other merged
// states.
// As an example, lets say you have a rootfs with the following directories:
//
//	/ /bin /etc /opt /tmp
//
// Now lets say you want to copy a directory /etc/foo from one state and a
// binary /bin/bar from another state.
// [Copy] makes a duplicate of file on top of another directory.
// Merge creates a directory whose contents is an overlay of 2 states on top of each other.
//
// With "Merge" you can do this:
//
//	fooState := Diff(rootfs, fooState)
//	barState := Diff(rootfs, barState)
//
// Then merge the results with:
//
//	Merge(rootfs, fooDiff, barDiff)
//
// The resulting state will have both /etc/foo and /bin/bar, but because Merge
// was used, changing the contents of "fooDiff" does not require copying
// "barDiff" again.
func Merge(inputs []State, opts ...ConstraintsOpt) State {
	// filter out any scratch inputs, which have no effect when merged
	var filteredInputs []State
	for _, input := range inputs {
		if input.Output() != nil {
			filteredInputs = append(filteredInputs, input)
		}
	}
	if len(filteredInputs) == 0 {
		// a merge of only scratch results in scratch
		return Scratch()
	}
	if len(filteredInputs) == 1 {
		// a merge of a single non-empty input results in that non-empty input
		return filteredInputs[0]
	}

	var c Constraints
	for _, o := range opts {
		o.SetConstraintsOption(&c)
	}
	addCap(&c, pb.CapMergeOp)
	return filteredInputs[0].WithOutput(NewMerge(filteredInputs, c).Output())
}
