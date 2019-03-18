package stacks // import "github.com/docker/docker/api/server/router/stacks"

import (
	"context"
	"net/http"

	"github.com/docker/docker/api/server/router"
)

type stacksRouter struct {
	routes []router.Route
}

func NewRouter() router.Router {
	r := &stacksRouter{}
	r.initRoutes()
	return r
}

func (sr *stacksRouter) initRoutes() {
	sr.routes = []router.Route{
		router.NewPostRoute("/stacks/create", sr.createStack),
		router.NewGetRoute("/stacks", sr.listStacks),
		router.NewGetRoute("/stacks/{id}", sr.inspectStack),
		router.NewPostRoute("/stacks/{id}/update", sr.updateStack),
		router.NewDeleteRoute("/stacks/{id}", sr.deleteStack),
	}
}

// TODO(dperny): these methods are stubs, present to illustrate the design. In
// the final implementation, they would go in a file called "stacks_routes.go"

func (sr *stacksRouter) createStack(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return nil
}

func (sr *stacksRouter) listStacks(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return nil
}

func (sr *stacksRouter) inspectStack(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return nil
}

func (sr *stacksRouter) updateStack(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return nil
}

func (sr *stacksRouter) deleteStack(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return nil
}
