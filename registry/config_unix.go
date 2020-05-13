// +build !windows

package registry // import "github.com/docker/docker/registry"

import (
	"path/filepath"

	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/docker/rootless"
)

// CertsDir is the directory where certificates are stored
func CertsDir() string {
	d := "/etc/docker/certs.d"

	if rootless.RunningWithRootlessKit() {
		configHome, err := homedir.GetConfigHome()
		if err == nil {
			d = filepath.Join(configHome, "docker/certs.d")
		}
	}
	return d
}

// cleanPath is used to ensure that a directory name is valid on the target
// platform. It will be passed in something *similar* to a URL such as
// https:/index.docker.io/v1. Not all platforms support directory names
// which contain those characters (such as : on Windows)
func cleanPath(s string) string {
	return s
}
