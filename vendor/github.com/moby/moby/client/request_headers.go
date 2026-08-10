package client

import (
	"context"
	"net/http"
)

type requestHeadersKey struct{}

// WithRequestHeaders returns a context that carries extra HTTP headers to
// set on every API request sent with it. It allows callers to attach
// request-scoped metadata (for example, headers attributing the request to
// a higher-level resource) without constructing a dedicated client with
// [WithHTTPHeaders], which applies to all requests of a client instance.
//
// It is the request-side counterpart of [WithResponseHook]. Headers carried
// by the context are set after the client's configured headers, and cannot
// override headers set by the client itself for a specific request.
// Headers already carried by ctx are replaced.
func WithRequestHeaders(ctx context.Context, headers http.Header) context.Context {
	return context.WithValue(ctx, requestHeadersKey{}, headers.Clone())
}

// requestHeadersFromContext returns the extra headers carried by ctx, or nil.
func requestHeadersFromContext(ctx context.Context) http.Header {
	headers, _ := ctx.Value(requestHeadersKey{}).(http.Header)
	return headers
}
