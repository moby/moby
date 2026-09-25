package sourceresolver

import (
	"context"
	"strings"

	"github.com/distribution/reference"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/imageutil"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type ImageMetaResolver interface {
	ResolveImageConfig(ctx context.Context, ref string, opt Opt) (string, digest.Digest, []byte, error)
}

type imageMetaResolver struct {
	mr MetaResolver
}

var _ ImageMetaResolver = &imageMetaResolver{}

func NewImageMetaResolver(mr MetaResolver) ImageMetaResolver {
	return &imageMetaResolver{
		mr: mr,
	}
}

func (imr *imageMetaResolver) ResolveImageConfig(ctx context.Context, ref string, opt Opt) (string, digest.Digest, []byte, error) {
	parsed, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return "", "", nil, errors.Wrapf(err, "could not parse reference %q", ref)
	}
	ref = parsed.String()
	op := &pb.SourceOp{
		Identifier: "docker-image://" + ref,
	}
	if opt := opt.OCILayoutOpt; opt != nil {
		op.Identifier = "oci-layout://" + ref
		op.Attrs = map[string]string{}
		if opt.Store.SessionID != "" {
			op.Attrs[pb.AttrOCILayoutSessionID] = opt.Store.SessionID
		}
		if opt.Store.StoreID != "" {
			op.Attrs[pb.AttrOCILayoutStoreID] = opt.Store.StoreID
		}
	}
	res, err := imr.mr.ResolveSourceMetadata(ctx, op, opt)
	if err != nil {
		return "", "", nil, errors.Wrapf(err, "failed to resolve source metadata for %s", ref)
	}
	if res.Image == nil {
		return "", "", nil, &imageutil.ResolveToNonImageError{Ref: ref, Updated: res.Op.Identifier}
	}
	ref = strings.TrimPrefix(res.Op.Identifier, "docker-image://")
	ref = strings.TrimPrefix(ref, "oci-layout://")
	return ref, res.Image.Digest, res.Image.Config, nil
}
