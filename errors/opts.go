package errors

// This file contains all of the errors that can be generated from the
// docker/opts component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeInvalidIPFormat is generated when ip address is incorrectly formatted.
	ErrorCodeInvalidIPFormat = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDIPFORMAT",
		Message:        "%s is not an ip address",
		Description:    "The specified IP address is incorrectly formatted",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidStream is generated when stream specified is not one of STDIN, STDOUT and STDERR.
	ErrorCodeInvalidStream = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDSTREAM",
		Message:        "valid streams are STDIN, STDOUT and STDERR",
		Description:    "The specified stream is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidVolPath is generated when volume path is incorrectly formatted.
	ErrorCodeInvalidVolPath = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDVOLPATH",
		Message:        "bad format for path: %s",
		Description:    "The specified path for the volume is incorrectly formatted",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidVolMode is generated when mode specified for the volume is incorrect.
	ErrorCodeInvalidVolMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDVOLMODE",
		Message:        "bad mode specified: %s",
		Description:    "The specified mode for the volume is incorrect",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidContainerPath is generated when container path is incorrectly formatted.
	ErrorCodeInvalidContainerPath = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDCONTAINERPATH",
		Message:        "%s is not an absolute path",
		Description:    "The specified container path is incorrectly formatted",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidDomain is generated when domain is incorrectly formatted.
	ErrorCodeInvalidDomain = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDDOMAIN",
		Message:        "%s is not a valid domain",
		Description:    "The specified Network Domain is incorrectly formatted",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidExtraHost is generated when host string in extra host is incorrectly formatted.
	ErrorCodeInvalidExtraHost = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDEXTRAHOST",
		Message:        "bad format for add-host: %q",
		Description:    "The specified host string in the extra host is incorrectly formatted",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidExtraHostIP is generated when host ip string in extra host is incorrectly formatted.
	ErrorCodeInvalidExtraHostIP = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDEXTRAHOSTIP",
		Message:        "invalid IP address in add-host: %q",
		Description:    "The specified host ip string in the extra host is incorrectly formatted",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidLabel is generated when label is incorrectly formatted.
	ErrorCodeInvalidLabel = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDLABEL",
		Message:        "bad attribute format: %s",
		Description:    "The specified Label is incorrectly formatted",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
