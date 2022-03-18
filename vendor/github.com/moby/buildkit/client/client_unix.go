//go:build !windows
// +build !windows

package client

import (
	"context"
	"net"
	"strings"

	"github.com/pkg/errors"
)

func dialer(ctx context.Context, address string) (net.Conn, error) {
	addrParts := strings.SplitN(address, "://", 2)
	if len(addrParts) != 2 {
		return nil, errors.Errorf("invalid address %s", address)
	}
	var d net.Dialer
	return d.DialContext(ctx, addrParts[0], addrParts[1])
}
