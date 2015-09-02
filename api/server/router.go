package server

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/version"

	restful "github.com/emicklei/go-restful"
)

const versionPath = "/{version:v[0-9.]+}"

var (
	corsAllowedHeaders = []string{"Origin", "X-Requested-With", "Content-Type, Accept", "X-Registry-Auth"}
	corsAllowedMethods = []string{"HEAD", "GET", "POST", "DELETE", "PUT", "OPTIONS"}
	requestIDName      = "docker-request-id"
)

type webLogger struct{}

func (w webLogger) Print(v ...interface{}) {
	logrus.Info(v)
}

func (w webLogger) Printf(format string, v ...interface{}) {
	logrus.Info(v)
}

func versionFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	v := strings.TrimPrefix(req.PathParameter("version"), "v")
	version := version.Version(v)
	if version == "" {
		version = api.Version
	}

	req.SetAttribute("version", version)
	chain.ProcessFilter(req, resp)
}

func (s *Server) userAgentFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if strings.Contains(req.HeaderParameter("User-Agent"), "Docker-Client/") {
		dockerVersion := version.Version(s.cfg.Version)
		userAgent := strings.Split(req.HeaderParameter("User-Agent"), "/")

		// v1.20 onwards includes the GOOS of the client after the version
		// such as Docker/1.7.0 (linux)
		if len(userAgent) == 2 && strings.Contains(userAgent[1], " ") {
			userAgent[1] = strings.Split(userAgent[1], " ")[0]
		}

		if len(userAgent) == 2 && !dockerVersion.Equal(version.Version(userAgent[1])) {
			logrus.Debugf("Warning: client and server don't have the same version (client: %s, server: %s)", userAgent[1], dockerVersion)
		}
	}

	resp.AddHeader("Server", "Docker/"+dockerversion.VERSION+" ("+runtime.GOOS+")")
	chain.ProcessFilter(req, resp)
}

func requestIDFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	reqID := stringid.TruncateID(stringid.GenerateNonCryptoID())
	req.SetAttribute(requestIDName, reqID)
	chain.ProcessFilter(req, resp)
}

func (s *Server) webLoggingFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if s.cfg.Logging {
		logrus.Infof("%s %s", req.Request.Method, req.Request.RequestURI)
	}
	chain.ProcessFilter(req, resp)
}

func (s *Server) configureCORS(ws *restful.WebService) {
	// If "api-cors-header" is not given, but "api-enable-cors" is true, we set cors to "*"
	// otherwise, all head values will be passed to HTTP handler
	corsHeaders := s.cfg.CorsHeaders
	if corsHeaders == "" && s.cfg.EnableCors {
		corsHeaders = "*"
	}

	if corsHeaders != "" {
		logrus.Debugf("CORS header is enabled and set to: %s", corsHeaders)
		cors := restful.CrossOriginResourceSharing{
			AllowedDomains: strings.Split(corsHeaders, " "),
			AllowedHeaders: corsAllowedHeaders,
			AllowedMethods: corsAllowedMethods,
		}

		ws.Filter(cors.Filter)
	}
}

func (s *Server) applyFilters(ws *restful.WebService) {
	ws.Filter(restful.OPTIONSFilter())
	s.configureCORS(ws)
	ws.Filter(requestIDFilter)
	ws.Filter(s.webLoggingFilter)
	ws.Filter(s.userAgentFilter)
	ws.Filter(versionFilter)
}

func (s *Server) newWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.ApiVersion(string(api.Version))
	s.applyFilters(ws)
	return ws
}

func mainRouter(s *Server) *restful.WebService {
	ws := s.newWebService()

	for method, handlers := range mainRoutes(s) {
		for route, fct := range handlers {
			handler := httpHandler(route, method, fct)
			ws.Route(ws.Method(method).Path(versionPath + route).To(handler))
			ws.Route(ws.Method(method).Path(route).To(handler))
		}
	}

	return ws
}

func httpHandler(route, method string, handlerFunc HTTPAPIFunc) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		logrus.Debugf("Calling %s %s", method, route)

		version := req.Attribute("version").(version.Version)
		if err := validateAPIVersion(version); err != nil {
			http.Error(resp, err.Error(), http.StatusBadRequest)
			return
		}

		if err := handlerFunc(version, resp, req); err != nil {
			logrus.Errorf("Handler for %s %s returned error: %s", method, route, err)
			httpError(resp, err)
		}
	}
}

func validateAPIVersion(version version.Version) error {
	if version.GreaterThan(api.Version) {
		return fmt.Errorf("client is newer than server (client API version: %s, server API version: %s)", version, api.Version)
	}
	if version.LessThan(api.MinVersion) {
		return fmt.Errorf("client is too old, minimum supported API version is %s, please upgrade your client to a newer version", api.MinVersion)
	}
	return nil
}

func mainRoutes(s *Server) map[string]map[string]HTTPAPIFunc {
	return map[string]map[string]HTTPAPIFunc{
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
	}
}
