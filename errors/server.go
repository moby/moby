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

	// ErrorCodeNoHijackConnection is generated when a request tries to attach to a container
	// but the connection to hijack is not provided.
	ErrorCodeNoHijackConnection = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "HIJACK_CONNECTION_MISSING",
		Message:        "error attaching to container %s, hijack connection missing",
		Description:    "The caller didn't provide a connection to hijack",
		HTTPStatusCode: http.StatusBadRequest,
	})

	// ErrorCodeNoAuthentication is generated when a request from a client
	// was rejected due to the server requiring authentication, but not
	// being able to offer any way to authenticate.
	ErrorCodeNoAuthentication = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOAUTHENTICATION",
		Message:        "server requires authentication, but doesn't support any methods",
		Description:    "The server is configured to require authentication, but can't offer it",
		HTTPStatusCode: http.StatusUnauthorized,
	})

	// ErrorCodeMustAuthenticate is generated when a request from a client
	// was rejected due to the client not having authenticated first
	ErrorCodeMustAuthenticate = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MUSTAUTHENTICATE",
		Message:        "server requires authentication, but client did not authenticate",
		Description:    "The client must authenticate to the server",
		HTTPStatusCode: http.StatusUnauthorized,
	})
)
