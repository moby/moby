//go:build windows

package builtin

import (
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/windowsipam"
)

// Register registers the built-in ipam services with libnetwork.
func Register(r ipamapi.Registerer) error {
	if err := registerBuiltin(r); err != nil {
		return err
	}

	return windowsipam.Register(windowsipam.DefaultIPAM, r)
}
