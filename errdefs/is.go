package errdefs

import cerrdefs "github.com/containerd/errdefs"

// IsNotFound returns if the passed in error is an [ErrNotFound],
func IsNotFound(err error) bool {
	return cerrdefs.IsNotFound(err)
}

// IsInvalidParameter returns if the passed in error is an [ErrInvalidParameter].
func IsInvalidParameter(err error) bool {
	return cerrdefs.IsInvalidArgument(err)
}

// IsConflict returns if the passed in error is an [ErrConflict].
func IsConflict(err error) bool {
	return cerrdefs.IsConflict(err)
}

// IsUnauthorized returns if the passed in error is an [ErrUnauthorized].
func IsUnauthorized(err error) bool {
	return cerrdefs.IsUnauthorized(err)
}

// IsUnavailable returns if the passed in error is an [ErrUnavailable].
func IsUnavailable(err error) bool {
	return cerrdefs.IsUnavailable(err)
}

// IsForbidden returns if the passed in error is an [ErrForbidden].
func IsForbidden(err error) bool {
	return cerrdefs.IsPermissionDenied(err)
}

// IsSystem returns if the passed in error is an [ErrSystem].
func IsSystem(err error) bool {
	return cerrdefs.IsInternal(err)
}

// IsNotModified returns if the passed in error is an [ErrNotModified].
func IsNotModified(err error) bool {
	return cerrdefs.IsNotModified(err)
}

// IsNotImplemented returns if the passed in error is an [ErrNotImplemented].
func IsNotImplemented(err error) bool {
	return cerrdefs.IsNotImplemented(err)
}

// IsUnknown returns if the passed in error is an [ErrUnknown].
func IsUnknown(err error) bool {
	return cerrdefs.IsUnknown(err)
}

// IsCancelled returns if the passed in error is an [ErrCancelled].
func IsCancelled(err error) bool {
	return cerrdefs.IsCanceled(err)
}

// IsDeadline returns if the passed in error is an [ErrDeadline].
func IsDeadline(err error) bool {
	return cerrdefs.IsDeadlineExceeded(err)
}

// IsDataLoss returns if the passed in error is an [ErrDataLoss].
func IsDataLoss(err error) bool {
	return cerrdefs.IsDataLoss(err)
}

// IsContext returns if the passed in error is due to context cancellation or deadline exceeded.
func IsContext(err error) bool {
	return cerrdefs.IsCanceled(err) || cerrdefs.IsDeadlineExceeded(err)
}
