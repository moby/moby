// +build experimental,!windows

package server

import (
	"github.com/docker/docker/context"
)

func (s *Server) registerSubRouter(ctx context.Context) {
	httpHandler := s.daemon.NetworkAPIRouter(ctx)

	subrouter := s.router.PathPrefix("/v{version:[0-9.]+}/networks").Subrouter()
	subrouter.Methods("GET", "POST", "PUT", "DELETE").HandlerFunc(httpHandler)
	subrouter = s.router.PathPrefix("/networks").Subrouter()
	subrouter.Methods("GET", "POST", "PUT", "DELETE").HandlerFunc(httpHandler)

	subrouter = s.router.PathPrefix("/v{version:[0-9.]+}/services").Subrouter()
	subrouter.Methods("GET", "POST", "PUT", "DELETE").HandlerFunc(httpHandler)
	subrouter = s.router.PathPrefix("/services").Subrouter()
	subrouter.Methods("GET", "POST", "PUT", "DELETE").HandlerFunc(httpHandler)

	subrouter = s.router.PathPrefix("/v{version:[0-9.]+}/sandboxes").Subrouter()
	subrouter.Methods("GET", "POST", "PUT", "DELETE").HandlerFunc(httpHandler)
	subrouter = s.router.PathPrefix("/sandboxes").Subrouter()
	subrouter.Methods("GET", "POST", "PUT", "DELETE").HandlerFunc(httpHandler)
}
