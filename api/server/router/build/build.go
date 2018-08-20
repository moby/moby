package build // import "github.com/docker/docker/api/server/router/build"

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/types"
)

// buildRouter is a router to talk with the build controller
type buildRouter struct {
	backend        Backend
	daemon         experimentalProvider
	routes         []router.Route
	builderVersion types.BuilderVersion
}

// NewRouter initializes a new build router
func NewRouter(b Backend, d experimentalProvider, bv types.BuilderVersion) router.Router {
	r := &buildRouter{backend: b, daemon: d, builderVersion: bv}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the build controller
func (r *buildRouter) Routes() []router.Route {
	return r.routes
}

func (r *buildRouter) initRoutes() {
	r.routes = []router.Route{
		router.NewPostRoute("/build", r.postBuild, router.WithCancel),
		router.NewPostRoute("/build/prune", r.postPrune, router.WithCancel),
		router.NewPostRoute("/build/cancel", r.postCancel),
	}
}
