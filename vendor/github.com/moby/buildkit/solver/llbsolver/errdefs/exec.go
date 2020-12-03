package errdefs

import (
	"github.com/moby/buildkit/solver"
)

// ExecError will be returned when an error is encountered when evaluating an op.
type ExecError struct {
	error
	Inputs []solver.Result
	Mounts []solver.Result
}

func (e *ExecError) Unwrap() error {
	return e.error
}

func (e *ExecError) EachRef(fn func(solver.Result) error) (err error) {
	for _, res := range e.Inputs {
		if res == nil {
			continue
		}
		if err1 := fn(res); err1 != nil && err == nil {
			err = err1
		}
	}
	for _, res := range e.Mounts {
		if res == nil {
			continue
		}
		if err1 := fn(res); err1 != nil && err == nil {
			err = err1
		}
	}
	return err
}

func WithExecError(err error, inputs, mounts []solver.Result) error {
	if err == nil {
		return nil
	}
	return &ExecError{
		error:  err,
		Inputs: inputs,
		Mounts: mounts,
	}
}
