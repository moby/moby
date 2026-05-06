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
