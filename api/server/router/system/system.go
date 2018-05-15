package system // import "github.com/docker/docker/api/server/router/system"

import (
	"github.com/docker/docker/api/server/router"
	buildkit "github.com/docker/docker/builder/builder-next"
	"github.com/docker/docker/builder/fscache"
)

// systemRouter provides information about the Docker system overall.
// It gathers information about host, daemon and container events.
type systemRouter struct {
	backend Backend
	cluster ClusterBackend
	routes  []router.Route
	fscache *fscache.FSCache // legacy
	builder *buildkit.Builder
}

// NewRouter initializes a new system router
func NewRouter(b Backend, c ClusterBackend, fscache *fscache.FSCache, builder *buildkit.Builder) router.Router {
	r := &systemRouter{
		backend: b,
		cluster: c,
		fscache: fscache,
		builder: builder,
	}

	r.routes = []router.Route{
		router.NewOptionsRoute("/{anyroute:.*}", optionsHandler),
		router.NewGetRoute("/_ping", pingHandler),
		router.NewGetRoute("/events", r.getEvents, router.WithCancel),
		router.NewGetRoute("/info", r.getInfo),
		router.NewGetRoute("/version", r.getVersion),
		router.NewGetRoute("/system/df", r.getDiskUsage, router.WithCancel),
		router.NewPostRoute("/auth", r.postAuth),
	}

	return r
}

// Routes returns all the API routes dedicated to the docker system
func (s *systemRouter) Routes() []router.Route {
	return s.routes
}
