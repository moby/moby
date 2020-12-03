package errdefs

import "github.com/moby/buildkit/solver/pb"

type OpError struct {
	error
	Op *pb.Op
}

func (e *OpError) Unwrap() error {
	return e.error
}

func WithOp(err error, iface interface{}) error {
	op, ok := iface.(*pb.Op)
	if err == nil || !ok {
		return err
	}
	return &OpError{error: err, Op: op}
}
