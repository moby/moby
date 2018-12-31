package httputils // import "github.com/docker/docker/api/server/httputils"

import (
	"fmt"
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/errdefs"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type causer interface {
	Cause() error
}

// GetHTTPErrorStatusCode retrieves status code from error message.
func GetHTTPErrorStatusCode(err error) int {
	if err == nil {
		logrus.WithFields(logrus.Fields{"error": err}).Error("unexpected HTTP error handling")
		return http.StatusInternalServerError
	}

	var statusCode int

	// Stop right there
	// Are you sure you should be adding a new error class here? Do one of the existing ones work?

	// Note that the below functions are already checking the error causal chain for matches.
	switch {
	case errdefs.IsNotFound(err):
		statusCode = http.StatusNotFound
	case errdefs.IsInvalidParameter(err):
		statusCode = http.StatusBadRequest
	case errdefs.IsConflict(err) || errdefs.IsAlreadyExists(err):
		statusCode = http.StatusConflict
	case errdefs.IsUnauthorized(err):
		statusCode = http.StatusUnauthorized
	case errdefs.IsUnavailable(err):
		statusCode = http.StatusServiceUnavailable
	case errdefs.IsForbidden(err):
		statusCode = http.StatusForbidden
	case errdefs.IsNotModified(err):
		statusCode = http.StatusNotModified
	case errdefs.IsNotImplemented(err):
		statusCode = http.StatusNotImplemented
	case errdefs.IsSystem(err) || errdefs.IsUnknown(err) || errdefs.IsDataLoss(err) || errdefs.IsDeadline(err) || errdefs.IsCancelled(err):
		statusCode = http.StatusInternalServerError
	default:
		statusCode = statusCodeFromGRPCError(err)
		if statusCode != http.StatusInternalServerError {
			return statusCode
		}
		statusCode = statusCodeFromDistributionError(err)
		if statusCode != http.StatusInternalServerError {
			return statusCode
		}
		if e, ok := err.(causer); ok {
			return GetHTTPErrorStatusCode(e.Cause())
		}

		logrus.WithFields(logrus.Fields{
			"module":     "api",
			"error_type": fmt.Sprintf("%T", err),
		}).Debugf("FIXME: Got an API for which error does not match any expected type!!!: %+v", err)
	}

	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	return statusCode
}

// FromStatusCode creates an errdef error, based on the provided status-code
func FromStatusCode(err error, statusCode int) error {
	if err == nil {
		return err
	}
	switch statusCode {
	case http.StatusNotFound:
		err = errdefs.NotFound(err)
	case http.StatusBadRequest:
		err = errdefs.InvalidParameter(err)
	case http.StatusConflict:
		err = errdefs.Conflict(err)
	case http.StatusUnauthorized:
		err = errdefs.Unauthorized(err)
	case http.StatusServiceUnavailable:
		err = errdefs.Unavailable(err)
	case http.StatusForbidden:
		err = errdefs.Forbidden(err)
	case http.StatusNotModified:
		err = errdefs.NotModified(err)
	case http.StatusNotImplemented:
		err = errdefs.NotImplemented(err)
	case http.StatusInternalServerError:
		if !errdefs.IsSystem(err) && !errdefs.IsUnknown(err) && !errdefs.IsDataLoss(err) && !errdefs.IsDeadline(err) && !errdefs.IsCancelled(err) {
			err = errdefs.System(err)
		}
	default:
		logrus.WithFields(logrus.Fields{
			"module":      "api",
			"status_code": fmt.Sprintf("%d", statusCode),
		}).Debugf("FIXME: Got an status-code for which error does not match any expected type!!!: %d", statusCode)

		switch {
		case statusCode >= 200 && statusCode < 400:
			// it's a client error
		case statusCode >= 400 && statusCode < 500:
			err = errdefs.InvalidParameter(err)
		case statusCode >= 500 && statusCode < 600:
			err = errdefs.System(err)
		default:
			err = errdefs.Unknown(err)
		}
	}
	return err
}

func apiVersionSupportsJSONErrors(version string) bool {
	const firstAPIVersionWithJSONErrors = "1.23"
	return version == "" || versions.GreaterThan(version, firstAPIVersionWithJSONErrors)
}

// MakeErrorHandler makes an HTTP handler that decodes a Docker error and
// returns it in the response.
func MakeErrorHandler(err error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusCode := GetHTTPErrorStatusCode(err)
		vars := mux.Vars(r)
		if apiVersionSupportsJSONErrors(vars["version"]) {
			response := &types.ErrorResponse{
				Message: err.Error(),
			}
			WriteJSON(w, statusCode, response)
		} else {
			http.Error(w, status.Convert(err).Message(), statusCode)
		}
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
		if e, ok := err.(causer); ok {
			return statusCodeFromGRPCError(e.Cause())
		}
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
	default:
		if e, ok := err.(causer); ok {
			return statusCodeFromDistributionError(e.Cause())
		}
	}
	return http.StatusInternalServerError
}
