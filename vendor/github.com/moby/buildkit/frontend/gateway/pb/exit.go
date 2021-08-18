package moby_buildkit_v1_frontend //nolint:golint

import (
	"fmt"

	"github.com/containerd/typeurl"
	"github.com/moby/buildkit/util/grpcerrors"
)

const (
	// UnknownExitStatus might be returned in (*ExitError).ExitCode via
	// ContainerProcess.Wait.  This can happen if the process never starts
	// or if an error was encountered when obtaining the exit status, it is set to 255.
	//
	// This const is defined here to prevent importing github.com/containerd/containerd
	// and corresponds with https://github.com/containerd/containerd/blob/40b22ef0741028917761d8c5d5d29e0d19038836/task.go#L52-L55
	UnknownExitStatus = 255
)

func init() {
	typeurl.Register((*ExitMessage)(nil), "github.com/moby/buildkit", "gatewayapi.ExitMessage+json")
}

// ExitError will be returned when the container process exits with a non-zero
// exit code.
type ExitError struct {
	ExitCode uint32
	Err      error
}

func (err *ExitError) ToProto() grpcerrors.TypedErrorProto {
	return &ExitMessage{
		Code: err.ExitCode,
	}
}

func (err *ExitError) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return fmt.Sprintf("exit code: %d", err.ExitCode)
}

func (err *ExitError) Unwrap() error {
	return err.Err
}

func (e *ExitMessage) WrapError(err error) error {
	return &ExitError{
		Err:      err,
		ExitCode: e.Code,
	}
}
