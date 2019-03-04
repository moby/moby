package router

import "github.com/docker/docker/api/server/router"

type stacksRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter creates a new Stacks Router.
func NewRouter(b Backend) router.Router {
	r := &stacksRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns all routes of the stacks router.
func (sr *stacksRouter) Routes() []router.Route {
	return sr.routes
}

func (sr *stacksRouter) initRoutes() {
	sr.routes = []router.Route{
		router.NewGetRoute("/stacks", sr.getStacks),
		router.NewPostRoute("/stacks", sr.createStack),
		router.NewGetRoute("/stacks/{id}", sr.getStack),
		router.NewDeleteRoute("/stacks/{id}", sr.removeStack),
		router.NewPostRoute("/stacks/{id}", sr.updateStack),
		router.NewPostRoute("/parsecompose", sr.parseComposeInput),
	}
}
