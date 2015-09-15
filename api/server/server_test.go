package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/context"
)

func TestMiddlewares(t *testing.T) {
	cfg := &Config{}
	srv := &Server{
		cfg: cfg,
	}

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	localHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if ctx.Version() == "" {
			t.Fatalf("Expected version, got empty string")
		}
		if ctx.RequestID() == "" {
			t.Fatalf("Expected request-id, got empty string")
		}
		return nil
	}

	handlerFunc := srv.handleWithGlobalMiddlewares(localHandler)
	if err := handlerFunc(ctx, resp, req, map[string]string{}); err != nil {
		t.Fatal(err)
	}
}
