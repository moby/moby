package system // import "github.com/docker/docker/pkg/system"

import "errors"

// ErrNotSupportedPlatform means the platform is not supported.
var ErrNotSupportedPlatform = errors.New("platform and architecture is not supported")
