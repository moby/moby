//go:build windows

package builtin

import (
	"errors"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/windowsipam"
)

// Init registers the built-in ipam services with libnetwork.
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

// Register registers the built-in ipam services with libnetwork.
func Register(r ipamapi.Registerer) error {
	if err := registerBuiltin(r); err != nil {
		return err
	}

	return windowsipam.Register(windowsipam.DefaultIPAM, r)
}
