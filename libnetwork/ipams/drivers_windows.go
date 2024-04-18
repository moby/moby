//go:build windows

package ipams

import (
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/defaultipam"
	"github.com/docker/docker/libnetwork/ipams/windowsipam"
	"github.com/docker/docker/libnetwork/ipamutils"
)

// Register registers the built-in ipam services with libnetwork.
func Register(r ipamapi.Registerer, addressPools []*ipamutils.NetworkToSplit) error {
	if err := defaultipam.Register(r, addressPools); err != nil {
		return err
	}

	return windowsipam.Register(windowsipam.DefaultIPAM, r)
}
