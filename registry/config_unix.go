// +build !windows

package registry

const (
	// DefaultV1Registry is the URI of the default v1 registry
	DefaultV1Registry = "https://index.docker.io"

	// DefaultV2Registry is the URI of the default v2 registry
	DefaultV2Registry = "https://registry-1.docker.io"

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
