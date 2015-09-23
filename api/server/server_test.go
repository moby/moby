package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/server/httputils"

	"golang.org/x/net/context"
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
		if httputils.VersionFromContext(ctx) == "" {
			t.Fatalf("Expected version, got empty string")
		}
		return nil
	}

	handlerFunc := srv.handleWithGlobalMiddlewares(localHandler)
	if err := handlerFunc(ctx, resp, req, map[string]string{}); err != nil {
		t.Fatal(err)
	}
}
