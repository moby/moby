package null

import (
	"context"
	"sync"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const NetworkType = "null"

type driver struct {
	network string
	sync.Mutex
}

// Register registers a new instance of the null driver.
func Register(r driverapi.Registerer) error {
	return r.RegisterDriver(NetworkType, &driver{}, driverapi.Capability{
		DataScope: scope.Local,
	})
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) CreateNetwork(ctx context.Context, id string, option map[string]any, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	d.Lock()
	defer d.Unlock()

	if d.network != "" {
		return types.ForbiddenErrorf("only one instance of %q network is allowed", NetworkType)
	}

	d.network = id

	return nil
}

func (d *driver) DeleteNetwork(nid string) error {
	return types.ForbiddenErrorf("network of type %q cannot be deleted", NetworkType)
}

func (d *driver) CreateEndpoint(_ context.Context, nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]any) error {
	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]any, error) {
	return make(map[string]any), nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(_ context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, _, _ map[string]any) error {
	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	return nil
}

func (d *driver) Type() string {
	return NetworkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}
