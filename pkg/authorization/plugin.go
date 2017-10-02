package authorization

import (
	"fmt"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
)

// Plugin allows third party plugins to authorize requests and responses
// in the context of docker API
type Plugin interface {
	// Name returns the registered plugin name
	Name() string

	// IsV1 is true if the plugin is a v1 plugin
	IsV1() bool

	// AuthZRequest authorizes the request from the client to the daemon
	AuthZRequest(*Request) (*Response, error)

	// AuthZResponse authorizes the response from the daemon to the client
	AuthZResponse(*Request) (*Response, error)
}

// newPlugins constructs and initializes the authorization plugins based on plugin names
func newPlugins(pg plugingetter.PluginGetter, names []string) ([]Plugin, error) {
	plugins := []Plugin{}
	pluginsMap := map[string]Plugin{}
	for _, name := range names {
		plugin, err := newAuthorizationPlugin(pg, name)
		if err != nil {
			return nil, fmt.Errorf("Error validating authorization plugin: %v", err)
		}
		name = plugin.Name()
		if p, ok := pluginsMap[name]; ok {
			if p.IsV1() == plugin.IsV1() {
				continue
			}
			if p.IsV1() {
				return nil, errV1V2Collision{"v1", "v2", name}
			}
			return nil, errV1V2Collision{"v2", "v1", name}
		}
		pluginsMap[name] = plugin
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

// authorizationPlugin is an internal adapter to docker plugin system
type authorizationPlugin struct {
	plugin *plugins.Client
	name   string
	isV1   bool
}

func newAuthorizationPlugin(pg plugingetter.PluginGetter, name string) (Plugin, error) {
	plugin, err := pg.Get(name, AuthZApiImplements, plugingetter.Lookup)
	if err != nil {
		return nil, err
	}
	return &authorizationPlugin{
		name:   plugin.Name(),
		plugin: plugin.Client(),
		isV1:   plugin.IsV1(),
	}, nil
}

func (a *authorizationPlugin) Name() string {
	return a.name
}

func (a *authorizationPlugin) IsV1() bool {
	return a.isV1
}

func (a *authorizationPlugin) AuthZRequest(authReq *Request) (*Response, error) {
	authRes := &Response{}
	if err := a.plugin.Call(AuthZApiRequest, authReq, authRes); err != nil {
		return nil, err
	}

	return authRes, nil
}

func (a *authorizationPlugin) AuthZResponse(authReq *Request) (*Response, error) {
	authRes := &Response{}
	if err := a.plugin.Call(AuthZApiResponse, authReq, authRes); err != nil {
		return nil, err
	}

	return authRes, nil
}
