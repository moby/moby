package checkpoint

import (
	"github.com/moby/moby/v2/daemon/server/router"
)

// checkpointRouter is a router to talk with the checkpoint controller
type checkpointRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new checkpoint router
func NewRouter(b Backend) router.Router {
	r := &checkpointRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the checkpoint controller
func (cr *checkpointRouter) Routes() []router.Route {
	return cr.routes
}

func (cr *checkpointRouter) initRoutes() {
	cr.routes = []router.Route{
		router.NewGetRoute("/containers/{name:.*}/checkpoints", cr.getContainerCheckpoints, router.Experimental),
		router.NewPostRoute("/containers/{name:.*}/checkpoints", cr.postContainerCheckpoint, router.Experimental),
		router.NewDeleteRoute("/containers/{name}/checkpoints/{checkpoint}", cr.deleteContainerCheckpoint, router.Experimental),
	}
}
