package host

import (
	"sync"

	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/types"
)

const networkType = "host"

type driver struct {
	network string
	sync.Mutex
}

// Init registers a new instance of host driver
func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	c := driverapi.Capability{
		DataScope: datastore.LocalScope,
	}
	return dc.RegisterDriver(networkType, &driver{}, c)
}

func (d *driver) CreateNetwork(id string, option map[string]interface{}) error {
	d.Lock()
	defer d.Unlock()

	if d.network != "" {
		return types.ForbiddenErrorf("only one instance of \"%s\" network is allowed", networkType)
	}

	d.network = id

	return nil
}

func (d *driver) DeleteNetwork(nid string) error {
	return types.ForbiddenErrorf("network of type \"%s\" cannot be deleted", networkType)
}

func (d *driver) CreateEndpoint(nid, eid string, epInfo driverapi.EndpointInfo, epOptions map[string]interface{}) error {
	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return make(map[string]interface{}, 0), nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	return nil
}

func (d *driver) Type() string {
	return networkType
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType driverapi.DiscoveryType, data interface{}) error {
	return nil
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dType driverapi.DiscoveryType, data interface{}) error {
	return nil
}
