//go:build !linux

package peercred

import (
	"context"
	"net"
)

// PeerCred holds the credentials of a peer connected via a Unix socket.
type PeerCred struct {
	PID int32
	UID uint32
	GID uint32
}

type peerCredKey struct{}

// WithPeerCred returns a new context with the given peer credentials.
func WithPeerCred(ctx context.Context, cred *PeerCred) context.Context {
	return context.WithValue(ctx, peerCredKey{}, cred)
}

// FromContext retrieves peer credentials from the context.
func FromContext(ctx context.Context) (*PeerCred, bool) {
	cred, ok := ctx.Value(peerCredKey{}).(*PeerCred)
	return cred, ok
}

// ConnContext is a no-op on non-Linux platforms.
func ConnContext(ctx context.Context, _ net.Conn) context.Context {
	return ctx
}
