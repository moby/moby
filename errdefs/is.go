package errdefs

import (
	"context"
	"errors"
)

type causer interface {
	Cause() error
}

type wrapErr interface {
	Unwrap() error
}

type wrapErrs interface {
	Unwrap() []error
}

func getImplementer(err error) error {
	switch e := err.(type) {
	case
		ErrNotFound,
		ErrInvalidParameter,
		ErrConflict,
		ErrUnauthorized,
		ErrUnavailable,
		ErrForbidden,
		ErrSystem,
		ErrNotModified,
		ErrNotImplemented,
		ErrCancelled,
		ErrDeadline,
		ErrDataLoss,
		ErrUnknown:
		return err
	case causer:
		return getImplementer(e.Cause())
	case wrapErr:
		return getImplementer(e.Unwrap())
	case wrapErrs:
		for _, err := range e.Unwrap() {
			switch err := getImplementer(err).(type) {
			case
				ErrNotFound,
				ErrInvalidParameter,
				ErrConflict,
				ErrUnauthorized,
				ErrUnavailable,
				ErrForbidden,
				ErrSystem,
				ErrNotModified,
				ErrNotImplemented,
				ErrCancelled,
				ErrDeadline,
				ErrDataLoss,
				ErrUnknown:
				return err
			}
		}
		return err
	default:
		return err
	}
}

// IsNotFound returns true if the first instances of one of the expected types
// implements ErrNotFound, otherwise false.
func IsNotFound(err error) bool {
	_, ok := getImplementer(err).(ErrNotFound)
	return ok
}

// IsInvalidParameter true if the first instances of one of the expected types
// implements ErrInvalidParameter, otherwise false.
func IsInvalidParameter(err error) bool {
	_, ok := getImplementer(err).(ErrInvalidParameter)
	return ok
}

// IsConflict true if the first instances of one of the expected types
// implements ErrConflict, otherwise false.
func IsConflict(err error) bool {
	_, ok := getImplementer(err).(ErrConflict)
	return ok
}

// IsUnauthorized true if the first instances of one of the expected types
// implements ErrUnauthorized, otherwise false.
func IsUnauthorized(err error) bool {
	_, ok := getImplementer(err).(ErrUnauthorized)
	return ok
}

// IsUnavailable true if the first instances of one of the expected types
// implements ErrUnavailable, otherwise false.
func IsUnavailable(err error) bool {
	_, ok := getImplementer(err).(ErrUnavailable)
	return ok
}

// IsForbidden true if the first instances of one of the expected types
// implements ErrForbidden, otherwise false.
func IsForbidden(err error) bool {
	_, ok := getImplementer(err).(ErrForbidden)
	return ok
}

// IsSystem true if the first instances of one of the expected types
// implements ErrSystem, otherwise false.
func IsSystem(err error) bool {
	_, ok := getImplementer(err).(ErrSystem)
	return ok
}

// IsNotModified returns if the passed in error is a NotModified error
func IsNotModified(err error) bool {
	_, ok := getImplementer(err).(ErrNotModified)
	return ok
}

// IsNotImplemented true if the first instances of one of the expected types
// implements ErrNotImplemented, otherwise false.
func IsNotImplemented(err error) bool {
	_, ok := getImplementer(err).(ErrNotImplemented)
	return ok
}

// IsUnknown true if the first instances of one of the expected types
// implements ErrUnknown, otherwise false.
func IsUnknown(err error) bool {
	_, ok := getImplementer(err).(ErrUnknown)
	return ok
}

// IsCancelled true if the first instances of one of the expected types
// implements ErrCancelled, otherwise false.
func IsCancelled(err error) bool {
	_, ok := getImplementer(err).(ErrCancelled)
	return ok
}

// IsDeadline true if the first instances of one of the expected types
// implements ErrDeadline, otherwise false.
func IsDeadline(err error) bool {
	_, ok := getImplementer(err).(ErrDeadline)
	return ok
}

// IsDataLoss true if the first instances of one of the expected types
// implements ErrDataLoss, otherwise false.
func IsDataLoss(err error) bool {
	_, ok := getImplementer(err).(ErrDataLoss)
	return ok
}

// IsContext returns if the passed in error is due to context cancellation or deadline exceeded.
func IsContext(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
