// +build !windows

package registry

import (
	"os"

	"github.com/spf13/pflag"
)

func initCertsDir() string {
	dir := os.Getenv("DOCKER_REGISTRY_CERTS_DIR")
	if len(dir) == 0 {
		dir = "/etc/docker/certs.d"
	}
	return dir
}

var (
	// CertsDir is the directory where certificates are stored
	CertsDir = initCertsDir()
)

// cleanPath is used to ensure that a directory name is valid on the target
// platform. It will be passed in something *similar* to a URL such as
// https:/index.docker.io/v1. Not all platforms support directory names
// which contain those characters (such as : on Windows)
func cleanPath(s string) string {
	return s
}

// installCliPlatformFlags handles any platform specific flags for the service.
func (options *ServiceOptions) installCliPlatformFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&options.V2Only, "disable-legacy-registry", false, "Disable contacting legacy registries")
}
