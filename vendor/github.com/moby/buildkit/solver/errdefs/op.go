package errdefs

import "github.com/moby/buildkit/solver/pb"

type OpError struct {
	error
	Op          *pb.Op
	Description map[string]string
}

func (e *OpError) Unwrap() error {
	return e.error
}

func WithOp(err error, anyOp interface{}, opDesc map[string]string) error {
	op, ok := anyOp.(*pb.Op)
	if err == nil || !ok {
		return err
	}
	return &OpError{error: err, Op: op, Description: opDesc}
}
