package volume // import "github.com/docker/docker/api/server/router/volume"

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/container"
)

// volumeRouter is a router to talk with the volumes controller
type volumeRouter struct {
	backend          Backend
	cluster          ClusterBackend
	routes           []router.Route
	containerBackend container.Backend
}

// NewRouter initializes a new volume router
func NewRouter(b Backend, cb ClusterBackend, ctrb container.Backend) router.Router {
	r := &volumeRouter{
		backend:          b,
		cluster:          cb,
		containerBackend: ctrb,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routes to the volumes controller
func (r *volumeRouter) Routes() []router.Route {
	return r.routes
}

func (r *volumeRouter) initRoutes() {
	r.routes = []router.Route{
		// GET
		router.NewGetRoute("/volumes/{name:.*}/export", r.getVolumeExport),
		router.NewGetRoute("/volumes", r.getVolumesList),
		router.NewGetRoute("/volumes/{name:.*}", r.getVolumeByName),
		// POST
		router.NewPostRoute("/volumes/create", r.postVolumesCreate),
		router.NewPostRoute("/volumes/prune", r.postVolumesPrune),
		// PUT
		router.NewPutRoute("/volumes/{name:.*}", r.putVolumesUpdate),
		// DELETE
		router.NewDeleteRoute("/volumes/{name:.*}", r.deleteVolumes),
	}
}
