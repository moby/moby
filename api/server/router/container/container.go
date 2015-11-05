package container

import (
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/api/server/router/local"
)

// containerRouter is a router to talk with the container controller
type containerRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new container router
func NewRouter(b Backend) router.Router {
	r := &containerRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the container controller
func (r *containerRouter) Routes() []router.Route {
	return r.routes
}

// initRoutes initializes the routes in container router
func (r *containerRouter) initRoutes() {
	r.routes = []router.Route{
		// HEAD
		local.NewHeadRoute("/containers/{name:.*}/archive", r.headContainersArchive),
		// GET
		local.NewGetRoute("/containers/json", r.getContainersJSON),
		local.NewGetRoute("/containers/{name:.*}/export", r.getContainersExport),
		local.NewGetRoute("/containers/{name:.*}/changes", r.getContainersChanges),
		local.NewGetRoute("/containers/{name:.*}/json", r.getContainersByName),
		local.NewGetRoute("/containers/{name:.*}/top", r.getContainersTop),
		local.NewGetRoute("/containers/{name:.*}/logs", r.getContainersLogs),
		local.NewGetRoute("/containers/{name:.*}/stats", r.getContainersStats),
		local.NewGetRoute("/containers/{name:.*}/attach/ws", r.wsContainersAttach),
		local.NewGetRoute("/exec/{id:.*}/json", r.getExecByID),
		local.NewGetRoute("/containers/{name:.*}/archive", r.getContainersArchive),
		// POST
		local.NewPostRoute("/containers/create", r.postContainersCreate),
		local.NewPostRoute("/containers/{name:.*}/kill", r.postContainersKill),
		local.NewPostRoute("/containers/{name:.*}/pause", r.postContainersPause),
		local.NewPostRoute("/containers/{name:.*}/unpause", r.postContainersUnpause),
		local.NewPostRoute("/containers/{name:.*}/restart", r.postContainersRestart),
		local.NewPostRoute("/containers/{name:.*}/start", r.postContainersStart),
		local.NewPostRoute("/containers/{name:.*}/stop", r.postContainersStop),
		local.NewPostRoute("/containers/{name:.*}/wait", r.postContainersWait),
		local.NewPostRoute("/containers/{name:.*}/resize", r.postContainersResize),
		local.NewPostRoute("/containers/{name:.*}/attach", r.postContainersAttach),
		local.NewPostRoute("/containers/{name:.*}/copy", r.postContainersCopy),
		local.NewPostRoute("/containers/{name:.*}/exec", r.postContainerExecCreate),
		local.NewPostRoute("/exec/{name:.*}/start", r.postContainerExecStart),
		local.NewPostRoute("/exec/{name:.*}/resize", r.postContainerExecResize),
		local.NewPostRoute("/containers/{name:.*}/rename", r.postContainerRename),
		// PUT
		local.NewPutRoute("/containers/{name:.*}/archive", r.putContainersArchive),
		// DELETE
		local.NewDeleteRoute("/containers/{name:.*}", r.deleteContainers),
	}
}
