package image

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
	"github.com/docker/docker/daemon"
)

// imageRouter is a router to talk with the image controller
type imageRouter struct {
	daemon *daemon.Daemon
	routes []router.Route
}

// NewRouter initializes a new image router
func NewRouter(daemon *daemon.Daemon) router.Router {
	r := &imageRouter{
		daemon: daemon,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routes to the image controller
func (r *imageRouter) Routes() []router.Route {
	return r.routes
}

// initRoutes initializes the routes in the image router
func (r *imageRouter) initRoutes() {
	r.routes = []router.Route{
		// GET
		local.NewGetRoute("/images/json", r.getImagesJSON),
		local.NewGetRoute("/images/search", r.getImagesSearch),
		local.NewGetRoute("/images/get", r.getImagesGet),
		local.NewGetRoute("/images/{name:.*}/get", r.getImagesGet),
		local.NewGetRoute("/images/{name:.*}/history", r.getImagesHistory),
		local.NewGetRoute("/images/{name:.*}/json", r.getImagesByName),
		// POST
		local.NewPostRoute("/commit", r.postCommit),
		local.NewPostRoute("/images/create", r.postImagesCreate),
		local.NewPostRoute("/images/load", r.postImagesLoad),
		local.NewPostRoute("/images/{name:.*}/push", r.postImagesPush),
		local.NewPostRoute("/images/{name:.*}/tag", r.postImagesTag),
		// DELETE
		local.NewDeleteRoute("/images/{name:.*}", r.deleteImages),
	}
}
