//go:build linux

package bridge

import (
	"net"
	"net/netip"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/v2/daemon/internal/netiputil"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/drvregistry"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"github.com/moby/moby/v2/internal/testutil/storeutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNoBrNetfilter(t *testing.T) {
	testcases := []struct {
		name         string
		icc          bool
		proxy        bool
		noForwarding bool
		auxDefaultGw bool
		expIsolated  bool
		expPrefixGws bool
	}{
		{
			name:         "icc=false",
			proxy:        true,
			expIsolated:  true,
			expPrefixGws: true,
		},
		{
			name:         "icc=true, no proxy",
			icc:          true,
			expPrefixGws: true,
		},
		{
			name:  "icc=true, proxy",
			icc:   true,
			proxy: true,
		},
		{
			name:         "icc=false, aux default gateway",
			proxy:        true,
			auxDefaultGw: true,
			expIsolated:  true,
			expPrefixGws: true,
		},
		{
			name:         "icc=true, no proxy, aux default gateway",
			icc:          true,
			auxDefaultGw: true,
			expPrefixGws: true,
		},
		{
			name:         "icc=false, no forwarding",
			proxy:        true,
			noForwarding: true,
			expIsolated:  true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			useStubFirewaller(t)

			fwd := "1"
			if tc.noForwarding {
				fwd = "0"
			}
			// Set "all" before creating the bridge, so the bridge inherits it for IPv6.
			for _, f := range []string{"/proc/sys/net/ipv4/ip_forward", "/proc/sys/net/ipv6/conf/all/forwarding"} {
				assert.NilError(t, os.WriteFile(f, []byte(fwd), 0o644))
			}

			d, err := newDriver(storeutils.NewTempStore(t), Configuration{
				EnableIPTables:  true,
				EnableIP6Tables: true,
				EnableProxy:     tc.proxy,
				ProxyPath:       "/dummy/path/to/proxy",
				NoBrNetfilter:   true,
			}, &drvregistry.PortMappers{})
			assert.NilError(t, err)

			const brName = "br-nobrnf"
			ipd4, ipd6 := getIPv4Data(t), getIPv6Data(t)
			// A default gateway that isn't the bridge's address, as set by
			// "--aux-address DefaultGatewayIPv4=..." or the daemon's --default-gateway.
			var auxGw4, auxGw6 netip.Addr
			if tc.auxDefaultGw {
				pool4, _ := netiputil.ToPrefix(ipd4[0].Pool)
				pool6, _ := netiputil.ToPrefix(ipd6[0].Pool)
				auxGw4 = netiputil.LastAddr(pool4).Prev()
				auxGw6 = netiputil.LastAddr(pool6).Prev()
				ipd4[0].AuxAddresses = map[string]*net.IPNet{
					DefaultGatewayV4AuxKey: {IP: auxGw4.AsSlice(), Mask: ipd4[0].Pool.Mask},
				}
				ipd6[0].AuxAddresses = map[string]*net.IPNet{
					DefaultGatewayV6AuxKey: {IP: auxGw6.AsSlice(), Mask: ipd6[0].Pool.Mask},
				}
			}
			err = d.CreateNetwork(t.Context(), "net1", map[string]any{
				netlabel.GenericData: &networkConfiguration{
					BridgeName: brName,
					EnableIPv4: true,
					EnableIPv6: true,
					EnableICC:  tc.icc,
				},
			}, nil, ipd4, ipd6)
			assert.NilError(t, err)

			te := newTestEndpoint46(ipd4[0].Pool, ipd6[0].Pool, 11)
			err = d.CreateEndpoint(t.Context(), "net1", "ep1", te.Interface(), nil)
			assert.NilError(t, err)

			if tc.expPrefixGws {
				gw4, _ := netiputil.ToPrefix(ipd4[0].Gateway)
				gw6, _ := netiputil.ToPrefix(ipd6[0].Gateway)
				assert.Check(t, is.Equal(te.iface.prefixGw4, gw4.Addr()))
				assert.Check(t, is.Equal(te.iface.prefixGw6, gw6.Addr()))
				if tc.auxDefaultGw {
					assert.Check(t, is.DeepEqual(te.iface.onLinkHosts, []netip.Addr{auxGw4, auxGw6}, cmpopts.EquateComparable(netip.Addr{})))
				} else {
					assert.Check(t, is.Len(te.iface.onLinkHosts, 0))
				}
			} else {
				assert.Check(t, is.Equal(te.iface.prefixGw4, netip.Addr{}))
				assert.Check(t, is.Equal(te.iface.prefixGw6, netip.Addr{}))
			}

			// Find the endpoint's port on the bridge, and check whether it's isolated.
			nlh := ns.NlHandle()
			br, err := nlh.LinkByName(brName)
			assert.NilError(t, err)
			links, err := nlh.LinkList()
			assert.NilError(t, err)
			var nPorts int
			for _, l := range links {
				if l.Attrs().MasterIndex != br.Attrs().Index {
					continue
				}
				nPorts++
				pi, err := nlh.LinkGetProtinfo(l)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(pi.Isolated, tc.expIsolated), "port %s", l.Attrs().Name)
			}
			assert.Check(t, is.Equal(nPorts, 1))

			// ICMP redirects must be dropped if traffic is routed via the host.
			fw := d.firewaller.(*firewaller.StubFirewaller)
			fwn := fw.Networks[brName]
			assert.Assert(t, fwn != nil)
			assert.Check(t, is.Equal(fwn.Config4.DropRedirects, tc.expPrefixGws))
			assert.Check(t, is.Equal(fwn.Config6.DropRedirects, tc.expPrefixGws))
		})
	}
}
