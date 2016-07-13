// +build experimental

package distribution

import "errors"

// ErrUnsupportedRegistry indicates that the registry does not support v2 protocol
var ErrUnsupportedRegistry = errors.New("only V2 repositories are supported for plugin distribution")

// ErrUnsupportedMediaType indicates we are pulling content that's not a plugin
var ErrUnsupportedMediaType = errors.New("content is not a plugin")

// Plugin related media types
const (
	MediaTypeManifest = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeConfig   = "application/vnd.docker.plugin.v0+json"
	MediaTypeLayer    = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	DefaultTag        = "latest"
)
