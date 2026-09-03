package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// Do sends an HTTP request to the Docker API using the client's configured
// HTTP client and transport (connection helpers, TLS, custom HTTP headers,
// tracing), and returns the HTTP response. It is intended for auxiliary or
// experimental endpoints exposed on the daemon socket for which this package
// provides no typed method, without callers having to hand-roll an
// [http.Client] around [Client.Dialer].
//
// The request path is completed like every other request in this package:
// it is joined with the daemon host's base path and the (possibly negotiated)
// API version prefix.
//
// A non-2xx status code is returned as an error, matching the error types
// produced by other methods of this client, and the response body is consumed.
// Otherwise the caller is responsible for closing the response body.
func (cli *Client) Do(ctx context.Context, method, path string, query url.Values, body io.Reader, headers http.Header) (*http.Response, error) {
	return cli.sendRequest(ctx, method, path, query, body, headers)
}
