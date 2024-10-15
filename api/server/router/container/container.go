package container // import "github.com/docker/docker/api/server/router/container"

import (
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
)

// containerRouter is a router to talk with the container controller
type containerRouter struct {
	backend Backend
	decoder httputils.ContainerDecoder
	routes  []router.Route
	cgroup2 bool
}

// NewRouter initializes a new container router
func NewRouter(b Backend, decoder httputils.ContainerDecoder, cgroup2 bool) router.Router {
	r := &containerRouter{
		backend: b,
		decoder: decoder,
		cgroup2: cgroup2,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routes to the container controller
func (c *containerRouter) Routes() []router.Route {
	return c.routes
}

// initRoutes initializes the routes in container router
func (c *containerRouter) initRoutes() {
	c.routes = []router.Route{
		// HEAD
		router.NewHeadRoute("/containers/{name:.*}/archive", c.headContainersArchive),
		// GET
		router.NewGetRoute("/containers/json", c.getContainersJSON),
		router.NewGetRoute("/containers/{name:.*}/export", c.getContainersExport),
		router.NewGetRoute("/containers/{name:.*}/changes", c.getContainersChanges),
		router.NewGetRoute("/containers/{name:.*}/json", c.getContainersByName),
		router.NewGetRoute("/containers/{name:.*}/top", c.getContainersTop),
		router.NewGetRoute("/containers/{name:.*}/logs", c.getContainersLogs),
		router.NewGetRoute("/containers/{name:.*}/stats", c.getContainersStats),
		router.NewGetRoute("/containers/{name:.*}/attach/ws", c.wsContainersAttach),
		router.NewGetRoute("/exec/{id:.*}/json", c.getExecByID),
		router.NewGetRoute("/containers/{name:.*}/archive", c.getContainersArchive),
		// POST
		router.NewPostRoute("/containers/create", c.postContainersCreate),
		router.NewPostRoute("/containers/{name:.*}/kill", c.postContainersKill),
		router.NewPostRoute("/containers/{name:.*}/pause", c.postContainersPause),
		router.NewPostRoute("/containers/{name:.*}/unpause", c.postContainersUnpause),
		router.NewPostRoute("/containers/{name:.*}/restart", c.postContainersRestart),
		router.NewPostRoute("/containers/{name:.*}/start", c.postContainersStart),
		router.NewPostRoute("/containers/{name:.*}/stop", c.postContainersStop),
		router.NewPostRoute("/containers/{name:.*}/wait", c.postContainersWait),
		router.NewPostRoute("/containers/{name:.*}/resize", c.postContainersResize),
		router.NewPostRoute("/containers/{name:.*}/attach", c.postContainersAttach),
		router.NewPostRoute("/containers/{name:.*}/exec", c.postContainerExecCreate),
		router.NewPostRoute("/exec/{name:.*}/start", c.postContainerExecStart),
		router.NewPostRoute("/exec/{name:.*}/resize", c.postContainerExecResize),
		router.NewPostRoute("/containers/{name:.*}/rename", c.postContainerRename),
		router.NewPostRoute("/containers/{name:.*}/update", c.postContainerUpdate),
		router.NewPostRoute("/containers/prune", c.postContainersPrune),
		router.NewPostRoute("/commit", c.postCommit),
		// PUT
		router.NewPutRoute("/containers/{name:.*}/archive", c.putContainersArchive),
		// DELETE
		router.NewDeleteRoute("/containers/{name:.*}", c.deleteContainers),
	}
}
