// +build experimental

package distribution

import "errors"

// ErrUnsupportedRegistry indicates that the registry does not support v2 protocol
var ErrUnsupportedRegistry = errors.New("only V2 repositories are supported for plugin distribution")

// ErrUnsupportedMediaType indicates we are pulling content that's not a plugin
var ErrUnsupportedMediaType = errors.New("content is not a plugin")

// DefaultTag is the default tag for plugins
const DefaultTag = "latest"
