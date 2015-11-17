package plugins

import "errors"

// Get returns the plugin given the specified name and requested implementation.
func Get(name, imp string) (*Plugin, error) {
	return nil, errors.New("Plugins Get() not implemented on Windows")
}

// Handle adds the specified function to the extpointHandlers.
// TODO Windows: We implement this to avoid a link error with libnetwork.
// With some refactoring in libnetwork and revendor, we should be able to
// remove this implementation.
func Handle(iface string, fn func(string, *Client)) {
}
