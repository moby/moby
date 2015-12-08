package errors

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeNewerClientVersion is generated when a request from a client
	// specifies a higher version than the server supports.
	ErrorCodeNewerClientVersion = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NEWERCLIENTVERSION",
		Message:        "client is newer than server (client API version: %s, server API version: %s)",
		Description:    "The client version is higher than the server version",
		HTTPStatusCode: http.StatusBadRequest,
	})

	// ErrorCodeOldClientVersion is generated when a request from a client
	// specifies a version lower than the minimum version supported by the server.
	ErrorCodeOldClientVersion = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "OLDCLIENTVERSION",
		Message:        "client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version",
		Description:    "The client version is too old for the server",
		HTTPStatusCode: http.StatusBadRequest,
	})

	// ErrorNetworkControllerNotEnabled is generated when the networking stack in not enabled
	// for certain platforms, like windows.
	ErrorNetworkControllerNotEnabled = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NETWORK_CONTROLLER_NOT_ENABLED",
		Message:        "the network controller is not enabled for this platform",
		Description:    "Docker's networking stack is disabled for this platform",
		HTTPStatusCode: http.StatusNotFound,
	})
)
