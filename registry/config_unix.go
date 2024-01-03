//go:build !windows

package registry // import "github.com/docker/docker/registry"

// defaultCertsDir is the platform-specific default directory where certificates
// are stored. On Linux, it may be overridden through certsDir, for example, when
// running in rootless mode.
const defaultCertsDir = "/etc/docker/certs.d"

// cleanPath is used to ensure that a directory name is valid on the target
// platform. It will be passed in something *similar* to a URL such as
// https:/index.docker.io/v1. Not all platforms support directory names
// which contain those characters (such as : on Windows)
func cleanPath(s string) string {
	return s
}
