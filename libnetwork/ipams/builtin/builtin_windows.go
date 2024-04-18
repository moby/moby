//go:build windows

package builtin

import (
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/windowsipam"
	"github.com/docker/docker/libnetwork/ipamutils"
)

// Register registers the built-in ipam services with libnetwork.
func Register(r ipamapi.Registerer, addressPools []*ipamutils.NetworkToSplit) error {
	if err := registerBuiltin(r, addressPools); err != nil {
		return err
	}

	return windowsipam.Register(windowsipam.DefaultIPAM, r)
}
