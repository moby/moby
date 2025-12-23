package sockets

import (
	"context"
	"net"
	"net/http"

	"github.com/Microsoft/go-winio"
)

func configureNpipeTransport(tr *http.Transport, addr string) error {
	// No need for compression in local communications.
	tr.DisableCompression = true
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return winio.DialPipeContext(ctx, addr)
	}
	return nil
}
