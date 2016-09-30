// +build !experimental

package checkpoint

func (r *checkpointRouter) initRoutes() {}

// Backend is empty so that the package can compile in non-experimental
type Backend interface{}
