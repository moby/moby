package llbsolver

import (
	"strings"

	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type vertex struct {
	sys     interface{}
	options solver.VertexOptions
	inputs  []solver.Edge
	digest  digest.Digest
	name    string
}

func (v *vertex) Digest() digest.Digest {
	return v.digest
}

func (v *vertex) Sys() interface{} {
	return v.sys
}

func (v *vertex) Options() solver.VertexOptions {
	return v.options
}

func (v *vertex) Inputs() []solver.Edge {
	return v.inputs
}

func (v *vertex) Name() string {
	return v.name
}

type LoadOpt func(*solver.VertexOptions)

func WithCacheSources(cms []solver.CacheManager) LoadOpt {
	return func(opt *solver.VertexOptions) {
		opt.CacheSources = cms
	}
}

func Load(def *pb.Definition, opts ...LoadOpt) (solver.Edge, error) {
	return loadLLB(def, func(dgst digest.Digest, pbOp *pb.Op, load func(digest.Digest) (solver.Vertex, error)) (solver.Vertex, error) {
		opMetadata := def.Metadata[dgst]
		vtx, err := newVertex(dgst, pbOp, &opMetadata, load, opts...)
		if err != nil {
			return nil, err
		}
		return vtx, nil
	})
}

func newVertex(dgst digest.Digest, op *pb.Op, opMeta *pb.OpMetadata, load func(digest.Digest) (solver.Vertex, error), opts ...LoadOpt) (*vertex, error) {
	opt := solver.VertexOptions{}
	if opMeta != nil {
		opt.IgnoreCache = opMeta.IgnoreCache
		opt.Description = opMeta.Description
		if opMeta.ExportCache != nil {
			opt.ExportCache = &opMeta.ExportCache.Value
		}
	}
	for _, fn := range opts {
		fn(&opt)
	}
	vtx := &vertex{sys: op.Op, options: opt, digest: dgst, name: llbOpName(op)}
	for _, in := range op.Inputs {
		sub, err := load(in.Digest)
		if err != nil {
			return nil, err
		}
		vtx.inputs = append(vtx.inputs, solver.Edge{Index: solver.Index(in.Index), Vertex: sub})
	}
	return vtx, nil
}

// loadLLB loads LLB.
// fn is executed sequentially.
func loadLLB(def *pb.Definition, fn func(digest.Digest, *pb.Op, func(digest.Digest) (solver.Vertex, error)) (solver.Vertex, error)) (solver.Edge, error) {
	if len(def.Def) == 0 {
		return solver.Edge{}, errors.New("invalid empty definition")
	}

	allOps := make(map[digest.Digest]*pb.Op)

	var dgst digest.Digest

	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return solver.Edge{}, errors.Wrap(err, "failed to parse llb proto op")
		}
		dgst = digest.FromBytes(dt)
		allOps[dgst] = &op
	}

	lastOp := allOps[dgst]
	delete(allOps, dgst)
	dgst = lastOp.Inputs[0].Digest

	cache := make(map[digest.Digest]solver.Vertex)

	var rec func(dgst digest.Digest) (solver.Vertex, error)
	rec = func(dgst digest.Digest) (solver.Vertex, error) {
		if v, ok := cache[dgst]; ok {
			return v, nil
		}
		v, err := fn(dgst, allOps[dgst], rec)
		if err != nil {
			return nil, err
		}
		cache[dgst] = v
		return v, nil
	}

	v, err := rec(dgst)
	if err != nil {
		return solver.Edge{}, err
	}
	return solver.Edge{Vertex: v, Index: solver.Index(lastOp.Inputs[0].Index)}, nil
}

func llbOpName(op *pb.Op) string {
	switch op := op.Op.(type) {
	case *pb.Op_Source:
		if id, err := source.FromLLB(op); err == nil {
			if id, ok := id.(*source.LocalIdentifier); ok {
				if len(id.IncludePatterns) == 1 {
					return op.Source.Identifier + " (" + id.IncludePatterns[0] + ")"
				}
			}
		}
		return op.Source.Identifier
	case *pb.Op_Exec:
		return strings.Join(op.Exec.Meta.Args, " ")
	case *pb.Op_Build:
		return "build"
	default:
		return "unknown"
	}
}
