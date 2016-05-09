package authorization

import "github.com/docker/docker/pkg/plugins"

// Plugin allows third party plugins to authorize requests and responses
// in the context of docker API
type Plugin interface {
	// Name returns the registered plugin name
	Name() string

	// AuthZRequest authorizes the request from the client to the daemon
	AuthZRequest(*Request) (*Response, error)

	// AuthZResponse authorizes the response from the daemon to the client
	AuthZResponse(*Request) (*Response, error)
}

// NewPlugins constructs and initializes the authorization plugins based on plugin names
func NewPlugins(names []string) []Plugin {
	plugins := []Plugin{}
	pluginsMap := make(map[string]struct{})
	for _, name := range names {
		if _, ok := pluginsMap[name]; ok {
			continue
		}
		pluginsMap[name] = struct{}{}
		plugins = append(plugins, newAuthorizationPlugin(name))
	}
	return plugins
}

// authorizationPlugin is an internal adapter to docker plugin system
type authorizationPlugin struct {
	plugin *plugins.Plugin
	name   string
}

func newAuthorizationPlugin(name string) Plugin {
	return &authorizationPlugin{name: name}
}

func (a *authorizationPlugin) Name() string {
	return a.name
}

func (a *authorizationPlugin) AuthZRequest(authReq *Request) (*Response, error) {
	if err := a.initPlugin(); err != nil {
		return nil, err
	}

	authRes := &Response{}
	if err := a.plugin.Client.Call(AuthZApiRequest, authReq, authRes); err != nil {
		return nil, err
	}

	return authRes, nil
}

func (a *authorizationPlugin) AuthZResponse(authReq *Request) (*Response, error) {
	if err := a.initPlugin(); err != nil {
		return nil, err
	}

	authRes := &Response{}
	if err := a.plugin.Client.Call(AuthZApiResponse, authReq, authRes); err != nil {
		return nil, err
	}

	return authRes, nil
}

// initPlugin initializes the authorization plugin if needed
func (a *authorizationPlugin) initPlugin() error {
	// Lazy loading of plugins
	if a.plugin == nil {
		var err error
		a.plugin, err = plugins.Get(a.name, AuthZApiImplements)
		if err != nil {
			return err
		}
	}
	return nil
}
