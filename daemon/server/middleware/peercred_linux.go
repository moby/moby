package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"

	"golang.org/x/sys/unix"
)

// PeerCredKey is the context key for storing peer credentials
var PeerCredKey = &struct{ name string }{"peercred"}

// PeerConnKey is the context key for storing the raw connection (set by ConnContext)
// We use a custom key instead of http.LocalAddrContextKey because the HTTP stack
// overwrites that key with the address, losing the original connection.
var PeerConnKey = &struct{ name string }{"peerconn"}

// PeerCredentials contains the credentials of a peer connection
type PeerCredentials struct {
	PID int // Process ID
	UID int // User ID
	GID int // Group ID
}

// PeerCredMiddleware extracts peer credentials from Unix socket connections
// and adds them to the request context.
type PeerCredMiddleware struct{}

// NewPeerCredMiddleware creates a new peer credential middleware
func NewPeerCredMiddleware() PeerCredMiddleware {
	return PeerCredMiddleware{}
}

// WrapHandler wraps an HTTP handler to extract peer credentials from Unix socket connections
func (m PeerCredMiddleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		// Attempt to extract peer credentials from the connection
		if creds, err := extractPeerCredentials(r); err == nil && creds != nil {
			// Add credentials to context for downstream handlers
			ctx = context.WithValue(ctx, PeerCredKey, creds)
		}

		return handler(ctx, w, r, vars)
	}
}

// extractPeerCredentials extracts the peer credentials from an HTTP request
// by accessing the underlying Unix socket file descriptor and calling SO_PEERCRED.
//
// This only works for Unix domain socket connections. For TCP connections or
// other transport types, this function returns nil, nil (no error, no credentials).
func extractPeerCredentials(r *http.Request) (*PeerCredentials, error) {
	// Try to get the underlying connection from the request context
	// We use PeerConnKey (set by ConnContext) instead of http.LocalAddrContextKey
	// because the HTTP stack overwrites that key with the address.
	conn, ok := r.Context().Value(PeerConnKey).(net.Conn)
	if !ok || conn == nil {
		// Not a direct connection or connection not available - this is expected for some scenarios
		return nil, nil
	}

	// Cast to syscall.Conn to get access to raw file descriptor operations
	sc, ok := conn.(syscall.Conn)
	if !ok {
		// Connection doesn't support syscall operations - probably not a Unix socket
		return nil, nil
	}

	// Get the raw syscall connection
	rc, err := sc.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("failed to get syscall connection: %w", err)
	}

	// Extract peer credentials using SO_PEERCRED
	var creds *PeerCredentials
	var ctrlErr error

	// Control() provides access to the underlying file descriptor
	err = rc.Control(func(fd uintptr) {
		ucred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			ctrlErr = fmt.Errorf("SO_PEERCRED failed: %w", err)
			return
		}

		creds = &PeerCredentials{
			PID: int(ucred.Pid),
			UID: int(ucred.Uid),
			GID: int(ucred.Gid),
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to access file descriptor: %w", err)
	}
	if ctrlErr != nil {
		return nil, ctrlErr
	}

	return creds, nil
}
