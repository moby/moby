package main

import (
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
	checkpointrouter "github.com/docker/docker/api/server/router/checkpoint"
	"github.com/docker/docker/daemon"
)

func addExperimentalRouters(routers []router.Router, d *daemon.Daemon, decoder httputils.ContainerDecoder) []router.Router {
	if !d.HasExperimental() {
		return []router.Router{}
	}
	return append(routers, checkpointrouter.NewRouter(d, decoder))
}
