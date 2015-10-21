package remote

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/ipams/remote/api"
	"github.com/docker/libnetwork/types"
)

type allocator struct {
	endpoint *plugins.Client
	name     string
}

// PluginResponse is the interface for the plugin request responses
type PluginResponse interface {
	IsSuccess() bool
	GetError() string
}

func newAllocator(name string, client *plugins.Client) ipamapi.Ipam {
	a := &allocator{name: name, endpoint: client}
	return a
}

// Init registers a remote ipam when its plugin is activated
func Init(cb ipamapi.Callback, l, g interface{}) error {
	plugins.Handle(ipamapi.PluginEndpointType, func(name string, client *plugins.Client) {
		if err := cb.RegisterIpamDriver(name, newAllocator(name, client)); err != nil {
			log.Errorf("error registering remote ipam %s due to %v", name, err)
		}
	})
	return nil
}

func (a *allocator) call(methodName string, arg interface{}, retVal PluginResponse) error {
	method := ipamapi.PluginEndpointType + "." + methodName
	err := a.endpoint.Call(method, arg, retVal)
	if err != nil {
		return err
	}
	if !retVal.IsSuccess() {
		return fmt.Errorf("remote: %s", retVal.GetError())
	}
	return nil
}

// GetDefaultAddressSpaces returns the local and global default address spaces
func (a *allocator) GetDefaultAddressSpaces() (string, string, error) {
	res := &api.GetAddressSpacesResponse{}
	if err := a.call("GetDefaultAddressSpaces", nil, res); err != nil {
		return "", "", err
	}
	return res.LocalDefaultAddressSpace, res.GlobalDefaultAddressSpace, nil
}

// RequestPool requests an address pool in the specified address space
func (a *allocator) RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error) {
	req := &api.RequestPoolRequest{AddressSpace: addressSpace, Pool: pool, SubPool: subPool, Options: options, V6: v6}
	res := &api.RequestPoolResponse{}
	if err := a.call("RequestPool", req, res); err != nil {
		return "", nil, nil, err
	}
	retPool, err := types.ParseCIDR(res.Pool)
	return res.PoolID, retPool, res.Data, err
}

// ReleasePool removes an address pool from the specified address space
func (a *allocator) ReleasePool(poolID string) error {
	req := &api.ReleasePoolRequest{PoolID: poolID}
	res := &api.ReleasePoolResponse{}
	return a.call("ReleasePool", req, res)
}

// RequestAddress requests an address from the address pool
func (a *allocator) RequestAddress(poolID string, address net.IP, options map[string]string) (*net.IPNet, map[string]string, error) {
	var (
		prefAddress string
		retAddress  *net.IPNet
		err         error
	)
	if address != nil {
		prefAddress = address.String()
	}
	req := &api.RequestAddressRequest{PoolID: poolID, Address: prefAddress, Options: options}
	res := &api.RequestAddressResponse{}
	if err := a.call("RequestAddress", req, res); err != nil {
		return nil, nil, err
	}
	if res.Address != "" {
		retAddress, err = types.ParseCIDR(res.Address)
	}
	return retAddress, res.Data, err
}

// ReleaseAddress releases the address from the specified address pool
func (a *allocator) ReleaseAddress(poolID string, address net.IP) error {
	var relAddress string
	if address != nil {
		relAddress = address.String()
	}
	req := &api.ReleaseAddressRequest{PoolID: poolID, Address: relAddress}
	res := &api.ReleaseAddressResponse{}
	return a.call("ReleaseAddress", req, res)
}
