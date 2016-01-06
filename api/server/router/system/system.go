package system

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
)

// systemRouter provides information about the Docker system overall.
// It gathers information about host, daemon and container events.
type systemRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new system router
func NewRouter(b Backend) router.Router {
	r := &systemRouter{
		backend: b,
	}

	r.routes = []router.Route{
		// OPTIONS
		local.NewOptionsRoute("/", optionsHandler),
		// GET
		local.NewGetRoute("/_ping", pingHandler),
		local.NewGetRoute("/events", r.getEvents),
		local.NewGetRoute("/info", r.getInfo),
		local.NewGetRoute("/version", r.getVersion),
		// POST
		local.NewPostRoute("/auth", r.postAuth),
	}

	return r
}

// Routes returns all the API routes dedicated to the docker system
func (s *systemRouter) Routes() []router.Route {
	return s.routes
}
