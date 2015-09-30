// +build experimental

package network

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/daemon"
	"github.com/docker/libnetwork/api"
	"github.com/gorilla/mux"
)

var httpMethods = []string{"GET", "POST", "PUT", "DELETE"}

// NewRouter initializes a new network router
func NewRouter(d *daemon.Daemon) router.Router {
	c := d.NetworkController()
	if c == nil {
		return networkRouter{}
	}

	var routes []router.Route
	netHandler := api.NewHTTPHandler(c)

	// TODO: libnetwork should stop hijacking request/response.
	// It should define API functions to add normally to the router.
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		netHandler(w, r)
		return nil
	}

	for _, path := range []string{"/networks", "/services", "/sandboxes"} {
		routes = append(routes, networkRoute{path, handler})
	}

	return networkRouter{routes}
}

// Register adds the filtered handler to the mux.
func (n networkRoute) Register(m *mux.Router, handler http.Handler) {
	logrus.Debugf("Registering %s, %v", n.path, httpMethods)
	subrouter := m.PathPrefix(router.VersionMatcher + n.path).Subrouter()
	subrouter.Methods(httpMethods...).Handler(handler)

	subrouter = m.PathPrefix(n.path).Subrouter()
	subrouter.Methods(httpMethods...).Handler(handler)
}
