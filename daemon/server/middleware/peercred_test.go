package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
)

func TestPeerCredentials_ContextValue(t *testing.T) {
	// Test that PeerCredKey can be used to store/retrieve credentials from context
	ctx := context.Background()

	creds := &PeerCredentials{
		PID: 1234,
		UID: 1000,
		GID: 1000,
	}

	ctx = context.WithValue(ctx, PeerCredKey, creds)

	retrieved, ok := ctx.Value(PeerCredKey).(*PeerCredentials)
	assert.Assert(t, ok, "should be able to retrieve peer credentials from context")
	assert.Equal(t, retrieved.PID, 1234)
	assert.Equal(t, retrieved.UID, 1000)
	assert.Equal(t, retrieved.GID, 1000)
}

func TestPeerCredentials_NilContext(t *testing.T) {
	// Test that retrieving from context without credentials returns nil gracefully
	ctx := context.Background()

	retrieved, ok := ctx.Value(PeerCredKey).(*PeerCredentials)
	assert.Assert(t, !ok || retrieved == nil, "should return nil when no credentials in context")
}

// TestPeerCredMiddleware_UnixSocket tests that the middleware properly handles Unix socket connections
// Note: This is a basic structure test. Actual SO_PEERCRED extraction can only be tested with real Unix sockets.
func TestPeerCredMiddleware_Structure(t *testing.T) {
	middleware := NewPeerCredMiddleware()

	handlerCalled := false
	testHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		handlerCalled = true
		return nil
	}

	wrapped := middleware.WrapHandler(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	// Call the wrapped handler
	err := wrapped(context.Background(), w, req, nil)

	assert.NilError(t, err)
	assert.Assert(t, handlerCalled, "handler should have been called")
}

func TestPeerCredMiddleware_NoConnection(t *testing.T) {
	// Test that middleware doesn't fail when no connection is in context
	middleware := NewPeerCredMiddleware()

	var capturedCtx context.Context
	testHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		capturedCtx = ctx
		return nil
	}

	wrapped := middleware.WrapHandler(testHandler)

	// Create a test request without connection in context
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	err := wrapped(context.Background(), w, req, nil)

	assert.NilError(t, err)
	// Verify no credentials were added (since no connection was available)
	creds, ok := capturedCtx.Value(PeerCredKey).(*PeerCredentials)
	assert.Assert(t, !ok || creds == nil, "should not have credentials when no connection")
}

func TestPeerConnKey_Uniqueness(t *testing.T) {
	// Verify that PeerConnKey is distinct from http.LocalAddrContextKey
	// This is important because http.LocalAddrContextKey gets overwritten by the HTTP stack
	ctx := context.Background()

	// Simulate what happens in ConnContext and the HTTP stack
	ctx = context.WithValue(ctx, PeerConnKey, "connection")
	ctx = context.WithValue(ctx, http.LocalAddrContextKey, "address")

	// Both values should be retrievable independently
	conn := ctx.Value(PeerConnKey)
	addr := ctx.Value(http.LocalAddrContextKey)

	assert.Equal(t, conn, "connection")
	assert.Equal(t, addr, "address")
}
