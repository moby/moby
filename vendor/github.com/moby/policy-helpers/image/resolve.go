package image

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"sync"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ReferrersProvider interface {
	content.Provider
	remotes.ReferrersFetcher
}

const (
	AnnotationDockerReferenceDigest = "vnd.docker.reference.digest"
	AnnotationDockerReferenceType   = "vnd.docker.reference.type"
	AttestationManifestType         = "attestation-manifest"
)

const (
	ArtifactTypeCosignSignature   = "application/vnd.dev.cosign.artifact.sig.v1+json"
	ArtifactTypeSigstoreBundle    = "application/vnd.dev.sigstore.bundle.v0.3+json"
	ArtifactTypeInTotoJSON        = "application/vnd.in-toto+json"
	MediaTypeCosignSimpleSigning  = "application/vnd.dev.cosign.simplesigning.v1+json"
	SLSAProvenancePredicateType02 = "https://slsa.dev/provenance/v0.2"
	SLSAProvenancePredicateType1  = "https://slsa.dev/provenance/v1"
)

func resolveImageManifest(idx ocispecs.Index, platform ocispecs.Platform) (ocispecs.Descriptor, error) {
	pMatcher := platforms.Only(platform)

	var descs []ocispecs.Descriptor
	for _, d := range idx.Manifests {
		// TODO: confirm handling of nested indexes
		if !images.IsManifestType(d.MediaType) {
			continue
		}
		if d.Platform == nil || pMatcher.Match(*d.Platform) {
			descs = append(descs, d)
		}
	}

	sort.SliceStable(descs, func(i, j int) bool {
		if descs[i].Platform == nil {
			return false
		}
		if descs[j].Platform == nil {
			return true
		}
		return pMatcher.Less(*descs[i].Platform, *descs[j].Platform)
	})

	if len(descs) == 0 {
		return ocispecs.Descriptor{}, errors.Wrapf(cerrdefs.ErrNotFound, "no manifest for platform %+v", platforms.FormatAll(platform))
	}
	return descs[0], nil
}

type Manifest struct {
	ocispecs.Descriptor
	mu       sync.Mutex
	manifest *ocispecs.Manifest
	data     []byte
}

type SignatureChain struct {
	ImageManifest       *Manifest
	AttestationManifest *Manifest
	SignatureManifest   *Manifest
	Provider            content.Provider
	DHI                 bool
}

func (sc *SignatureChain) ManifestBytes(ctx context.Context, m *Manifest) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data != nil {
		return m.data, nil
	}
	dt, err := ReadBlob(ctx, sc.Provider, m.Descriptor)
	if err != nil {
		return nil, err
	}
	m.data = dt
	return dt, nil
}

func (sc *SignatureChain) OCIManifest(ctx context.Context, m *Manifest) (*ocispecs.Manifest, error) {
	m.mu.Lock()
	if m.manifest != nil {
		m.mu.Unlock()
		return m.manifest, nil
	}
	m.mu.Unlock()
	dt, err := sc.ManifestBytes(ctx, m)
	if err != nil {
		return nil, err
	}
	var manifest ocispecs.Manifest
	if err := json.Unmarshal(dt, &manifest); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling manifest %s", m.Digest)
	}
	m.mu.Lock()
	m.manifest = &manifest
	m.mu.Unlock()
	return &manifest, nil
}

func ResolveSignatureChain(ctx context.Context, provider ReferrersProvider, desc ocispecs.Descriptor, platform *ocispecs.Platform) (*SignatureChain, error) {
	if desc.MediaType != ocispecs.MediaTypeImageIndex {
		return nil, errors.Errorf("expected image index descriptor, got %s", desc.MediaType)
	}

	dt, err := ReadBlob(ctx, provider, desc)
	if err != nil {
		return nil, err
	}
	var index ocispecs.Index
	if err := json.Unmarshal(dt, &index); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling image index")
	}

	isDHI := isDHIIndex(index)

	if platform == nil {
		p := platforms.Normalize(platforms.DefaultSpec())
		platform = &p
	}

	manifestDesc, err := resolveImageManifest(index, *platform)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving image manifest for platform %+v", platform)
	}

	var attestationDesc *ocispecs.Descriptor
	if isDHI {
		provider = &dhiReferrersProvider{ReferrersProvider: provider}
		allRefs, err := provider.FetchReferrers(ctx, manifestDesc.Digest,
			remotes.WithReferrerArtifactTypes(ArtifactTypeInTotoJSON),
			remotes.WithReferrerQueryFilter("predicateType", SLSAProvenancePredicateType02),
			remotes.WithReferrerQueryFilter("predicateType", SLSAProvenancePredicateType1),
		)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching referrers for manifest %s", manifestDesc.Digest)
		}
		refs := make([]ocispecs.Descriptor, 0, len(allRefs))
		for _, r := range allRefs {
			if r.ArtifactType == ArtifactTypeInTotoJSON {
				switch r.Annotations["in-toto.io/predicate-type"] {
				case SLSAProvenancePredicateType02, SLSAProvenancePredicateType1:
					refs = append(refs, r)
				}
			}
		}
		if len(refs) == 0 {
			return nil, errors.Errorf("no attestation referrers found for DHI manifest %s", manifestDesc.Digest)
		}
		attestationDesc = &refs[0]
	} else {
		for _, d := range index.Manifests {
			if d.Annotations[AnnotationDockerReferenceType] == AttestationManifestType && d.Annotations[AnnotationDockerReferenceDigest] == manifestDesc.Digest.String() {
				attestationDesc = &d
				break
			}
		}
	}
	sh := &SignatureChain{
		ImageManifest: &Manifest{
			Descriptor: manifestDesc,
		},
		Provider: provider,
		DHI:      isDHI,
	}

	if attestationDesc == nil {
		return sh, nil
	}

	sh.AttestationManifest = &Manifest{
		Descriptor: *attestationDesc,
	}

	// currently not setting WithReferrerArtifactTypes in here as some registries(e.g. aws) don't know how to filter two types at once.
	allRefs, err := provider.FetchReferrers(ctx, attestationDesc.Digest)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching referrers for attestation manifest %s", attestationDesc.Digest)
	}

	refs := make([]ocispecs.Descriptor, 0, len(allRefs))
	for _, r := range allRefs {
		if r.ArtifactType == ArtifactTypeSigstoreBundle || r.ArtifactType == ArtifactTypeCosignSignature {
			refs = append(refs, r)
		}
	}

	if len(refs) == 0 {
		return sh, nil
	}

	// only allowing one signature manifest for now
	// if multiple are found, prefer bundle format
	slices.SortStableFunc(refs, func(a, b ocispecs.Descriptor) int {
		aIsBundle := a.ArtifactType == ArtifactTypeSigstoreBundle
		bIsBundle := b.ArtifactType == ArtifactTypeSigstoreBundle
		if aIsBundle && !bIsBundle {
			return -1
		} else if !aIsBundle && bIsBundle {
			return 1
		}
		return 0
	})

	sh.SignatureManifest = &Manifest{
		Descriptor: refs[0],
	}
	return sh, nil
}

func ReadBlob(ctx context.Context, provider content.Provider, desc ocispecs.Descriptor) ([]byte, error) {
	dt, err := content.ReadBlob(ctx, provider, desc)
	if err != nil {
		return nil, errors.Wrapf(err, "reading blob %s", desc.Digest)
	}
	if desc.Digest != digest.FromBytes(dt) {
		return nil, errors.Wrapf(err, "digest mismatch for blob %s", desc.Digest)
	}
	return dt, nil
}
