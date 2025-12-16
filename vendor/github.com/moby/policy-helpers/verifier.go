package verifier

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"time"

	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/moby/policy-helpers/image"
	"github.com/moby/policy-helpers/roots"
	"github.com/moby/policy-helpers/roots/dhi"
	"github.com/moby/policy-helpers/types"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"golang.org/x/sync/singleflight"
)

type Config struct {
	UpdateInterval time.Duration
	RequireOnline  bool
	StateDir       string
}

type Verifier struct {
	cfg Config
	sf  singleflight.Group
	tp  *roots.TrustProvider // tp may be nil if initialization failed
}

func NewVerifier(cfg Config) (*Verifier, error) {
	if cfg.StateDir == "" {
		return nil, errors.Errorf("state directory must be provided")
	}
	v := &Verifier{cfg: cfg}

	v.loadTrustProvider() // initialization fails on expired root/timestamp

	return v, nil
}

func (v *Verifier) VerifyArtifact(ctx context.Context, dgst digest.Digest, bundleBytes []byte, opt ...ArtifactVerifyOpt) (*types.SignatureInfo, error) {
	opts := &ArtifactVerifyOpts{}
	for _, o := range opt {
		o(opts)
	}

	anyCert, err := anyCerificateIdentity()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	alg, rawDgst, err := rawDigest(dgst)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	policy := verify.NewPolicy(verify.WithArtifactDigest(alg, rawDgst), anyCert)

	b, err := loadBundle(bundleBytes)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	tp, err := v.loadTrustProvider()
	if err != nil {
		return nil, errors.Wrap(err, "loading trust provider")
	}

	trustedRoot, st, err := tp.TrustedRoot(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting trusted root")
	}

	gv, err := verify.NewVerifier(trustedRoot, verify.WithSignedCertificateTimestamps(1), verify.WithTransparencyLog(1), verify.WithObserverTimestamps(1))
	if err != nil {
		return nil, errors.Wrap(err, "creating verifier")
	}

	result, err := gv.Verify(b, policy)
	if err != nil {
		return nil, errors.Wrap(err, "verifying bundle")
	}

	if result.Signature == nil || result.Signature.Certificate == nil {
		return nil, errors.Errorf("no valid signatures found")
	}

	if !opts.SLSANotRequired && !isSLSAPredicateType(result.Statement.PredicateType) {
		return nil, errors.Errorf("unexpected predicate type %q, expecting SLSA provenance", result.Statement.PredicateType)
	}

	si := &types.SignatureInfo{
		TrustRootStatus: toRootStatus(st),
		Signer:          result.Signature.Certificate,
		Timestamps:      toTimestamps(result.VerifiedTimestamps),
		SignatureType:   types.SignatureBundleV03,
	}
	si.Kind = si.DetectKind()
	return si, nil
}

