package runtime

import "github.com/docker/docker/api/server/router"

// runtimeRouter is a router to talk with the runtime controller
type runtimeRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new runtime router
func NewRouter(b Backend) router.Router {
	r := &runtimeRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the runtime controller
func (rr *runtimeRouter) Routes() []router.Route {
	return rr.routes
}

func (rr *runtimeRouter) initRoutes() {
	rr.routes = []router.Route{
		// GET
		router.NewGetRoute("/runtimes", rr.getRuntimes),

		// POST
		router.NewPostRoute("/runtimes/{id}/default", rr.postRuntimeDefault),
	}
}
