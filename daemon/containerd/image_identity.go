package containerd

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/pkg/labels"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	imagetypes "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/v2/daemon/internal/builder-next/exporter"
	policyimage "github.com/moby/policy-helpers/image"
	policytypes "github.com/moby/policy-helpers/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func (i *ImageService) imageIdentity(ctx context.Context, desc ocispec.Descriptor, multi *multiPlatformSummary) (*imagetypes.Identity, error) {
	info, err := i.content.Info(ctx, desc.Digest)
	if err != nil {
		return nil, err
	}
	identity := &imagetypes.Identity{}

	seenRepos := make(map[string]struct{})

	for k, v := range info.Labels {
		if ref, ok := strings.CutPrefix(k, exporter.BuildRefLabel); ok {
			var val exporter.BuildRefLabelValue
			if err := json.Unmarshal([]byte(v), &val); err == nil {
				var createdAt time.Time
				if val.CreatedAt != nil {
					createdAt = *val.CreatedAt
				}
				identity.Build = append(identity.Build, imagetypes.BuildIdentity{
					Ref:       ref,
					CreatedAt: createdAt,
				})
			}
		}
		if registry, ok := strings.CutPrefix(k, labels.LabelDistributionSource+"."); ok {
			for repo := range strings.SplitSeq(v, ",") {
				ref, err := reference.ParseNormalizedNamed(registry + "/" + repo)
				if err != nil {
					log.G(ctx).WithError(err).Error("failed to parse image name as reference")
					continue
				}
				name := ref.Name()
				if _, ok := seenRepos[name]; ok {
					continue
				}
				seenRepos[name] = struct{}{}
				identity.Pull = append(identity.Pull, imagetypes.PullIdentity{
					Repository: name,
				})
			}
		}
	}

	if multi.Best != nil {
		si, err := i.signatureIdentity(ctx, desc, multi.Best, multi.BestPlatform)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to validate image signature")
		}
		if si != nil {
			identity.Signature = append(identity.Signature, *si)
		}
	}

	// return nil if there is no identity information
	if len(identity.Build) == 0 && len(identity.Pull) == 0 && len(identity.Signature) == 0 {
		return nil, nil
	}

	slices.SortFunc(identity.Build, func(a, b imagetypes.BuildIdentity) int {
		return cmp.Compare(a.Ref, b.Ref)
	})

	return identity, nil
}

func (i *ImageService) signatureIdentity(ctx context.Context, desc ocispec.Descriptor, img *ImageManifest, platform ocispec.Platform) (*imagetypes.SignatureIdentity, error) {
	rp := &referrersProvider{Store: i.content}
	sc, err := policyimage.ResolveSignatureChain(ctx, rp, desc, &platform)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving signature chain for image %s", desc.Digest)
	}
	if sc.SignatureManifest == nil || sc.ImageManifest == nil {
		return nil, nil
	}

	if sc.ImageManifest.Digest != img.Target().Digest {
		log.L.Infof("signature chain image manifest digest mismatch: %s != %s", sc.ImageManifest.Digest, img.Target().Digest)
		return nil, nil
	}

	v, err := i.policyVerifier()
	if err != nil {
		return nil, err
	}

	out := &imagetypes.SignatureIdentity{}

	si, err := v.VerifyImage(ctx, rp, desc, &platform)
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}

	out.Name = si.Name()
	out.DockerReference = si.DockerReference

	if si.IsDHI { // update upstream to also use known signer type instead of bool
		out.KnownSigner = imagetypes.KnownSignerDHI
	}

	switch si.SignatureType {
	case policytypes.SignatureBundleV03:
		out.SignatureType = imagetypes.SignatureTypeBundleV03
	case policytypes.SignatureSimpleSigningV1:
		out.SignatureType = imagetypes.SignatureTypeSimpleSigningV1
	}

	for _, ts := range si.Timestamps {
		out.Timestamps = append(out.Timestamps, imagetypes.SignatureTimestamp{
			Type:      imagetypes.SignatureTimestampType(ts.Type),
			URI:       ts.URI,
			Timestamp: ts.Timestamp,
		})
	}

	if signer := si.Signer; signer != nil {
		out.Signer = &imagetypes.SignerIdentity{
			CertificateIssuer:                   signer.CertificateIssuer,
			SubjectAlternativeName:              signer.SubjectAlternativeName,
			Issuer:                              signer.Issuer,
			BuildSignerURI:                      signer.BuildSignerURI,
			BuildSignerDigest:                   signer.BuildSignerDigest,
			RunnerEnvironment:                   signer.RunnerEnvironment,
			SourceRepositoryURI:                 signer.SourceRepositoryURI,
			SourceRepositoryDigest:              signer.SourceRepositoryDigest,
			SourceRepositoryRef:                 signer.SourceRepositoryRef,
			SourceRepositoryIdentifier:          signer.SourceRepositoryIdentifier,
			SourceRepositoryOwnerURI:            signer.SourceRepositoryOwnerURI,
			SourceRepositoryOwnerIdentifier:     signer.SourceRepositoryOwnerIdentifier,
			BuildConfigURI:                      signer.BuildConfigURI,
			BuildConfigDigest:                   signer.BuildConfigDigest,
			BuildTrigger:                        signer.BuildTrigger,
			RunInvocationURI:                    signer.RunInvocationURI,
			SourceRepositoryVisibilityAtSigning: signer.SourceRepositoryVisibilityAtSigning,
		}
	}

	if si.TrustRootStatus.Error != "" {
		out.Warnings = append(out.Warnings, si.TrustRootStatus.Error)
	}

	return out, nil
}

type referrersProvider struct {
	content.Store
}

var _ policyimage.ReferrersProvider = &referrersProvider{}

func (p *referrersProvider) FetchReferrers(ctx context.Context, dgst digest.Digest, opts ...remotes.FetchReferrersOpt) ([]ocispec.Descriptor, error) {
	rfe := &referrersForExport{store: p.Store}
	descs, err := rfe.Referrers(ctx, ocispec.Descriptor{Digest: dgst})
	if err != nil {
		return nil, errors.Wrapf(err, "fetching referrers for %s", dgst)
	}

	if len(descs) == 0 {
		return nil, nil
	}

	cfg := &remotes.FetchReferrersConfig{}
	for _, opt := range opts {
		if err := opt(ctx, cfg); err != nil {
			return nil, err
		}
	}
	filter := make(map[string]struct{})
	for _, t := range cfg.ArtifactTypes {
		filter[t] = struct{}{}
	}

	if len(filter) == 0 {
		return descs, nil
	}

	out := make([]ocispec.Descriptor, 0, len(descs))
	for _, d := range descs {
		if _, ok := filter[d.MediaType]; ok {
			out = append(out, d)
		}
	}
	return out, nil
}
