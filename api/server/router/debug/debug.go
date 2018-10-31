package debug // import "github.com/docker/docker/api/server/router/debug"

import (
	"context"
	"expvar"
	"io"
	"net/http"
	"net/http/pprof"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
)

// NewRouter creates a new debug router
// The debug router holds endpoints for debug the daemon, such as those for pprof.
func NewRouter(b Backend) router.Router {
	r := &debugRouter{}
	r.initRoutes(b)
	return r
}

type debugRouter struct {
	routes []router.Route
}

// Backend for debugging
type Backend interface {
	SupportDump(context.Context) (io.Reader, error)
}

func (r *debugRouter) initRoutes(b Backend) {
	r.routes = []router.Route{
		router.NewGetRoute("/debug/vars", frameworkAdaptHandler(expvar.Handler())),
		router.NewGetRoute("/debug/pprof/", frameworkAdaptHandlerFunc(pprof.Index)),
		router.NewGetRoute("/debug/pprof/cmdline", frameworkAdaptHandlerFunc(pprof.Cmdline)),
		router.NewGetRoute("/debug/pprof/profile", frameworkAdaptHandlerFunc(pprof.Profile)),
		router.NewGetRoute("/debug/pprof/symbol", frameworkAdaptHandlerFunc(pprof.Symbol)),
		router.NewGetRoute("/debug/pprof/trace", frameworkAdaptHandlerFunc(pprof.Trace)),
		router.NewGetRoute("/debug/pprof/{name}", handlePprof),
		router.NewPostRoute("/debug/dump", handleDumpFunc(b)),
	}
}

func (r *debugRouter) Routes() []router.Route {
	return r.routes
}

func frameworkAdaptHandler(handler http.Handler) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		handler.ServeHTTP(w, r)
		return nil
	}
}

func frameworkAdaptHandlerFunc(handler http.HandlerFunc) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		handler(w, r)
		return nil
	}
}
