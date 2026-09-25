//go:build linux

package ipvlan

import (
	"net"
	"sync"

	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
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

type driver struct {
	store *datastore.Store

	// mu protects the networks map.
	mu       sync.Mutex
	networks map[string]*network
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
	id     string
	driver *driver
	config *configuration

	// mu protects the endpoints map.
	mu        sync.Mutex
	endpoints map[string]*endpoint
}

// Register initializes and registers the libnetwork ipvlan driver.
func Register(r driverapi.Registerer, store *datastore.Store) error {
	d := &driver{
		store:    store,
		networks: map[string]*network{},
	}
	if err := d.initStore(); err != nil {
		return err
	}
	return r.RegisterDriver(NetworkType, d, driverapi.Capability{
		DataScope:         scope.Local,
		ConnectivityScope: scope.Global,
	})
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
