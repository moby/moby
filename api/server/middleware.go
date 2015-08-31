package server

import (
	"net/http"

	"github.com/docker/docker/api/middleware"
	"github.com/docker/docker/pkg/version"
)

// HTTPAPIFunc is an adapter to allow the use of ordinary functions as Docker API endpoints.
// Any function that has the appropriate signature can be register as a API endpoint (e.g. getVersion).
type HTTPAPIFunc func(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error

// formParser is an interface for struct that store forms.
type formParser interface {
	ParseForm() error
	FormValue(string) string
}

// handleContainer is a middleware to handle containers.
// It creates a middleware.ContainerRequest to send it to the right handler.
func (s *Server) handleContainer(h middleware.ContainerHandler) HTTPAPIFunc {
	return func(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		cReq := middleware.NewContainerRequest(r, version, vars)

		name, err := cReq.GetVar("name")
		if err != nil {
			return err
		}

		container, err := s.daemon.Get(name)
		if err != nil {
			return err
		}
		cReq.Container = container

		return h(w, cReq)
	}
}
