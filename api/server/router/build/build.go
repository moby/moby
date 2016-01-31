package build

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
	"github.com/docker/docker/daemon"
)

// buildRouter is a router to talk with the build controller
type buildRouter struct {
	backend *daemon.Daemon
	routes  []router.Route
}

// NewRouter initializes a new build router
func NewRouter(b *daemon.Daemon) router.Router {
	r := &buildRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the build controller
func (r *buildRouter) Routes() []router.Route {
	return r.routes
}

func (r *buildRouter) initRoutes() {
	r.routes = []router.Route{
		local.NewPostRoute("/build", r.postBuild),
	}
}
