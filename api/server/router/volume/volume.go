package volume

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
)

// volumeRouter is a router to talk with the volumes controller
type volumeRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new volumeRouter
func NewRouter(b Backend) router.Router {
	r := &volumeRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

//Routes returns the available routers to the volumes controller
func (r *volumeRouter) Routes() []router.Route {
	return r.routes
}

func (r *volumeRouter) initRoutes() {
	r.routes = []router.Route{
		// GET
		local.NewGetRoute("/volumes", r.getVolumesList),
		local.NewGetRoute("/volumes/{name:.*}", r.getVolumeByName),
		// POST
		local.NewPostRoute("/volumes/create", r.postVolumesCreate),
		// DELETE
		local.NewDeleteRoute("/volumes/{name:.*}", r.deleteVolumes),
	}
}
