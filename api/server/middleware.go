package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/version"
	"golang.org/x/net/context"
)

// middleware is an adapter to allow the use of ordinary functions as Docker API filters.
// Any function that has the appropriate signature can be register as a middleware.
type middleware func(handler httputils.APIFunc) httputils.APIFunc

// debugRequestMiddleware dumps the request to logger
func debugRequestMiddleware(handler httputils.APIFunc) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		logrus.Debugf("%s %s", r.Method, r.RequestURI)

		if r.Method != "POST" {
			return handler(ctx, w, r, vars)
		}
		if err := httputils.CheckForJSON(r); err != nil {
			return handler(ctx, w, r, vars)
		}
		maxBodySize := 4096 // 4KB
		if r.ContentLength > int64(maxBodySize) {
			return handler(ctx, w, r, vars)
		}

		body := r.Body
		bufReader := bufio.NewReaderSize(body, maxBodySize)
		r.Body = ioutils.NewReadCloserWrapper(bufReader, func() error { return body.Close() })

		b, err := bufReader.Peek(maxBodySize)
		if err != io.EOF {
			// either there was an error reading, or the buffer is full (in which case the request is too large)
			return handler(ctx, w, r, vars)
		}

		var postForm map[string]interface{}
		if err := json.Unmarshal(b, &postForm); err == nil {
			if _, exists := postForm["password"]; exists {
				postForm["password"] = "*****"
			}
			formStr, errMarshal := json.Marshal(postForm)
			if errMarshal == nil {
				logrus.Debugf("form data: %s", string(formStr))
			} else {
				logrus.Debugf("form data: %q", postForm)
			}
		}

		return handler(ctx, w, r, vars)
	}
}

// authorizationMiddleware perform authorization on the request.
func (s *Server) authorizationMiddleware(handler httputils.APIFunc) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		// FIXME: fill when authN gets in
		// User and UserAuthNMethod are taken from AuthN plugins
		// Currently tracked in https://github.com/docker/docker/pull/13994
		user := ""
		userAuthNMethod := ""
		authCtx := authorization.NewCtx(s.authZPlugins, user, userAuthNMethod, r.Method, r.RequestURI)

		if err := authCtx.AuthZRequest(w, r); err != nil {
			logrus.Errorf("AuthZRequest for %s %s returned error: %s", r.Method, r.RequestURI, err)
			return err
		}

		rw := authorization.NewResponseModifier(w)

		if err := handler(ctx, rw, r, vars); err != nil {
			logrus.Errorf("Handler for %s %s returned error: %s", r.Method, r.RequestURI, err)
			return err
		}

		if err := authCtx.AuthZResponse(rw, r); err != nil {
			logrus.Errorf("AuthZResponse for %s %s returned error: %s", r.Method, r.RequestURI, err)
			return err
		}
		return nil
	}
}

// userAgentMiddleware checks the User-Agent header looking for a valid docker client spec.
func (s *Server) userAgentMiddleware(handler httputils.APIFunc) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if strings.Contains(r.Header.Get("User-Agent"), "Docker-Client/") {
			dockerVersion := version.Version(s.cfg.Version)

			userAgent := strings.Split(r.Header.Get("User-Agent"), "/")

			// v1.20 onwards includes the GOOS of the client after the version
			// such as Docker/1.7.0 (linux)
			if len(userAgent) == 2 && strings.Contains(userAgent[1], " ") {
				userAgent[1] = strings.Split(userAgent[1], " ")[0]
			}

			if len(userAgent) == 2 && !dockerVersion.Equal(version.Version(userAgent[1])) {
				logrus.Debugf("Client and server don't have the same version (client: %s, server: %s)", userAgent[1], dockerVersion)
			}
		}
		return handler(ctx, w, r, vars)
	}
}

// corsMiddleware sets the CORS header expectations in the server.
func (s *Server) corsMiddleware(handler httputils.APIFunc) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		// If "api-cors-header" is not given, but "api-enable-cors" is true, we set cors to "*"
		// otherwise, all head values will be passed to HTTP handler
		corsHeaders := s.cfg.CorsHeaders
		if corsHeaders == "" && s.cfg.EnableCors {
			corsHeaders = "*"
		}

		if corsHeaders != "" {
			writeCorsHeaders(w, r, corsHeaders)
		}
		return handler(ctx, w, r, vars)
	}
}

// versionMiddleware checks the api version requirements before passing the request to the server handler.
func versionMiddleware(handler httputils.APIFunc) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		apiVersion := version.Version(vars["version"])
		if apiVersion == "" {
			apiVersion = api.DefaultVersion
		}

		if apiVersion.GreaterThan(api.DefaultVersion) {
			return errors.ErrorCodeNewerClientVersion.WithArgs(apiVersion, api.DefaultVersion)
		}
		if apiVersion.LessThan(api.MinVersion) {
			return errors.ErrorCodeOldClientVersion.WithArgs(apiVersion, api.MinVersion)
		}

		w.Header().Set("Server", "Docker/"+dockerversion.Version+" ("+runtime.GOOS+")")
		ctx = context.WithValue(ctx, httputils.APIVersionKey, apiVersion)
		return handler(ctx, w, r, vars)
	}
}

// handleWithGlobalMiddlwares wraps the handler function for a request with
// the server's global middlewares. The order of the middlewares is backwards,
// meaning that the first in the list will be evaluated last.
//
// Example: handleWithGlobalMiddlewares(s.getContainersName)
//
//	s.loggingMiddleware(
//		s.userAgentMiddleware(
//			s.corsMiddleware(
//				versionMiddleware(s.getContainersName)
//			)
//		)
//	)
// )
func (s *Server) handleWithGlobalMiddlewares(handler httputils.APIFunc) httputils.APIFunc {
	middlewares := []middleware{
		versionMiddleware,
		s.corsMiddleware,
		s.userAgentMiddleware,
	}

	// Only want this on debug level
	if s.cfg.Logging && logrus.GetLevel() == logrus.DebugLevel {
		middlewares = append(middlewares, debugRequestMiddleware)
	}

	if len(s.cfg.AuthorizationPluginNames) > 0 {
		s.authZPlugins = authorization.NewPlugins(s.cfg.AuthorizationPluginNames)
		middlewares = append(middlewares, s.authorizationMiddleware)
	}

	h := handler
	for _, m := range middlewares {
		h = m(h)
	}
	return h
}
