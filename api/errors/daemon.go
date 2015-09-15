package errors

// This file contains all of the errors that can be generated from the
// docker/daemon component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeNoSuchContainer is generated when we look for a container by
	// name or ID and we can't find it.
	ErrorCodeNoSuchContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHCONTAINER",
		Message:        "no such id: %s",
		Description:    "The specified container can not be found",
		HTTPStatusCode: http.StatusNotFound,
	})
)
