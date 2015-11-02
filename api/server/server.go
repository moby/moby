package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/gorilla/mux"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/docker/api"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/context"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/sockets"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/version"
)

// Config provides the configuration for the API server
type Config struct {
	Logging     bool
	EnableCors  bool
	CorsHeaders string
	Version     string
	SocketGroup string
	TLSConfig   *tls.Config
}

// Server contains instance details for the server
type Server struct {
	daemon  *daemon.Daemon
	cfg     *Config
	router  *mux.Router
	start   chan struct{}
	servers []serverCloser
}

// New returns a new instance of the server based on the specified configuration.
func New(cfg *Config) *Server {
	srv := &Server{
		cfg:   cfg,
		start: make(chan struct{}),
	}
	r := createRouter(srv)
	srv.router = r
	return srv
}

// Close closes servers and thus stop receiving requests
func (s *Server) Close() {
	for _, srv := range s.servers {
		if err := srv.Close(); err != nil {
			logrus.Error(err)
		}
	}
}

type serverCloser interface {
	Serve() error
	Close() error
}

// ServeAPI loops through all of the protocols sent in to docker and spawns
// off a go routine to setup a serving http.Server for each.
func (s *Server) ServeAPI(protoAddrs []string) error {
	var chErrors = make(chan error, len(protoAddrs))

	for _, protoAddr := range protoAddrs {
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		if len(protoAddrParts) != 2 {
			return fmt.Errorf("bad format, expected PROTO://ADDR")
		}
		srv, err := s.newServer(protoAddrParts[0], protoAddrParts[1])
		if err != nil {
			return err
		}
		s.servers = append(s.servers, srv...)

		for _, s := range srv {
			logrus.Infof("Listening for HTTP on %s (%s)", protoAddrParts[0], protoAddrParts[1])
			go func(s serverCloser) {
				if err := s.Serve(); err != nil && strings.Contains(err.Error(), "use of closed network connection") {
					err = nil
				}
				chErrors <- err
			}(s)
		}
	}

	for i := 0; i < len(protoAddrs); i++ {
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

// HTTPAPIFunc is an adapter to allow the use of ordinary functions as Docker API endpoints.
// Any function that has the appropriate signature can be register as a API endpoint (e.g. getVersion).
type HTTPAPIFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error

func hijackServer(w http.ResponseWriter) (io.ReadCloser, io.Writer, error) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	// Flush the options to make sure the client sets the raw mode
	conn.Write([]byte{})
	return conn, conn, nil
}

func closeStreams(streams ...interface{}) {
	for _, stream := range streams {
		if tcpc, ok := stream.(interface {
			CloseWrite() error
		}); ok {
			tcpc.CloseWrite()
		} else if closer, ok := stream.(io.Closer); ok {
			closer.Close()
		}
	}
}

// checkForJSON makes sure that the request's Content-Type is application/json.
func checkForJSON(r *http.Request) error {
	ct := r.Header.Get("Content-Type")

	// No Content-Type header is ok as long as there's no Body
	if ct == "" {
		if r.Body == nil || r.ContentLength == 0 {
			return nil
		}
	}

	// Otherwise it better be json
	if api.MatchesContentType(ct, "application/json") {
		return nil
	}
	return fmt.Errorf("Content-Type specified (%s) must be 'application/json'", ct)
}

//If we don't do this, POST method without Content-type (even with empty body) will fail
func parseForm(r *http.Request) error {
	if r == nil {
		return nil
	}
	if err := r.ParseForm(); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

func parseMultipartForm(r *http.Request) error {
	if err := r.ParseMultipartForm(4096); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

func httpError(w http.ResponseWriter, err error) {
	if err == nil || w == nil {
		logrus.WithFields(logrus.Fields{"error": err, "writer": w}).Error("unexpected HTTP error handling")
		return
	}

	statusCode := http.StatusInternalServerError
	errMsg := err.Error()

	// Based on the type of error we get we need to process things
	// slightly differently to extract the error message.
	// In the 'errcode.*' cases there are two different type of
	// error that could be returned. errocode.ErrorCode is the base
	// type of error object - it is just an 'int' that can then be
	// used as the look-up key to find the message. errorcode.Error
	// extends errorcode.Error by adding error-instance specific
	// data, like 'details' or variable strings to be inserted into
	// the message.
	//
	// Ideally, we should just be able to call err.Error() for all
	// cases but the errcode package doesn't support that yet.
	//
	// Additionally, in both errcode cases, there might be an http
	// status code associated with it, and if so use it.
	switch err.(type) {
	case errcode.ErrorCode:
		daError, _ := err.(errcode.ErrorCode)
		statusCode = daError.Descriptor().HTTPStatusCode
		errMsg = daError.Message()

	case errcode.Error:
		// For reference, if you're looking for a particular error
		// then you can do something like :
		//   import ( derr "github.com/docker/docker/api/errors" )
		//   if daError.ErrorCode() == derr.ErrorCodeNoSuchContainer { ... }

		daError, _ := err.(errcode.Error)
		statusCode = daError.ErrorCode().Descriptor().HTTPStatusCode
		errMsg = daError.Message

	default:
		// This part of will be removed once we've
		// converted everything over to use the errcode package

		// FIXME: this is brittle and should not be necessary.
		// If we need to differentiate between different possible error types,
		// we should create appropriate error types with clearly defined meaning
		errStr := strings.ToLower(err.Error())
		for keyword, status := range map[string]int{
			"not found":             http.StatusNotFound,
			"no such":               http.StatusNotFound,
			"bad parameter":         http.StatusBadRequest,
			"conflict":              http.StatusConflict,
			"impossible":            http.StatusNotAcceptable,
			"wrong login/password":  http.StatusUnauthorized,
			"hasn't been activated": http.StatusForbidden,
		} {
			if strings.Contains(errStr, keyword) {
				statusCode = status
				break
			}
		}
	}

	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	logrus.WithFields(logrus.Fields{"statusCode": statusCode, "err": err}).Error("HTTP Error")
	http.Error(w, errMsg, statusCode)
}

// writeJSON writes the value v to the http response stream as json with standard
// json encoding.
func writeJSON(w http.ResponseWriter, code int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(v)
}

func (s *Server) optionsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.WriteHeader(http.StatusOK)
	return nil
}
func writeCorsHeaders(w http.ResponseWriter, r *http.Request, corsHeaders string) {
	logrus.Debugf("CORS header is enabled and set to: %s", corsHeaders)
	w.Header().Add("Access-Control-Allow-Origin", corsHeaders)
	w.Header().Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth")
	w.Header().Add("Access-Control-Allow-Methods", "HEAD, GET, POST, DELETE, PUT, OPTIONS")
}

func (s *Server) ping(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	_, err := w.Write([]byte{'O', 'K'})
	return err
}

func (s *Server) initTCPSocket(addr string) (l net.Listener, err error) {
	if s.cfg.TLSConfig == nil || s.cfg.TLSConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		logrus.Warn("/!\\ DON'T BIND ON ANY IP ADDRESS WITHOUT setting -tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
	}
	if l, err = sockets.NewTCPSocket(addr, s.cfg.TLSConfig, s.start); err != nil {
		return nil, err
	}
	if err := allocateDaemonPort(addr); err != nil {
		return nil, err
	}
	return
}

func makeHTTPHandler(logging bool, localMethod string, localRoute string, handlerFunc HTTPAPIFunc, corsHeaders string, dockerVersion version.Version) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Define the context that we'll pass around to share info
		// like the docker-request-id.
		//
		// The 'context' will be used for global data that should
		// apply to all requests. Data that is specific to the
		// immediate function being called should still be passed
		// as 'args' on the function call.

		reqID := stringid.TruncateID(stringid.GenerateNonCryptoID())
		apiVersion := version.Version(mux.Vars(r)["version"])
		if apiVersion == "" {
			apiVersion = api.Version
		}

		ctx := context.Background()
		ctx = context.WithValue(ctx, context.RequestID, reqID)
		ctx = context.WithValue(ctx, context.APIVersion, apiVersion)

		// log the request
		logrus.Debugf("Calling %s %s", localMethod, localRoute)

		if logging {
			logrus.Infof("%s %s", r.Method, r.RequestURI)
		}

		if strings.Contains(r.Header.Get("User-Agent"), "Docker-Client/") {
			userAgent := strings.Split(r.Header.Get("User-Agent"), "/")

			// v1.20 onwards includes the GOOS of the client after the version
			// such as Docker/1.7.0 (linux)
			if len(userAgent) == 2 && strings.Contains(userAgent[1], " ") {
				userAgent[1] = strings.Split(userAgent[1], " ")[0]
			}

			if len(userAgent) == 2 && !dockerVersion.Equal(version.Version(userAgent[1])) {
				logrus.Debugf("Warning: client and server don't have the same version (client: %s, server: %s)", userAgent[1], dockerVersion)
			}
		}
		if corsHeaders != "" {
			writeCorsHeaders(w, r, corsHeaders)
		}

		if apiVersion.GreaterThan(api.Version) {
			http.Error(w, fmt.Errorf("client is newer than server (client API version: %s, server API version: %s)", apiVersion, api.Version).Error(), http.StatusBadRequest)
			return
		}
		if apiVersion.LessThan(api.MinVersion) {
			http.Error(w, fmt.Errorf("client is too old, minimum supported API version is %s, please upgrade your client to a newer version", api.MinVersion).Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Server", "Docker/"+dockerversion.VERSION+" ("+runtime.GOOS+")")

		if err := handlerFunc(ctx, w, r, mux.Vars(r)); err != nil {
			logrus.Errorf("Handler for %s %s returned error: %s", localMethod, localRoute, err)
			httpError(w, err)
		}
	}
}

// we keep enableCors just for legacy usage, need to be removed in the future
func createRouter(s *Server) *mux.Router {
	r := mux.NewRouter()
	if os.Getenv("DEBUG") != "" {
		profilerSetup(r, "/debug/")
	}
	m := map[string]map[string]HTTPAPIFunc{
		"HEAD": {
			"/containers/{name:.*}/archive": s.headContainersArchive,
		},
		"GET": {
			"/_ping":                          s.ping,
			"/events":                         s.getEvents,
			"/info":                           s.getInfo,
			"/version":                        s.getVersion,
			"/images/json":                    s.getImagesJSON,
			"/images/search":                  s.getImagesSearch,
			"/images/get":                     s.getImagesGet,
			"/images/{name:.*}/get":           s.getImagesGet,
			"/images/{name:.*}/history":       s.getImagesHistory,
			"/images/{name:.*}/json":          s.getImagesByName,
			"/containers/json":                s.getContainersJSON,
			"/containers/{name:.*}/export":    s.getContainersExport,
			"/containers/{name:.*}/changes":   s.getContainersChanges,
			"/containers/{name:.*}/json":      s.getContainersByName,
			"/containers/{name:.*}/top":       s.getContainersTop,
			"/containers/{name:.*}/logs":      s.getContainersLogs,
			"/containers/{name:.*}/stats":     s.getContainersStats,
			"/containers/{name:.*}/attach/ws": s.wsContainersAttach,
			"/exec/{id:.*}/json":              s.getExecByID,
			"/containers/{name:.*}/archive":   s.getContainersArchive,
			"/volumes":                        s.getVolumesList,
			"/volumes/{name:.*}":              s.getVolumeByName,
		},
		"POST": {
			"/auth":                         s.postAuth,
			"/commit":                       s.postCommit,
			"/build":                        s.postBuild,
			"/images/create":                s.postImagesCreate,
			"/images/load":                  s.postImagesLoad,
			"/images/{name:.*}/push":        s.postImagesPush,
			"/images/{name:.*}/tag":         s.postImagesTag,
			"/containers/modresources":      s.postModifyResources,
			"/containers/create":            s.postContainersCreate,
			"/containers/{name:.*}/kill":    s.postContainersKill,
			"/containers/{name:.*}/pause":   s.postContainersPause,
			"/containers/{name:.*}/unpause": s.postContainersUnpause,
			"/containers/{name:.*}/restart": s.postContainersRestart,
			"/containers/{name:.*}/start":   s.postContainersStart,
			"/containers/{name:.*}/stop":    s.postContainersStop,
			"/containers/{name:.*}/wait":    s.postContainersWait,
			"/containers/{name:.*}/resize":  s.postContainersResize,
			"/containers/{name:.*}/attach":  s.postContainersAttach,
			"/containers/{name:.*}/copy":    s.postContainersCopy,
			"/containers/{name:.*}/exec":    s.postContainerExecCreate,
			"/exec/{name:.*}/start":         s.postContainerExecStart,
			"/exec/{name:.*}/resize":        s.postContainerExecResize,
			"/containers/{name:.*}/rename":  s.postContainerRename,
			"/volumes":                      s.postVolumesCreate,
		},
		"PUT": {
			"/containers/{name:.*}/archive": s.putContainersArchive,
		},
		"DELETE": {
			"/containers/{name:.*}": s.deleteContainers,
			"/images/{name:.*}":     s.deleteImages,
			"/volumes/{name:.*}":    s.deleteVolumes,
		},
		"OPTIONS": {
			"": s.optionsHandler,
		},
	}

	// If "api-cors-header" is not given, but "api-enable-cors" is true, we set cors to "*"
	// otherwise, all head values will be passed to HTTP handler
	corsHeaders := s.cfg.CorsHeaders
	if corsHeaders == "" && s.cfg.EnableCors {
		corsHeaders = "*"
	}

	for method, routes := range m {
		for route, fct := range routes {
			logrus.Debugf("Registering %s, %s", method, route)
			// NOTE: scope issue, make sure the variables are local and won't be changed
			localRoute := route
			localFct := fct
			localMethod := method

			// build the handler function
			f := makeHTTPHandler(s.cfg.Logging, localMethod, localRoute, localFct, corsHeaders, version.Version(s.cfg.Version))

			// add the new route
			if localRoute == "" {
				r.Methods(localMethod).HandlerFunc(f)
			} else {
				r.Path("/v{version:[0-9.]+}" + localRoute).Methods(localMethod).HandlerFunc(f)
				r.Path(localRoute).Methods(localMethod).HandlerFunc(f)
			}
		}
	}

	return r
}
