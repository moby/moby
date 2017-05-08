// +build !experimental

package plugin

func (r *pluginRouter) initRoutes() {}

// Backend is empty so that the package can compile in non-experimental
// (Needed by volume driver)
type Backend interface{}
