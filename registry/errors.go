package registry

import (
	"net/url"

	"github.com/docker/distribution/registry/api/errcode"
)

type notFoundError string

func (e notFoundError) Error() string {
	return string(e)
}

func (notFoundError) NotFound() {}

type validationError struct {
	cause error
}

func (e validationError) Error() string {
	return e.cause.Error()
}

func (e validationError) InvalidParameter() {}

func (e validationError) Cause() error {
	return e.cause
}

type unauthorizedError struct {
	cause error
}

func (e unauthorizedError) Error() string {
	return e.cause.Error()
}

func (e unauthorizedError) Unauthorized() {}

func (e unauthorizedError) Cause() error {
	return e.cause
}

type systemError struct {
	cause error
}

func (e systemError) Error() string {
	return e.cause.Error()
}

func (e systemError) SystemError() {}

func (e systemError) Cause() error {
	return e.cause
}

type notActivatedError struct {
	cause error
}

func (e notActivatedError) Error() string {
	return e.cause.Error()
}

func (e notActivatedError) Forbidden() {}

func (e notActivatedError) Cause() error {
	return e.cause
}

func translateV2AuthError(err error) error {
	switch e := err.(type) {
	case *url.Error:
		switch e2 := e.Err.(type) {
		case errcode.Error:
			switch e2.Code {
			case errcode.ErrorCodeUnauthorized:
				return unauthorizedError{err}
			}
		}
	}

	return err
}
