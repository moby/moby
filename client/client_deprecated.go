package client

import "net/http"

// NewClient initializes a new API client for the given host and API version.
// It uses the given http client as transport.
// It also initializes the custom http headers to add to each request.
//
// It won't send any version information if the version number is empty. It is
// highly recommended that you set a version or your client may break if the
// server is upgraded.
//
// This function is deprecated, and is no longer functional.
//
// If you're using this function, replace it with the code below to get
// the equivalent (non-deprecated) code to initialize a client with a
// fixed API version:
//
//    client.NewClientWithOpts(
//        WithHost(host),
//        WithVersion(version),
//        WithHTTPClient(client),
//        WithHTTPHeaders(httpHeaders),
//    )
//
// We recommend to enable API version negotiation, so that the client
// automatically negotiates the API version to use when connecting with the
// daemon. To enable API version negotiation, use the WithAPIVersionNegotiation()
// option instead of WithVersion(version):
//
//    client.NewClientWithOpts(
//        WithHost(host),
//        WithAPIVersionNegotiation(),
//        WithHTTPClient(client),
//        WithHTTPHeaders(httpHeaders),
//    )
//
// Deprecated: use NewClientWithOpts
func NewClient(host string, version string, client *http.Client, httpHeaders map[string]string) {}

// NewEnvClient initializes a new API client based on environment variables.
// See FromEnv for a list of support environment variables.
//
// This function is deprecated, and is no longer functional.
//
// If you're using this function, replace it with the code below to get
// the equivalent (non-deprecated) code:
//
//    client.NewClientWithOpts(FromEnv)
//
// We recommend to enable API version negotiation, so that the client
// automatically negotiates the API version to use when connecting with the
// daemon. To enable API version negotiation, add the WithAPIVersionNegotiation()
// option:
//
//    client.NewClientWithOpts(FromEnv, WithAPIVersionNegotiation())
//
// Deprecated: use NewClientWithOpts
func NewEnvClient() {}
