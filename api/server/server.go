package server

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/utils"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

// versionMatcher defines a variable matcher to be parsed by the router
// when a request is about to be served.
const versionMatcher = "/v{version:[0-9.]+}"

// Config provides the configuration for the API server
type Config struct {
	Logging                  bool
	EnableCors               bool
	CorsHeaders              string
	AuthorizationPluginNames []string
	Version                  string
	SocketGroup              string
	TLSConfig                *tls.Config
}

// Server contains instance details for the server
type Server struct {
	cfg           *Config
	servers       []*HTTPServer
	routers       []router.Router
	authZPlugins  []authorization.Plugin
	routerSwapper *routerSwapper
}

// New returns a new instance of the server based on the specified configuration.
// It allocates resources which will be needed for ServeAPI(ports, unix-sockets).
func New(cfg *Config) *Server {
	return &Server{
		cfg: cfg,
	}
}

// Accept sets a listener the server accepts connections into.
func (s *Server) Accept(addr string, listeners ...net.Listener) {
	for _, listener := range listeners {
		httpServer := &HTTPServer{
			srv: &http.Server{
				Addr: addr,
			},
			l: listener,
		}
		s.servers = append(s.servers, httpServer)
	}
}

// Close closes servers and thus stop receiving requests
func (s *Server) Close() {
	for _, srv := range s.servers {
		if err := srv.Close(); err != nil {
			logrus.Error(err)
		}
	}
}

// serveAPI loops through all initialized servers and spawns goroutine
// with Server method for each. It sets createMux() as Handler also.
func (s *Server) serveAPI() error {
	s.initRouterSwapper()

	var chErrors = make(chan error, len(s.servers))
	for _, srv := range s.servers {
		srv.srv.Handler = s.routerSwapper
		go func(srv *HTTPServer) {
			var err error
			logrus.Infof("API listen on %s", srv.l.Addr())
			if err = srv.Serve(); err != nil && strings.Contains(err.Error(), "use of closed network connection") {
				err = nil
			}
			chErrors <- err
		}(srv)
	}

	for i := 0; i < len(s.servers); i++ {
		err := <-chErrors
		if err != nil {
			return err
		}
	}

	return nil
}

// HTTPServer contains an instance of http server and the listener.
// srv *http.Server, contains configuration to create a http server and a mux router with all api end points.
// l   net.Listener, is a TCP or Socket listener that dispatches incoming request to the router.
type HTTPServer struct {
	srv *http.Server
	l   net.Listener
}

// Serve starts listening for inbound requests.
func (s *HTTPServer) Serve() error {
	return s.srv.Serve(s.l)
}

// Close closes the HTTPServer from listening for the inbound requests.
func (s *HTTPServer) Close() error {
	return s.l.Close()
}

func writeCorsHeaders(w http.ResponseWriter, r *http.Request, corsHeaders string) {
	logrus.Debugf("CORS header is enabled and set to: %s", corsHeaders)
	w.Header().Add("Access-Control-Allow-Origin", corsHeaders)
	w.Header().Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth")
	w.Header().Add("Access-Control-Allow-Methods", "HEAD, GET, POST, DELETE, PUT, OPTIONS")
}

func (s *Server) makeHTTPHandler(handler httputils.APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// log the handler call
		logrus.Debugf("Calling %s %s", r.Method, r.URL.Path)

		// Define the context that we'll pass around to share info
		// like the docker-request-id.
		//
		// The 'context' will be used for global data that should
		// apply to all requests. Data that is specific to the
		// immediate function being called should still be passed
		// as 'args' on the function call.
		ctx := context.Background()
		handlerFunc := s.handleWithGlobalMiddlewares(handler)

		vars := mux.Vars(r)
		if vars == nil {
			vars = make(map[string]string)
		}

		if err := handlerFunc(ctx, w, r, vars); err != nil {
			logrus.Errorf("Handler for %s %s returned error: %s", r.Method, r.URL.Path, utils.GetErrorMessage(err))
			httputils.WriteError(w, err)
		}
	}
}

// AddRouters initializes a list of routers for the server.
func (s *Server) AddRouters(routers ...router.Router) {
	for _, r := range routers {
		s.addRouter(r)
	}
}

// addRouter adds a new router to the server.
func (s *Server) addRouter(r router.Router) {
	s.routers = append(s.routers, r)
}

// createMux initializes the main router the server uses.
func (s *Server) createMux() *mux.Router {
	m := mux.NewRouter()
	if utils.IsDebugEnabled() {
		profilerSetup(m, "/debug/")
	}

	logrus.Debugf("Registering routers")
	for _, apiRouter := range s.routers {
		for _, r := range apiRouter.Routes() {
			f := s.makeHTTPHandler(r.Handler())

			logrus.Debugf("Registering %s, %s", r.Method(), r.Path())
			m.Path(versionMatcher + r.Path()).Methods(r.Method()).Handler(f)
			m.Path(r.Path()).Methods(r.Method()).Handler(f)
		}
	}

	return m
}

// Wait blocks the server goroutine until it exits.
// It sends an error message if there is any error during
// the API execution.
func (s *Server) Wait(waitChan chan error) {
	if err := s.serveAPI(); err != nil {
		logrus.Errorf("ServeAPI error: %v", err)
		waitChan <- err
		return
	}
	waitChan <- nil
}

func (s *Server) initRouterSwapper() {
	s.routerSwapper = &routerSwapper{
		router: s.createMux(),
	}
}

// Reload reads configuration changes and modifies the
// server according to those changes.
// Currently, only the --debug configuration is taken into account.
func (s *Server) Reload(debug bool) {
	debugEnabled := utils.IsDebugEnabled()
	switch {
	case debugEnabled && !debug: // disable debug
		utils.DisableDebug()
		s.routerSwapper.Swap(s.createMux())
	case debug && !debugEnabled: // enable debug
		utils.EnableDebug()
		s.routerSwapper.Swap(s.createMux())
	}
}
