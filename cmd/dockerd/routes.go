// +build !experimental

package main

import "github.com/docker/docker/api/server/router"

func addExperimentalRouters(routers []router.Router) []router.Router {
	return routers
}
