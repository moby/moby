package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/docker/docker/daemon/libnetwork/portmapperapi"
	"github.com/docker/docker/daemon/libnetwork/types"
	"github.com/docker/docker/internal/sliceutil"
	"github.com/docker/docker/internal/testutils/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPortMapping(t *testing.T) {
	ctrIP4 := net.ParseIP("10.0.10.1")

	testcases := []struct {
		name         string
		busyPortIPv4 int
		reqs         []portmapperapi.PortBindingReq

		expErr        string
		expPBs        []types.PortBinding
		expReleaseErr string
	}{
		{
			name: "successful mapping",
			reqs: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv4zero, HostPort: 9000, HostPortEnd: 9000}},
				{PortBinding: types.PortBinding{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv6zero, HostPort: 9000, HostPortEnd: 9000}},
			},
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv4zero, HostPort: 9000, HostPortEnd: 9000},
				{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv6zero, HostPort: 9000, HostPortEnd: 9000},
			},
		},
		{
			name:         "busy port",
			busyPortIPv4: 9000,
			reqs: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv4zero, HostPort: 9000, HostPortEnd: 9001}},
				{PortBinding: types.PortBinding{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv6zero, HostPort: 9000, HostPortEnd: 9001}},
			},
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv4zero, HostPort: 9001, HostPortEnd: 9001},
				{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv6zero, HostPort: 9001, HostPortEnd: 9001},
			},
		},
		{
			name: "error unmapping ports",
			reqs: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv4zero, HostPort: 9000, HostPortEnd: 9001}},
				{PortBinding: types.PortBinding{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv6zero, HostPort: 9000, HostPortEnd: 9001}},
			},
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv4zero, HostPort: 9000, HostPortEnd: 9000},
				{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv6zero, HostPort: 9000, HostPortEnd: 9000},
			},
			expReleaseErr: "failed to stop userland proxy: can't stop now",
		},
		{
			name:         "error mapping ports",
			busyPortIPv4: 9000,
			reqs: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv4zero, HostPort: 9000, HostPortEnd: 9000}},
				{PortBinding: types.PortBinding{Proto: types.TCP, IP: ctrIP4, Port: 9000, HostIP: net.IPv6zero, HostPort: 9000, HostPortEnd: 9000}},
			},
			expErr: "failed to bind host port 0.0.0.0:9000/tcp: address already in use",
			expPBs: []types.PortBinding{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()

			proxyMgr := stubProxyManager{
				proxies:        make(map[proxyCall]*stubProxy),
				runningProxies: make(map[proxyCall]bool),
				busyPortIPv4:   tc.busyPortIPv4,
				expReleaseErr:  tc.expReleaseErr,
			}
			pm := NewPortMapper(proxyMgr)

			if tc.busyPortIPv4 != 0 {
				tl, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero, Port: tc.busyPortIPv4})
				assert.NilError(t, err)
				defer tl.Close()
				ul, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: tc.busyPortIPv4})
				assert.NilError(t, err)
				defer ul.Close()
			}

			pbs, err := pm.MapPorts(context.Background(), tc.reqs, nil)
			bindings := sliceutil.Map(pbs, func(pb portmapperapi.PortBinding) types.PortBinding { return pb.PortBinding })
			assert.DeepEqual(t, tc.expPBs, bindings)

			if tc.expErr != "" {
				assert.ErrorContains(t, err, tc.expErr)
				return
			} else {
				assert.NilError(t, err)
			}

			err = pm.UnmapPorts(context.Background(), pbs, nil)
			if tc.expReleaseErr != "" {
				assert.ErrorContains(t, err, tc.expReleaseErr)
			} else {
				assert.NilError(t, err)
			}

			// Check a docker-proxy was started and stopped for each expected port binding.
			expProxies := map[proxyCall]bool{}
			for _, expPB := range tc.expPBs {
				p := newProxyCall(expPB.Proto.String(),
					expPB.HostIP, int(expPB.HostPort),
					expPB.IP, int(expPB.Port))
				expProxies[p] = tc.expReleaseErr != ""
			}
			assert.Check(t, is.DeepEqual(expProxies, proxyMgr.runningProxies))
		})
	}
}

// Type for tracking calls to StartProxy.
type proxyCall struct{ proto, host, container string }

func newProxyCall(proto string,
	hostIP net.IP, hostPort int,
	containerIP net.IP, containerPort int,
) proxyCall {
	return proxyCall{
		proto:     proto,
		host:      fmt.Sprintf("%v:%v", hostIP, hostPort),
		container: fmt.Sprintf("%v:%v", containerIP, containerPort),
	}
}

type stubProxyManager struct {
	busyPortIPv4   int
	expReleaseErr  string
	proxies        map[proxyCall]*stubProxy
	runningProxies map[proxyCall]bool // proxy -> is not stopped
}

func (pm stubProxyManager) StartProxy(pb types.PortBinding, _ *os.File) (portmapperapi.Proxy, error) {
	if pm.busyPortIPv4 > 0 && pm.busyPortIPv4 == int(pb.HostPort) && pb.HostIP.To4() != nil {
		return nil, errors.New("busy port")
	}
	c := newProxyCall(pb.Proto.String(), pb.HostIP, int(pb.HostPort), pb.IP, int(pb.Port))
	if _, ok := pm.proxies[c]; ok {
		return nil, fmt.Errorf("duplicate proxy: %#v", c)
	}
	pm.proxies[c] = &stubProxy{
		stop: func() error {
			if pm.expReleaseErr != "" {
				return errors.New("can't stop now")
			}
			if pm.proxies[c] == nil {
				return errors.New("already stopped")
			}
			delete(pm.proxies, c)
			pm.runningProxies[c] = false
			return nil
		},
	}
	pm.runningProxies[c] = true
	return pm.proxies[c], nil
}

type stubProxy struct {
	stop func() error
}

func (sp *stubProxy) Stop() error {
	return sp.stop()
}
