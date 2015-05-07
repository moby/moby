package remote

import (
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/types"
)

var errNoCallback = errors.New("No Callback handler registered with Driver")

type driver struct {
	endpoint    *plugins.Client
	networkType string
}

// Init does the necessary work to register remote drivers
func Init(dc driverapi.DriverCallback) error {
	plugins.Handle(driverapi.NetworkPluginEndpointType, func(name string, client *plugins.Client) {

		// TODO : Handhake with the Remote Plugin goes here

		newDriver := &driver{networkType: name, endpoint: client}
		if err := dc.RegisterDriver(name, newDriver); err != nil {
			log.Errorf("Error registering Driver for %s due to %v", name, err)
		}
	})
	return nil
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

func (d *driver) CreateEndpoint(nid, eid types.UUID, epInfo driverapi.EndpointInfo, epOptions map[string]interface{}) error {
	return driverapi.ErrNotImplemented
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	return driverapi.ErrNotImplemented
}

func (d *driver) EndpointOperInfo(nid, eid types.UUID) (map[string]interface{}, error) {
	return nil, driverapi.ErrNotImplemented
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid types.UUID, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	return driverapi.ErrNotImplemented
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid types.UUID) error {
	return driverapi.ErrNotImplemented
}

func (d *driver) Type() string {
	return d.networkType
}
