package image

import "github.com/docker/docker/api/server/router"

// imageRouter is a router to talk with the image controller
type imageRouter struct {
	daemon Backend
	routes []router.Route
}

// NewRouter initializes a new image router
func NewRouter(daemon Backend) router.Router {
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
		router.NewGetRoute("/images/json", r.getImagesJSON),
		router.NewGetRoute("/images/search", r.getImagesSearch),
		router.NewGetRoute("/images/get", r.getImagesGet),
		router.NewGetRoute("/images/{name:.*}/get", r.getImagesGet),
		router.NewGetRoute("/images/{name:.*}/history", r.getImagesHistory),
		router.NewGetRoute("/images/{name:.*}/json", r.getImagesByName),
		// POST
		router.NewPostRoute("/commit", r.postCommit),
		router.NewPostRoute("/images/create", r.postImagesCreate),
		router.NewPostRoute("/images/load", r.postImagesLoad),
		router.NewPostRoute("/images/{name:.*}/push", r.postImagesPush),
		router.NewPostRoute("/images/{name:.*}/tag", r.postImagesTag),
		// DELETE
		router.NewDeleteRoute("/images/{name:.*}", r.deleteImages),
	}
}