func (v *Verifier) VerifyImage(ctx context.Context, provider image.ReferrersProvider, desc ocispecs.Descriptor, platform *ocispecs.Platform) (*types.SignatureInfo, error) {
	sc, err := image.ResolveSignatureChain(ctx, provider, desc, platform)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving signature chain for image %s", desc.Digest)
	}

	if sc.AttestationManifest == nil || sc.SignatureManifest == nil {
		return nil, errors.WithStack(&NoSigChainError{
			Target:         desc.Digest,
			HasAttestation: sc.AttestationManifest != nil,
		})
	}

	attestationBytes, err := sc.ManifestBytes(ctx, sc.AttestationManifest)
	if err != nil {
		return nil, errors.Wrapf(err, "reading attestation manifest %s", sc.AttestationManifest.Digest)
	}

	var attestation ocispecs.Manifest
	if err := json.Unmarshal(attestationBytes, &attestation); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling attestation manifest %s", sc.AttestationManifest.Digest)
	}

	if attestation.Subject == nil {
		return nil, errors.Errorf("attestation manifest %s has no subject", sc.AttestationManifest.Digest)
	}
	if attestation.Subject.Digest != sc.ImageManifest.Digest {
		return nil, errors.Errorf("attestation manifest %s subject digest %s does not match image manifest digest %s", sc.AttestationManifest.Digest, attestation.Subject.Digest, sc.ImageManifest.Digest)
	}
	if attestation.Subject.MediaType != ocispecs.MediaTypeImageManifest && attestation.Subject.MediaType != ocispecs.MediaTypeImageIndex {
		return nil, errors.Errorf("attestation manifest %s subject media type %s is not an image manifest or index", sc.AttestationManifest.Digest, attestation.Subject.MediaType)
	}
	if attestation.Subject.Size != sc.ImageManifest.Size {
		return nil, errors.Errorf("attestation manifest %s subject size %d does not match image manifest size %d", sc.AttestationManifest.Digest, attestation.Subject.Size, sc.ImageManifest.Size)
	}
	hasSLSA := false
	for _, l := range attestation.Layers {
		if isSLSAPredicateType(l.Annotations["in-toto.io/predicate-type"]) {
			hasSLSA = true
			break
		}
	}
	if !hasSLSA {
		return nil, errors.Errorf("attestation manifest %s has no SLSA provenance layer", sc.AttestationManifest.Digest)
	}

	anyCert, err := anyCerificateIdentity()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var artifactPolicy verify.ArtifactPolicyOption

	tp, err := v.loadTrustProvider()
	if err != nil {
		return nil, errors.Wrap(err, "loading trust provider")
	}
	var trustedRoot root.TrustedMaterial
	fulcioRoot, st, err := tp.TrustedRoot(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting trusted root")
	}

	sigBytes, err := sc.ManifestBytes(ctx, sc.SignatureManifest)
	if err != nil {
		return nil, errors.Wrapf(err, "reading signature manifest %s", sc.SignatureManifest.Digest)
	}

	var mfst ocispecs.Manifest
	if err := json.Unmarshal(sigBytes, &mfst); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling signature manifest %s", sc.SignatureManifest.Digest)
	}

	// basic validations
	if mfst.Subject == nil {
		return nil, errors.Errorf("signature manifest %s has no subject", sc.SignatureManifest.Digest)
	}
	if mfst.Subject.Digest != sc.AttestationManifest.Digest {
		return nil, errors.Errorf("signature manifest %s subject digest %s does not match attestation manifest digest %s", sc.SignatureManifest.Digest, mfst.Subject.Digest, sc.AttestationManifest.Digest)
	}
	if mfst.Subject.MediaType != ocispecs.MediaTypeImageManifest && mfst.Subject.MediaType != ocispecs.MediaTypeImageIndex {
		return nil, errors.Errorf("signature manifest %s subject media type %s is not an image manifest or index", sc.SignatureManifest.Digest, mfst.Subject.MediaType)
	}
	if mfst.Subject.Size != sc.AttestationManifest.Size {
		return nil, errors.Errorf("signature manifest %s subject size %d does not match attestation manifest size %d", sc.SignatureManifest.Digest, mfst.Subject.Size, sc.AttestationManifest.Size)
	}
	if len(mfst.Layers) == 0 {
		return nil, errors.Errorf("signature manifest %s has %d layers, expected 1", sc.SignatureManifest.Digest, len(mfst.Layers))
	}
	layer := mfst.Layers[0]

	var dockerReference string

	var se verify.SignedEntity
	sigType := types.SignatureBundleV03
	switch layer.MediaType {
	case image.ArtifactTypeSigstoreBundle:
		if mfst.ArtifactType != image.ArtifactTypeSigstoreBundle {
			return nil, errors.Errorf("signature manifest %s is not a bundle (artifact type %q)", sc.SignatureManifest.Digest, mfst.ArtifactType)
		}
		bundleBytes, err := image.ReadBlob(ctx, sc.Provider, layer)
		if err != nil {
			return nil, errors.Wrapf(err, "reading bundle layer %s from signature manifest %s", layer.Digest, sc.SignatureManifest.Digest)
		}
		b, err := loadBundle(bundleBytes)
		if err != nil {
			return nil, errors.Wrapf(err, "loading signature bundle from manifest %s", sc.SignatureManifest.Digest)
		}
		se = b

		alg, rawDgst, err := rawDigest(sc.AttestationManifest.Digest)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		artifactPolicy = verify.WithArtifactDigest(alg, rawDgst)
	case image.MediaTypeCosignSimpleSigning:
		sigType = types.SignatureSimpleSigningV1
		payloadBytes, err := image.ReadBlob(ctx, sc.Provider, layer)
		if err != nil {
			return nil, errors.Wrapf(err, "reading bundle layer %s from signature manifest %s", layer.Digest, sc.SignatureManifest.Digest)
		}
		var payload struct {
			Critical struct {
				Identity struct {
					DockerReference string `json:"docker-reference"`
				} `json:"identity"`
				Image struct {
					DockerManifestDigest string `json:"docker-manifest-digest"`
				} `json:"image"`
				Type string `json:"type"`
			} `json:"critical"`
			Optional map[string]any `json:"optional"`
		}
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, errors.Wrapf(err, "unmarshaling simple signing payload from manifest %s", sc.SignatureManifest.Digest)
		}
		if payload.Critical.Image.DockerManifestDigest != sc.AttestationManifest.Digest.String() {
			return nil, errors.Errorf("simple signing payload in manifest %s has docker-manifest-digest %s which does not match attestation manifest digest %s", sc.SignatureManifest.Digest, payload.Critical.Image.DockerManifestDigest, sc.AttestationManifest.Digest)
		}
		if payload.Critical.Type != "cosign container image signature" {
			return nil, errors.Errorf("simple signing payload in manifest %s has invalid type %q", sc.SignatureManifest.Digest, payload.Critical.Type)
		}
		dockerReference = payload.Critical.Identity.DockerReference
		// TODO: are more consistency checks needed for hashedrekord payload vs annotations?

		hrse, err := newHashedRecordSignedEntity(&mfst, sc.DHI)
		if err != nil {
			return nil, errors.Wrapf(err, "loading hashed record signed entity from manifest %s", sc.SignatureManifest.Digest)
		}
		se = hrse
		alg, rawDgst, err := rawDigest(layer.Digest)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		artifactPolicy = verify.WithArtifactDigest(alg, rawDgst)
	default:
		return nil, errors.Errorf("signature manifest %s layer has invalid media type %s", sc.SignatureManifest.Digest, layer.MediaType)
	}

	verifierOpts := []verify.VerifierOption{}

	if sc.DHI {
		trustedRoot, err = dhi.TrustedRoot(fulcioRoot)
		if err != nil {
			return nil, errors.Wrap(err, "getting DHI trust root")
		}
		// DHI signature may or may not have transparency data
		// validation needs to be done in a later additional policy step
		if _, hasBundleAnnotation := layer.Annotations["dev.sigstore.cosign/bundle"]; !hasBundleAnnotation {
			verifierOpts = append(verifierOpts, verify.WithNoObserverTimestamps())
		} else {
			verifierOpts = append(verifierOpts,
				verify.WithObserverTimestamps(1),
				verify.WithTransparencyLog(1),
			)
		}
		// signed with pubkey without cert identity
		anyCert = verify.WithoutIdentitiesUnsafe()
	} else {
		trustedRoot = fulcioRoot
		verifierOpts = append(verifierOpts,
			verify.WithObserverTimestamps(1),
			verify.WithTransparencyLog(1),
			verify.WithSignedCertificateTimestamps(1),
		)
	}
	gv, err := verify.NewVerifier(trustedRoot, verifierOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "creating verifier")
	}

	policy := verify.NewPolicy(artifactPolicy, anyCert)

	result, err := gv.Verify(se, policy)
	if err != nil {
		return nil, errors.Wrap(err, "verifying bundle")
	}

	if result.Signature == nil || (result.Signature.Certificate == nil && !sc.DHI) {
		return nil, errors.Errorf("no valid signatures found")
	}

	si := &types.SignatureInfo{
		TrustRootStatus: toRootStatus(st),
		Signer:          result.Signature.Certificate,
		Timestamps:      toTimestamps(result.VerifiedTimestamps),
		DockerReference: dockerReference,
		IsDHI:           sc.DHI,
		SignatureType:   sigType,
	}
	si.Kind = si.DetectKind()
	return si, nil
}

