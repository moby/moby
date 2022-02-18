package network // import "github.com/moby/moby/api/server/router/network"

import (
	"github.com/moby/moby/api/server/router"
)

// networkRouter is a router to talk with the network controller
type networkRouter struct {
	backend Backend
	cluster ClusterBackend
	routes  []router.Route
}

// NewRouter initializes a new network router
func NewRouter(b Backend, c ClusterBackend) router.Router {
	r := &networkRouter{
		backend: b,
		cluster: c,
	}
	r.initRoutes()
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
		router.NewGetRoute("/networks/", r.getNetworksList),
		router.NewGetRoute("/networks/{id:.+}", r.getNetwork),
		// POST
		router.NewPostRoute("/networks/create", r.postNetworkCreate),
		router.NewPostRoute("/networks/{id:.*}/connect", r.postNetworkConnect),
		router.NewPostRoute("/networks/{id:.*}/disconnect", r.postNetworkDisconnect),
		router.NewPostRoute("/networks/prune", r.postNetworksPrune),
		// DELETE
		router.NewDeleteRoute("/networks/{id:.*}", r.deleteNetwork),
	}
}
