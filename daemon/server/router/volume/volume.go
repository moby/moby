package volume

import "github.com/moby/moby/v2/daemon/server/router"

// volumeRouter is a router to talk with the volumes controller
type volumeRouter struct {
	backend    Backend
	cluster    ClusterBackend
	containers ContainerNamer
	routes     []router.Route
}

// NewRouter initializes a new volume router.
//
// containers provides container ID -> name resolution used to populate the
// `Containers` field on volume list/inspect responses.
func NewRouter(b Backend, cb ClusterBackend, containers ContainerNamer) router.Router {
	r := &volumeRouter{
		backend:    b,
		cluster:    cb,
		containers: containers,
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
		router.NewPostRoute("/volumes/prune", v.postVolumesPrune, router.WithMinimumAPIVersion("1.25")),
		// PUT
		router.NewPutRoute("/volumes/{name:.*}", v.putVolumesUpdate, router.WithMinimumAPIVersion("1.42")),
		// DELETE
		router.NewDeleteRoute("/volumes/{name:.*}", v.deleteVolumes),
	}
}
