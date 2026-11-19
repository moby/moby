//go:build go1.27

package session

import (
	"context"
	"math"
	"net/http"

	"github.com/moby/buildkit/util/bklog"
	"golang.org/x/net/http2"
)

// newHTTP2Server returns the HTTP/2 server for session connections.
//
// Since Go 1.27, x/net/http2 is backed by net/http, which limits requests to
// http.DefaultMaxHeaderValueCount (500) header values. Session metadata, such
// as filesync include/exclude patterns, uses one value per item and can exceed
// that. The total header size is still bounded by http.DefaultMaxHeaderBytes.
func newHTTP2Server(ctx context.Context) *http2.Server { //nolint:staticcheck // ServeConn on an established connection needs x/net/http2.
	h2 := &http2.Server{} //nolint:staticcheck // ServeConn on an established connection needs x/net/http2.
	// h1 only carries config for h2 connections and never serves on its own.
	h1 := &http.Server{MaxHeaderValueCount: math.MaxInt}  //nolint:gosec // G112: session streams are long-lived, no read timeouts.
	if err := http2.ConfigureServer(h1, h2); err != nil { //nolint:staticcheck // h1 is never served, Protocols does not apply.
		bklog.G(ctx).WithError(err).Warn("failed to configure session http2 server")
		return &http2.Server{} //nolint:staticcheck // ServeConn on an established connection needs x/net/http2.
	}
	return h2
}
