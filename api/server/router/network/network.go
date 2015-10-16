package network

import (
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/errors"
	"golang.org/x/net/context"
)

// networkRouter is a router to talk with the network controller
type networkRouter struct {
	daemon *daemon.Daemon
	routes []router.Route
}

// NewRouter initializes a new network router
func NewRouter(d *daemon.Daemon) router.Router {
	r := &networkRouter{
		daemon: d,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routes to the network controller
func (r *networkRouter) Routes() []router.Route {
	return r.routes
}

func (r *networkRouter) initRoutes() {
	r.routes = []router.Route{
		// GET
		local.NewGetRoute("/networks", r.controllerEnabledMiddleware(r.getNetworksList)),
		local.NewGetRoute("/networks/{id:.*}", r.controllerEnabledMiddleware(r.getNetwork)),
		// POST
		local.NewPostRoute("/networks/create", r.controllerEnabledMiddleware(r.postNetworkCreate)),
		local.NewPostRoute("/networks/{id:.*}/connect", r.controllerEnabledMiddleware(r.postNetworkConnect)),
		local.NewPostRoute("/networks/{id:.*}/disconnect", r.controllerEnabledMiddleware(r.postNetworkDisconnect)),
		// DELETE
		local.NewDeleteRoute("/networks/{id:.*}", r.controllerEnabledMiddleware(r.deleteNetwork)),
	}
}

func (r *networkRouter) controllerEnabledMiddleware(handler httputils.APIFunc) httputils.APIFunc {
	if r.daemon.NetworkControllerEnabled() {
		return handler
	}
	return networkControllerDisabled
}

func networkControllerDisabled(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return errors.ErrorNetworkControllerNotEnabled.WithArgs()
}
