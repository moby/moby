package errdefs

import (
	"context"
	"errors"

	cerrdefs "github.com/containerd/errdefs"
)

// IsNotFound returns if the passed in error is an ErrNotFound
//
// Deprecated: use containerd [errdefs.IsNotFound]
var IsNotFound = cerrdefs.IsNotFound

// IsInvalidParameter returns if the passed in error is an ErrInvalidParameter
//
// Deprecated: use containerd [errdefs.IsInvalidArgument]
var IsInvalidParameter = cerrdefs.IsInvalidArgument

// IsConflict returns if the passed in error is an ErrConflict
//
// Deprecated: use containerd [errdefs.IsConflict]
var IsConflict = cerrdefs.IsConflict

// IsUnauthorized returns if the passed in error is an ErrUnauthorized
//
// Deprecated: use containerd [errdefs.IsUnauthorized]
var IsUnauthorized = cerrdefs.IsUnauthorized

// IsUnavailable returns if the passed in error is an ErrUnavailable
//
// Deprecated: use containerd [errdefs.IsUnavailable]
var IsUnavailable = cerrdefs.IsUnavailable

// IsForbidden returns if the passed in error is an ErrForbidden
//
// Deprecated: use containerd [errdefs.IsPermissionDenied]
var IsForbidden = cerrdefs.IsPermissionDenied

// IsSystem returns if the passed in error is an ErrSystem
//
// Deprecated: use containerd [errdefs.IsInternal]
var IsSystem = cerrdefs.IsInternal

// IsNotModified returns if the passed in error is a NotModified error
//
// Deprecated: use containerd [errdefs.IsNotModified]
var IsNotModified = cerrdefs.IsNotModified

// IsNotImplemented returns if the passed in error is an ErrNotImplemented
//
// Deprecated: use containerd [errdefs.IsNotImplemented]
var IsNotImplemented = cerrdefs.IsNotImplemented

// IsUnknown returns if the passed in error is an ErrUnknown
//
// Deprecated: use containerd [errdefs.IsUnknown]
var IsUnknown = cerrdefs.IsUnknown

// IsCancelled returns if the passed in error is an ErrCancelled
//
// Deprecated: use containerd [errdefs.IsCanceled]
var IsCancelled = cerrdefs.IsCanceled

// IsDeadline returns if the passed in error is an ErrDeadline
//
// Deprecated: use containerd [errdefs.IsDeadlineExceeded]
var IsDeadline = cerrdefs.IsDeadlineExceeded

// IsDataLoss returns if the passed in error is an ErrDataLoss
//
// Deprecated: use containerd [errdefs.IsDataLoss]
var IsDataLoss = cerrdefs.IsDataLoss

// IsContext returns if the passed in error is due to context cancellation or deadline exceeded.
func IsContext(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
