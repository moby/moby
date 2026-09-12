//go:build linux

package peercred

import (
	"context"
	"net"

	"golang.org/x/sys/unix"
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

// ConnContext extracts SO_PEERCRED from a Unix socket connection and stores it in the context.
// It is intended to be used as http.Server.ConnContext.
func ConnContext(ctx context.Context, c net.Conn) context.Context {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return ctx
	}
	rawConn, err := uc.SyscallConn()
	if err != nil {
		return ctx
	}
	var cred *unix.Ucred
	var gerr error
	if err := rawConn.Control(func(fd uintptr) {
		cred, gerr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return ctx
	}
	if gerr != nil || cred == nil {
		return ctx
	}
	pc := &PeerCred{PID: cred.Pid, UID: cred.Uid, GID: cred.Gid}
	return WithPeerCred(ctx, pc)
}
