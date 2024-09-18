package libnetwork

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/Microsoft/hcsshim"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAddEpToResolver(t *testing.T) {
	const (
		ep1v4      = "192.0.2.11"
		ep2v4      = "192.0.2.12"
		epFiveDNS  = "192.0.2.13"
		epNoIntDNS = "192.0.2.14"
		ep1v6      = "2001:db8:aaaa::2"
		gw1v4      = "192.0.2.1"
		gw2v4      = "192.0.2.2"
		gw1v6      = "2001:db8:aaaa::1"
		dns1v4     = "198.51.100.1"
		dns2v4     = "198.51.100.2"
		dns3v4     = "198.51.100.3"
	)
	hnsEndpoints := map[string]hcsshim.HNSEndpoint{
		ep1v4: {
			IPAddress:         net.ParseIP(ep1v4),
			GatewayAddress:    gw1v4,
			DNSServerList:     gw1v4 + "," + dns1v4,
			EnableInternalDNS: true,
		},
		ep2v4: {
			IPAddress:         net.ParseIP(ep2v4),
			GatewayAddress:    gw1v4,
			DNSServerList:     gw1v4 + "," + dns2v4,
			EnableInternalDNS: true,
		},
		epFiveDNS: {
			IPAddress:         net.ParseIP(epFiveDNS),
			GatewayAddress:    gw1v4,
			DNSServerList:     gw1v4 + "," + dns1v4 + "," + dns2v4 + "," + dns3v4 + ",198.51.100.4",
			EnableInternalDNS: true,
		},
		epNoIntDNS: {
			IPAddress:      net.ParseIP(epNoIntDNS),
			GatewayAddress: gw1v4,
			DNSServerList:  gw1v4 + "," + dns1v4,
			// EnableInternalDNS: false,
		},
		ep1v6: {
			IPv6Address:       net.ParseIP(ep1v6),
			GatewayAddressV6:  gw1v6,
			DNSServerList:     gw1v6 + "," + dns1v4,
			EnableInternalDNS: true,
		},
	}

	makeIPNet := func(addr, netmask string) *net.IPNet {
		t.Helper()
		ip, ipnet, err := net.ParseCIDR(addr + "/" + netmask)
		assert.NilError(t, err)
		return &net.IPNet{IP: ip, Mask: ipnet.Mask}
	}

	testcases := []struct {
		name           string
		epToAdd        *EndpointInterface
		hnsEndpoints   []hcsshim.HNSEndpoint
		resolverLAs    []string
		expIPToExtDNS  map[netip.Addr][maxExtDNS]extDNSEntry
		expResolverIdx int
	}{
		{
			name: "ipv4",
			epToAdd: &EndpointInterface{
				addr: makeIPNet(ep1v4, "32"),
			},
			hnsEndpoints: []hcsshim.HNSEndpoint{
				hnsEndpoints[ep1v4],
			},
			resolverLAs: []string{gw1v4},
			expIPToExtDNS: map[netip.Addr][maxExtDNS]extDNSEntry{
				netip.MustParseAddr(ep1v4): {{IPStr: dns1v4}},
			},
		},
		{
			name: "limit of three dns servers",
			epToAdd: &EndpointInterface{
				addr: makeIPNet(epFiveDNS, "32"),
			},
			hnsEndpoints: []hcsshim.HNSEndpoint{
				hnsEndpoints[epFiveDNS],
			},
			resolverLAs: []string{gw1v4},
			// Expect the internal resolver to keep the first three ext-servers.
			expIPToExtDNS: map[netip.Addr][maxExtDNS]extDNSEntry{
				netip.MustParseAddr(epFiveDNS): {
					{IPStr: dns1v4},
					{IPStr: dns2v4},
					{IPStr: dns3v4},
				},
			},
		},
		{
			name: "disabled internal resolver",
			epToAdd: &EndpointInterface{
				addr: makeIPNet(epNoIntDNS, "32"),
			},
			hnsEndpoints: []hcsshim.HNSEndpoint{
				hnsEndpoints[epNoIntDNS],
				hnsEndpoints[ep2v4],
			},
			resolverLAs: []string{gw1v4},
		},
		{
			name: "missing internal resolver",
			epToAdd: &EndpointInterface{
				addr: makeIPNet(ep1v4, "32"),
			},
			hnsEndpoints: []hcsshim.HNSEndpoint{
				hnsEndpoints[ep1v4],
			},
			// The only resolver is for the gateway on a different network.
			resolverLAs: []string{gw2v4},
		},
		{
			name: "multiple resolvers and endpoints",
			epToAdd: &EndpointInterface{
				addr: makeIPNet(ep2v4, "32"),
			},
			hnsEndpoints: []hcsshim.HNSEndpoint{
				hnsEndpoints[ep1v4],
				hnsEndpoints[ep2v4],
			},
			// Put the internal resolver for this network second in the list.
			expResolverIdx: 1,
			resolverLAs:    []string{gw2v4, gw1v4},
			expIPToExtDNS: map[netip.Addr][maxExtDNS]extDNSEntry{
				netip.MustParseAddr(ep2v4): {{IPStr: dns2v4}},
			},
		},
		{
			name: "ipv6",
			epToAdd: &EndpointInterface{
				addrv6: makeIPNet(ep1v6, "80"),
			},
			hnsEndpoints: []hcsshim.HNSEndpoint{
				hnsEndpoints[ep1v6],
			},
			resolverLAs: []string{gw1v6},
			expIPToExtDNS: map[netip.Addr][maxExtDNS]extDNSEntry{
				netip.MustParseAddr(ep1v6): {{IPStr: dns1v4}},
			},
		},
	}

	eMapCmpOpts := []cmp.Option{
		cmpopts.EquateEmpty(),
		cmpopts.EquateComparable(netip.Addr{}),
		cmpopts.IgnoreUnexported(extDNSEntry{}),
	}
	emptyEMap := map[netip.Addr][maxExtDNS]extDNSEntry{}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up resolvers with the required listen-addresses.
			var resolvers []*Resolver
			for _, la := range tc.resolverLAs {
				resolvers = append(resolvers, NewResolver(la, true, nil))
			}

			// Add the endpoint and check expected results.
			err := addEpToResolverImpl(context.TODO(),
				"netname", "epname", tc.epToAdd, resolvers, tc.hnsEndpoints)
			assert.Check(t, err)
			for i, resolver := range resolvers {
				if i == tc.expResolverIdx {
					assert.Check(t, is.DeepEqual(resolver.ipToExtDNS.eMap, tc.expIPToExtDNS,
						eMapCmpOpts...), fmt.Sprintf("resolveridx=%d", i))
				} else {
					assert.Check(t, is.DeepEqual(resolver.ipToExtDNS.eMap, emptyEMap,
						eMapCmpOpts...), fmt.Sprintf("resolveridx=%d", i))
				}
			}

			// Delete the endpoint, check nothing got left behind.
			err = deleteEpFromResolverImpl("epname", tc.epToAdd, resolvers, tc.hnsEndpoints)
			assert.Check(t, err)
			for i, resolver := range resolvers {
				assert.Check(t, is.DeepEqual(resolver.ipToExtDNS.eMap, emptyEMap,
					eMapCmpOpts...), fmt.Sprintf("resolveridx=%d", i))
			}
		})
	}
}
