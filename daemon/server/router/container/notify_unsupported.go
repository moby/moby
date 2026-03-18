//go:build !linux

package container

import (
	"context"
	"net"
)

func notifyClosed(ctx context.Context, conn net.Conn, notify func()) {}
