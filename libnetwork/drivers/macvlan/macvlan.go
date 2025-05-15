//go:build linux

package macvlan

import (
	"context"
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
func Register(r driverapi.Registerer, store *datastore.Store, _ map[string]interface{}) error {
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

func (d *driver) NetworkAllocate(_ string, _ map[string]string, _, _ []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(_ string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EndpointOperInfo(_, _ string) (map[string]interface{}, error) {
	return make(map[string]interface{}), nil
}

func (d *driver) Type() string {
	return NetworkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}

func (d *driver) ProgramExternalConnectivity(_ context.Context, _, _ string, _ map[string]interface{}) error {
	return nil
}

func (d *driver) RevokeExternalConnectivity(_, _ string) error {
	return nil
}

func (d *driver) EventNotify(_ driverapi.EventType, _, _, _ string, _ []byte) {
}

func (d *driver) DecodeTableEntry(_ string, _ string, _ []byte) (string, map[string]string) {
	return "", nil
}
