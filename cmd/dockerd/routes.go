// +build !experimental

package main

import (
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/daemon"
)

func addExperimentalRouters(routers []router.Router, d *daemon.Daemon, decoder httputils.ContainerDecoder) []router.Router {
	return routers
}
