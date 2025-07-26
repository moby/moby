package network

import (
	"github.com/moby/moby/v2/daemon/server/router"
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
func (n *networkRouter) Routes() []router.Route {
	return n.routes
}

func (n *networkRouter) initRoutes() {
	n.routes = []router.Route{
		// GET
		router.NewGetRoute("/networks", n.getNetworksList),
		router.NewGetRoute("/networks/", n.getNetworksList),
		router.NewGetRoute("/networks/{id:.+}", n.getNetwork),
		// POST
		router.NewPostRoute("/networks/create", n.postNetworkCreate),
		router.NewPostRoute("/networks/{id:.*}/connect", n.postNetworkConnect),
		router.NewPostRoute("/networks/{id:.*}/disconnect", n.postNetworkDisconnect),
		router.NewPostRoute("/networks/prune", n.postNetworksPrune),
		// DELETE
		router.NewDeleteRoute("/networks/{id:.*}", n.deleteNetwork),
	}
}
