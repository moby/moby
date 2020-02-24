package llb

import (
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// DefinitionOp implements llb.Vertex using a marshalled definition.
//
// For example, after marshalling a LLB state and sending over the wire, the
// LLB state can be reconstructed from the definition.
type DefinitionOp struct {
	MarshalCache
	ops       map[digest.Digest]*pb.Op
	defs      map[digest.Digest][]byte
	metas     map[digest.Digest]pb.OpMetadata
	platforms map[digest.Digest]*specs.Platform
	dgst      digest.Digest
	index     pb.OutputIndex
}

// NewDefinitionOp returns a new operation from a marshalled definition.
func NewDefinitionOp(def *pb.Definition) (*DefinitionOp, error) {
	ops := make(map[digest.Digest]*pb.Op)
	defs := make(map[digest.Digest][]byte)

	var dgst digest.Digest
	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return nil, errors.Wrap(err, "failed to parse llb proto op")
		}
		dgst = digest.FromBytes(dt)
		ops[dgst] = &op
		defs[dgst] = dt
	}

	var index pb.OutputIndex
	if dgst != "" {
		index = ops[dgst].Inputs[0].Index
		dgst = ops[dgst].Inputs[0].Digest
	}

	return &DefinitionOp{
		ops:       ops,
		defs:      defs,
		metas:     def.Metadata,
		platforms: make(map[digest.Digest]*specs.Platform),
		dgst:      dgst,
		index:     index,
	}, nil
}

func (d *DefinitionOp) ToInput(c *Constraints) (*pb.Input, error) {
	return d.Output().ToInput(c)
}

func (d *DefinitionOp) Vertex() Vertex {
	return d
}

func (d *DefinitionOp) Validate() error {
	// Scratch state has no digest, ops or metas.
	if d.dgst == "" {
		return nil
	}

	if len(d.ops) == 0 || len(d.defs) == 0 || len(d.metas) == 0 {
		return errors.Errorf("invalid definition op with no ops %d %d", len(d.ops), len(d.metas))
	}

	_, ok := d.ops[d.dgst]
	if !ok {
		return errors.Errorf("invalid definition op with unknown op %q", d.dgst)
	}

	_, ok = d.defs[d.dgst]
	if !ok {
		return errors.Errorf("invalid definition op with unknown def %q", d.dgst)
	}

	_, ok = d.metas[d.dgst]
	if !ok {
		return errors.Errorf("invalid definition op with unknown metas %q", d.dgst)
	}

	// It is possible for d.index >= len(d.ops[d.dgst]) when depending on scratch
	// images.
	if d.index < 0 {
		return errors.Errorf("invalid definition op with invalid index")
	}

	return nil
}

func (d *DefinitionOp) Marshal(c *Constraints) (digest.Digest, []byte, *pb.OpMetadata, error) {
	if d.dgst == "" {
		return "", nil, nil, errors.Errorf("cannot marshal empty definition op")
	}

	if err := d.Validate(); err != nil {
		return "", nil, nil, err
	}

	meta := d.metas[d.dgst]
	return d.dgst, d.defs[d.dgst], &meta, nil

}

func (d *DefinitionOp) Output() Output {
	if d.dgst == "" {
		return nil
	}

	return &output{vertex: d, platform: d.platform(), getIndex: func() (pb.OutputIndex, error) {
		return d.index, nil
	}}
}

func (d *DefinitionOp) Inputs() []Output {
	if d.dgst == "" {
		return nil
	}

	var inputs []Output

	op := d.ops[d.dgst]
	for _, input := range op.Inputs {
		vtx := &DefinitionOp{
			ops:       d.ops,
			defs:      d.defs,
			metas:     d.metas,
			platforms: d.platforms,
			dgst:      input.Digest,
			index:     input.Index,
		}
		inputs = append(inputs, &output{vertex: vtx, platform: d.platform(), getIndex: func() (pb.OutputIndex, error) {
			return pb.OutputIndex(vtx.index), nil
		}})
	}

	return inputs
}

func (d *DefinitionOp) platform() *specs.Platform {
	platform, ok := d.platforms[d.dgst]
	if ok {
		return platform
	}

	op := d.ops[d.dgst]
	if op.Platform != nil {
		spec := op.Platform.Spec()
		platform = &spec
	}

	d.platforms[d.dgst] = platform
	return platform
}
