package errdefs

import (
	"context"
	"runtime"

	"github.com/moby/buildkit/solver"
	"github.com/sirupsen/logrus"
)

// ExecError will be returned when an error is encountered when evaluating an op.
type ExecError struct {
	error
	Inputs        []solver.Result
	Mounts        []solver.Result
	OwnerBorrowed bool
}

func (e *ExecError) Unwrap() error {
	return e.error
}

func (e *ExecError) EachRef(fn func(solver.Result) error) (err error) {
	m := map[solver.Result]struct{}{}
	for _, res := range e.Inputs {
		if res == nil {
			continue
		}
		if _, ok := m[res]; ok {
			continue
		}
		m[res] = struct{}{}
		if err1 := fn(res); err1 != nil && err == nil {
			err = err1
		}
	}
	for _, res := range e.Mounts {
		if res == nil {
			continue
		}
		if _, ok := m[res]; ok {
			continue
		}
		m[res] = struct{}{}
		if err1 := fn(res); err1 != nil && err == nil {
			err = err1
		}
	}
	return err
}

func (e *ExecError) Release() error {
	if e.OwnerBorrowed {
		return nil
	}
	err := e.EachRef(func(r solver.Result) error {
		r.Release(context.TODO())
		return nil
	})
	e.OwnerBorrowed = true
	return err
}

func WithExecError(err error, inputs, mounts []solver.Result) error {
	if err == nil {
		return nil
	}
	ee := &ExecError{
		error:  err,
		Inputs: inputs,
		Mounts: mounts,
	}
	runtime.SetFinalizer(ee, func(e *ExecError) {
		if !e.OwnerBorrowed {
			e.EachRef(func(r solver.Result) error {
				logrus.Warn("leaked execError detected and released")
				r.Release(context.TODO())
				return nil
			})
		}
	})
	return ee
}
