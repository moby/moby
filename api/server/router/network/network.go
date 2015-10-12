package network

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
	"github.com/docker/docker/daemon"
)

// networkRouter is a router to talk with the network controller
type networkRouter struct {
	daemon *daemon.Daemon
	routes []router.Route
}

// NewRouter initializes a new network router
func NewRouter(d *daemon.Daemon) router.Router {
	r := &networkRouter{
		daemon: d,
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
		local.NewGetRoute("/networks", r.getNetworksList),
		local.NewGetRoute("/networks/{id:.*}", r.getNetwork),
		// POST
		local.NewPostRoute("/networks/create", r.postNetworkCreate),
		local.NewPostRoute("/networks/{id:.*}/connect", r.postNetworkConnect),
		local.NewPostRoute("/networks/{id:.*}/disconnect", r.postNetworkDisconnect),
		// DELETE
		local.NewDeleteRoute("/networks/{id:.*}", r.deleteNetwork),
	}
}
