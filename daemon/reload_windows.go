package daemon // import "github.com/docker/docker/daemon"

import "github.com/docker/docker/daemon/config"

// reloadPlatform updates configuration with platform specific options
// and updates the passed attributes
func (daemon *Daemon) reloadPlatform(conf *config.Config) (func(attributes map[string]string), error) {
	return func(map[string]string) {}, nil
}
