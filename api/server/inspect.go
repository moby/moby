package server

import (
	"fmt"
	"net/http"

	"github.com/docker/docker/api/server/decorators"
	"github.com/docker/docker/context"
)

// getContainersByName inspects containers configuration and serializes it as json.
func (s *Server) getContainersByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	writer := decorators.NewInspectDecorator(ctx.Version(), w)
	return s.daemon.ContainerInspect(vars["name"], writer.HandleFunc)
}
