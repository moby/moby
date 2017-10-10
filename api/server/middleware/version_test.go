package middleware

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/api/server/httputils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestVersionMiddleware(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if httputils.VersionFromContext(ctx) == "" {
			t.Fatal("Expected version, got empty string")
		}
		return nil
	}

	defaultVersion := "1.10.0"
	minVersion := "1.2.0"
	m := NewVersionMiddleware(defaultVersion, defaultVersion, minVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()
	if err := h(ctx, resp, req, map[string]string{}); err != nil {
		t.Fatal(err)
	}
}

func TestVersionMiddlewareVersionTooOld(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if httputils.VersionFromContext(ctx) == "" {
			t.Fatal("Expected version, got empty string")
		}
		return nil
	}

	defaultVersion := "1.10.0"
	minVersion := "1.2.0"
	m := NewVersionMiddleware(defaultVersion, defaultVersion, minVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	vars := map[string]string{"version": "0.1"}
	err := h(ctx, resp, req, vars)

	if !strings.Contains(err.Error(), "client version 0.1 is too old. Minimum supported API version is 1.2.0") {
		t.Fatalf("Expected too old client error, got %v", err)
	}
}

func TestVersionMiddlewareVersionTooNew(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if httputils.VersionFromContext(ctx) == "" {
			t.Fatal("Expected version, got empty string")
		}
		return nil
	}

	defaultVersion := "1.10.0"
	minVersion := "1.2.0"
	m := NewVersionMiddleware(defaultVersion, defaultVersion, minVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	vars := map[string]string{"version": "9999.9999"}
	err := h(ctx, resp, req, vars)

	if !strings.Contains(err.Error(), "client version 9999.9999 is too new. Maximum supported API version is 1.10.0") {
		t.Fatalf("Expected too new client error, got %v", err)
	}
}

func TestVersionMiddlewareWithErrorsReturnsHeaders(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if httputils.VersionFromContext(ctx) == "" {
			t.Fatal("Expected version, got empty string")
		}
		return nil
	}

	defaultVersion := "1.10.0"
	minVersion := "1.2.0"
	m := NewVersionMiddleware(defaultVersion, defaultVersion, minVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	vars := map[string]string{"version": "0.1"}
	err := h(ctx, resp, req, vars)

	assert.Error(t, err)
	hdr := resp.Result().Header
	assert.Contains(t, hdr.Get("Server"), "Docker/"+defaultVersion)
	assert.Contains(t, hdr.Get("Server"), runtime.GOOS)
	assert.Equal(t, hdr.Get("API-Version"), defaultVersion)
	assert.Equal(t, hdr.Get("OSType"), runtime.GOOS)
}
