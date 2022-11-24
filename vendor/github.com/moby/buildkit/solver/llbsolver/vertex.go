package llbsolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/ops/opsutils"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/util/entitlements"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
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
	if name, ok := v.options.Description["llb.customname"]; ok {
		return name
	}
	return v.name
}

type LoadOpt func(*pb.Op, *pb.OpMetadata, *solver.VertexOptions) error

func WithValidateCaps() LoadOpt {
	cs := pb.Caps.CapSet(pb.Caps.All())
	return func(_ *pb.Op, md *pb.OpMetadata, opt *solver.VertexOptions) error {
		if md != nil {
			for c := range md.Caps {
				if err := cs.Supports(c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func WithCacheSources(cms []solver.CacheManager) LoadOpt {
	return func(_ *pb.Op, _ *pb.OpMetadata, opt *solver.VertexOptions) error {
		opt.CacheSources = cms
		return nil
	}
}

func NormalizeRuntimePlatforms() LoadOpt {
	var defaultPlatform *pb.Platform
	return func(op *pb.Op, _ *pb.OpMetadata, opt *solver.VertexOptions) error {
		if op.Platform == nil {
			if defaultPlatform == nil {
				p := platforms.DefaultSpec()
				defaultPlatform = &pb.Platform{
					OS:           p.OS,
					Architecture: p.Architecture,
					Variant:      p.Variant,
				}
			}
			op.Platform = defaultPlatform
		}
		platform := ocispecs.Platform{OS: op.Platform.OS, Architecture: op.Platform.Architecture, Variant: op.Platform.Variant}
		normalizedPlatform := platforms.Normalize(platform)

		op.Platform = &pb.Platform{
			OS:           normalizedPlatform.OS,
			Architecture: normalizedPlatform.Architecture,
			Variant:      normalizedPlatform.Variant,
		}

		return nil
	}
}

func ValidateEntitlements(ent entitlements.Set) LoadOpt {
	return func(op *pb.Op, _ *pb.OpMetadata, opt *solver.VertexOptions) error {
		switch op := op.Op.(type) {
		case *pb.Op_Exec:
			if op.Exec.Network == pb.NetMode_HOST {
				if !ent.Allowed(entitlements.EntitlementNetworkHost) {
					return errors.Errorf("%s is not allowed", entitlements.EntitlementNetworkHost)
				}
			}

			if op.Exec.Security == pb.SecurityMode_INSECURE {
				if !ent.Allowed(entitlements.EntitlementSecurityInsecure) {
					return errors.Errorf("%s is not allowed", entitlements.EntitlementSecurityInsecure)
				}
			}
		}
		return nil
	}
}

type detectPrunedCacheID struct {
	ids map[string]struct{}
}

func (dpc *detectPrunedCacheID) Load(op *pb.Op, md *pb.OpMetadata, opt *solver.VertexOptions) error {
	if md == nil || !md.IgnoreCache {
		return nil
	}
	switch op := op.Op.(type) {
	case *pb.Op_Exec:
		for _, m := range op.Exec.GetMounts() {
			if m.MountType == pb.MountType_CACHE {
				if m.CacheOpt != nil {
					id := m.CacheOpt.ID
					if id == "" {
						id = m.Dest
					}
					if dpc.ids == nil {
						dpc.ids = map[string]struct{}{}
					}
					dpc.ids[id] = struct{}{}
				}
			}
		}
	}
	return nil
}

func Load(ctx context.Context, def *pb.Definition, polEngine SourcePolicyEvaluator, opts ...LoadOpt) (solver.Edge, error) {
	return loadLLB(ctx, def, polEngine, func(dgst digest.Digest, pbOp *pb.Op, load func(digest.Digest) (solver.Vertex, error)) (solver.Vertex, error) {
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
		opt.ProgressGroup = opMeta.ProgressGroup
	}
	for _, fn := range opts {
		if err := fn(op, opMeta, &opt); err != nil {
			return nil, err
		}
	}

	name, err := llbOpName(op, load)
	if err != nil {
		return nil, err
	}
	vtx := &vertex{sys: op, options: opt, digest: dgst, name: name}
	for _, in := range op.Inputs {
		sub, err := load(in.Digest)
		if err != nil {
			return nil, err
		}
		vtx.inputs = append(vtx.inputs, solver.Edge{Index: solver.Index(in.Index), Vertex: sub})
	}
	return vtx, nil
}

func recomputeDigests(ctx context.Context, all map[digest.Digest]*pb.Op, visited map[digest.Digest]digest.Digest, dgst digest.Digest) (digest.Digest, error) {
	if dgst, ok := visited[dgst]; ok {
		return dgst, nil
	}
	op := all[dgst]

	var mutated bool
	for _, input := range op.Inputs {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		iDgst, err := recomputeDigests(ctx, all, visited, input.Digest)
		if err != nil {
			return "", err
		}
		if input.Digest != iDgst {
			mutated = true
			input.Digest = iDgst
		}
	}

	if !mutated {
		return dgst, nil
	}

	dt, err := op.Marshal()
	if err != nil {
		return "", err
	}
	newDgst := digest.FromBytes(dt)
	visited[dgst] = newDgst
	all[newDgst] = op
	delete(all, dgst)
	return newDgst, nil
}

// loadLLB loads LLB.
// fn is executed sequentially.
func loadLLB(ctx context.Context, def *pb.Definition, polEngine SourcePolicyEvaluator, fn func(digest.Digest, *pb.Op, func(digest.Digest) (solver.Vertex, error)) (solver.Vertex, error)) (solver.Edge, error) {
	if len(def.Def) == 0 {
		return solver.Edge{}, errors.New("invalid empty definition")
	}

	allOps := make(map[digest.Digest]*pb.Op)
	mutatedDigests := make(map[digest.Digest]digest.Digest) // key: old, val: new

	var lastDgst digest.Digest

	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return solver.Edge{}, errors.Wrap(err, "failed to parse llb proto op")
		}
		dgst := digest.FromBytes(dt)
		if polEngine != nil {
			mutated, err := polEngine.Evaluate(ctx, &op)
			if err != nil {
				return solver.Edge{}, errors.Wrap(err, "error evaluating the source policy")
			}
			if mutated {
				dtMutated, err := op.Marshal()
				if err != nil {
					return solver.Edge{}, err
				}
				dgstMutated := digest.FromBytes(dtMutated)
				mutatedDigests[dgst] = dgstMutated
				dgst = dgstMutated
			}
		}
		allOps[dgst] = &op
		lastDgst = dgst
	}

	for dgst := range allOps {
		_, err := recomputeDigests(ctx, allOps, mutatedDigests, dgst)
		if err != nil {
			return solver.Edge{}, err
		}
	}

	if len(allOps) < 2 {
		return solver.Edge{}, errors.Errorf("invalid LLB with %d vertexes", len(allOps))
	}

	for {
		newDgst, ok := mutatedDigests[lastDgst]
		if !ok {
			break
		}
		lastDgst = newDgst
	}

	lastOp := allOps[lastDgst]
	delete(allOps, lastDgst)
	if len(lastOp.Inputs) == 0 {
		return solver.Edge{}, errors.Errorf("invalid LLB with no inputs on last vertex")
	}
	dgst := lastOp.Inputs[0].Digest

	cache := make(map[digest.Digest]solver.Vertex)

	var rec func(dgst digest.Digest) (solver.Vertex, error)
	rec = func(dgst digest.Digest) (solver.Vertex, error) {
		if v, ok := cache[dgst]; ok {
			return v, nil
		}
		op, ok := allOps[dgst]
		if !ok {
			return nil, errors.Errorf("invalid missing input digest %s", dgst)
		}

		if err := opsutils.Validate(op); err != nil {
			return nil, err
		}

		v, err := fn(dgst, op, rec)
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

func llbOpName(pbOp *pb.Op, load func(digest.Digest) (solver.Vertex, error)) (string, error) {
	switch op := pbOp.Op.(type) {
	case *pb.Op_Source:
		if id, err := source.FromLLB(op, nil); err == nil {
			if id, ok := id.(*source.LocalIdentifier); ok {
				if len(id.IncludePatterns) == 1 {
					return op.Source.Identifier + " (" + id.IncludePatterns[0] + ")", nil
				}
			}
		}
		return op.Source.Identifier, nil
	case *pb.Op_Exec:
		return strings.Join(op.Exec.Meta.Args, " "), nil
	case *pb.Op_File:
		return fileOpName(op.File.Actions), nil
	case *pb.Op_Build:
		return "build", nil
	case *pb.Op_Merge:
		subnames := make([]string, len(pbOp.Inputs))
		for i, inp := range pbOp.Inputs {
			subvtx, err := load(inp.Digest)
			if err != nil {
				return "", err
			}
			subnames[i] = subvtx.Name()
		}
		return "merge " + fmt.Sprintf("(%s)", strings.Join(subnames, ", ")), nil
	case *pb.Op_Diff:
		var lowerName string
		if op.Diff.Lower.Input == -1 {
			lowerName = "scratch"
		} else {
			lowerVtx, err := load(pbOp.Inputs[op.Diff.Lower.Input].Digest)
			if err != nil {
				return "", err
			}
			lowerName = fmt.Sprintf("(%s)", lowerVtx.Name())
		}
		var upperName string
		if op.Diff.Upper.Input == -1 {
			upperName = "scratch"
		} else {
			upperVtx, err := load(pbOp.Inputs[op.Diff.Upper.Input].Digest)
			if err != nil {
				return "", err
			}
			upperName = fmt.Sprintf("(%s)", upperVtx.Name())
		}
		return "diff " + lowerName + " -> " + upperName, nil
	default:
		return "unknown", nil
	}
}

func fileOpName(actions []*pb.FileAction) string {
	names := make([]string, 0, len(actions))
	for _, action := range actions {
		switch a := action.Action.(type) {
		case *pb.FileAction_Mkdir:
			names = append(names, fmt.Sprintf("mkdir %s", a.Mkdir.Path))
		case *pb.FileAction_Mkfile:
			names = append(names, fmt.Sprintf("mkfile %s", a.Mkfile.Path))
		case *pb.FileAction_Rm:
			names = append(names, fmt.Sprintf("rm %s", a.Rm.Path))
		case *pb.FileAction_Copy:
			names = append(names, fmt.Sprintf("copy %s %s", a.Copy.Src, a.Copy.Dest))
		}
	}

	return strings.Join(names, ", ")
}
