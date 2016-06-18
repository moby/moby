// +build experimental

package distribution

import "errors"

// ErrUnSupportedRegistry indicates that the registry does not support v2 protocol
var ErrUnSupportedRegistry = errors.New("Only V2 repositories are supported for plugin distribution")

// Plugin related media types
const (
	MediaTypeManifest = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeConfig   = "application/vnd.docker.plugin.v0+json"
	MediaTypeLayer    = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	DefaultTag        = "latest"
)
