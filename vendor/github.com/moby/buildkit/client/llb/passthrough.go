package llb

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type PassthroughInput struct {
	State  State
	Output bool
}

type PassthroughOp struct {
	cache       MarshalCache
	id          string
	inputs      []Output
	outputs     []Output
	outputMap   []int
	constraints Constraints
}

func NewPassthroughOp(id string, inputs []PassthroughInput, opts ...ConstraintsOpt) *PassthroughOp {
	var c Constraints
	for _, o := range opts {
		o.SetConstraintsOption(&c)
	}
	addCap(&c, pb.CapPassthroughOp)

	op := &PassthroughOp{id: id, constraints: c}
	for _, input := range inputs {
		out := input.State.Output()
		if out == nil {
			continue
		}
		inputIndex := len(op.inputs)
		op.inputs = append(op.inputs, out)
		if input.Output {
			outputIndex := len(op.outputMap)
			op.outputMap = append(op.outputMap, inputIndex)
			op.outputs = append(op.outputs, &output{vertex: op, getIndex: func() (pb.OutputIndex, error) {
				return pb.OutputIndex(outputIndex), nil
			}})
		}
	}
	return op
}

func NewPassthrough(id string, inputs []PassthroughInput, opts ...ConstraintsOpt) State {
	op := NewPassthroughOp(id, inputs, opts...)
	return NewState(op.Output())
}

func (p *PassthroughOp) Validate(context.Context, *Constraints) error {
	if len(p.inputs) == 0 {
		return errors.Errorf("passthrough must have at least one input")
	}
	if p.id == "" {
		return errors.Errorf("passthrough requires an id")
	}
	if len(p.outputMap) == 0 {
		return errors.Errorf("passthrough must have at least one output")
	}
	for _, input := range p.outputMap {
		if input < 0 || input >= len(p.inputs) {
			return errors.Errorf("passthrough output references invalid input %d", input)
		}
	}
	return nil
}

func (p *PassthroughOp) Marshal(ctx context.Context, constraints *Constraints) (digest.Digest, []byte, *pb.OpMetadata, []*SourceLocation, error) {
	cache := p.cache.Acquire()
	defer cache.Release()

	if dgst, dt, md, srcs, err := cache.Load(constraints); err == nil {
		return dgst, dt, md, srcs, nil
	}

	if err := p.Validate(ctx, constraints); err != nil {
		return "", nil, nil, nil, err
	}

	pop, md := MarshalConstraints(constraints, &p.constraints)
	pop.Platform = nil

	op := &pb.PassthroughOp{Id: p.id}
	for _, input := range p.inputs {
		pbInput, err := input.ToInput(ctx, constraints)
		if err != nil {
			return "", nil, nil, nil, err
		}
		pop.Inputs = append(pop.Inputs, pbInput)
	}
	for _, input := range p.outputMap {
		op.Outputs = append(op.Outputs, int64(input))
	}
	pop.Op = &pb.Op_Passthrough{Passthrough: op}

	dt, err := deterministicMarshal(pop)
	if err != nil {
		return "", nil, nil, nil, err
	}

	return cache.Store(dt, md, p.constraints.SourceLocations, constraints)
}

func (p *PassthroughOp) Output() Output {
	return p.OutputAt(0)
}

func (p *PassthroughOp) OutputAt(index int) Output {
	if index < 0 || index >= len(p.outputs) {
		return &output{vertex: p, err: errors.Errorf("invalid passthrough output %d", index)}
	}
	return p.outputs[index]
}

func (p *PassthroughOp) Inputs() []Output {
	return p.inputs
}
