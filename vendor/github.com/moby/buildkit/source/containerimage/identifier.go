package containerimage

import (
	"github.com/containerd/containerd/reference"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/solver/llbsolver/provenance"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/resolver"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ImageIdentifier struct {
	Reference   reference.Spec
	Platform    *ocispecs.Platform
	ResolveMode resolver.ResolveMode
	RecordType  client.UsageRecordType
	LayerLimit  *int
}

func NewImageIdentifier(str string) (*ImageIdentifier, error) {
	ref, err := reference.Parse(str)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if ref.Object == "" {
		return nil, errors.WithStack(reference.ErrObjectRequired)
	}
	return &ImageIdentifier{Reference: ref}, nil
}

var _ source.Identifier = (*ImageIdentifier)(nil)

func (*ImageIdentifier) Scheme() string {
	return srctypes.DockerImageScheme
}

func (id *ImageIdentifier) Capture(c *provenance.Capture, pin string) error {
	dgst, err := digest.Parse(pin)
	if err != nil {
		return errors.Wrapf(err, "failed to parse image digest %s", pin)
	}
	c.AddImage(provenancetypes.ImageSource{
		Ref:      id.Reference.String(),
		Platform: id.Platform,
		Digest:   dgst,
	})
	return nil
}

type OCIIdentifier struct {
	Reference  reference.Spec
	Platform   *ocispecs.Platform
	SessionID  string
	StoreID    string
	LayerLimit *int
}

func NewOCIIdentifier(str string) (*OCIIdentifier, error) {
	ref, err := reference.Parse(str)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if ref.Object == "" {
		return nil, errors.WithStack(reference.ErrObjectRequired)
	}
	return &OCIIdentifier{Reference: ref}, nil
}

var _ source.Identifier = (*OCIIdentifier)(nil)

func (*OCIIdentifier) Scheme() string {
	return srctypes.OCIScheme
}

func (id *OCIIdentifier) Capture(c *provenance.Capture, pin string) error {
	dgst, err := digest.Parse(pin)
	if err != nil {
		return errors.Wrapf(err, "failed to parse OCI digest %s", pin)
	}
	c.AddImage(provenancetypes.ImageSource{
		Ref:      id.Reference.String(),
		Platform: id.Platform,
		Digest:   dgst,
		Local:    true,
	})
	return nil
}
