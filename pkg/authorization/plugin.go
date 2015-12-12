package authorization

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
)

// Plugin allows third party plugins to authorize requests and responses
// in the context of docker API
type Plugin interface {

	// AuthZRequest authorize the request from the client to the daemon
	AuthZRequest(authReq *Request) (authRes *Response, err error)

	// AuthZResponse authorize the response from the daemon to the client
	AuthZResponse(authReq *Request) (authRes *Response, err error)
}

// NewPlugins constructs and initialize the authorization plugins based on plugin names
func NewPlugins(names []string) []Plugin {
	plugins := make([]Plugin, len(names))
	for i, name := range names {
		plugins[i] = newAuthorizationPlugin(name)
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

func (a *authorizationPlugin) AuthZRequest(authReq *Request) (authRes *Response, err error) {

	logrus.Debugf("AuthZ requset using plugins %s", a.name)

	err = a.initPlugin()
	if err != nil {
		return nil, err
	}

	authRes = &Response{}
	err = a.plugin.Client.Call(AuthZApiRequest, authReq, authRes)

	if err != nil {
		return nil, err
	}

	return authRes, nil
}

func (a *authorizationPlugin) AuthZResponse(authReq *Request) (authRes *Response, err error) {

	logrus.Debugf("AuthZ response using plugins %s", a.name)

	err = a.initPlugin()
	if err != nil {
		return nil, err
	}

	authRes = &Response{}
	err = a.plugin.Client.Call(AuthZApiResponse, authReq, authRes)

	if err != nil {
		return nil, err
	}

	return authRes, nil
}

// initPlugin initialize the authorization plugin if needed
func (a *authorizationPlugin) initPlugin() (err error) {

	// Lazy loading of plugins
	if a.plugin == nil {
		a.plugin, err = plugins.Get(a.name, AuthZApiImplements)
		if err != nil {
			return err
		}
	}
	return nil
}
