package registry

import (
	"net/url"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/pkg/errors"
)

func translateV2AuthError(err error) error {
	switch e := err.(type) {
	case *url.Error:
		switch e2 := e.Err.(type) {
		case errcode.Error:
			switch e2.Code {
			case errcode.ErrorCodeUnauthorized:
				return unauthorizedErr{err}
			}
		}
	}

	return err
}

func invalidParam(err error) error {
	return invalidParameterErr{err}
}

func invalidParamf(format string, args ...interface{}) error {
	return invalidParameterErr{errors.Errorf(format, args...)}
}

func invalidParamWrapf(err error, format string, args ...interface{}) error {
	return invalidParameterErr{errors.Wrapf(err, format, args...)}
}

type unauthorizedErr struct{ error }

func (unauthorizedErr) Unauthorized() {}

func (e unauthorizedErr) Cause() error {
	return e.error
}

func (e unauthorizedErr) Unwrap() error {
	return e.error
}

type invalidParameterErr struct{ error }

func (invalidParameterErr) InvalidParameter() {}

func (e invalidParameterErr) Unwrap() error {
	return e.error
}

type systemErr struct{ error }

func (systemErr) System() {}

func (e systemErr) Unwrap() error {
	return e.error
}

type errUnknown struct{ error }

func (errUnknown) Unknown() {}

func (e errUnknown) Unwrap() error {
	return e.error
}
