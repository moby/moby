// +build !windows

package authorization

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestMiddleware(t *testing.T) {
	pluginNames := []string{"testPlugin1", "testPlugin2"}
	pluginGetter := newPluginGetterMock(t, "testPlugin1", "testPlugin2")
	m := NewMiddleware(pluginGetter)
	m.SetPlugins(pluginNames)
	authPlugins := m.getAuthzPlugins()
	require.Equal(t, 2, len(authPlugins))
	require.EqualValues(t, pluginNames[0], authPlugins[0].Name())
	require.EqualValues(t, pluginNames[1], authPlugins[1].Name())
}

func TestMiddlewareWrapHandler(t *testing.T) {
	server := authZPluginTestServer{t: t}
	server.start()
	defer server.stop()

	pluginGetter := newPluginGetterMock(t, "My Test Plugin")
	middleWare := NewMiddleware(pluginGetter)
	err := middleWare.SetPlugins([]string{"My Test Plugin"})
	require.Nil(t, err)
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		return nil
	}

	mdHandler := middleWare.WrapHandler(handler)
	require.NotNil(t, mdHandler)

	addr := "www.example.com/auth"
	req, _ := http.NewRequest("GET", addr, nil)
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
			require.Error(t, err)
		}

	})

	t.Run("Positive Test Case :", func(t *testing.T) {
		server.replayResponse = Response{
			Allow: true,
			Msg:   "Server Auth Allowed",
		}
		if err := mdHandler(ctx, resp, req, map[string]string{}); err != nil {
			require.NoError(t, err)
		}

	})

}

type pluginGetterMock []*authorizationPlugin

func (m pluginGetterMock) Get(name, capability string, mode int) (plugingetter.CompatPlugin, error) {
	for _, plugin := range m {
		if name == plugin.Name() {
			return (*compatPluginMock)(plugin), nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (m pluginGetterMock) GetAllByCap(capability string) ([]plugingetter.CompatPlugin, error) {
	return m.GetAllManagedPluginsByCap(capability), nil
}

func (m pluginGetterMock) GetAllManagedPluginsByCap(capability string) []plugingetter.CompatPlugin {
	plugins := []plugingetter.CompatPlugin{}
	for _, plugin := range m {
		plugins = append(plugins, (*compatPluginMock)(plugin))
	}
	return plugins
}

func (m pluginGetterMock) Handle(capability string, callback func(string, *plugins.Client)) {
}

func newPluginGetterMock(t *testing.T, names ...string) plugingetter.PluginGetter {
	plugins := []*authorizationPlugin{}

	for _, name := range names {
		plugins = append(plugins, createTestPlugin(t, name))
	}

	return pluginGetterMock(plugins)
}

type compatPluginMock authorizationPlugin

func (p *compatPluginMock) Client() *plugins.Client {
	return p.plugin
}

func (p *compatPluginMock) Name() string {
	return p.name
}

func (p *compatPluginMock) BasePath() string {
	return ""
}

func (p *compatPluginMock) IsV1() bool {
	return false
}
