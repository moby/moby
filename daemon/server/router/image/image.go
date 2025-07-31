package image

import (
	"github.com/moby/moby/v2/daemon/server/router"
)

// imageRouter is a router to talk with the image controller
type imageRouter struct {
	backend  Backend
	searcher Searcher
	routes   []router.Route
}

// NewRouter initializes a new image router
func NewRouter(backend Backend, searcher Searcher) router.Router {
	ir := &imageRouter{
		backend:  backend,
		searcher: searcher,
	}
	ir.initRoutes()
	return ir
}

// Routes returns the available routes to the image controller
func (ir *imageRouter) Routes() []router.Route {
	return ir.routes
}

// initRoutes initializes the routes in the image router
func (ir *imageRouter) initRoutes() {
	ir.routes = []router.Route{
		// GET
		router.NewGetRoute("/images/json", ir.getImagesJSON),
		router.NewGetRoute("/images/search", ir.getImagesSearch),
		router.NewGetRoute("/images/get", ir.getImagesGet),
		router.NewGetRoute("/images/{name:.*}/get", ir.getImagesGet),
		router.NewGetRoute("/images/{name:.*}/history", ir.getImagesHistory),
		router.NewGetRoute("/images/{name:.*}/json", ir.getImagesByName),
		// POST
		router.NewPostRoute("/images/load", ir.postImagesLoad),
		router.NewPostRoute("/images/create", ir.postImagesCreate),
		router.NewPostRoute("/images/{name:.*}/push", ir.postImagesPush),
		router.NewPostRoute("/images/{name:.*}/tag", ir.postImagesTag),
		router.NewPostRoute("/images/prune", ir.postImagesPrune),
		// DELETE
		router.NewDeleteRoute("/images/{name:.*}", ir.deleteImages),
	}
}
