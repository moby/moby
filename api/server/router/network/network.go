package network

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/pkg/locker"
)

// networkRouter is a router to talk with the network controller
type networkRouter struct {
	backend Backend
	routes  []router.Route
	locks   *locker.Locker
}

// NewRouter initializes a new network router
func NewRouter(b Backend) router.Router {
	r := &networkRouter{
		backend: b,
	}
	r.initRoutes()
	r.locks = locker.New()
	return r
}

// Routes returns the available routes to the network controller
func (r *networkRouter) Routes() []router.Route {
	return r.routes
}

func (r *networkRouter) initRoutes() {
	r.routes = []router.Route{
		// GET
		router.NewGetRoute("/networks", r.getNetworksList),
		router.NewGetRoute("/networks/{id:.*}", r.getNetwork),
		// POST
		router.NewPostRoute("/networks/create", r.postNetworkCreate),
		router.NewPostRoute("/networks/{id:.*}/connect", r.postNetworkConnect),
		router.NewPostRoute("/networks/{id:.*}/disconnect", r.postNetworkDisconnect),
		// DELETE
		router.NewDeleteRoute("/networks/{id:.*}", r.deleteNetwork),
	}
}
