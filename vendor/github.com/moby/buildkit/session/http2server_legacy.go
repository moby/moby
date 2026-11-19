//go:build !go1.27

package session

import (
	"context"

	"golang.org/x/net/http2"
)

// newHTTP2Server returns the HTTP/2 server for session connections. Before Go
// 1.27 x/net/http2 has no header value count limit, so no extra setup is needed.
func newHTTP2Server(_ context.Context) *http2.Server { //nolint:staticcheck // ServeConn on an established connection needs x/net/http2.
	return &http2.Server{} //nolint:staticcheck // ServeConn on an established connection needs x/net/http2.
}
