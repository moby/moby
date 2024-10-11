package errdefs

import (
	fmt "fmt"

	"github.com/containerd/typeurl/v2"
	"github.com/moby/buildkit/util/grpcerrors"
)

func init() {
	typeurl.Register((*FrontendCap)(nil), "github.com/moby/buildkit", "errdefs.FrontendCap+json")
}

type UnsupportedFrontendCapError struct {
	*FrontendCap
	error
}

func (e *UnsupportedFrontendCapError) Error() string {
	msg := fmt.Sprintf("unsupported frontend capability %s", e.FrontendCap.Name)
	if e.error != nil {
		msg += ": " + e.error.Error()
	}
	return msg
}

func (e *UnsupportedFrontendCapError) Unwrap() error {
	return e.error
}

func (e *UnsupportedFrontendCapError) ToProto() grpcerrors.TypedErrorProto {
	return e.FrontendCap
}

func NewUnsupportedFrontendCapError(name string) error {
	return &UnsupportedFrontendCapError{FrontendCap: &FrontendCap{Name: name}}
}

func (v *FrontendCap) WrapError(err error) error {
	return &UnsupportedFrontendCapError{error: err, FrontendCap: v}
}
