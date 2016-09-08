// +build experimental

package system

import (
	"github.com/docker/docker/api/server/router"
)

func newExperimentalRoutes(r *systemRouter) []router.Route {
	return []router.Route{
		router.NewGetRoute("/tunnel/ws", r.wsTunnel),
	}
}
