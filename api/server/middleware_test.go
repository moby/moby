package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/errors"
	"golang.org/x/net/context"
)

func TestVersionMiddleware(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if httputils.VersionFromContext(ctx) == "" {
			t.Fatalf("Expected version, got empty string")
		}
		return nil
	}

	h := versionMiddleware(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()
	if err := h(ctx, resp, req, map[string]string{}); err != nil {
		t.Fatal(err)
	}
}

func TestVersionMiddlewareWithErrors(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if httputils.VersionFromContext(ctx) == "" {
			t.Fatalf("Expected version, got empty string")
		}
		return nil
	}

	h := versionMiddleware(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	vars := map[string]string{"version": "0.1"}
	err := h(ctx, resp, req, vars)
	if derr, ok := err.(errcode.Error); !ok || derr.ErrorCode() != errors.ErrorCodeOldClientVersion {
		t.Fatalf("Expected ErrorCodeOldClientVersion, got %v", err)
	}

	vars["version"] = "100000"
	err = h(ctx, resp, req, vars)
	if derr, ok := err.(errcode.Error); !ok || derr.ErrorCode() != errors.ErrorCodeNewerClientVersion {
		t.Fatalf("Expected ErrorCodeNewerClientVersion, got %v", err)
	}
}
