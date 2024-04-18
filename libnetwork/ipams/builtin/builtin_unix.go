//go:build linux || freebsd || darwin

package builtin

import (
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
)

// Register registers the built-in ipam service with libnetwork.
func Register(r ipamapi.Registerer, addressPools []*ipamutils.NetworkToSplit) error {
	return registerBuiltin(r, addressPools)
}
