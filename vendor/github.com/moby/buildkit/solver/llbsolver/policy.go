package llbsolver

import (
	"context"
	"slices"
	"strings"

	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/frontend/gateway"
	gatewaypb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/sourcepolicy"
	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/sourcepolicy/policysession"
	"github.com/moby/buildkit/worker"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	keySourcePolicy        = "llb.sourcepolicy"
	keySourcePolicySession = "llb.sourcepolicysession"
)

// SourcePolicyEvaluator evaluates source operations against configured policies.
type SourcePolicyEvaluator interface {
	Evaluate(ctx context.Context, op *pb.Op) (bool, error)
}

type policyEvaluator struct {
	*llbBridge
	engine *sourcepolicy.Engine
}

func normalizedSourceIdentifier(w worker.Worker, op *pb.SourceOp) string {
	if w == nil {
		return op.GetIdentifier()
	}
	id, err := w.ParseSource(op, nil)
	if err != nil {
		return op.GetIdentifier()
	}
	return id.String()
}

func (p *policyEvaluator) Evaluate(ctx context.Context, op *pb.Op) (bool, error) {
	return p.evaluate(ctx, op, 10)
}

func (p *policyEvaluator) evaluate(ctx context.Context, op *pb.Op, max int) (bool, error) {
	mutated, err := p.evaluateSource(ctx, op, max)
	if err != nil {
		return false, err
	}

	nestedMutated, err := p.evaluateNestedSources(ctx, op, max)
	if err != nil {
		return false, err
	}
	return mutated || nestedMutated, nil
}

// evaluateNestedSources applies policy to resources embedded in a source op
// that are fetched independently of the source identified by the op itself.
func (p *policyEvaluator) evaluateNestedSources(ctx context.Context, op *pb.Op, max int) (bool, error) {
	// A Git bundle is transported as a container blob, so evaluate its locator
	// as the equivalent containerblob source before the Git source can use it.
	bundleOp, ok, err := gitBundleSourceOp(op)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	bundleMutated, err := p.evaluateSource(ctx, bundleOp, max)
	if err != nil {
		return false, errors.Wrap(err, "error evaluating git bundle source policy")
	}
	if bundleMutated {
		applyGitBundleSourceOp(op.GetSource(), bundleOp.GetSource())
	}
	return bundleMutated, nil
}

