package router

import (
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/gorilla/mux"
)

// VersionMatcher defines a variable matcher to be parsed by the router
// when a request is about to be served.
const VersionMatcher = "/v{version:[0-9.]+}"

// Router defines an interface to specify a group of routes to add the the docker server.
type Router interface {
	Routes() []Route
}

// Route defines an individual API route in the docker server.
type Route interface {
	// Register adds the handler route to the docker mux.
	Register(*mux.Router, http.Handler)
	// Handler returns the raw function to create the http handler.
	Handler() httputils.APIFunc
}
