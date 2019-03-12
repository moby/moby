package stacks

import (
	"github.com/docker/docker/api/server/router"
	stacksrouter "github.com/docker/stacks/pkg/controller/router"
)

// NewRouter creates a new instance of the server-side stacks router.
//
// This function simply re-exports the NewRouter function from the
// github.com/docker/stacks/pkg/controller/router package. This is done to
// preserve the pattern that all routers are defined in the api/server/router
// tree.
func NewRouter(b stacksrouter.Backend) router.Router {
	return stacksrouter.NewRouter(b)
}
