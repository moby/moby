package provenance

import (
	"context"
	"fmt"

	"github.com/moby/buildkit/solver"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// AddBuildConfig populates the build configuration and source info
// on a SLSA v1 provenance predicate from the given capture and result.
func AddBuildConfig(ctx context.Context, p *provenancetypes.ProvenancePredicateSLSA1, c *Capture, rp solver.ResultProxy, withUsage bool) (map[digest.Digest]int, error) {
	def := rp.Definition()
	steps, indexes, err := toBuildSteps(def, c, withUsage)
	if err != nil {
		return nil, err
	}

	bc := &provenancetypes.BuildConfig{
		Definition:    steps,
		DigestMapping: digestMap(indexes),
	}

	p.BuildDefinition.InternalParameters.BuildConfig = bc

	if def.Source != nil {
		sis := make([]provenancetypes.SourceInfo, len(def.Source.Infos))
		for i, si := range def.Source.Infos {
			steps, indexes, err := toBuildSteps(si.Definition, c, withUsage)
			if err != nil {
				return nil, err
			}
			s := provenancetypes.SourceInfo{
				Filename:      si.Filename,
				Data:          si.Data,
				Language:      si.Language,
				Definition:    steps,
				DigestMapping: digestMap(indexes),
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

			if p.RunDetails.Metadata == nil {
				p.RunDetails.Metadata = &provenancetypes.ProvenanceMetadataSLSA1{}
			}
			p.RunDetails.Metadata.BuildKitMetadata.Source = &provenancetypes.Source{
				Infos:     sis,
				Locations: locs,
			}
		}
	}

	return indexes, nil
}

func digestMap(idx map[digest.Digest]int) map[digest.Digest]string {
	m := map[digest.Digest]string{}
	for k, v := range idx {
		m[k] = fmt.Sprintf("step%d", v)
	}
	return m
}

func toBuildSteps(def *pb.Definition, c *Capture, withUsage bool) ([]provenancetypes.BuildStep, map[digest.Digest]int, error) {
	if def == nil || len(def.Def) == 0 {
		return nil, nil, nil
	}

	ops := make(map[digest.Digest]*pb.Op)
	defs := make(map[digest.Digest][]byte)

	var dgst digest.Digest
	for _, dt := range def.Def {
		var op pb.Op
		if err := op.UnmarshalVT(dt); err != nil {
			return nil, nil, errors.Wrap(err, "failed to parse llb proto op")
		}
		if src := op.GetSource(); src != nil {
			for k := range src.Attrs {
				if k == "local.session" || k == "local.unique" {
					delete(src.Attrs, k)
				}
			}
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
	indexes := map[digest.Digest]int{}
	for i, dgst := range dgsts {
		indexes[dgst] = i
	}

	out := make([]provenancetypes.BuildStep, 0, len(dgsts))
	for i, dgst := range dgsts {
		op := ops[dgst].CloneVT()
		inputs := make([]string, len(op.Inputs))
		for i, inp := range op.Inputs {
			inputs[i] = fmt.Sprintf("step%d:%d", indexes[digest.Digest(inp.Digest)], inp.Index)
		}
		op.Inputs = nil
		s := provenancetypes.BuildStep{
			ID:     fmt.Sprintf("step%d", i),
			Inputs: inputs,
			Op:     op,
		}
		if withUsage {
			s.ResourceUsage = c.Samples[dgst]
		}
		out = append(out, s)
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
	visited[dgst] = struct{}{}
	for _, inp := range op.Inputs {
		var err error
		dgsts, err = walkDigests(dgsts, ops, digest.Digest(inp.Digest), visited)
		if err != nil {
			return nil, err
		}
	}
	dgsts = append(dgsts, dgst)
	return dgsts, nil
}
