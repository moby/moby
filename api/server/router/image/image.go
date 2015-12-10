package image

import "github.com/docker/docker/api/server/router"

// imageRouter is a docker router that talks with a docker backend.
type imageRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes an image router with a backend.
func NewRouter(backend Backend) router.Router {
	r := &imageRouter{
		backend: backend,
	}

	r.routes = []router.Route{
		// OPTIONS
		// GET
		router.NewGetRoute("/images/json", r.getImagesJSON),
		router.NewGetRoute("/images/search", r.getImagesSearch),
		router.NewGetRoute("/images/get", r.getImagesGet),
		router.NewGetRoute("/images/{name:.*}/get", r.getImagesGet),
		router.NewGetRoute("/images/{name:.*}/history", r.getImagesHistory),
		router.NewGetRoute("/images/{name:.*}/json", r.getImagesByName),
		// POST
		router.NewPostRoute("/commit", r.postCommit),
		router.NewPostRoute("/build", r.postBuild),
		router.NewPostRoute("/images/create", r.postImagesCreate),
		router.NewPostRoute("/images/load", r.postImagesLoad),
		router.NewPostRoute("/images/{name:.*}/push", r.postImagesPush),
		router.NewPostRoute("/images/{name:.*}/tag", r.postImagesTag),
		// DELETE
		router.NewDeleteRoute("/images/{name:.*}", r.deleteImages),
	}

	return r
}

// Routes returns the list of routes registered in the router.
func (r *imageRouter) Routes() []router.Route {
	return r.routes
}
