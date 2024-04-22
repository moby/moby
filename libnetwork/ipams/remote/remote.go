package remote

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/remote/api"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/pkg/errors"
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

// Register registers a remote ipam when its plugin is activated.
func Register(cb ipamapi.Registerer, pg plugingetter.PluginGetter) error {
	newPluginHandler := func(name string, client *plugins.Client) {
		a := newAllocator(name, client)
		if cps, err := a.(*allocator).getCapabilities(); err == nil {
			if err := cb.RegisterIpamDriverWithCapabilities(name, a, cps); err != nil {
				log.G(context.TODO()).Errorf("error registering remote ipam driver %s due to %v", name, err)
			}
		} else {
			log.G(context.TODO()).Infof("remote ipam driver %s does not support capabilities", name)
			log.G(context.TODO()).Debug(err)
			if err := cb.RegisterIpamDriver(name, a); err != nil {
				log.G(context.TODO()).Errorf("error registering remote ipam driver %s due to %v", name, err)
			}
		}
	}

	// Unit test code is unaware of a true PluginStore. So we fall back to v1 plugins.
	handleFunc := plugins.Handle
	if pg != nil {
		handleFunc = pg.Handle
		activePlugins := pg.GetAllManagedPluginsByCap(ipamapi.PluginEndpointType)
		for _, ap := range activePlugins {
			client, err := getPluginClient(ap)
			if err != nil {
				return err
			}
			newPluginHandler(ap.Name(), client)
		}
	}
	handleFunc(ipamapi.PluginEndpointType, newPluginHandler)
	return nil
}

func getPluginClient(p plugingetter.CompatPlugin) (*plugins.Client, error) {
	if v1, ok := p.(plugingetter.PluginWithV1Client); ok {
		return v1.Client(), nil
	}

	pa, ok := p.(plugingetter.PluginAddr)
	if !ok {
		return nil, errors.Errorf("unknown plugin type %T", p)
	}

	if pa.Protocol() != plugins.ProtocolSchemeHTTPV1 {
		return nil, errors.Errorf("unsupported plugin protocol %s", pa.Protocol())
	}

	addr := pa.Addr()
	client, err := plugins.NewClientWithTimeout(addr.Network()+"://"+addr.String(), nil, pa.Timeout())
	if err != nil {
		return nil, errors.Wrap(err, "error creating plugin client")
	}
	return client, nil
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

func (a *allocator) getCapabilities() (*ipamapi.Capability, error) {
	var res api.GetCapabilityResponse
	if err := a.call("GetCapabilities", nil, &res); err != nil {
		return nil, err
	}
	return res.ToCapability(), nil
}

// GetDefaultAddressSpaces returns the local and global default address spaces
func (a *allocator) GetDefaultAddressSpaces() (string, string, error) {
	res := &api.GetAddressSpacesResponse{}
	if err := a.call("GetDefaultAddressSpaces", nil, res); err != nil {
		return "", "", err
	}
	return res.LocalDefaultAddressSpace, res.GlobalDefaultAddressSpace, nil
}

// RequestPool requests an address pool in the specified address space.
//
// This is a bug-for-bug re-implementation of the logic originally found in
// requestPoolHelper prior to v27. See https://github.com/moby/moby/blob/faf84d7f0a1f2e6badff6f720a3e1e559c356fff/libnetwork/network.go#L1518-L1570
func (a *allocator) RequestPool(req ipamapi.PoolRequest) (ipamapi.AllocatedPool, error) {
	var tmpPoolLeases []string
	defer func() {
		// Release all pools we held on to.
		for _, pID := range tmpPoolLeases {
			if err := a.ReleasePool(pID); err != nil {
				log.G(context.TODO()).Warnf("Failed to release overlapping pool")
			}
		}
	}()

	_, globalSpace, err := a.GetDefaultAddressSpaces()
	if err != nil {
		return ipamapi.AllocatedPool{}, err
	}

	remoteReq := &api.RequestPoolRequest{
		AddressSpace: req.AddressSpace,
		Pool:         req.Pool,
		SubPool:      req.SubPool,
		Options:      req.Options,
		V6:           req.V6,
	}

	for {
		alloc, err := a.requestPool(remoteReq)
		if err != nil {
			return alloc, err
		}

		// If the network pool was explicitly chosen, the network belongs to
		// global address space, or it is invalid ("0.0.0.0/0"), then we don't
		// perform check for overlaps.
		//
		// FIXME(thaJeztah): why are we ignoring invalid pools here?
		//
		// The "invalid" conditions was added in [libnetwork#1095][1], which
		// moved code to reduce os-specific dependencies in the ipam package,
		// but also introduced a types.IsIPNetValid() function, which considers
		// "0.0.0.0/0" invalid, and added it to the conditions below.
		//
		// Unfortunately review does not mention this change, so there's no
		// context why. Possibly this was done to prevent errors further down
		// the line (when checking for overlaps), but returning an error here
		// instead would likely have avoided that as well, so we can only guess.
		//
		// [1]: https://github.com/moby/libnetwork/commit/5ca79d6b87873264516323a7b76f0af7d0298492#diff-bdcd879439d041827d334846f9aba01de6e3683ed8fdd01e63917dae6df23846
		if req.Pool != "" || req.AddressSpace == globalSpace || alloc.Pool.String() == "0.0.0.0/0" {
			return alloc, nil
		}

		// Check for overlap and if none found, we have found the right pool.
		if !checkOverlaps(alloc, req.Exclude) {
			return alloc, nil
		}

		// Pool obtained in this iteration is overlapping. Hold onto the pool
		// and don't release it yet, because we don't want IPAM to give us back
		// the same pool over again. But make sure we still do a deferred release
		// when we have either obtained a non-overlapping pool or ran out of
		// pre-defined pools.
		tmpPoolLeases = append(tmpPoolLeases, alloc.PoolID)
	}
}

func (a *allocator) requestPool(req *api.RequestPoolRequest) (ipamapi.AllocatedPool, error) {
	res := &api.RequestPoolResponse{}
	if err := a.call("RequestPool", req, res); err != nil {
		return ipamapi.AllocatedPool{}, err
	}

	retPool, err := netip.ParsePrefix(res.Pool)
	return ipamapi.AllocatedPool{
		PoolID: res.PoolID,
		Pool:   retPool,
		Meta:   res.Data,
	}, err
}

// checkOverlaps returns true if the 'pool' overlaps with some prefix in 'reserved'.
func checkOverlaps(pool ipamapi.AllocatedPool, reserved []netip.Prefix) bool {
	for _, r := range reserved {
		if r.Overlaps(pool.Pool) {
			return true
		}
	}
	return false
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
	} else {
		return nil, nil, ipamapi.ErrNoIPReturned
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

func (a *allocator) IsBuiltIn() bool {
	return false
}
