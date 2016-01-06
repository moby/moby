package local

import (
	dkrouter "github.com/docker/docker/api/server/router"
	"github.com/docker/docker/daemon"
)

// router is a docker router that talks with the local docker daemon.
type router struct {
	daemon *daemon.Daemon
	routes []dkrouter.Route
}

// NewRouter initializes a local router with a new daemon.
func NewRouter(daemon *daemon.Daemon) dkrouter.Router {
	r := &router{
		daemon: daemon,
	}
	r.initRoutes()
	return r
}

// Routes returns the list of routes registered in the router.
func (r *router) Routes() []dkrouter.Route {
	return r.routes
}

// initRoutes initializes the routes in this router
func (r *router) initRoutes() {
	r.routes = []dkrouter.Route{
		// GET
		NewGetRoute("/images/json", r.getImagesJSON),
		NewGetRoute("/images/search", r.getImagesSearch),
		NewGetRoute("/images/get", r.getImagesGet),
		NewGetRoute("/images/{name:.*}/get", r.getImagesGet),
		NewGetRoute("/images/{name:.*}/history", r.getImagesHistory),
		NewGetRoute("/images/{name:.*}/json", r.getImagesByName),
		// POST
		NewPostRoute("/commit", r.postCommit),
		NewPostRoute("/images/create", r.postImagesCreate),
		NewPostRoute("/images/load", r.postImagesLoad),
		NewPostRoute("/images/{name:.*}/push", r.postImagesPush),
		NewPostRoute("/images/{name:.*}/tag", r.postImagesTag),
		// DELETE
		NewDeleteRoute("/images/{name:.*}", r.deleteImages),
	}
}
