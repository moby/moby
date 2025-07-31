package session

import "github.com/moby/moby/v2/daemon/server/router"

// sessionRouter is a router to talk with the session controller
type sessionRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new session router
func NewRouter(b Backend) router.Router {
	r := &sessionRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the session controller
func (sr *sessionRouter) Routes() []router.Route {
	return sr.routes
}

func (sr *sessionRouter) initRoutes() {
	sr.routes = []router.Route{
		router.NewPostRoute("/session", sr.startSession),
	}
}
