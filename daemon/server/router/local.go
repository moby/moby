package router

import (
	"context"
	"net/http"

	"github.com/moby/moby/v2/daemon/internal/versions"
	"github.com/moby/moby/v2/daemon/server/httputils"
)

// RouteWrapper wraps a route with extra functionality.
// It is passed in when creating a new route.
type RouteWrapper func(r Route) Route

// localRoute defines an individual API route to connect
// with the docker daemon. It implements Route.
type localRoute struct {
	method  string
	path    string
	handler httputils.APIFunc
}

// Handler returns the APIFunc to let the server wrap it in middlewares.
func (l localRoute) Handler() httputils.APIFunc {
	return l.handler
}

// Method returns the http method that the route responds to.
func (l localRoute) Method() string {
	return l.method
}

// Path returns the subpath where the route responds to.
func (l localRoute) Path() string {
	return l.path
}

// NewRoute initializes a new local route for the router.
func NewRoute(method, path string, handler httputils.APIFunc, opts ...RouteWrapper) Route {
	var r Route = localRoute{method: method, path: path, handler: handler}
	for _, o := range opts {
		r = o(r)
	}
	return r
}

// NewGetRoute initializes a new route with the http method GET.
func NewGetRoute(path string, handler httputils.APIFunc, opts ...RouteWrapper) Route {
	return NewRoute(http.MethodGet, path, handler, opts...)
}

// NewPostRoute initializes a new route with the http method POST.
func NewPostRoute(path string, handler httputils.APIFunc, opts ...RouteWrapper) Route {
	return NewRoute(http.MethodPost, path, handler, opts...)
}

// NewPutRoute initializes a new route with the http method PUT.
func NewPutRoute(path string, handler httputils.APIFunc, opts ...RouteWrapper) Route {
	return NewRoute(http.MethodPut, path, handler, opts...)
}

// NewDeleteRoute initializes a new route with the http method DELETE.
func NewDeleteRoute(path string, handler httputils.APIFunc, opts ...RouteWrapper) Route {
	return NewRoute(http.MethodDelete, path, handler, opts...)
}

// NewOptionsRoute initializes a new route with the http method OPTIONS.
func NewOptionsRoute(path string, handler httputils.APIFunc, opts ...RouteWrapper) Route {
	return NewRoute(http.MethodOptions, path, handler, opts...)
}

// NewHeadRoute initializes a new route with the http method HEAD.
func NewHeadRoute(path string, handler httputils.APIFunc, opts ...RouteWrapper) Route {
	return NewRoute(http.MethodHead, path, handler, opts...)
}

// WithMinimumAPIVersion configures the minimum API version required for
// a route. It produces a 400 (Invalid Request) error when accessing the
// endpoint on API versions lower than "minAPIVersion".
//
// Note that technically, it should produce a 404 ("not found") error,
// as the endpoint should be considered "non-existing" on such API versions,
// but 404 status-codes are used in business logic for various endpoints.
func WithMinimumAPIVersion(minAPIVersion string) RouteWrapper {
	return func(route Route) Route {
		return localRoute{
			method: route.Method(),
			path:   route.Path(),
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
				if v := httputils.VersionFromContext(ctx); v != "" && versions.LessThan(v, minAPIVersion) {
					return versionError(route.Method() + " " + route.Path() + " requires minimum API version " + minAPIVersion)
				}
				return route.Handler()(ctx, w, r, vars)
			},
		}
	}
}

type versionError string

func (e versionError) Error() string {
	return string(e)
}
func (e versionError) InvalidParameter() {}
