package errdefs // import "github.com/docker/docker/errdefs"

type causer interface {
	Cause() error
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
	default:
		return err
	}
}

// IsHTTPOnlyError checks whether err is one of the un-exported HTTP error type provided by this package. These errors
// only define what HTTP status code should be returned, but don't add additional context to the actual error they
// wrap. By contrast, error types implementing the interfaces defined in this package are also adding such additional
// context.
func IsHTTPOnlyError(err error) bool {
	switch err.(type) {
	case
		errNotFound,
		errInvalidParameter,
		errConflict,
		errUnauthorized,
		errUnavailable,
		errForbidden,
		errSystem,
		errNotModified,
		errNotImplemented,
		errCancelled,
		errDeadline,
		errDataLoss,
		errUnknown:
		return true
	default:
		return false
	}
}

// IsNotFound returns if the passed in error is an ErrNotFound
func IsNotFound(err error) bool {
	_, ok := getImplementer(err).(ErrNotFound)
	return ok
}

// IsInvalidParameter returns if the passed in error is an ErrInvalidParameter
func IsInvalidParameter(err error) bool {
	_, ok := getImplementer(err).(ErrInvalidParameter)
	return ok
}

// IsConflict returns if the passed in error is an ErrConflict
func IsConflict(err error) bool {
	_, ok := getImplementer(err).(ErrConflict)
	return ok
}

// IsUnauthorized returns if the passed in error is an ErrUnauthorized
func IsUnauthorized(err error) bool {
	_, ok := getImplementer(err).(ErrUnauthorized)
	return ok
}

// IsUnavailable returns if the passed in error is an ErrUnavailable
func IsUnavailable(err error) bool {
	_, ok := getImplementer(err).(ErrUnavailable)
	return ok
}

// IsForbidden returns if the passed in error is an ErrForbidden
func IsForbidden(err error) bool {
	_, ok := getImplementer(err).(ErrForbidden)
	return ok
}

// IsSystem returns if the passed in error is an ErrSystem
func IsSystem(err error) bool {
	_, ok := getImplementer(err).(ErrSystem)
	return ok
}

// IsNotModified returns if the passed in error is a NotModified error
func IsNotModified(err error) bool {
	_, ok := getImplementer(err).(ErrNotModified)
	return ok
}

// IsNotImplemented returns if the passed in error is an ErrNotImplemented
func IsNotImplemented(err error) bool {
	_, ok := getImplementer(err).(ErrNotImplemented)
	return ok
}

// IsUnknown returns if the passed in error is an ErrUnknown
func IsUnknown(err error) bool {
	_, ok := getImplementer(err).(ErrUnknown)
	return ok
}

// IsCancelled returns if the passed in error is an ErrCancelled
func IsCancelled(err error) bool {
	_, ok := getImplementer(err).(ErrCancelled)
	return ok
}

// IsDeadline returns if the passed in error is an ErrDeadline
func IsDeadline(err error) bool {
	_, ok := getImplementer(err).(ErrDeadline)
	return ok
}

// IsDataLoss returns if the passed in error is an ErrDataLoss
func IsDataLoss(err error) bool {
	_, ok := getImplementer(err).(ErrDataLoss)
	return ok
}
