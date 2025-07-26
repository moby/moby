package plugin

import "github.com/moby/moby/v2/daemon/server/router"

// pluginRouter is a router to talk with the plugin controller
type pluginRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new plugin router
func NewRouter(b Backend) router.Router {
	r := &pluginRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the plugin controller
func (pr *pluginRouter) Routes() []router.Route {
	return pr.routes
}

func (pr *pluginRouter) initRoutes() {
	pr.routes = []router.Route{
		router.NewGetRoute("/plugins", pr.listPlugins),
		router.NewGetRoute("/plugins/{name:.*}/json", pr.inspectPlugin),
		router.NewGetRoute("/plugins/privileges", pr.getPrivileges),
		router.NewDeleteRoute("/plugins/{name:.*}", pr.removePlugin),
		router.NewPostRoute("/plugins/{name:.*}/enable", pr.enablePlugin),
		router.NewPostRoute("/plugins/{name:.*}/disable", pr.disablePlugin),
		router.NewPostRoute("/plugins/pull", pr.pullPlugin),
		router.NewPostRoute("/plugins/{name:.*}/push", pr.pushPlugin),
		router.NewPostRoute("/plugins/{name:.*}/upgrade", pr.upgradePlugin),
		router.NewPostRoute("/plugins/{name:.*}/set", pr.setPlugin),
		router.NewPostRoute("/plugins/create", pr.createPlugin),
	}
}
