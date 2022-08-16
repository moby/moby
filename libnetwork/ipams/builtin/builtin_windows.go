//go:build windows
// +build windows

package builtin

import (
	"errors"

	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/ipam"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"

	windowsipam "github.com/docker/docker/libnetwork/ipams/windowsipam"
)

var (
	// defaultAddressPool Stores user configured subnet list
	defaultAddressPool []*ipamutils.NetworkToSplit
)

// InitDockerDefault registers the built-in ipam service with libnetwork
func InitDockerDefault(ic ipamapi.Callback, l, g interface{}) error {
	var (
		ok                bool
		localDs, globalDs datastore.DataStore
	)

	if l != nil {
		if localDs, ok = l.(datastore.DataStore); !ok {
			return errors.New("incorrect local datastore passed to built-in ipam init")
		}
	}

	if g != nil {
		if globalDs, ok = g.(datastore.DataStore); !ok {
			return errors.New("incorrect global datastore passed to built-in ipam init")
		}
	}

	ipamutils.ConfigLocalScopeDefaultNetworks(nil)

	a, err := ipam.NewAllocator(localDs, globalDs)
	if err != nil {
		return err
	}

	cps := &ipamapi.Capability{RequiresRequestReplay: true}

	return ic.RegisterIpamDriverWithCapabilities(ipamapi.DefaultIPAM, a, cps)
}

// Init registers the built-in ipam service with libnetwork
func Init(ic ipamapi.Callback, l, g interface{}) error {
	initFunc := windowsipam.GetInit(windowsipam.DefaultIPAM)

	err := InitDockerDefault(ic, l, g)
	if err != nil {
		return err
	}

	return initFunc(ic, l, g)
}

// SetDefaultIPAddressPool stores default address pool .
func SetDefaultIPAddressPool(addressPool []*ipamutils.NetworkToSplit) {
	defaultAddressPool = addressPool
}

// GetDefaultIPAddressPool returns default address pool .
func GetDefaultIPAddressPool() []*ipamutils.NetworkToSplit {
	return defaultAddressPool
}
