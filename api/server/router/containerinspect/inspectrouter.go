package containerinspect

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
)

// inspectRouter is a router to route container Exec commands
type inspectRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new inspectRouter
func NewRouter(b Backend) router.Router {
	r := &inspectRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routes for inspectRouter
func (r *inspectRouter) Routes() []router.Route {
	return r.routes
}

// initRoutes initializes the routes in container router
func (r *inspectRouter) initRoutes() {
	r.routes = []router.Route{
		// GET
		local.NewGetRoute("/containers/{name:.*}/json", r.getContainersByName),
	}
}
