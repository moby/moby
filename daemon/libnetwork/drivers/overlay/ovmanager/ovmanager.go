package ovmanager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/bitmap"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/overlay/overlayutils"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const (
	networkType = "overlay"
	// The lowest VNI value to auto-assign. Windows does not support VXLAN IDs
	// which overlap the range of 802.1Q VLAN IDs [0, 4095].
	vxlanIDStart = 4096
	// The largest VNI value permitted by RFC 7348.
	vxlanIDEnd = (1 << 24) - 1
)

type networkTable map[string]*network

type driver struct {
	mu       sync.Mutex
	networks networkTable
	vxlanIdm *bitmap.Bitmap
}

type subnet struct {
	subnetIP *net.IPNet
	gwIP     *net.IPNet
	vni      uint32
}

type network struct {
	id      string
	driver  *driver
	subnets []*subnet
}

// Register registers a new instance of the overlay driver.
func Register(r driverapi.Registerer) error {
	return r.RegisterDriver(networkType, newDriver(), driverapi.Capability{
		DataScope:         scope.Global,
		ConnectivityScope: scope.Global,
	})
}

func newDriver() *driver {
	return &driver{
		networks: networkTable{},
		vxlanIdm: bitmap.New(vxlanIDEnd + 1), // The full range of valid vxlan IDs: [0, 2^24).
	}
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	if id == "" {
		return nil, errors.New("invalid network id for overlay network")
	}

	if ipV4Data == nil {
		return nil, errors.New("empty ipv4 data passed during overlay network creation")
	}

	n := &network{
		id:      id,
		driver:  d,
		subnets: []*subnet{},
	}

	opts := make(map[string]string)
	vxlanIDList := make([]uint32, 0, len(ipV4Data))
	for key, val := range option {
		if key == netlabel.OverlayVxlanIDList {
			log.G(context.TODO()).Debugf("overlay network option: %s", val)
			var err error
			vxlanIDList, err = overlayutils.AppendVNIList(vxlanIDList, val)
			if err != nil {
				return nil, err
			}
		} else {
			opts[key] = val
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	for i, ipd := range ipV4Data {
		s := &subnet{
			subnetIP: ipd.Pool,
			gwIP:     ipd.Gateway,
		}

		if len(vxlanIDList) > i { // The VNI for this subnet was specified in the network options.
			s.vni = vxlanIDList[i]
			err := d.vxlanIdm.Set(uint64(s.vni)) // Mark VNI as in-use.
			if err != nil {
				// The VNI is already in use by another subnet/network.
				n.releaseVxlanID()
				return nil, fmt.Errorf("could not assign vxlan id %v to pool %s: %v", s.vni, s.subnetIP, err)
			}
		} else {
			// Allocate an available VNI for the subnet, outside the range of 802.1Q VLAN IDs.
			vni, err := d.vxlanIdm.SetAnyInRange(vxlanIDStart, vxlanIDEnd, true)
			if err != nil {
				n.releaseVxlanID()
				return nil, fmt.Errorf("could not obtain vxlan id for pool %s: %v", s.subnetIP, err)
			}
			s.vni = uint32(vni)
		}

		n.subnets = append(n.subnets, s)
	}

	val := strconv.FormatUint(uint64(n.subnets[0].vni), 10)
	for _, s := range n.subnets[1:] {
		val = val + "," + strconv.FormatUint(uint64(s.vni), 10)
	}
	opts[netlabel.OverlayVxlanIDList] = val

	if _, ok := d.networks[id]; ok {
		n.releaseVxlanID()
		return nil, fmt.Errorf("network %s already exists", id)
	}
	d.networks[id] = n

	return opts, nil
}

func (d *driver) NetworkFree(id string) error {
	if id == "" {
		return errors.New("invalid network id passed while freeing overlay network")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.networks[id]

	if !ok {
		return fmt.Errorf("overlay network with id %s not found", id)
	}

	// Release all vxlan IDs in one shot.
	n.releaseVxlanID()

	delete(d.networks, id)

	return nil
}

func (n *network) releaseVxlanID() {
	for _, s := range n.subnets {
		n.driver.vxlanIdm.Unset(uint64(s.vni))
		s.vni = 0
	}
}

func (d *driver) CreateNetwork(ctx context.Context, id string, option map[string]any, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) DeleteNetwork(nid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) CreateEndpoint(_ context.Context, nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]any) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]any, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(_ context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, _, _ map[string]any) error {
	return types.NotImplementedErrorf("not implemented")
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) Type() string {
	return networkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}
