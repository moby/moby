package nat

import (
	"context"
	"errors"
	"net"
	"net/netip"
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
	pbs, err := pm.MapPorts(context.Background(), cfg)
	assert.Check(t, is.Error(err, "port binding mismatch 80/tcp:8080-8080, 80/tcp:8080-8081"))
	assert.Check(t, is.Nil(pbs))
}

type unsupportedPortDriverClient struct{}

func (unsupportedPortDriverClient) ChildHostIP(proto string, hostIP netip.Addr) netip.Addr {
	return netip.Addr{}
}

func (unsupportedPortDriverClient) AddPort(ctx context.Context, proto string, hostIP, childIP netip.Addr, hostPort int) (func() error, error) {
	return nil, errors.New("unexpected call to AddPort")
}

func TestMapPortsUnsupportedByPortDriver(t *testing.T) {
	cfg := []portmapperapi.PortBindingReq{
		{
			PortBinding: types.PortBinding{
				Proto:       types.TCP,
				Port:        80,
				HostIP:      net.IPv6loopback,
				HostPort:    8080,
				HostPortEnd: 8080,
			},
		},
	}
	pm := &PortMapper{pdc: unsupportedPortDriverClient{}}
	pbs, err := pm.MapPorts(context.Background(), cfg)
	assert.Check(t, is.Error(err, "port mapping [[::1]:8080:80/tcp] is not supported by the RootlessKit port driver"))
	assert.Check(t, is.Nil(pbs))
}
