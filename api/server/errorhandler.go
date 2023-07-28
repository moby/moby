package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/server/httpstatus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/errdefs"
	"github.com/gorilla/mux"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc/status"
)

// makeErrorHandler makes an HTTP handler that decodes a Docker error and
// returns it in the response.
func makeErrorHandler(err error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusCode := httpstatus.FromError(err)
		vars := mux.Vars(r)
		version := vars["version"]

		const firstAPIVersionWithJSONErrors = "1.24"
		if version != "" && versions.LessThan(version, firstAPIVersionWithJSONErrors) {
			http.Error(w, status.Convert(err).Message(), statusCode)
			return
		}

		response := marshalErrorResponse(err)
		const firstAPIVersionWithJSONMultiErrors = "1.44"
		if version != "" && versions.LessThan(version, firstAPIVersionWithJSONMultiErrors) {
			response.Errors = nil
		}

		_ = httputils.WriteJSON(w, statusCode, response)
	}
}

type joiningError interface {
	error
	Unwrap() []error
}

type wrappingError interface {
	error
	Unwrap() error
}

// marshalErrorResponse returns a tree of [*types.ErrorResponse] representing the argument err.
func marshalErrorResponse(err error) *types.ErrorResponse {
	if errdefs.IsHTTPOnlyError(err) {
		if wrappedErr := errors.Unwrap(err); wrappedErr != nil {
			return marshalErrorResponse(wrappedErr)
		}
	}

	switch err := err.(type) {
	case joiningError:
		return marshalJoinedErrors(err)
	case *multierror.Error:
		return marshalMultiErrors(err)
	case wrappingError:
		return &types.ErrorResponse{
			Message: err.Error(),
			Errors:  []*types.ErrorResponse{marshalErrorResponse(err.Unwrap())},
		}
	default:
		return &types.ErrorResponse{
			Message: err.Error(),
		}
	}
}

func formatErrors(errs []*types.ErrorResponse) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d errors occurred:", len(errs)))
	for _, err := range errs {
		b.WriteString("\n\t* " + strings.Replace(err.Message, "\n", "\n\t", -1))
	}
	return b.String()
}

func marshalJoinedErrors(err joiningError) *types.ErrorResponse {
	var subErrors []*types.ErrorResponse
	for _, subErr := range err.Unwrap() {
		if subErr != nil {
			subErrors = append(subErrors, marshalErrorResponse(subErr))
		}
	}

	errMsg := err.Error()
	if len(subErrors) > 1 && err.Error() == errors.Join(err.Unwrap()...).Error() {
		errMsg = formatErrors(subErrors)
	} else if len(subErrors) == 1 && errMsg == subErrors[0].Message {
		return subErrors[0]
	}

	return &types.ErrorResponse{
		Message: errMsg,
		Errors:  subErrors,
	}
}

func marshalMultiErrors(err *multierror.Error) *types.ErrorResponse {
	var subErrors []*types.ErrorResponse
	for _, subErr := range err.WrappedErrors() {
		if subErr != nil {
			subErrors = append(subErrors, marshalErrorResponse(subErr))
		}
	}

	if err.ErrorFormat != nil {
		return &types.ErrorResponse{
			Message: err.Error(),
			Errors:  subErrors,
		}
	}

	return &types.ErrorResponse{
		Message: formatErrors(subErrors),
		Errors:  subErrors,
	}
}
