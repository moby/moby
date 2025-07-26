package volume

import "github.com/moby/moby/v2/daemon/server/router"

// volumeRouter is a router to talk with the volumes controller
type volumeRouter struct {
	backend Backend
	cluster ClusterBackend
	routes  []router.Route
}

// NewRouter initializes a new volume router
func NewRouter(b Backend, cb ClusterBackend) router.Router {
	r := &volumeRouter{
		backend: b,
		cluster: cb,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routes to the volumes controller
func (v *volumeRouter) Routes() []router.Route {
	return v.routes
}

func (v *volumeRouter) initRoutes() {
	v.routes = []router.Route{
		// GET
		router.NewGetRoute("/volumes", v.getVolumesList),
		router.NewGetRoute("/volumes/{name:.*}", v.getVolumeByName),
		// POST
		router.NewPostRoute("/volumes/create", v.postVolumesCreate),
		router.NewPostRoute("/volumes/prune", v.postVolumesPrune),
		// PUT
		router.NewPutRoute("/volumes/{name:.*}", v.putVolumesUpdate),
		// DELETE
		router.NewDeleteRoute("/volumes/{name:.*}", v.deleteVolumes),
	}
}
