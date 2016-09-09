// +build experimental

package checkpoint

import (
	"github.com/docker/docker/api/server/router"
)

func (r *checkpointRouter) initRoutes() {
	r.routes = []router.Route{
		router.NewGetRoute("/containers/{name:.*}/checkpoints", r.getContainerCheckpoints),
		router.NewPostRoute("/containers/{name:.*}/checkpoints", r.postContainerCheckpoint),
		router.NewDeleteRoute("/containers/{name}/checkpoints/{checkpoint}", r.deleteContainerCheckpoint),
	}
}
