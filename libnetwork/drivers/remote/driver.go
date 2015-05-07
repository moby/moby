package remote

import (
	"errors"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
)

var errNoCallback = errors.New("No Callback handler registered with Driver")

const remoteNetworkType = "remote"

type driver struct {
	networkType string
	callback    driverapi.DriverCallback
}

// New instance of remote driver returned to LibNetwork
func New(dc driverapi.DriverCallback) (string, driverapi.Driver) {
	d := &driver{networkType: remoteNetworkType}
	d.callback = dc
	return d.networkType, d
}

// Internal Convenience method to register a remote driver.
// The implementation of this method will change based on the dynamic driver registration design
func (d *driver) registerRemoteDriver(networkType string) (driverapi.Driver, error) {
	newDriver := &driver{networkType: networkType}
	if d.callback == nil {
		return nil, errNoCallback
	}
	if err := d.callback.RegisterDriver(networkType, newDriver); err != nil {
		return nil, err
	}
	return newDriver, nil
}

func (d *driver) Config(option map[string]interface{}) error {
	return driverapi.ErrNotImplemented
}

func (d *driver) CreateNetwork(id types.UUID, option map[string]interface{}) error {
	return driverapi.ErrNotImplemented
}

func (d *driver) DeleteNetwork(nid types.UUID) error {
	return driverapi.ErrNotImplemented
}

func (d *driver) CreateEndpoint(nid, eid types.UUID, epOptions map[string]interface{}) (*sandbox.Info, error) {
	return nil, driverapi.ErrNotImplemented
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	return driverapi.ErrNotImplemented
}

func (d *driver) EndpointInfo(nid, eid types.UUID) (map[string]interface{}, error) {
	return nil, driverapi.ErrNotImplemented
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid types.UUID, sboxKey string, options map[string]interface{}) (*driverapi.JoinInfo, error) {
	return nil, driverapi.ErrNotImplemented
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid types.UUID, options map[string]interface{}) error {
	return driverapi.ErrNotImplemented
}

func (d *driver) Type() string {
	return d.networkType
}
