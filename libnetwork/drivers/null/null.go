package null

import (
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
)

const networkType = "null"

type driver struct{}

// New provides a new instance of null driver
func New() (string, driverapi.Driver) {
	return networkType, &driver{}
}

func (d *driver) Config(option interface{}) error {
	return nil
}

func (d *driver) CreateNetwork(id types.UUID, option interface{}) error {
	return nil
}

func (d *driver) DeleteNetwork(nid types.UUID) error {
	return nil
}

func (d *driver) CreateEndpoint(nid, eid types.UUID, epOptions interface{}) (*sandbox.Info, error) {
	return nil, nil
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	return nil
}

func (d *driver) Type() string {
	return networkType
}
