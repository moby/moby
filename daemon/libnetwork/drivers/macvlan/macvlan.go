//go:build linux

package macvlan

import (
	"net"
	"sync"

	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const (
	containerVethPrefix = "eth"
	vethPrefix          = "veth"
	vethLen             = len(vethPrefix) + 7
	NetworkType         = "macvlan"      // driver type name
	modePrivate         = "private"      // macvlan mode private
	modeVepa            = "vepa"         // macvlan mode vepa
	modeBridge          = "bridge"       // macvlan mode bridge
	modePassthru        = "passthru"     // macvlan mode passthrough
	parentOpt           = "parent"       // parent interface -o parent
	driverModeOpt       = "macvlan_mode" // macvlan mode ux opt suffix
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

// Register initializes and registers the libnetwork macvlan driver
func Register(r driverapi.Registerer, store *datastore.Store, _ map[string]any) error {
	d := &driver{
		store:    store,
		networks: networkTable{},
	}
	if err := d.initStore(); err != nil {
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

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]any, error) {
	return make(map[string]any), nil
}

func (d *driver) Type() string {
	return NetworkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}
