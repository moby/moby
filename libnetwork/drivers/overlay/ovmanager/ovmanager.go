package ovmanager

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/bitmap"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drivers/overlay/overlayutils"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
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

type network struct {
	id   string
	vnis []uint32
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
		return nil, fmt.Errorf("invalid network id for overlay network")
	}

	if ipV4Data == nil {
		return nil, fmt.Errorf("empty ipv4 data passed during overlay network creation")
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
	if _, ok := d.networks[id]; ok {
		return nil, fmt.Errorf("network %s already exists", id)
	}

	n := &network{id: id}
	for i, ipd := range ipV4Data {
		var vni uint32

		if len(vxlanIDList) > i { // The VNI for this subnet was specified in the network options.
			vni = vxlanIDList[i]
			err := d.vxlanIdm.Set(uint64(vni)) // Mark VNI as in-use.
			if err != nil {
				// The VNI is already in use by another subnet/network.
				d.releaseVXLANIDs(n)
				return nil, fmt.Errorf("could not assign vxlan id %v to pool %s: %v", vni, ipd.Pool, err)
			}
		} else {
			// Allocate an available VNI for the subnet, outside the range of 802.1Q VLAN IDs.
			v, err := d.vxlanIdm.SetAnyInRange(vxlanIDStart, vxlanIDEnd, true)
			if err != nil {
				d.releaseVXLANIDs(n)
				return nil, fmt.Errorf("could not obtain vxlan id for pool %s: %v", ipd.Pool, err)
			}
			vni = uint32(v)
		}

		n.vnis = append(n.vnis, vni)
	}

	val := strconv.FormatUint(uint64(n.vnis[0]), 10)
	for _, vni := range n.vnis[1:] {
		val = val + "," + strconv.FormatUint(uint64(vni), 10)
	}
	opts[netlabel.OverlayVxlanIDList] = val

	d.networks[id] = n
	return opts, nil
}

func (d *driver) NetworkFree(id string) error {
	if id == "" {
		return fmt.Errorf("invalid network id passed while freeing overlay network")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.networks[id]

	if !ok {
		return fmt.Errorf("overlay network with id %s not found", id)
	}

	// Release all vxlan IDs in one shot.
	d.releaseVXLANIDs(n)

	delete(d.networks, id)

	return nil
}

func (d *driver) releaseVXLANIDs(n *network) {
	for _, vni := range n.vnis {
		d.vxlanIdm.Unset(uint64(vni))
	}
	n.vnis = nil
}

func (d *driver) CreateNetwork(id string, option map[string]interface{}, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
}

func (d *driver) DecodeTableEntry(tablename string, key string, value []byte) (string, map[string]string) {
	return "", nil
}

func (d *driver) DeleteNetwork(nid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
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

func (d *driver) ProgramExternalConnectivity(nid, eid string, options map[string]interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) RevokeExternalConnectivity(nid, eid string) error {
	return types.NotImplementedErrorf("not implemented")
}
