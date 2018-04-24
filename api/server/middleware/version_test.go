package middleware // import "github.com/docker/docker/api/server/middleware"

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/docker/docker/api/server/httputils"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func TestVersionMiddlewareVersion(t *testing.T) {
	defaultVersion := "1.10.0"
	minVersion := "1.2.0"
	expectedVersion := defaultVersion
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		v := httputils.VersionFromContext(ctx)
		assert.Check(t, is.Equal(expectedVersion, v))
		return nil
	}

	m := NewVersionMiddleware(defaultVersion, defaultVersion, minVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	tests := []struct {
		reqVersion      string
		expectedVersion string
		errString       string
	}{
		{
			expectedVersion: "1.10.0",
		},
		{
			reqVersion:      "1.9.0",
			expectedVersion: "1.9.0",
		},
		{
			reqVersion: "0.1",
			errString:  "client version 0.1 is too old. Minimum supported API version is 1.2.0, please upgrade your client to a newer version",
		},
		{
			reqVersion: "9999.9999",
			errString:  "client version 9999.9999 is too new. Maximum supported API version is 1.10.0",
		},
	}

	for _, test := range tests {
		expectedVersion = test.expectedVersion

		err := h(ctx, resp, req, map[string]string{"version": test.reqVersion})

		if test.errString != "" {
			assert.Check(t, is.Error(err, test.errString))
		} else {
			assert.Check(t, err)
		}
	}
}

func TestVersionMiddlewareWithErrorsReturnsHeaders(t *testing.T) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		v := httputils.VersionFromContext(ctx)
		assert.Check(t, len(v) != 0)
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
	assert.Check(t, is.ErrorContains(err, ""))

	hdr := resp.Result().Header
	assert.Check(t, is.Contains(hdr.Get("Server"), "Docker/"+defaultVersion))
	assert.Check(t, is.Contains(hdr.Get("Server"), runtime.GOOS))
	assert.Check(t, is.Equal(hdr.Get("API-Version"), defaultVersion))
	assert.Check(t, is.Equal(hdr.Get("OSType"), runtime.GOOS))
}
