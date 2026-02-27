package server

import (
	"context"
	"net/http"

	"github.com/containerd/log"
	"github.com/gorilla/mux"
	"github.com/moby/moby/api/types/common"
	"github.com/moby/moby/v2/daemon/internal/otelutil"
	"github.com/moby/moby/v2/daemon/internal/versions"
	"github.com/moby/moby/v2/daemon/server/httpstatus"
	"github.com/moby/moby/v2/daemon/server/httputils"
	"github.com/moby/moby/v2/daemon/server/middleware"
	"github.com/moby/moby/v2/daemon/server/router"
	"github.com/moby/moby/v2/dockerversion"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/baggage"
)

// versionMatcher defines a variable matcher to be parsed by the router
// when a request is about to be served.
const versionMatcher = "/v{version:[0-9.]+}"

// Server contains instance details for the server
type Server struct {
	middlewares []middleware.Middleware
}

// UseMiddleware appends a new middleware to the request chain.
// This needs to be called before the API routes are configured.
func (s *Server) UseMiddleware(m middleware.Middleware) {
	s.middlewares = append(s.middlewares, m)
}

func (s *Server) makeHTTPHandler(r router.Route) http.HandlerFunc {
	handler := r.Handler()
	operation := r.Method() + " " + r.Path()
	return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Define the context that we'll pass around to share info
		// like the docker-request-id.
		//
		// The 'context' will be used for global data that should
		// apply to all requests. Data that is specific to the
		// immediate function being called should still be passed
		// as 'args' on the function call.

		// use intermediate variable to prevent "should not use basic type
		// string as key in context.WithValue" golint errors
		ua := r.Header.Get("User-Agent")
		ctx := baggage.ContextWithBaggage(context.WithValue(r.Context(), dockerversion.UAStringKey{}, ua), otelutil.MustNewBaggage(
			otelutil.MustNewMemberRaw(otelutil.TriggerKey, "api"),
		))

		r = r.WithContext(ctx)
		handlerFunc := s.handlerWithGlobalMiddlewares(handler)

		vars := mux.Vars(r)
		if vars == nil {
			vars = make(map[string]string)
		}

		// Close the body when the context is done
		context.AfterFunc(ctx, func() { r.Body.Close() })

		if err := handlerFunc(ctx, w, r, vars); err != nil {
			statusCode := httpstatus.FromError(err)
			if statusCode >= http.StatusInternalServerError {
				log.G(ctx).Errorf("Handler for %s %s returned error: %v", r.Method, r.URL.Path, err)
			}
			// While we no longer support API versions older than 1.24 [config.DefaultMinAPIVersion],
			// a client may try to connect using an older version and expect a plain-text error
			// instead of a JSON error. This would result in an "API version too old" error
			// formatted in JSON being printed as-is.
			//
			// Let's be nice, and return errors in plain-text to provide a more readable error
			// to help the user understand the API version they're using is no longer supported.
			if v := vars["version"]; v != "" && versions.LessThan(v, "1.24") {
				http.Error(w, err.Error(), statusCode)
			} else {
				_ = httputils.WriteJSON(w, statusCode, &common.ErrorResponse{
					Message: err.Error(),
				})
			}
		}
	}), operation).ServeHTTP
}

// CreateMux returns a new mux with all the routers registered.
func (s *Server) CreateMux(ctx context.Context, routers ...router.Router) *mux.Router {
	log.G(ctx).Debug("Registering routers")
	m := mux.NewRouter()
	for _, apiRouter := range routers {
		for _, r := range apiRouter.Routes() {
			if ctx.Err() != nil {
				return m
			}
			log.G(ctx).WithFields(log.Fields{"method": r.Method(), "path": r.Path()}).Debug("Registering route")
			f := s.makeHTTPHandler(r)
			m.Path(versionMatcher + r.Path()).Methods(r.Method()).Handler(f)
			m.Path(r.Path()).Methods(r.Method()).Handler(f)
		}
	}

	// Setup handlers for undefined paths and methods
	notFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = httputils.WriteJSON(w, http.StatusNotFound, &common.ErrorResponse{
			Message: "page not found",
		})
	})

	m.HandleFunc(versionMatcher+"/{path:.*}", notFoundHandler)
	m.NotFoundHandler = notFoundHandler
	m.MethodNotAllowedHandler = notFoundHandler

	return m
}
