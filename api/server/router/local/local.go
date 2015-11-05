package local

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/server/httputils"
	dkrouter "github.com/docker/docker/api/server/router"
	"github.com/docker/docker/daemon"
)

// router is a docker router that talks with the local docker daemon.
type router struct {
	daemon *daemon.Daemon
	routes []dkrouter.Route
}

// localRoute defines an individual API route to connect with the docker daemon.
// It implements router.Route.
type localRoute struct {
	method  string
	path    string
	handler httputils.APIFunc
}

// Handler returns the APIFunc to let the server wrap it in middlewares
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

// NewRoute initialies a new local route for the reouter
func NewRoute(method, path string, handler httputils.APIFunc) dkrouter.Route {
	return localRoute{method, path, handler}
}

// NewGetRoute initializes a new route with the http method GET.
func NewGetRoute(path string, handler httputils.APIFunc) dkrouter.Route {
	return NewRoute("GET", path, handler)
}

// NewPostRoute initializes a new route with the http method POST.
func NewPostRoute(path string, handler httputils.APIFunc) dkrouter.Route {
	return NewRoute("POST", path, handler)
}

// NewPutRoute initializes a new route with the http method PUT.
func NewPutRoute(path string, handler httputils.APIFunc) dkrouter.Route {
	return NewRoute("PUT", path, handler)
}

// NewDeleteRoute initializes a new route with the http method DELETE.
func NewDeleteRoute(path string, handler httputils.APIFunc) dkrouter.Route {
	return NewRoute("DELETE", path, handler)
}

// NewOptionsRoute initializes a new route with the http method OPTIONS
func NewOptionsRoute(path string, handler httputils.APIFunc) dkrouter.Route {
	return NewRoute("OPTIONS", path, handler)
}

// NewHeadRoute initializes a new route with the http method HEAD.
func NewHeadRoute(path string, handler httputils.APIFunc) dkrouter.Route {
	return NewRoute("HEAD", path, handler)
}

// NewRouter initializes a local router with a new daemon.
func NewRouter(daemon *daemon.Daemon) dkrouter.Router {
	r := &router{
		daemon: daemon,
	}
	r.initRoutes()
	return r
}

// Routes returns the list of routes registered in the router.
func (r *router) Routes() []dkrouter.Route {
	return r.routes
}

// initRoutes initializes the routes in this router
func (r *router) initRoutes() {
	r.routes = []dkrouter.Route{
		// OPTIONS
		NewOptionsRoute("/", optionsHandler),
		// GET
		NewGetRoute("/_ping", pingHandler),
		NewGetRoute("/events", r.getEvents),
		NewGetRoute("/info", r.getInfo),
		NewGetRoute("/version", r.getVersion),
		NewGetRoute("/images/json", r.getImagesJSON),
		NewGetRoute("/images/search", r.getImagesSearch),
		NewGetRoute("/images/get", r.getImagesGet),
		NewGetRoute("/images/{name:.*}/get", r.getImagesGet),
		NewGetRoute("/images/{name:.*}/history", r.getImagesHistory),
		NewGetRoute("/images/{name:.*}/json", r.getImagesByName),
		// POST
		NewPostRoute("/auth", r.postAuth),
		NewPostRoute("/commit", r.postCommit),
		NewPostRoute("/build", r.postBuild),
		NewPostRoute("/images/create", r.postImagesCreate),
		NewPostRoute("/images/load", r.postImagesLoad),
		NewPostRoute("/images/{name:.*}/push", r.postImagesPush),
		NewPostRoute("/images/{name:.*}/tag", r.postImagesTag),
		// DELETE
		NewDeleteRoute("/images/{name:.*}", r.deleteImages),
	}
}

func optionsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.WriteHeader(http.StatusOK)
	return nil
}

func pingHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	_, err := w.Write([]byte{'O', 'K'})
	return err
}
