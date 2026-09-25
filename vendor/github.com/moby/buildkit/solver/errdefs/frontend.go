package errdefs

import (
	"errors"
	"fmt"

	"github.com/containerd/typeurl/v2"
	"github.com/moby/buildkit/util/grpcerrors"
)

func init() {
	typeurl.Register((*Frontend)(nil), "github.com/moby/buildkit", "errdefs.Frontend+json")
}

type FrontendError struct {
	*Frontend
	error
}

func (e *FrontendError) Error() string {
	// These can be nested, so avoid adding any details to the error message
	// if we already have an error. Otherwise the resulting error message
	// can be very long and not very useful.
	if e.error != nil {
		return e.error.Error()
	}
	return fmt.Sprintf("frontend %s failed", e.Name)
}

func (e *FrontendError) Unwrap() error {
	return e.error
}

func (e *FrontendError) ToProto() grpcerrors.TypedErrorProto {
	return e.Frontend
}

func (v *Frontend) WrapError(err error) error {
	return &FrontendError{error: err, Frontend: v}
}

func Frontends(err error) []*Frontend {
	var out []*Frontend
	var es *FrontendError
	if errors.As(err, &es) {
		out = Frontends(es.Unwrap())
		out = append(out, es.CloneVT())
	}
	return out
}
