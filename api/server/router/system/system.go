package system

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/daemon/cluster"
)

// systemRouter provides information about the Docker system overall.
// It gathers information about host, daemon and container events.
type systemRouter struct {
	backend         Backend
	clusterProvider *cluster.Cluster
	routes          []router.Route
}

// NewRouter initializes a new system router
func NewRouter(b Backend, c *cluster.Cluster) router.Router {
	r := &systemRouter{
		backend:         b,
		clusterProvider: c,
	}

	r.routes = []router.Route{
		router.NewOptionsRoute("/{anyroute:.*}", optionsHandler),
		router.NewGetRoute("/_ping", pingHandler),
		router.Cancellable(router.NewGetRoute("/events", r.getEvents)),
		router.NewGetRoute("/info", r.getInfo),
		router.NewGetRoute("/version", r.getVersion),
		router.NewGetRoute("/system/df", r.getDiskUsage),
		router.NewPostRoute("/auth", r.postAuth),
	}

	return r
}

// Routes returns all the API routes dedicated to the docker system
func (s *systemRouter) Routes() []router.Route {
	return s.routes
}
