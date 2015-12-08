package image

import (
	dkrouter "github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
)

// imageRouter is a docker router that talks with the local docker daemon.
type imageRouter struct {
	backend Backend
	routes  []dkrouter.Route
}

// NewRouter initializes a local router with a new daemon.
func NewRouter(backend Backend) dkrouter.Router {
	r := &imageRouter{
		backend: backend,
	}

	r.routes = []dkrouter.Route{
		// OPTIONS
		// GET
		local.NewGetRoute("/images/json", r.getImagesJSON),
		local.NewGetRoute("/images/search", r.getImagesSearch),
		local.NewGetRoute("/images/get", r.getImagesGet),
		local.NewGetRoute("/images/{name:.*}/get", r.getImagesGet),
		local.NewGetRoute("/images/{name:.*}/history", r.getImagesHistory),
		local.NewGetRoute("/images/{name:.*}/json", r.getImagesByName),
		// POST
		local.NewPostRoute("/commit", r.postCommit),
		local.NewPostRoute("/build", r.postBuild),
		local.NewPostRoute("/images/create", r.postImagesCreate),
		local.NewPostRoute("/images/load", r.postImagesLoad),
		local.NewPostRoute("/images/{name:.*}/push", r.postImagesPush),
		local.NewPostRoute("/images/{name:.*}/tag", r.postImagesTag),
		// DELETE
		local.NewDeleteRoute("/images/{name:.*}", r.deleteImages),
	}

	return r
}

// Routes returns the list of routes registered in the router.
func (r *imageRouter) Routes() []dkrouter.Route {
	return r.routes
}
