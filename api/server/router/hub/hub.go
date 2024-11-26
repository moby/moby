package hub

import (
	"github.com/docker/docker/api/server/router"
)

type hubRouter struct {
	routes []router.Route
}

func NewRouter() router.Router {
	hr := &hubRouter{}
	hr.routes = []router.Route{
		router.NewGetRoute("/hub/image/{name:.*}/get", hr.getHubImageTags),
		router.NewGetRoute("/hub/image/search", hr.getHubImageSearch),
	}
	return hr
}

func (hr *hubRouter) Routes() []router.Route {
	return hr.routes
}
