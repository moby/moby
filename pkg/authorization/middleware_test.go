package authorization // import "github.com/docker/docker/pkg/authorization"

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/plugingetter"
	"gotest.tools/v3/assert"
)

func TestMiddleware(t *testing.T) {
	pluginNames := []string{"testPlugin1", "testPlugin2"}
	var pluginGetter plugingetter.PluginGetter
	m := NewMiddleware(pluginNames, pluginGetter)
	authPlugins := m.getAuthzPlugins()
	assert.Equal(t, 2, len(authPlugins))
	assert.Equal(t, pluginNames[0], authPlugins[0].Name())
	assert.Equal(t, pluginNames[1], authPlugins[1].Name())
}

func TestNewResponseModifier(t *testing.T) {
	recorder := httptest.NewRecorder()
	modifier := NewResponseModifier(recorder)
	modifier.Header().Set("H1", "V1")
	modifier.Write([]byte("body"))
	assert.Assert(t, !modifier.Hijacked())
	modifier.WriteHeader(http.StatusInternalServerError)
	assert.Assert(t, modifier.RawBody() != nil)

	raw, err := modifier.RawHeaders()
	assert.Assert(t, raw != nil)
	assert.NilError(t, err)

	headerData := strings.Split(strings.TrimSpace(string(raw)), ":")
	assert.Equal(t, "H1", strings.TrimSpace(headerData[0]))
	assert.Equal(t, "V1", strings.TrimSpace(headerData[1]))

	modifier.Flush()
	modifier.FlushAll()

	if recorder.Header().Get("H1") != "V1" {
		t.Fatalf("Header value must exists %s", recorder.Header().Get("H1"))
	}
}

func setAuthzPlugins(m *Middleware, plugins []Plugin) {
	m.mu.Lock()
	m.plugins = plugins
	m.mu.Unlock()
}
