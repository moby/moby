package plugin

import (
	"context"

	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/component/plugincontroller"
)

// pluginRouter is a router to talk with the plugin controller
type pluginRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new plugin router
func NewRouter() router.Router {
	b := plugincontroller.Wait(context.Background())
	r := &pluginRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the plugin controller
func (r *pluginRouter) Routes() []router.Route {
	return r.routes
}

func (r *pluginRouter) initRoutes() {
	r.routes = []router.Route{
		router.NewGetRoute("/plugins", r.listPlugins),
		router.NewGetRoute("/plugins/{name:.*}/json", r.inspectPlugin),
		router.NewGetRoute("/plugins/privileges", r.getPrivileges),
		router.NewDeleteRoute("/plugins/{name:.*}", r.removePlugin),
		router.NewPostRoute("/plugins/{name:.*}/enable", r.enablePlugin),
		router.NewPostRoute("/plugins/{name:.*}/disable", r.disablePlugin),
		router.NewPostRoute("/plugins/pull", r.pullPlugin, router.WithCancel),
		router.NewPostRoute("/plugins/{name:.*}/push", r.pushPlugin, router.WithCancel),
		router.NewPostRoute("/plugins/{name:.*}/upgrade", r.upgradePlugin, router.WithCancel),
		router.NewPostRoute("/plugins/{name:.*}/set", r.setPlugin),
		router.NewPostRoute("/plugins/create", r.createPlugin),
	}
}
