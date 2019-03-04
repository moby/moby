package stacks

// The stacks package defines the router used for docker server-side stacks
// functionality. Notably, the stacks package just re-exports the router from
// the github.com/docker/stacks/pkg/controller/router package. This is done to
// preserve the pattern that all routers are defined in the api/server/router
// tree.

import (
	"github.com/docker/docker/api/server/router"
	stacksrouter "github.com/docker/stacks/pkg/controller/router"
)

func NewRouter(b stacksrouter.Backend) router.Router {
	return stacksrouter.NewRouter(b)
}