func (p *policyEvaluator) evaluateSource(ctx context.Context, op *pb.Op, max int) (bool, error) {
	source := op.GetSource()
	if source == nil {
		return false, nil
	}
	w, err := p.resolveWorker()
	if err != nil {
		return false, err
	}
	source.Identifier = normalizedSourceIdentifier(w, source)
	ok, err := p.engine.Evaluate(ctx, source)
	if err != nil {
		return false, err
	}
	sid, err := loadSourcePolicySession(p.builder)
	if err != nil {
		return false, err
	}
	if sid == "" {
		return ok, nil
	}
	caller, err := p.sm.Get(ctx, sid, false)
	if err != nil {
		return false, err
	}

	verifier := policysession.NewPolicyVerifierClient(caller.Conn())
	ctx = caller.Context(ctx)
	req := &policysession.CheckPolicyRequest{
		Platform: op.Platform,
		Source: &gatewaypb.ResolveSourceMetaResponse{
			Source: source,
		},
	}

	for {
		max--
		if max < 0 { // TODO: better loop detection
			return false, errors.New("too many policy requests")
		}
		resp, err := verifier.CheckPolicy(ctx, req)
		if err != nil {
			return false, err
		}

		metareq := resp.GetRequest()
		if metareq != nil {
			op := sourceresolver.Opt{
				LogName: metareq.LogName,
			}
			if metareq.Source.Identifier != source.Identifier {
				return false, errors.Errorf("policy requested different source identifier: %q != %q", metareq.Source.Identifier, source.Identifier)
			}
			if err := mapsEqual(source.Attrs, metareq.Source.Attrs); err != nil {
				return false, errors.Wrap(err, "policy requested different source attrs")
			}
			if metareq.ResolveMode != "" {
				if strings.HasPrefix(metareq.Source.Identifier, "docker-image://") {
					op.ImageOpt = &sourceresolver.ResolveImageOpt{
						ResolveMode: metareq.ResolveMode,
					}
				}
			}
			if strings.HasPrefix(metareq.Source.Identifier, "docker-image://") {
				if op.ImageOpt == nil {
					op.ImageOpt = &sourceresolver.ResolveImageOpt{}
				}
				op.ImageOpt.Platform = toOCIPlatform(metareq.Platform)
			}
			if strings.HasPrefix(metareq.Source.Identifier, "oci-layout://") {
				op.OCILayoutOpt = &sourceresolver.ResolveOCILayoutOpt{
					Platform: toOCIPlatform(metareq.Platform),
				}
			}

			if metareq.Image != nil {
				if op.ImageOpt == nil {
					op.ImageOpt = &sourceresolver.ResolveImageOpt{}
				}
				op.ImageOpt.NoConfig = metareq.Image.NoConfig
				op.ImageOpt.AttestationChain = metareq.Image.AttestationChain
				op.ImageOpt.ResolveAttestations = slices.Clone(metareq.Image.ResolveAttestations)
			}

			if metareq.Git != nil {
				op.GitOpt = &sourceresolver.ResolveGitOpt{
					ReturnObject: metareq.Git.ReturnObject,
				}
			}
			if metareq.HTTP != nil && metareq.HTTP.ChecksumRequest != nil {
				op.HTTPOpt = &sourceresolver.ResolveHTTPOpt{
					ChecksumReq: &sourceresolver.ResolveHTTPChecksumRequest{
						Algo:   fromPBHTTPChecksumAlgo(metareq.HTTP.ChecksumRequest.Algo),
						Suffix: slices.Clone(metareq.HTTP.ChecksumRequest.Suffix),
					},
				}
			}

			resp, err := p.resolveSourceMetadata(ctx, metareq.Source, op, false)
			if err != nil {
				return false, errors.Wrap(err, "error resolving source metadata from policy request")
			}
			req.Source = gateway.ToPBResolveSourceMetaResponse(resp)
			continue
		}

		decision := resp.GetDecision()
		if decision == nil {
			return false, errors.New("no decision in policy response")
		}
		if decision.Action == spb.PolicyAction_CONVERT {
			newSrc := decision.Update
			if newSrc == nil {
				return false, errors.New("convert action requires updated source")
			}
			source.Identifier = newSrc.Identifier
			source.Attrs = newSrc.Attrs
			_, err = p.evaluateSource(ctx, op, max)
			if err != nil {
				return false, err
			}
			return true, nil
		}
		if decision.Action != spb.PolicyAction_ALLOW {
			err := errors.Errorf("source %q not allowed by policy: action %s", source.Identifier, decision.Action.String())
			return false, policysession.WrapDenyMessages(err, decision.GetDenyMessages())
		}
		return ok, nil
	}
}

func gitBundleSourceOp(op *pb.Op) (*pb.Op, bool, error) {
	source := op.GetSource()
	if source == nil || !strings.HasPrefix(source.Identifier, srctypes.GitScheme+"://") {
		return nil, false, nil
	}

	bundle := source.Attrs[pb.AttrGitBundle]
	if bundle == "" {
		return nil, false, nil
	}
	scheme, ref, ok := strings.Cut(bundle, "://")
	if !ok {
		return nil, false, errors.Errorf("failed to parse git.bundle locator %q: missing scheme", bundle)
	}
	if scheme != srctypes.DockerImageBlobScheme && scheme != srctypes.OCIBlobScheme {
		return nil, false, errors.Errorf("git.bundle locator scheme %q is not supported", scheme)
	}
	// Parse the reference to match containerblob identifier normalization. The
	// Git source remains responsible for bundle-specific validation such as the
	// required digest algorithm.
	parsed, err := reference.Parse(ref)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to parse git.bundle locator %q", bundle)
	}

	attrs := map[string]string{}
	if scheme == srctypes.OCIBlobScheme {
		if value, ok := source.Attrs[pb.AttrOCILayoutSessionID]; ok {
			attrs[pb.AttrOCILayoutSessionID] = value
		}
		if storeID := source.Attrs[pb.AttrOCILayoutStoreID]; storeID != "" {
			attrs[pb.AttrOCILayoutStoreID] = storeID
		} else {
			// Git bundle locators use the reference locator as the default OCI
			// store, while a standalone containerblob source requires it as an
			// explicit attribute.
			attrs[pb.AttrOCILayoutStoreID] = parsed.Locator
		}
	}
	return &pb.Op{
		Op: &pb.Op_Source{Source: &pb.SourceOp{
			Identifier: scheme + "://" + parsed.String(),
			Attrs:      attrs,
		}},
		Platform: op.Platform,
	}, true, nil
}

