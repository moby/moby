package errdefs

import (
	"bytes"
	"errors"

	"github.com/containerd/typeurl"
	"github.com/golang/protobuf/jsonpb" //nolint:staticcheck
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/grpcerrors"
)

func init() {
	typeurl.Register((*Solve)(nil), "github.com/moby/buildkit", "errdefs.Solve+json")
}

//nolint:golint
type IsSolve_Subject isSolve_Subject

// SolveError will be returned when an error is encountered during a solve that
// has an exec op.
type SolveError struct {
	Solve
	Err error
}

func (e *SolveError) Error() string {
	return e.Err.Error()
}

func (e *SolveError) Unwrap() error {
	return e.Err
}

func (e *SolveError) ToProto() grpcerrors.TypedErrorProto {
	return &e.Solve
}

func WithSolveError(err error, subject IsSolve_Subject, inputIDs, mountIDs []string) error {
	if err == nil {
		return nil
	}
	var (
		oe *OpError
		op *pb.Op
	)
	if errors.As(err, &oe) {
		op = oe.Op
	}
	return &SolveError{
		Err: err,
		Solve: Solve{
			InputIDs: inputIDs,
			MountIDs: mountIDs,
			Op:       op,
			Subject:  subject,
		},
	}
}

func (v *Solve) WrapError(err error) error {
	return &SolveError{Err: err, Solve: *v}
}

func (v *Solve) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{}
	buf := new(bytes.Buffer)
	err := m.Marshal(buf, v)
	return buf.Bytes(), err
}

func (v *Solve) UnmarshalJSON(b []byte) error {
	return jsonpb.Unmarshal(bytes.NewReader(b), v)
}
