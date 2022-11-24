package provenance

import (
	"context"
	"fmt"

	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type BuildConfig struct {
	Definition []BuildStep `json:"llbDefinition,omitempty"`
}

type BuildStep struct {
	ID     string      `json:"id,omitempty"`
	Op     interface{} `json:"op,omitempty"`
	Inputs []string    `json:"inputs,omitempty"`
}

type Source struct {
	Locations map[string]*pb.Locations `json:"locations,omitempty"`
	Infos     []SourceInfo             `json:"infos,omitempty"`
}

type SourceInfo struct {
	Filename   string      `json:"filename,omitempty"`
	Data       []byte      `json:"data,omitempty"`
	Definition []BuildStep `json:"llbDefinition,omitempty"`
}

func AddBuildConfig(ctx context.Context, p *ProvenancePredicate, rp solver.ResultProxy) (map[digest.Digest]int, error) {
	def := rp.Definition()
	steps, indexes, err := toBuildSteps(def)
	if err != nil {
		return nil, err
	}

	bc := &BuildConfig{
		Definition: steps,
	}

	p.BuildConfig = bc

	if def.Source != nil {
		sis := make([]SourceInfo, len(def.Source.Infos))
		for i, si := range def.Source.Infos {
			steps, _, err := toBuildSteps(si.Definition)
			if err != nil {
				return nil, err
			}
			s := SourceInfo{
				Filename:   si.Filename,
				Data:       si.Data,
				Definition: steps,
			}
			sis[i] = s
		}

		if len(def.Source.Infos) != 0 {
			locs := map[string]*pb.Locations{}
			for k, l := range def.Source.Locations {
				idx, ok := indexes[digest.Digest(k)]
				if !ok {
					continue
				}
				locs[fmt.Sprintf("step%d", idx)] = l
			}

			if p.Metadata == nil {
				p.Metadata = &ProvenanceMetadata{}
			}
			p.Metadata.BuildKitMetadata.Source = &Source{
				Infos:     sis,
				Locations: locs,
			}
		}
	}

	return indexes, nil
}

func toBuildSteps(def *pb.Definition) ([]BuildStep, map[digest.Digest]int, error) {
	if def == nil || len(def.Def) == 0 {
		return nil, nil, nil
	}

	ops := make(map[digest.Digest]*pb.Op)
	defs := make(map[digest.Digest][]byte)

	var dgst digest.Digest
	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return nil, nil, errors.Wrap(err, "failed to parse llb proto op")
		}
		dgst = digest.FromBytes(dt)
		ops[dgst] = &op
		defs[dgst] = dt
	}

	if dgst == "" {
		return nil, nil, nil
	}

	// depth first backwards
	dgsts := make([]digest.Digest, 0, len(def.Def))
	op := ops[dgst]

	if op.Op != nil {
		return nil, nil, errors.Errorf("invalid last vertex: %T", op.Op)
	}

	if len(op.Inputs) != 1 {
		return nil, nil, errors.Errorf("invalid last vertex inputs: %v", len(op.Inputs))
	}

	visited := map[digest.Digest]struct{}{}
	dgsts, err := walkDigests(dgsts, ops, dgst, visited)
	if err != nil {
		return nil, nil, err
	}
	for i := 0; i < len(dgsts)/2; i++ {
		j := len(dgsts) - 1 - i
		dgsts[i], dgsts[j] = dgsts[j], dgsts[i]
	}

	indexes := map[digest.Digest]int{}
	for i, dgst := range dgsts {
		indexes[dgst] = i
	}

	out := make([]BuildStep, 0, len(dgsts))
	for i, dgst := range dgsts {
		op := *ops[dgst]
		inputs := make([]string, len(op.Inputs))
		for i, inp := range op.Inputs {
			inputs[i] = fmt.Sprintf("step%d:%d", indexes[inp.Digest], inp.Index)
		}
		op.Inputs = nil
		out = append(out, BuildStep{
			ID:     fmt.Sprintf("step%d", i),
			Inputs: inputs,
			Op:     op,
		})
	}
	return out, indexes, nil
}

func walkDigests(dgsts []digest.Digest, ops map[digest.Digest]*pb.Op, dgst digest.Digest, visited map[digest.Digest]struct{}) ([]digest.Digest, error) {
	if _, ok := visited[dgst]; ok {
		return dgsts, nil
	}
	op, ok := ops[dgst]
	if !ok {
		return nil, errors.Errorf("failed to find input %v", dgst)
	}
	if op == nil {
		return nil, errors.Errorf("invalid nil input %v", dgst)
	}
	dgsts = append(dgsts, dgst)
	visited[dgst] = struct{}{}
	for _, inp := range op.Inputs {
		var err error
		dgsts, err = walkDigests(dgsts, ops, inp.Digest, visited)
		if err != nil {
			return nil, err
		}
	}
	return dgsts, nil
}
