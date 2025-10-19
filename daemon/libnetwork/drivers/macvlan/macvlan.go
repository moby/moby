//go:build linux

package macvlan

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
	NetworkType         = "macvlan"      // driver type name
	modePrivate         = "private"      // macvlan mode private
	modeVepa            = "vepa"         // macvlan mode vepa
	modeBridge          = "bridge"       // macvlan mode bridge
	modePassthru        = "passthru"     // macvlan mode passthrough
	parentOpt           = "parent"       // parent interface -o parent
	driverModeOpt       = "macvlan_mode" // macvlan mode ux opt suffix
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

// Register initializes and registers the libnetwork macvlan driver
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
