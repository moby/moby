package sbom

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	"github.com/moby/buildkit/client/llb"
	gatewaypb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/result"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	CoreSBOMName    = "sbom"
	ExtraSBOMPrefix = CoreSBOMName + "-"

	srcDir = "/run/src/"
	outDir = "/run/out/"
)

// Scanner is a function type for scanning the contents of a state and
// returning a new attestation and state representing the scan results.
//
// A scanner is designed a scan a single state, however, additional states can
// also be attached, for attaching additional information, such as scans of
// build-contexts or multi-stage builds. Handling these separately allows the
// scanner to optionally ignore these or to mark them as such in the
// attestation.
type Scanner func(ctx context.Context, name string, ref llb.State, extras map[string]llb.State, opts ...llb.ConstraintsOpt) (result.Attestation[llb.State], error)

func CreateSBOMScanner(ctx context.Context, resolver llb.ImageMetaResolver, scanner string) (Scanner, error) {
	if scanner == "" {
		return nil, nil
	}

	_, dt, err := resolver.ResolveImageConfig(ctx, scanner, llb.ResolveImageConfigOpt{})
	if err != nil {
		return nil, err
	}

	var cfg ocispecs.Image
	if err := json.Unmarshal(dt, &cfg); err != nil {
		return nil, err
	}

	var args []string
	args = append(args, cfg.Config.Entrypoint...)
	args = append(args, cfg.Config.Cmd...)
	if len(args) == 0 {
		return nil, errors.Errorf("scanner %s does not have cmd", scanner)
	}

	return func(ctx context.Context, name string, ref llb.State, extras map[string]llb.State, opts ...llb.ConstraintsOpt) (result.Attestation[llb.State], error) {
		var env []string
		env = append(env, cfg.Config.Env...)
		env = append(env, "BUILDKIT_SCAN_DESTINATION="+outDir)
		env = append(env, "BUILDKIT_SCAN_SOURCE="+path.Join(srcDir, "core", CoreSBOMName))
		if len(extras) > 0 {
			env = append(env, "BUILDKIT_SCAN_SOURCE_EXTRAS="+path.Join(srcDir, "extras/"))
		}

		runOpts := []llb.RunOption{
			llb.WithCustomName(fmt.Sprintf("[%s] generating sbom using %s", name, scanner)),
		}
		for _, opt := range opts {
			runOpts = append(runOpts, opt)
		}
		runOpts = append(runOpts, llb.Dir(cfg.Config.WorkingDir))
		runOpts = append(runOpts, llb.Args(args))
		for _, e := range env {
			k, v, _ := strings.Cut(e, "=")
			runOpts = append(runOpts, llb.AddEnv(k, v))
		}

		runscan := llb.Image(scanner).Run(runOpts...)
		runscan.AddMount("/tmp", llb.Scratch(), llb.Tmpfs())

		runscan.AddMount(path.Join(srcDir, "core", CoreSBOMName), ref, llb.Readonly)
		for k, extra := range extras {
			runscan.AddMount(path.Join(srcDir, "extras", ExtraSBOMPrefix+k), extra, llb.Readonly)
		}

		stsbom := runscan.AddMount(outDir, llb.Scratch())
		return result.Attestation[llb.State]{
			Kind: gatewaypb.AttestationKindBundle,
			Ref:  stsbom,
			Metadata: map[string][]byte{
				result.AttestationReasonKey: []byte(result.AttestationReasonSBOM),
				result.AttestationSBOMCore:  []byte(CoreSBOMName),
			},
			InToto: result.InTotoAttestation{
				PredicateType: intoto.PredicateSPDX,
			},
		}, nil
	}, nil
}

func HasSBOM[T any](res *result.Result[T]) bool {
	for _, as := range res.Attestations {
		for _, a := range as {
			if a.InToto.PredicateType == intoto.PredicateSPDX {
				return true
			}
		}
	}
	return false
}
