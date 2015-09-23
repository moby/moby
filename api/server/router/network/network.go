package network

import (
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
)

// networkRouter is a router to talk with the network controller
type networkRouter struct {
	routes []router.Route
}

// Routes returns the available routes to the network controller
func (n networkRouter) Routes() []router.Route {
	return n.routes
}

type networkRoute struct {
	path    string
	handler httputils.APIFunc
}

// Handler returns the APIFunc to let the server wrap it in middlewares
func (l networkRoute) Handler() httputils.APIFunc {
	return l.handler
}
