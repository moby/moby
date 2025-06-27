package build

import (
	"runtime"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/daemon/server/router"
)

// buildRouter is a router to talk with the build controller
type buildRouter struct {
	backend Backend
	daemon  experimentalProvider
	routes  []router.Route
}

// NewRouter initializes a new build router
func NewRouter(b Backend, d experimentalProvider) router.Router {
	r := &buildRouter{
		backend: b,
		daemon:  d,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the build controller
func (br *buildRouter) Routes() []router.Route {
	return br.routes
}

func (br *buildRouter) initRoutes() {
	br.routes = []router.Route{
		router.NewPostRoute("/build", br.postBuild),
		router.NewPostRoute("/build/prune", br.postPrune),
		router.NewPostRoute("/build/cancel", br.postCancel),
	}
}

// BuilderVersion derives the default docker builder version from the config.
//
// The default on Linux is version "2" (BuildKit), but the daemon can be
// configured to recommend version "1" (classic Builder). Windows does not
// yet support BuildKit for native Windows images, and uses "1" (classic builder)
// as a default.
//
// This value is only a recommendation as advertised by the daemon, and it is
// up to the client to choose which builder to use.
func BuilderVersion(features map[string]bool) build.BuilderVersion {
	// TODO(thaJeztah) move the default to daemon/config
	bv := build.BuilderBuildKit
	if runtime.GOOS == "windows" {
		// BuildKit is not yet the default on Windows.
		bv = build.BuilderV1
	}

	// Allow the features field in the daemon config to override the
	// default builder to advertise.
	if enable, ok := features["buildkit"]; ok {
		if enable {
			bv = build.BuilderBuildKit
		} else {
			bv = build.BuilderV1
		}
	}
	return bv
}
