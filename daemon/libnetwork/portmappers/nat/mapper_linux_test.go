package nat

import (
	"context"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/portmapperapi"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestBindHostPortsError(t *testing.T) {
	cfg := []portmapperapi.PortBindingReq{
		{
			PortBinding: types.PortBinding{
				Proto:       types.TCP,
				Port:        80,
				HostPort:    8080,
				HostPortEnd: 8080,
			},
		},
		{
			PortBinding: types.PortBinding{
				Proto:       types.TCP,
				Port:        80,
				HostPort:    8080,
				HostPortEnd: 8081,
			},
		},
	}
	pm := &PortMapper{}
	pbs, err := pm.MapPorts(context.Background(), cfg, nil)
	assert.Check(t, is.Error(err, "port binding mismatch 80/tcp:8080-8080, 80/tcp:8080-8081"))
	assert.Check(t, is.Nil(pbs))
}
