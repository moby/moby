// +build !windows

package registry

import (
	flag "github.com/docker/docker/pkg/mflag"
)

var (
	// CertsDir is the directory where certificates are stored
	CertsDir = "/etc/docker/certs.d"
)

// cleanPath is used to ensure that a directory name is valid on the target
// platform. It will be passed in something *similar* to a URL such as
// https:/index.docker.io/v1. Not all platforms support directory names
// which contain those characters (such as : on Windows)
func cleanPath(s string) string {
	return s
}

// installCliPlatformFlags handles any platform specific flags for the service.
func (options *ServiceOptions) installCliPlatformFlags(cmd *flag.FlagSet, usageFn func(string) string) {
	cmd.BoolVar(&options.V2Only, []string{"-disable-legacy-registry"}, false, usageFn("Disable contacting legacy registries"))
}
