package errors

// This file contains all of the errors that can be generated from the
// docker/image component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeInvalidImageID is generated when image id specified is incorrectly formatted.
	ErrorCodeInvalidImageID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDIMAGEID",
		Message:        "image ID '%s' is invalid ",
		Description:    "The specified image id is incorrectly formatted",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
