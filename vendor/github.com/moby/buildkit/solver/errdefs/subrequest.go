package errdefs

import (
	fmt "fmt"

	"github.com/containerd/typeurl/v2"
	"github.com/moby/buildkit/util/grpcerrors"
)

func init() {
	typeurl.Register((*Subrequest)(nil), "github.com/moby/buildkit", "errdefs.Subrequest+json")
}

type UnsupportedSubrequestError struct {
	*Subrequest
	error
}

func (e *UnsupportedSubrequestError) Error() string {
	msg := fmt.Sprintf("unsupported request %s", e.Subrequest.Name)
	if e.error != nil {
		msg += ": " + e.error.Error()
	}
	return msg
}

func (e *UnsupportedSubrequestError) Unwrap() error {
	return e.error
}

func (e *UnsupportedSubrequestError) ToProto() grpcerrors.TypedErrorProto {
	return e.Subrequest
}

func NewUnsupportedSubrequestError(name string) error {
	return &UnsupportedSubrequestError{Subrequest: &Subrequest{Name: name}}
}

func (v *Subrequest) WrapError(err error) error {
	return &UnsupportedSubrequestError{error: err, Subrequest: v}
}
