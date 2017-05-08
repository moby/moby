package plugin

import "github.com/docker/docker/api/server/router"

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
func (r *pluginRouter) Routes() []router.Route {
	return r.routes
}