func applyGitBundleSourceOp(gitSource, bundleSource *pb.SourceOp) {
	implicitStoreID := gitBundleImplicitStoreID(gitSource)
	gitSource.Attrs[pb.AttrGitBundle] = bundleSource.Identifier
	for _, key := range []string{pb.AttrOCILayoutSessionID, pb.AttrOCILayoutStoreID} {
		delete(gitSource.Attrs, key)
		if value, ok := bundleSource.Attrs[key]; ok {
			if key == pb.AttrOCILayoutStoreID && value == implicitStoreID {
				continue
			}
			gitSource.Attrs[key] = value
		}
	}
}

func gitBundleImplicitStoreID(source *pb.SourceOp) string {
	if source.Attrs[pb.AttrOCILayoutStoreID] != "" {
		return ""
	}
	bundle := source.Attrs[pb.AttrGitBundle]
	scheme, ref, ok := strings.Cut(bundle, "://")
	if !ok || scheme != srctypes.OCIBlobScheme {
		return ""
	}
	parsed, err := reference.Parse(ref)
	if err != nil {
		return ""
	}
	return parsed.Locator
}

func mapsEqual[K comparable, V comparable](a, b map[K]V) error {
	if len(a) != len(b) {
		return errors.Errorf("map length mismatch: %d != %d", len(a), len(b))
	}
	for k, v := range a {
		vb, ok := b[k]
		if !ok {
			return errors.Errorf("key %v missing from second map", k)
		}
		if vb != v {
			return errors.Errorf("value mismatch for key %v: %v != %v", k, v, vb)
		}
	}
	return nil
}

func toPBPlatform(p *ocispecs.Platform) *pb.Platform {
	if p == nil {
		return nil
	}
	return &pb.Platform{
		Architecture: p.Architecture,
		OS:           p.OS,
		Variant:      p.Variant,
		OSVersion:    p.OSVersion,
		OSFeatures:   p.OSFeatures,
	}
}

func toOCIPlatform(p *pb.Platform) *ocispecs.Platform {
	if p == nil {
		return nil
	}
	return &ocispecs.Platform{
		Architecture: p.Architecture,
		OS:           p.OS,
		Variant:      p.Variant,
		OSVersion:    p.OSVersion,
		OSFeatures:   p.OSFeatures,
	}
}

func fromPBHTTPChecksumAlgo(in gatewaypb.ChecksumRequest_ChecksumAlgo) sourceresolver.ResolveHTTPChecksumAlgo {
	switch in {
	case gatewaypb.ChecksumRequest_CHECKSUM_ALGO_SHA256:
		return sourceresolver.ResolveHTTPChecksumAlgoSHA256
	case gatewaypb.ChecksumRequest_CHECKSUM_ALGO_SHA384:
		return sourceresolver.ResolveHTTPChecksumAlgoSHA384
	case gatewaypb.ChecksumRequest_CHECKSUM_ALGO_SHA512:
		return sourceresolver.ResolveHTTPChecksumAlgoSHA512
	default:
		return sourceresolver.ResolveHTTPChecksumAlgo(in)
	}
}

func validateSourcePolicy(pol *spb.Policy) error {
	for _, r := range pol.Rules {
		if r == nil {
			return errors.New("invalid nil rule in policy")
		}
		if r.Selector == nil {
			return errors.New("invalid nil selector in policy")
		}
		for _, c := range r.Selector.Constraints {
			if c == nil {
				return errors.New("invalid nil constraint in policy")
			}
		}
	}
	return nil
}

func loadSourcePolicy(b solver.Builder) (*spb.Policy, error) {
	var srcPol spb.Policy
	err := b.EachValue(context.TODO(), keySourcePolicy, func(v any) error {
		x, ok := v.(*spb.Policy)
		if !ok {
			return errors.Errorf("invalid source policy %T", v)
		}
		for _, f := range x.Rules {
			if f == nil {
				return errors.New("invalid nil policy rule")
			}
			srcPol.Rules = append(srcPol.Rules, f.CloneVT())
		}
		srcPol.Version = x.Version
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &srcPol, nil
}

func loadSourcePolicySession(b solver.Builder) (string, error) {
	var session string
	err := b.EachValue(context.TODO(), keySourcePolicySession, func(v any) error {
		x, ok := v.(string)
		if !ok {
			return errors.Errorf("invalid source policy session %T", v)
		}
		if x != "" {
			session = x
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return session, nil
}
