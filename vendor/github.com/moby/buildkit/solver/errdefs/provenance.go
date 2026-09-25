package errdefs

import (
	"github.com/containerd/typeurl/v2"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/pkg/errors"
)

func init() {
	typeurl.Register((*ProvenanceMaterialsIncomplete)(nil), "github.com/moby/buildkit", "errdefs.ProvenanceMaterialsIncomplete+json")
}

type ProvenanceMaterialsIncompleteError struct {
	*ProvenanceMaterialsIncomplete
	error
}

func (e *ProvenanceMaterialsIncompleteError) Unwrap() error {
	return e.error
}

func (e *ProvenanceMaterialsIncompleteError) ToProto() grpcerrors.TypedErrorProto {
	return e.ProvenanceMaterialsIncomplete
}

func (p *ProvenanceMaterialsIncomplete) WrapError(err error) error {
	return &ProvenanceMaterialsIncompleteError{
		error:                         err,
		ProvenanceMaterialsIncomplete: p,
	}
}

func WithProvenanceMaterialsIncomplete(err error, incomplete []*ProvenanceMaterialIncomplete) error {
	if err == nil {
		return nil
	}
	return &ProvenanceMaterialsIncompleteError{
		error: err,
		ProvenanceMaterialsIncomplete: &ProvenanceMaterialsIncomplete{
			Incomplete: incomplete,
		},
	}
}

func NewProvenanceMaterialsIncomplete(incomplete []*ProvenanceMaterialIncomplete) error {
	return WithProvenanceMaterialsIncomplete(errors.New("provenance materials are incomplete"), incomplete)
}
