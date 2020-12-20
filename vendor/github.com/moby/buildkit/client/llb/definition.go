package llb

import (
	"context"
	"sync"

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
	mu         sync.Mutex
	ops        map[digest.Digest]*pb.Op
	defs       map[digest.Digest][]byte
	metas      map[digest.Digest]pb.OpMetadata
	sources    map[digest.Digest][]*SourceLocation
	platforms  map[digest.Digest]*specs.Platform
	dgst       digest.Digest
	index      pb.OutputIndex
	inputCache map[digest.Digest][]*DefinitionOp
}

// NewDefinitionOp returns a new operation from a marshalled definition.
func NewDefinitionOp(def *pb.Definition) (*DefinitionOp, error) {
	ops := make(map[digest.Digest]*pb.Op)
	defs := make(map[digest.Digest][]byte)
	platforms := make(map[digest.Digest]*specs.Platform)

	var dgst digest.Digest
	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return nil, errors.Wrap(err, "failed to parse llb proto op")
		}
		dgst = digest.FromBytes(dt)
		ops[dgst] = &op
		defs[dgst] = dt

		var platform *specs.Platform
		if op.Platform != nil {
			spec := op.Platform.Spec()
			platform = &spec
		}
		platforms[dgst] = platform
	}

	srcs := map[digest.Digest][]*SourceLocation{}

	if def.Source != nil {
		sourceMaps := make([]*SourceMap, len(def.Source.Infos))
		for i, info := range def.Source.Infos {
			var st *State
			sdef := info.Definition
			if sdef != nil {
				op, err := NewDefinitionOp(sdef)
				if err != nil {
					return nil, err
				}
				state := NewState(op)
				st = &state
			}
			sourceMaps[i] = NewSourceMap(st, info.Filename, info.Data)
		}

		for dgst, locs := range def.Source.Locations {
			for _, loc := range locs.Locations {
				if loc.SourceIndex < 0 || int(loc.SourceIndex) >= len(sourceMaps) {
					return nil, errors.Errorf("failed to find source map with index %d", loc.SourceIndex)
				}

				srcs[digest.Digest(dgst)] = append(srcs[digest.Digest(dgst)], &SourceLocation{
					SourceMap: sourceMaps[int(loc.SourceIndex)],
					Ranges:    loc.Ranges,
				})
			}
		}
	}

	var index pb.OutputIndex
	if dgst != "" {
		index = ops[dgst].Inputs[0].Index
		dgst = ops[dgst].Inputs[0].Digest
	}

	return &DefinitionOp{
		ops:        ops,
		defs:       defs,
		metas:      def.Metadata,
		sources:    srcs,
		platforms:  platforms,
		dgst:       dgst,
		index:      index,
		inputCache: make(map[digest.Digest][]*DefinitionOp),
	}, nil
}

func (d *DefinitionOp) ToInput(ctx context.Context, c *Constraints) (*pb.Input, error) {
	return d.Output().ToInput(ctx, c)
}

func (d *DefinitionOp) Vertex(context.Context) Vertex {
	return d
}

func (d *DefinitionOp) Validate(context.Context) error {
	// Scratch state has no digest, ops or metas.
	if d.dgst == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

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

func (d *DefinitionOp) Marshal(ctx context.Context, c *Constraints) (digest.Digest, []byte, *pb.OpMetadata, []*SourceLocation, error) {
	if d.dgst == "" {
		return "", nil, nil, nil, errors.Errorf("cannot marshal empty definition op")
	}

	if err := d.Validate(ctx); err != nil {
		return "", nil, nil, nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	meta := d.metas[d.dgst]
	return d.dgst, d.defs[d.dgst], &meta, d.sources[d.dgst], nil

}

func (d *DefinitionOp) Output() Output {
	if d.dgst == "" {
		return nil
	}

	d.mu.Lock()
	platform := d.platforms[d.dgst]
	d.mu.Unlock()

	return &output{vertex: d, platform: platform, getIndex: func() (pb.OutputIndex, error) {
		return d.index, nil
	}}
}

func (d *DefinitionOp) Inputs() []Output {
	if d.dgst == "" {
		return nil
	}

	var inputs []Output

	d.mu.Lock()
	op := d.ops[d.dgst]
	platform := d.platforms[d.dgst]
	d.mu.Unlock()

	for _, input := range op.Inputs {
		var vtx *DefinitionOp
		d.mu.Lock()
		if existingIndexes, ok := d.inputCache[input.Digest]; ok {
			if int(input.Index) < len(existingIndexes) && existingIndexes[input.Index] != nil {
				vtx = existingIndexes[input.Index]
			}
		}
		if vtx == nil {
			vtx = &DefinitionOp{
				ops:        d.ops,
				defs:       d.defs,
				metas:      d.metas,
				platforms:  d.platforms,
				dgst:       input.Digest,
				index:      input.Index,
				inputCache: d.inputCache,
			}
			existingIndexes := d.inputCache[input.Digest]
			indexDiff := int(input.Index) - len(existingIndexes)
			if indexDiff >= 0 {
				// make room in the slice for the new index being set
				existingIndexes = append(existingIndexes, make([]*DefinitionOp, indexDiff+1)...)
			}
			existingIndexes[input.Index] = vtx
			d.inputCache[input.Digest] = existingIndexes
		}
		d.mu.Unlock()

		inputs = append(inputs, &output{vertex: vtx, platform: platform, getIndex: func() (pb.OutputIndex, error) {
			return pb.OutputIndex(vtx.index), nil
		}})
	}

	return inputs
}
