//go:build windows

package libnetwork

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/windows"
	winlibnetwork "github.com/moby/moby/v2/daemon/libnetwork/drivers/windows"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	networkSettings "github.com/moby/moby/v2/daemon/network"
	"github.com/pkg/errors"
)

type platformNetwork struct {
	resolverOnce   sync.Once
	dnsCompartment uint32
}

func executeInCompartment(compartmentID uint32, x func()) {
	runtime.LockOSThread()

	if err := hcsshim.SetCurrentThreadCompartmentId(compartmentID); err != nil {
		log.G(context.TODO()).Error(err)
	}
	defer func() {
		hcsshim.SetCurrentThreadCompartmentId(0)
		runtime.UnlockOSThread()
	}()

	x()
}

func (n *Network) ExecFunc(f func()) error {
	executeInCompartment(n.dnsCompartment, f)
	return nil
}

func (n *Network) startResolver() {
	if n.networkType == "ics" {
		return
	}
	n.resolverOnce.Do(func() {
		log.G(context.TODO()).Debugf("Launching DNS server for network %q", n.Name())
		hnsid := n.DriverOptions()[windows.HNSID]
		if hnsid == "" {
			return
		}

		hnsresponse, err := hcsshim.HNSNetworkRequest(http.MethodGet, hnsid, "")
		if err != nil {
			log.G(context.TODO()).Errorf("Resolver Setup/Start failed for container %s, %q", n.Name(), err)
			return
		}

		for _, subnet := range hnsresponse.Subnets {
			if subnet.GatewayAddress != "" {
				for i := 0; i < 3; i++ {
					resolver := NewResolver(subnet.GatewayAddress, true, n)
					log.G(context.TODO()).Debugf("Binding a resolver on network %s gateway %s", n.Name(), subnet.GatewayAddress)
					n.dnsCompartment = hnsresponse.DNSServerCompartment
					n.ExecFunc(resolver.SetupFunc(53))

					if err = resolver.Start(); err != nil {
						log.G(context.TODO()).Errorf("Resolver Setup/Start failed for container %s, %q", n.Name(), err)
						time.Sleep(1 * time.Second)
					} else {
						log.G(context.TODO()).Debugf("Resolver bound successfully for network %s", n.Name())
						n.resolver = append(n.resolver, resolver)
						break
					}
				}
			}
		}
	})
}

var hnsOwnedNetNames = map[string]struct{}{
	"Default Switch":         {},
	"WSL (Hyper-V firewall)": {},
}

// IsPruneable returns true if n can be considered for removal as part of a
// "docker network prune" (or system prune). The caller must still check that the
// network should be removed. For example, it may have active endpoints.
func (n *Network) IsPruneable() bool {
	// Predefined networks (nat, host, ...) should never be removed.
	name := n.Name()
	if networkSettings.IsPredefined(name) {
		return false
	}
	// Don't prune well-known host networks that Docker did not create, but
	// that were adopted (added to the network store) before the HNSOwned
	// label existed.
	if _, ok := hnsOwnedNetNames[name]; ok {
		return false
	}
	gd, ok := n.generic[netlabel.GenericData]
	if !ok {
		return true
	}
	// Don't prune networks that were marked as being created outside Docker
	// when they were found on the host and adopted as Docker networks.
	gdm, ok := gd.(map[string]string)
	if !ok {
		return true
	}
	_, ok = gdm[winlibnetwork.HNSOwned]
	return !ok
}

// addEpToResolver configures the internal DNS resolver for an endpoint.
//
// Windows resolvers don't consistently fall back to a secondary server if they
// get a SERVFAIL from our resolver. So, our resolver needs to forward the query
// upstream.
//
// To retrieve the list of DNS Servers to use for requests originating from an
// endpoint, this method finds the HNSEndpoint represented by the endpoint. If
// HNSEndpoint's list of DNS servers includes the HNSEndpoint's gateway address,
// it's the Resolver running at that address. Other DNS servers in the
// list have either come from config ('--dns') or have been set up by HNS as
// external resolvers, these are the external servers the Resolver should
// use for DNS requests from that endpoint.
func addEpToResolver(
	ctx context.Context,
	netName, epName string,
	config *containerConfig,
	epIface *EndpointInterface,
	resolvers []*Resolver,
) error {
	hnsEndpoints, err := hcsshim.HNSListEndpointRequest()
	if err != nil {
		return nil
	}
	return addEpToResolverImpl(ctx, netName, epName, epIface, resolvers, hnsEndpoints)
}

