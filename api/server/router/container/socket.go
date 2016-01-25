package container

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/vendor/src/github.com/docker/engine-api/types"

	"golang.org/x/net/context"
)

func (s *containerRouter) postContainerSocket(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var cfg types.ContainerForwardSocketConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		return err
	}

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return err
	}
	defer httputils.CloseStreams(conn)
	conn.Write([]byte{})

	return s.backend.ContainerForwardSocket(vars["name"], cfg.ContainerPath, conn)
}
