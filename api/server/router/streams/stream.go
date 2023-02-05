package streams

import "github.com/docker/docker/api/server/router"

type streamRouter struct {
	backend Backend
	routes  []router.Route
}

func (r *streamRouter) Routes() []router.Route {
	ret := make([]router.Route, 0, len(r.routes))
	for _, route := range r.routes {
		ret = append(ret, route)
	}
	return ret
}

func (r *streamRouter) initRoutes() {
	r.routes = []router.Route{
		router.NewPostRoute("/streams/create", r.createStream),
		router.NewGetRoute("/streams/{id:.*}", r.getStream),
		router.NewDeleteRoute("/streams/{id:.*}", r.deleteStream),
	}
}

func NewRouter(b Backend) router.Router {
	r := &streamRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}
