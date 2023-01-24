//go:build linux || freebsd || darwin
// +build linux freebsd darwin

package builtin

import (
	"errors"

	"github.com/docker/docker/libnetwork/ipamapi"
)

// Init registers the built-in ipam service with libnetwork
//
// Deprecated: use [Register].
func Init(ic ipamapi.Callback, l, g interface{}) error {
	if l != nil {
		return errors.New("non-nil local datastore passed to built-in ipam init")
	}

	if g != nil {
		return errors.New("non-nil global datastore passed to built-in ipam init")
	}

	return Register(ic)
}

// Register registers the built-in ipam service with libnetwork.
func Register(r ipamapi.Registerer) error {
	return registerBuiltin(r)
}
