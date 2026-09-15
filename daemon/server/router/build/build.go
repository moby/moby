package build

import "github.com/moby/moby/v2/daemon/server/router"

// buildRouter is a router to talk with the build controller
type buildRouter struct {
	backend Backend
	daemon  experimentalProvider
	routes  []router.Route
}

// NewRouter initializes a new build router
func NewRouter(b Backend, d experimentalProvider) router.Router {
	r := &buildRouter{
		backend: b,
		daemon:  d,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the build controller
func (br *buildRouter) Routes() []router.Route {
	return br.routes
}

func (br *buildRouter) initRoutes() {
	br.routes = []router.Route{
		router.NewPostRoute("/build", br.postBuild),
		router.NewPostRoute("/build/prune", br.postPrune, router.WithMinimumAPIVersion("1.31")),
		router.NewPostRoute("/build/cancel", br.postCancel),
	}
}
