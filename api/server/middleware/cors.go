package middleware

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	"golang.org/x/net/context"
)

// NewCORSMiddleware creates a new CORS middleware.
func NewCORSMiddleware(defaultHeaders string) Middleware {
	return func(handler httputils.APIFunc) httputils.APIFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
			// If "api-cors-header" is not given, but "api-enable-cors" is true, we set cors to "*"
			// otherwise, all head values will be passed to HTTP handler
			corsHeaders := defaultHeaders
			if corsHeaders == "" {
				corsHeaders = "*"
			}

			writeCorsHeaders(w, r, corsHeaders)
			return handler(ctx, w, r, vars)
		}
	}
}

func writeCorsHeaders(w http.ResponseWriter, r *http.Request, corsHeaders string) {
	logrus.Debugf("CORS header is enabled and set to: %s", corsHeaders)
	w.Header().Add("Access-Control-Allow-Origin", corsHeaders)
	w.Header().Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth")
	w.Header().Add("Access-Control-Allow-Methods", "HEAD, GET, POST, DELETE, PUT, OPTIONS")
}
