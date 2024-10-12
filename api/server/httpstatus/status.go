package httpstatus // import "github.com/docker/docker/api/server/httpstatus"

import (
	"context"
	"fmt"
	"net/http"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/docker/errdefs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type causer interface {
	Cause() error
}

// FromError retrieves status code from error message.
func FromError(err error) int {
	if err == nil {
		log.G(context.TODO()).WithError(err).Error("unexpected HTTP error handling")
		return http.StatusInternalServerError
	}

	// Stop right there
	// Are you sure you should be adding a new error class here? Do one of the existing ones work?

	// Note that the below functions are already checking the error causal chain for matches.
	switch {
	case errdefs.IsNotFound(err):
		return http.StatusNotFound
	case errdefs.IsInvalidParameter(err):
		return http.StatusBadRequest
	case errdefs.IsConflict(err):
		return http.StatusConflict
	case errdefs.IsUnauthorized(err):
		return http.StatusUnauthorized
	case errdefs.IsUnavailable(err):
		return http.StatusServiceUnavailable
	case errdefs.IsForbidden(err):
		return http.StatusForbidden
	case errdefs.IsNotModified(err):
		return http.StatusNotModified
	case errdefs.IsNotImplemented(err):
		return http.StatusNotImplemented
	case errdefs.IsSystem(err) || errdefs.IsUnknown(err) || errdefs.IsDataLoss(err) || errdefs.IsDeadline(err) || errdefs.IsCancelled(err):
		return http.StatusInternalServerError
	default:
		if statusCode := statusCodeFromGRPCError(err); statusCode != http.StatusInternalServerError {
			return statusCode
		}
		if statusCode := statusCodeFromContainerdError(err); statusCode != http.StatusInternalServerError {
			return statusCode
		}
		if statusCode := statusCodeFromDistributionError(err); statusCode != http.StatusInternalServerError {
			return statusCode
		}
		if e, ok := err.(causer); ok {
			return FromError(e.Cause())
		}

		log.G(context.TODO()).WithFields(log.Fields{
			"module":     "api",
			"error":      err,
			"error_type": fmt.Sprintf("%T", err),
		}).Debug("FIXME: Got an API for which error does not match any expected type!!!")

		return http.StatusInternalServerError
	}
}

// statusCodeFromGRPCError returns status code according to gRPC error
func statusCodeFromGRPCError(err error) int {
	switch status.Code(err) {
	case codes.InvalidArgument: // code 3
		return http.StatusBadRequest
	case codes.NotFound: // code 5
		return http.StatusNotFound
	case codes.AlreadyExists: // code 6
		return http.StatusConflict
	case codes.PermissionDenied: // code 7
		return http.StatusForbidden
	case codes.FailedPrecondition: // code 9
		return http.StatusBadRequest
	case codes.Unauthenticated: // code 16
		return http.StatusUnauthorized
	case codes.OutOfRange: // code 11
		return http.StatusBadRequest
	case codes.Unimplemented: // code 12
		return http.StatusNotImplemented
	case codes.Unavailable: // code 14
		return http.StatusServiceUnavailable
	default:
		// codes.Canceled(1)
		// codes.Unknown(2)
		// codes.DeadlineExceeded(4)
		// codes.ResourceExhausted(8)
		// codes.Aborted(10)
		// codes.Internal(13)
		// codes.DataLoss(15)
		return http.StatusInternalServerError
	}
}

// statusCodeFromDistributionError returns status code according to registry errcode
// code is loosely based on errcode.ServeJSON() in docker/distribution
func statusCodeFromDistributionError(err error) int {
	switch errs := err.(type) {
	case errcode.Errors:
		if len(errs) < 1 {
			return http.StatusInternalServerError
		}
		if _, ok := errs[0].(errcode.ErrorCoder); ok {
			return statusCodeFromDistributionError(errs[0])
		}
	case errcode.ErrorCoder:
		return errs.ErrorCode().Descriptor().HTTPStatusCode
	}
	return http.StatusInternalServerError
}

// statusCodeFromContainerdError returns status code for containerd errors when
// consumed directly (not through gRPC)
func statusCodeFromContainerdError(err error) int {
	switch {
	case cerrdefs.IsInvalidArgument(err):
		return http.StatusBadRequest
	case cerrdefs.IsNotFound(err):
		return http.StatusNotFound
	case cerrdefs.IsAlreadyExists(err):
		return http.StatusConflict
	case cerrdefs.IsFailedPrecondition(err):
		return http.StatusPreconditionFailed
	case cerrdefs.IsUnavailable(err):
		return http.StatusServiceUnavailable
	case cerrdefs.IsNotImplemented(err):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
