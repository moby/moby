package context

import (
	"testing"

	"github.com/docker/docker/pkg/version"
)

func TestContext(t *testing.T) {
	ctx := Background()

	// First make sure getting non-existent values doesn't break
	if id := ctx.RequestID(); id != "" {
		t.Fatalf("RequestID() should have been '', was: %q", id)
	}

	if ver := ctx.Version(); ver != "" {
		t.Fatalf("Version() should have been '', was: %q", ver)
	}

	// Test basic set/get
	ctx = WithValue(ctx, RequestID, "123")
	if ctx.RequestID() != "123" {
		t.Fatalf("RequestID() should have been '123'")
	}

	// Now make sure after a 2nd set we can still get both
	ctx = WithValue(ctx, APIVersion, version.Version("x.y"))
	if id := ctx.RequestID(); id != "123" {
		t.Fatalf("RequestID() should have been '123', was %q", id)
	}
	if ver := ctx.Version(); ver != "x.y" {
		t.Fatalf("Version() should have been 'x.y', was %q", ver)
	}
}
