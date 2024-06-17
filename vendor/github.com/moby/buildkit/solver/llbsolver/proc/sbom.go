package proc

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/executor/resources"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/attestations/sbom"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/result"
	"github.com/moby/buildkit/util/tracing"
	"github.com/pkg/errors"
)

func SBOMProcessor(scannerRef string, useCache bool, resolveMode string) llbsolver.Processor {
	return func(ctx context.Context, res *llbsolver.Result, s *llbsolver.Solver, j *solver.Job, usage *resources.SysSampler) (*llbsolver.Result, error) {
		// skip sbom generation if we already have an sbom
		if sbom.HasSBOM(res.Result) {
			return res, nil
		}

		span, ctx := tracing.StartSpan(ctx, "create sbom attestation")
		defer span.End()

		ps, err := exptypes.ParsePlatforms(res.Metadata)
		if err != nil {
			return nil, err
		}

		scanner, err := sbom.CreateSBOMScanner(ctx, s.Bridge(j), scannerRef, sourceresolver.Opt{
			ImageOpt: &sourceresolver.ResolveImageOpt{
				ResolveMode: resolveMode,
			},
		})
		if err != nil {
			return nil, err
		}
		if scanner == nil {
			return res, nil
		}

		for _, p := range ps.Platforms {
			ref, ok := res.FindRef(p.ID)
			if !ok {
				return nil, errors.Errorf("could not find ref %s", p.ID)
			}
			if ref == nil {
				continue
			}

			defop, err := llb.NewDefinitionOp(ref.Definition())
			if err != nil {
				return nil, err
			}
			st := llb.NewState(defop)

			var opts []llb.ConstraintsOpt
			if !useCache {
				opts = append(opts, llb.IgnoreCache)
			}
			att, err := scanner(ctx, p.ID, st, nil, opts...)
			if err != nil {
				return nil, err
			}
			attSolve, err := result.ConvertAttestation(&att, func(st *llb.State) (solver.ResultProxy, error) {
				def, err := st.Marshal(ctx)
				if err != nil {
					return nil, err
				}

				r, err := s.Bridge(j).Solve(ctx, frontend.SolveRequest{
					Definition: def.ToPB(),
				}, j.SessionID)
				if err != nil {
					return nil, err
				}
				return r.Ref, nil
			})
			if err != nil {
				return nil, err
			}
			res.AddAttestation(p.ID, *attSolve)
		}
		return res, nil
	}
}