func addEpToResolverImpl(
	ctx context.Context,
	netName, epName string,
	epIface *EndpointInterface,
	resolvers []*Resolver,
	hnsEndpoints []hcsshim.HNSEndpoint,
) error {
	// Find the HNSEndpoint represented by ep, matching on endpoint address.
	hnsEp := findHNSEp(epIface.addr, epIface.addrv6, hnsEndpoints)
	if hnsEp == nil || !hnsEp.EnableInternalDNS {
		return nil
	}

	// Find the resolver for that HNSEndpoint, matching on gateway address.
	resolver := findResolver(resolvers, hnsEp.GatewayAddress, hnsEp.GatewayAddressV6)
	if resolver == nil {
		log.G(ctx).Debug("No internal DNS resolver to configure")
		return nil
	}

	// Get the list of DNS servers HNS has set up for this Endpoint.
	var dnsList []extDNSEntry
	dnsServers := strings.Split(hnsEp.DNSServerList, ",")

	// Create an extDNSEntry for each DNS server, apart from 'resolver' itself.
	var foundSelf bool
	hnsGw4, _ := netip.ParseAddr(hnsEp.GatewayAddress)
	hnsGw6, _ := netip.ParseAddr(hnsEp.GatewayAddressV6)
	for _, dnsServer := range dnsServers {
		dnsAddr, _ := netip.ParseAddr(dnsServer)
		if dnsAddr.IsValid() && (dnsAddr == hnsGw4 || dnsAddr == hnsGw6) {
			foundSelf = true
		} else {
			dnsList = append(dnsList, extDNSEntry{IPStr: dnsServer})
		}
	}
	if !foundSelf {
		log.G(ctx).Debug("Endpoint is not configured to use internal DNS resolver")
		return nil
	}

	// If the internal resolver is configured as one of this endpoint's DNS servers,
	// tell it which ext servers to use for requests from this endpoint's addresses.
	log.G(ctx).Infof("External DNS servers for '%s': %v", epName, dnsList)
	if srcAddr, ok := netip.AddrFromSlice(hnsEp.IPAddress); ok {
		if err := resolver.SetExtServersForSrc(srcAddr.Unmap(), dnsList); err != nil {
			return errors.Wrapf(err, "failed to set external DNS servers for %s address %s",
				epName, hnsEp.IPAddress)
		}
	}
	if srcAddr, ok := netip.AddrFromSlice(hnsEp.IPv6Address); ok {
		if err := resolver.SetExtServersForSrc(srcAddr, dnsList); err != nil {
			return errors.Wrapf(err, "failed to set external DNS servers for %s address %s",
				epName, hnsEp.IPv6Address)
		}
	}
	return nil
}

func deleteEpFromResolver(epName string, epIface *EndpointInterface, resolvers []*Resolver) error {
	hnsEndpoints, err := hcsshim.HNSListEndpointRequest()
	if err != nil {
		return nil
	}
	return deleteEpFromResolverImpl(epName, epIface, resolvers, hnsEndpoints)
}

func deleteEpFromResolverImpl(
	epName string,
	epIface *EndpointInterface,
	resolvers []*Resolver,
	hnsEndpoints []hcsshim.HNSEndpoint,
) error {
	// Find the HNSEndpoint represented by ep, matching on endpoint address.
	hnsEp := findHNSEp(epIface.addr, epIface.addrv6, hnsEndpoints)
	if hnsEp == nil {
		return fmt.Errorf("no HNS endpoint for %s", epName)
	}

	// Find the resolver for that HNSEndpoint, matching on gateway address.
	resolver := findResolver(resolvers, hnsEp.GatewayAddress, hnsEp.GatewayAddressV6)
	if resolver == nil {
		return nil
	}

	// Delete external DNS servers for the endpoint's IP addresses.
	if srcAddr, ok := netip.AddrFromSlice(hnsEp.IPAddress); ok {
		if err := resolver.SetExtServersForSrc(srcAddr.Unmap(), nil); err != nil {
			return errors.Wrapf(err, "failed to delete external DNS servers for %s address %s",
				epName, hnsEp.IPv6Address)
		}
	}
	if srcAddr, ok := netip.AddrFromSlice(hnsEp.IPv6Address); ok {
		if err := resolver.SetExtServersForSrc(srcAddr, nil); err != nil {
			return errors.Wrapf(err, "failed to delete external DNS servers for %s address %s",
				epName, hnsEp.IPv6Address)
		}
	}

	return nil
}

func findHNSEp(ip4, ip6 *net.IPNet, hnsEndpoints []hcsshim.HNSEndpoint) *hcsshim.HNSEndpoint {
	if ip4 == nil && ip6 == nil {
		return nil
	}
	for _, hnsEp := range hnsEndpoints {
		if ip4 != nil && hnsEp.IPAddress != nil && hnsEp.IPAddress.Equal(ip4.IP) {
			return &hnsEp
		}
		if ip6 != nil && hnsEp.IPv6Address != nil && hnsEp.IPv6Address.Equal(ip6.IP) {
			return &hnsEp
		}
	}
	return nil
}

func findResolver(resolvers []*Resolver, gw4, gw6 string) *Resolver {
	gw4addr, _ := netip.ParseAddr(gw4)
	gw6addr, _ := netip.ParseAddr(gw6)
	for _, resolver := range resolvers {
		ns := resolver.NameServer()
		if ns.IsValid() && (ns == gw4addr || ns == gw6addr) {
			return resolver
		}
	}
	return nil
}

func (n *Network) validatedAdvertiseAddrNMsgs() (*int, error) {
	return nil, nil
}

func (n *Network) validatedAdvertiseAddrInterval() (*time.Duration, error) {
	return nil, nil
}
