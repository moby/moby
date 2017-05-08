// +build experimental

package main

import (
	"github.com/docker/docker/api/server/router"
	pluginrouter "github.com/docker/docker/api/server/router/plugin"
	"github.com/docker/docker/plugin"
)

func addExperimentalRouters(routers []router.Router) []router.Router {
	return append(routers, pluginrouter.NewRouter(plugin.GetManager()))
}
