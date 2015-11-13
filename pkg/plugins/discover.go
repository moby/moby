package plugins

import "errors"

// TODO Windows: This is only present to avoid link errors in docker from
// libnetwork. With some refactoring in libnetwork and revendor into docker,
// this can probably be moved to be Unix specific.
var (
	// ErrNotFound plugin not found
	ErrNotFound = errors.New("Plugin not found")
)
