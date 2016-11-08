package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/useragent"
	"golang.org/x/net/context"
)

func TestUserAgentMiddleware(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if !strings.Contains(useragent.FromContext(ctx), "Docker-Client/1.9.1") {
			return errors.New("Missing upstream useragent from context")
		}
		return nil
	}

	serverVersion := "1.9.1"
	m := NewUserAgentMiddleware(serverVersion, "")
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	req.Header.Set("User-Agent", "Docker-Client/1.9.1")
	resp := httptest.NewRecorder()
	ctx := context.Background()

	if err := h(ctx, resp, req, map[string]string{}); err != nil {
		t.Fatal(err)
	}
}

func TestUserAgentMiddlewareWithCustomUA(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		ua := useragent.FromContext(ctx)
		if !strings.Contains(ua, "custom/my-annotation") {
			t.Fatalf("Expected `custom/my-annotation` in user-agent, got: %s", ua)
		}
		return nil
	}

	serverVersion := "1.9.1"
	m := NewUserAgentMiddleware(serverVersion, "my-annotation")
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	req.Header.Set("User-Agent", "Docker-Client/1.9.1")
	resp := httptest.NewRecorder()
	ctx := context.Background()

	if err := h(ctx, resp, req, map[string]string{}); err != nil {
		t.Fatal(err)
	}
}
