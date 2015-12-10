package router

import "github.com/docker/docker/api/server/httputils"

// Router defines an interface to specify a group of routes to add the the docker server.
type Router interface {
	Routes() []Route
}

// Route defines an individual API route in the docker server.
type Route struct {
	method  string
	path    string
	handler httputils.APIFunc
}

// Handler returns the APIFunc to let the server wrap it in middlewares
func (l Route) Handler() httputils.APIFunc {
	return l.handler
}

// Method returns the http method that the route responds to.
func (l Route) Method() string {
	return l.method
}

// Path returns the subpath where the route responds to.
func (l Route) Path() string {
	return l.path
}

// NewRoute initialies a new local route for the reouter
func NewRoute(method, path string, handler httputils.APIFunc) Route {
	return Route{method, path, handler}
}

// NewGetRoute initializes a new route with the http method GET.
func NewGetRoute(path string, handler httputils.APIFunc) Route {
	return NewRoute("GET", path, handler)
}

// NewPostRoute initializes a new route with the http method POST.
func NewPostRoute(path string, handler httputils.APIFunc) Route {
	return NewRoute("POST", path, handler)
}

// NewPutRoute initializes a new route with the http method PUT.
func NewPutRoute(path string, handler httputils.APIFunc) Route {
	return NewRoute("PUT", path, handler)
}

// NewDeleteRoute initializes a new route with the http method DELETE.
func NewDeleteRoute(path string, handler httputils.APIFunc) Route {
	return NewRoute("DELETE", path, handler)
}

// NewOptionsRoute initializes a new route with the http method OPTIONS
func NewOptionsRoute(path string, handler httputils.APIFunc) Route {
	return NewRoute("OPTIONS", path, handler)
}

// NewHeadRoute initializes a new route with the http method HEAD.
func NewHeadRoute(path string, handler httputils.APIFunc) Route {
	return NewRoute("HEAD", path, handler)
}
