package middleware // import "github.com/docker/docker/api/server/middleware"

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/server/httputils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVersionMiddlewareVersion(t *testing.T) {
	expectedVersion := "<not set>"
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		v := httputils.VersionFromContext(ctx)
		assert.Check(t, is.Equal(expectedVersion, v))
		return nil
	}

	m := NewVersionMiddleware("1.2.3", api.DefaultVersion, api.MinSupportedAPIVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest(http.MethodGet, "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	tests := []struct {
		reqVersion      string
		expectedVersion string
		errString       string
	}{
		{
			expectedVersion: api.DefaultVersion,
		},
		{
			reqVersion:      api.MinSupportedAPIVersion,
			expectedVersion: api.MinSupportedAPIVersion,
		},
		{
			reqVersion: "0.1",
			errString:  fmt.Sprintf("client version 0.1 is too old. Minimum supported API version is %s, please upgrade your client to a newer version", api.MinSupportedAPIVersion),
		},
		{
			reqVersion: "9999.9999",
			errString:  fmt.Sprintf("client version 9999.9999 is too new. Maximum supported API version is %s", api.DefaultVersion),
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

	m := NewVersionMiddleware("1.2.3", api.DefaultVersion, api.MinSupportedAPIVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest(http.MethodGet, "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	vars := map[string]string{"version": "0.1"}
	err := h(ctx, resp, req, vars)
	assert.Check(t, is.ErrorContains(err, ""))

	hdr := resp.Result().Header
	assert.Check(t, is.Contains(hdr.Get("Server"), "Docker/1.2.3"))
	assert.Check(t, is.Contains(hdr.Get("Server"), runtime.GOOS))
	assert.Check(t, is.Equal(hdr.Get("API-Version"), api.DefaultVersion))
	assert.Check(t, is.Equal(hdr.Get("OSType"), runtime.GOOS))
}