func (v *Verifier) loadTrustProvider() (*roots.TrustProvider, error) {
	var tpCache *roots.TrustProvider
	_, err, _ := v.sf.Do("", func() (any, error) {
		if v.tp != nil {
			tpCache = v.tp
			return nil, nil
		}
		tp, err := roots.NewTrustProvider(roots.SigstoreRootsConfig{
			CachePath:      filepath.Join(v.cfg.StateDir, "tuf"),
			UpdateInterval: v.cfg.UpdateInterval,
			RequireOnline:  v.cfg.RequireOnline,
		})
		if err != nil {
			return nil, err
		}
		v.tp = tp
		tpCache = tp
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	return tpCache, nil
}

func anyCerificateIdentity() (verify.PolicyOption, error) {
	sanMatcher, err := verify.NewSANMatcher("", ".*")
	if err != nil {
		return nil, err
	}

	issuerMatcher, err := verify.NewIssuerMatcher("", ".*")
	if err != nil {
		return nil, err
	}

	extensions := certificate.Extensions{}

	certID, err := verify.NewCertificateIdentity(sanMatcher, issuerMatcher, extensions)
	if err != nil {
		return nil, err
	}

	return verify.WithCertificateIdentity(certID), nil
}

type ArtifactVerifyOpts struct {
	SLSANotRequired bool
}

type ArtifactVerifyOpt func(*ArtifactVerifyOpts)

func WithSLSANotRequired() ArtifactVerifyOpt {
	return func(o *ArtifactVerifyOpts) {
		o.SLSANotRequired = true
	}
}

func loadBundle(dt []byte) (*bundle.Bundle, error) {
	var bundle bundle.Bundle
	bundle.Bundle = new(protobundle.Bundle)

	err := bundle.UnmarshalJSON(dt)
	if err != nil {
		return nil, err
	}

	return &bundle, nil
}

func rawDigest(d digest.Digest) (string, []byte, error) {
	alg := d.Algorithm().String()
	b, err := hex.DecodeString(d.Encoded())
	if err != nil {
		return "", nil, errors.Wrapf(err, "decoding digest %s", d)
	}
	return alg, b, nil
}

func isSLSAPredicateType(v string) bool {
	switch v {
	case slsa1.PredicateSLSAProvenance, slsa02.PredicateSLSAProvenance:
		return true
	default:
		return false
	}
}

func toTimestamps(ts []verify.TimestampVerificationResult) []types.TimestampVerificationResult {
	tsout := make([]types.TimestampVerificationResult, len(ts))
	for i, t := range ts {
		tsout[i] = types.TimestampVerificationResult{
			Type:      t.Type,
			URI:       t.URI,
			Timestamp: t.Timestamp,
		}
	}
	return tsout
}

func toRootStatus(st roots.Status) types.TrustRootStatus {
	trs := types.TrustRootStatus{
		LastUpdated: st.LastUpdated,
	}
	if st.Error != nil {
		trs.Error = st.Error.Error()
	}
	return trs
}
