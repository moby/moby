//go:build linux

package ipvlan

import (
	"net"
	"sync"

	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/scope"
	"github.com/docker/docker/libnetwork/types"
)

const (
	containerVethPrefix = "eth"
	vethPrefix          = "veth"
	vethLen             = len(vethPrefix) + 7

	NetworkType   = "ipvlan"      // driver type name
	parentOpt     = "parent"      // parent interface -o parent
	driverModeOpt = "ipvlan_mode" // mode -o ipvlan_mode
	driverFlagOpt = "ipvlan_flag" // flag -o ipvlan_flag

	modeL2  = "l2"  // ipvlan L2 mode (default)
	modeL3  = "l3"  // ipvlan L3 mode
	modeL3S = "l3s" // ipvlan L3S mode

	flagBridge  = "bridge"  // ipvlan flag bridge (default)
	flagPrivate = "private" // ipvlan flag private
	flagVepa    = "vepa"    // ipvlan flag vepa
)

type endpointTable map[string]*endpoint

type networkTable map[string]*network

type driver struct {
	networks networkTable
	sync.Once
	sync.Mutex
	store *datastore.Store
}

type endpoint struct {
	id       string
	nid      string
	mac      net.HardwareAddr
	addr     *net.IPNet
	addrv6   *net.IPNet
	srcName  string
	dbIndex  uint64
	dbExists bool
}

type network struct {
	id        string
	endpoints endpointTable
	driver    *driver
	config    *configuration
	sync.Mutex
}

// Register initializes and registers the libnetwork ipvlan driver.
func Register(r driverapi.Registerer, config map[string]interface{}) error {
	d := &driver{
		networks: networkTable{},
	}
	if err := d.initStore(config); err != nil {
		return err
	}
	return r.RegisterDriver(NetworkType, d, driverapi.Capability{
		DataScope:         scope.Local,
		ConnectivityScope: scope.Global,
	})
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}

func (d *driver) Type() string {
	return NetworkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}

func (d *driver) ProgramExternalConnectivity(nid, eid string, options map[string]interface{}) error {
	return nil
}

func (d *driver) RevokeExternalConnectivity(nid, eid string) error {
	return nil
}

func (d *driver) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
}

func (d *driver) DecodeTableEntry(tablename string, key string, value []byte) (string, map[string]string) {
	return "", nil
}
