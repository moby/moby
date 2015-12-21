package authorization

import "github.com/docker/docker/pkg/plugins"

// Plugin allows third party plugins to authorize requests and responses
// in the context of docker API
type Plugin interface {
	// Name returns the registered plugin name
	Name() string

	// AuthZRequest authorize the request from the client to the daemon
	AuthZRequest(*Request) (*Response, error)

	// AuthZResponse authorize the response from the daemon to the client
	AuthZResponse(*Request) (*Response, error)
}

// NewPlugins constructs and initialize the authorization plugins based on plugin names
func NewPlugins(names []string) ([]Plugin, error) {
	plugins := []Plugin{}
	pluginsMap := make(map[string]struct{})
	for _, name := range names {
		if _, ok := pluginsMap[name]; ok {
			continue
		}
		pluginsMap[name] = struct{}{}
		plugin, err := newAuthorizationPlugin(name)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

// authorizationPlugin is an internal adapter to docker plugin system
type authorizationPlugin struct {
	plugin *plugins.Plugin
	name   string
}

func newAuthorizationPlugin(name string) (Plugin, error) {
	plugin, err := plugins.Get(name, AuthZApiImplements)
	if err != nil {
		return nil, err
	}
	return &authorizationPlugin{name: name, plugin: plugin}, nil
}

func (a *authorizationPlugin) Name() string {
	return a.name
}

func (a *authorizationPlugin) AuthZRequest(authReq *Request) (*Response, error) {
	authRes := &Response{}
	if err := a.plugin.Client.Call(AuthZApiRequest, authReq, authRes); err != nil {
		return nil, err
	}
	return authRes, nil
}

func (a *authorizationPlugin) AuthZResponse(authReq *Request) (*Response, error) {
	authRes := &Response{}
	if err := a.plugin.Client.Call(AuthZApiResponse, authReq, authRes); err != nil {
		return nil, err
	}
	return authRes, nil
}
