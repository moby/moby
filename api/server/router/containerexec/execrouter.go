package containerexec

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
)

// execRouter is a router to route container Exec commands
type execRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new execRouter
func NewRouter(b Backend) router.Router {
	r := &execRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routes for execRouter
func (r *execRouter) Routes() []router.Route {
	return r.routes
}

// initRoutes initializes the routes in container router
func (r *execRouter) initRoutes() {
	r.routes = []router.Route{
		// GET
		local.NewGetRoute("/exec/{id:.*}/json", r.getExecByID),
		// POST
		local.NewPostRoute("/containers/{name:.*}/exec", r.postContainerExecCreate),
		local.NewPostRoute("/exec/{name:.*}/start", r.postContainerExecStart),
		local.NewPostRoute("/exec/{name:.*}/resize", r.postContainerExecResize),
	}
}
