package proc

import (
	"context"
	"encoding/json"
	"strconv"

	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	gatewaypb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/result"
	"github.com/pkg/errors"
)

func ProvenanceProcessor(attrs map[string]string) llbsolver.Processor {
	return func(ctx context.Context, res *llbsolver.Result, s *llbsolver.Solver, j *solver.Job) (*llbsolver.Result, error) {
		ps, err := exptypes.ParsePlatforms(res.Metadata)
		if err != nil {
			return nil, err
		}

		var inlineOnly bool
		if v, err := strconv.ParseBool(attrs["inline-only"]); v && err == nil {
			inlineOnly = true
		}

		for _, p := range ps.Platforms {
			cp, ok := res.Provenance.FindRef(p.ID)
			if !ok {
				return nil, errors.Errorf("no build info found for provenance %s", p.ID)
			}

			if cp == nil {
				continue
			}

			ref, ok := res.FindRef(p.ID)
			if !ok {
				return nil, errors.Errorf("could not find ref %s", p.ID)
			}

			pc, err := llbsolver.NewProvenanceCreator(ctx, cp, ref, attrs, j)
			if err != nil {
				return nil, err
			}

			filename := "provenance.json"
			if v, ok := attrs["filename"]; ok {
				filename = v
			}

			res.AddAttestation(p.ID, llbsolver.Attestation{
				Kind: gatewaypb.AttestationKindInToto,
				Metadata: map[string][]byte{
					result.AttestationReasonKey:     []byte(result.AttestationReasonProvenance),
					result.AttestationInlineOnlyKey: []byte(strconv.FormatBool(inlineOnly)),
				},
				InToto: result.InTotoAttestation{
					PredicateType: slsa02.PredicateSLSAProvenance,
				},
				Path: filename,
				ContentFunc: func() ([]byte, error) {
					pr, err := pc.Predicate()
					if err != nil {
						return nil, err
					}

					return json.MarshalIndent(pr, "", "  ")
				},
			})
		}

		return res, nil
	}
}
