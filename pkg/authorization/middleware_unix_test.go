//go:build !windows

package authorization

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moby/moby/v2/pkg/plugingetter"
	"gotest.tools/v3/assert"
)

func TestMiddlewareWrapHandler(t *testing.T) {
	server := authZPluginTestServer{t: t}
	server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(t, server.socketAddress())
	pluginNames := []string{authZPlugin.name}

	var pluginGetter plugingetter.PluginGetter
	middleWare := NewMiddleware(pluginNames, pluginGetter)
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		return nil
	}

	authList := []Plugin{authZPlugin}
	middleWare.SetPlugins([]string{"My Test Plugin"})
	setAuthzPlugins(middleWare, authList)
	mdHandler := middleWare.WrapHandler(handler)
	assert.Assert(t, mdHandler != nil)

	addr := "www.example.com/auth"
	req, _ := http.NewRequest(http.MethodGet, addr, http.NoBody)
	req.RequestURI = addr
	req.Header.Add("header", "value")

	resp := httptest.NewRecorder()
	ctx := context.Background()

	t.Run("Error Test Case :", func(t *testing.T) {
		server.replayResponse = Response{
			Allow: false,
			Msg:   "Server Auth Not Allowed",
		}
		if err := mdHandler(ctx, resp, req, map[string]string{}); err == nil {
			assert.ErrorContains(t, err, "")
		}
	})

	t.Run("Positive Test Case :", func(t *testing.T) {
		server.replayResponse = Response{
			Allow: true,
			Msg:   "Server Auth Allowed",
		}
		if err := mdHandler(ctx, resp, req, map[string]string{}); err != nil {
			assert.NilError(t, err)
		}
	})
}
