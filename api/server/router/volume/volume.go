package volume

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
	"github.com/docker/docker/daemon"
)

// volumesRouter is a router to talk with the volumes controller
type volumeRouter struct {
	daemon *daemon.Daemon
	routes []router.Route
}

// NewRouter initializes a new volumes router
func NewRouter(d *daemon.Daemon) router.Router {
	r := &volumeRouter{
		daemon: d,
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
