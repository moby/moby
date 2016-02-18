// +build !windows

package registry

import (
	"net/url"
)

var (
	// DefaultV1Registry is the URI of the default v1 registry
	DefaultV1Registry = &url.URL{
		Scheme: "https",
		Host:   "index.docker.io",
	}

	// DefaultV2Registry is the URI of the default v2 registry
	DefaultV2Registry = &url.URL{
		Scheme: "https",
		Host:   "registry-1.docker.io",
	}
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
