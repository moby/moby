package containerd

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes"
	cerrdefs "github.com/containerd/errdefs"
	imagetypes "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/moby/moby/v2/errdefs"
	policyimage "github.com/moby/policy-helpers/image"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// inTotoPredicateTypeAnnotation is the OCI layer annotation key that BuildKit
// uses to record the in-toto predicate type of a statement layer inside an
// attestation manifest.
const inTotoPredicateTypeAnnotation = "in-toto.io/predicate-type"

// localReferrersProvider adapts a local content.Provider to the
// policyimage.ReferrersProvider interface. FetchReferrers is a no-op because
// moby's local content store cannot resolve OCI referrers; this means DHI image
// resolution and Sigstore signature manifest discovery are not supported.
// Standard BuildKit attestations are attached as sibling manifests in the
// image index and do not require FetchReferrers.
type localReferrersProvider struct {
	content.Provider
}

func (p *localReferrersProvider) FetchReferrers(ctx context.Context, dgst digest.Digest, opts ...remotes.FetchReferrersOpt) ([]ocispec.Descriptor, error) {
	return nil, nil
}

// ImageAttestations returns the in-toto attestation statements attached to the
// given image for the specified platform.
//
// The chain walk (locating the image manifest for the platform and the
// associated attestation manifest) is delegated to
// policyimage.ResolveSignatureChain so that moby and BuildKit agree on how to
// interpret the attestation storage format. The statement-layer iteration and
// blob reading is inlined: when statement bodies are requested it fails fast
// on the first unreadable blob, reads matching blobs eagerly into memory, and
// produces AttestationStatement values directly for
// GET /images/{name}/attestations.
//
// Behaviour:
//   - If the image is not an OCI index (no possibility of sibling attestation
//     manifests), returns (nil, nil).
//   - If the requested platform has no matching image manifest, returns
//     errdefs.NotFound.
//   - If the platform image manifest has no associated attestation manifest,
//     returns (nil, nil).
//   - Layers without an in-toto.io/predicate-type annotation are skipped.
//   - When predicateTypes is empty, all annotated statement layers are
//     returned. When non-empty, only matching ones are returned. If no
//     layers match, returns (nil, nil).
//   - When IncludeStatement is true the statement blob is read and attached
//     to each returned entry; a read failure propagates as an error. When
//     false, statement blobs are never read and the Statement field is left
//     nil.
func (i *ImageService) ImageAttestations(ctx context.Context, refOrID string, opts imagebackend.AttestationOpts) ([]imagetypes.AttestationStatement, error) {
	img, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	// Only image indexes can carry sibling attestation manifests. This matches
	// the entry check in policyimage.ResolveSignatureChain, which rejects any other
	// media type (see github.com/moby/policy-helpers/blob/main/image/resolve.go).
	if img.Target.MediaType != ocispec.MediaTypeImageIndex {
		return nil, nil
	}

	prov := &localReferrersProvider{Provider: i.content}

	sc, err := policyimage.ResolveSignatureChain(ctx, prov, img.Target, opts.Platform)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, errdefs.NotFound(err)
		}
		return nil, err
	}

	if sc.AttestationManifest == nil {
		return nil, nil
	}

	mfst, err := sc.OCIManifest(ctx, sc.AttestationManifest)
	if err != nil {
		return nil, err
	}

	var statements []imagetypes.AttestationStatement
	for _, layer := range mfst.Layers {
		predicateType := layer.Annotations[inTotoPredicateTypeAnnotation]
		if predicateType == "" {
			continue
		}
		if len(opts.PredicateTypes) > 0 && !slices.Contains(opts.PredicateTypes, predicateType) {
			continue
		}
		stmt := imagetypes.AttestationStatement{
			Descriptor:    layer,
			PredicateType: predicateType,
		}
		if opts.IncludeStatement {
			data, err := content.ReadBlob(ctx, i.content, layer)
			if err != nil {
				return nil, err
			}
			raw := json.RawMessage(data)
			stmt.Statement = &raw
		}
		statements = append(statements, stmt)
	}

	if len(statements) == 0 {
		return nil, nil
	}
	return statements, nil
}
