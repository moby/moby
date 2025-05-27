package libnetwork

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Linux-specific container configuration flags.
type containerConfigOS struct{} //nolint:nolintlint,unused // only populated on windows

func releaseOSSboxResources(ns *osl.Namespace, ep *Endpoint) {
	for _, i := range ns.Interfaces() {
		// Only remove the interfaces owned by this endpoint from the sandbox.
		if ep.hasInterface(i.SrcName()) {
			if err := i.Remove(); err != nil {
				log.G(context.TODO()).Debugf("Remove interface %s failed: %v", i.SrcName(), err)
			}
		}
	}

	ep.mu.Lock()
	joinInfo := ep.joinInfo
	vip := ep.virtualIP
	lbModeIsDSR := ep.network.loadBalancerMode == loadBalancerModeDSR
	ep.mu.Unlock()

	if len(vip) > 0 && lbModeIsDSR {
		ipNet := &net.IPNet{IP: vip, Mask: net.CIDRMask(32, 32)}
		if err := ns.RemoveAliasIP(ns.GetLoopbackIfaceName(), ipNet); err != nil {
			log.G(context.TODO()).WithError(err).Debugf("failed to remove virtual ip %v to loopback", ipNet)
		}
	}

	if joinInfo == nil {
		return
	}

	// Remove non-interface routes.
	for _, r := range joinInfo.StaticRoutes {
		if err := ns.RemoveStaticRoute(r); err != nil {
			log.G(context.TODO()).Debugf("Remove route failed: %v", err)
		}
	}
}

// Statistics retrieves the interfaces' statistics for the sandbox.
func (sb *Sandbox) Statistics() (map[string]*types.InterfaceStatistics, error) {
	m := make(map[string]*types.InterfaceStatistics)

	sb.mu.Lock()
	osb := sb.osSbox
	sb.mu.Unlock()
	if osb == nil {
		return m, nil
	}

	var err error
	for _, i := range osb.Interfaces() {
		if m[i.DstName()], err = i.Statistics(); err != nil {
			return m, err
		}
	}

	return m, nil
}

func (sb *Sandbox) updateGateway(ep4, ep6 *Endpoint) error {
	var populated4, populated6 bool
	sb.mu.Lock()
	osSbox := sb.osSbox
	if ep4 != nil {
		_, populated4 = sb.populatedEndpoints[ep4.ID()]
	}
	if ep6 != nil {
		_, populated6 = sb.populatedEndpoints[ep6.ID()]
	}
	sb.mu.Unlock()
	if osSbox == nil {
		return nil
	}
	osSbox.UnsetGateway()     //nolint:errcheck
	osSbox.UnsetGatewayIPv6() //nolint:errcheck
	if err := osSbox.UnsetDefaultRouteIPv4(); err != nil {
		log.G(context.TODO()).WithError(err).Warn("removing IPv4 default route")
	}
	if err := osSbox.UnsetDefaultRouteIPv6(); err != nil {
		log.G(context.TODO()).WithError(err).Warn("removing IPv6 default route")
	}

	if populated4 {
		ep4.mu.Lock()
		joinInfo := ep4.joinInfo
		ep4.mu.Unlock()

		if joinInfo.gw != nil {
			if err := osSbox.SetGateway(joinInfo.gw); err != nil {
				return fmt.Errorf("failed to set gateway: %v", err)
			}
		} else {
			if err := osSbox.SetDefaultRouteIPv4(ep4.iface.srcName); err != nil {
				return fmt.Errorf("failed to set IPv4 default route: %v", err)
			}
		}
	}

	if populated6 {
		ep6.mu.Lock()
		joinInfo := ep6.joinInfo
		ep6.mu.Unlock()

		if joinInfo.gw6 != nil {
			if err := osSbox.SetGatewayIPv6(joinInfo.gw6); err != nil {
				return fmt.Errorf("failed to set IPv6 gateway: %v", err)
			}
		} else {
			if err := osSbox.SetDefaultRouteIPv6(ep6.iface.srcName); err != nil {
				return fmt.Errorf("failed to set IPv6 default route: %v", err)
			}
		}
	}

	return nil
}

func (sb *Sandbox) ExecFunc(f func()) error {
	sb.mu.Lock()
	osSbox := sb.osSbox
	sb.mu.Unlock()
	if osSbox != nil {
		return osSbox.InvokeFunc(f)
	}
	return fmt.Errorf("osl sandbox unavailable in ExecFunc for %v", sb.ContainerID())
}

// SetKey updates the Sandbox Key.
func (sb *Sandbox) SetKey(ctx context.Context, basePath string) error {
	start := time.Now()
	defer func() {
		log.G(ctx).Debugf("sandbox set key processing took %s for container %s", time.Since(start), sb.ContainerID())
	}()

	if basePath == "" {
		return types.InvalidParameterErrorf("invalid sandbox key")
	}

	sb.mu.Lock()
	if sb.inDelete {
		sb.mu.Unlock()
		return types.ForbiddenErrorf("failed to SetKey: sandbox %q delete in progress", sb.id)
	}
	oldosSbox := sb.osSbox
	sb.mu.Unlock()

	if oldosSbox != nil {
		// If we already have an OS sandbox, release the network resources from that
		// and destroy the OS snab. We are moving into a new home further down. Note that none
		// of the network resources gets destroyed during the move.
		if err := sb.releaseOSSbox(); err != nil {
			log.G(ctx).WithError(err).Error("Error destroying os sandbox")
		}
	}

	osSbox, err := osl.GetSandboxForExternalKey(basePath, sb.Key())
	if err != nil {
		return err
	}

	sb.mu.Lock()
	sb.osSbox = osSbox
	sb.mu.Unlock()

	// If the resolver was setup before stop it and set it up in the
	// new osl sandbox.
	if oldosSbox != nil && sb.resolver != nil {
		sb.resolver.Stop()

		if err := sb.osSbox.InvokeFunc(sb.resolver.SetupFunc(0)); err == nil {
			if err := sb.resolver.Start(); err != nil {
				log.G(ctx).Errorf("Resolver Start failed for container %s, %q", sb.ContainerID(), err)
			}
		} else {
			log.G(ctx).Errorf("Resolver Setup Function failed for container %s, %q", sb.ContainerID(), err)
		}
	}

	osSbox.RefreshIPv6LoEnabled()
	if err := sb.rebuildHostsFile(ctx); err != nil {
		return err
	}

	for _, ep := range sb.Endpoints() {
		if err = sb.populateNetworkResources(ctx, ep); err != nil {
			return err
		}
	}

	return nil
}

// NetnsPath returns the network namespace's path and true, if a network has been
// created - else the empty string and false.
func (sb *Sandbox) NetnsPath() (path string, ok bool) {
	sb.mu.Lock()
	osSbox := sb.osSbox
	sb.mu.Unlock()
	if osSbox == nil {
		return "", false
	}
	return osSbox.Key(), true
}

// IPv6Enabled determines whether a container supports IPv6.
// IPv6 support can always be determined for host networking. For other network
// types it can only be determined once there's a container namespace to probe,
// return ok=false in that case.
func (sb *Sandbox) IPv6Enabled() (enabled, ok bool) {
	// For host networking, IPv6 support depends on the host.
	if sb.config.useDefaultSandBox {
		return netutils.IsV6Listenable(), true
	}

	// For other network types, look at whether the container's loopback interface has an IPv6 address.
	sb.mu.Lock()
	osSbox := sb.osSbox
	sb.mu.Unlock()

	if osSbox == nil {
		return false, false
	}
	return osSbox.IPv6LoEnabled(), true
}

func (sb *Sandbox) releaseOSSbox() error {
	sb.mu.Lock()
	osSbox := sb.osSbox
	sb.osSbox = nil
	sb.mu.Unlock()

	if osSbox == nil {
		return nil
	}

	for _, ep := range sb.Endpoints() {
		releaseOSSboxResources(osSbox, ep)
	}

	return osSbox.Destroy()
}

func (sb *Sandbox) restoreOslSandbox() error {
	var routes []*types.StaticRoute

	// restore osl sandbox
	interfaces := make(map[osl.Iface][]osl.IfaceOption)
	for _, ep := range sb.endpoints {
		ep.mu.Lock()
		joinInfo := ep.joinInfo
		i := ep.iface
		ep.mu.Unlock()

		if i == nil {
			log.G(context.TODO()).Errorf("error restoring endpoint %s for container %s", ep.Name(), sb.ContainerID())
			continue
		}

		ifaceOptions := []osl.IfaceOption{
			osl.WithIPv4Address(i.addr),
			osl.WithRoutes(i.routes),
		}
		if i.addrv6 != nil && i.addrv6.IP.To16() != nil {
			ifaceOptions = append(ifaceOptions, osl.WithIPv6Address(i.addrv6))
		}
		if i.mac != nil {
			ifaceOptions = append(ifaceOptions, osl.WithMACAddress(i.mac))
		}
		if len(i.llAddrs) != 0 {
			ifaceOptions = append(ifaceOptions, osl.WithLinkLocalAddresses(i.llAddrs))
		}
		iface := osl.Iface{SrcName: i.srcName, DstPrefix: i.dstPrefix, DstName: i.dstName}
		interfaces[iface] = ifaceOptions
		if joinInfo != nil {
			routes = append(routes, joinInfo.StaticRoutes...)
		}
		if ep.needResolver() {
			sb.startResolver(true)
		}
	}

	if err := sb.osSbox.RestoreInterfaces(interfaces); err != nil {
		return err
	}
	if len(routes) > 0 {
		sb.osSbox.RestoreRoutes(routes)
	}
	if gwEp4, gwEp6 := sb.getGatewayEndpoint(); gwEp4 != nil || gwEp6 != nil {
		if gwEp4 != nil {
			sb.osSbox.RestoreGateway(true, gwEp4.joinInfo.gw, gwEp4.iface.srcName)
		}
		if gwEp6 != nil {
			sb.osSbox.RestoreGateway(false, gwEp6.joinInfo.gw6, gwEp6.iface.srcName)
		}
	}

	return nil
}

func (sb *Sandbox) populateNetworkResources(ctx context.Context, ep *Endpoint) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.Sandbox.populateNetworkResources", trace.WithAttributes(
		attribute.String("endpoint.Name", ep.Name())))
	defer span.End()

	sb.mu.Lock()
	if sb.osSbox == nil {
		sb.mu.Unlock()
		return nil
	}
	inDelete := sb.inDelete
	sb.mu.Unlock()

	ep.mu.Lock()
	joinInfo := ep.joinInfo
	i := ep.iface
	lbModeIsDSR := ep.network.loadBalancerMode == loadBalancerModeDSR
	ep.mu.Unlock()

	if ep.needResolver() {
		sb.startResolver(false)
	}

	if i != nil && i.srcName != "" {
		var ifaceOptions []osl.IfaceOption

		ifaceOptions = append(ifaceOptions, osl.WithIPv4Address(i.addr), osl.WithRoutes(i.routes))
		if i.addrv6 != nil && i.addrv6.IP.To16() != nil {
			ifaceOptions = append(ifaceOptions, osl.WithIPv6Address(i.addrv6))
		}
		if len(i.llAddrs) != 0 {
			ifaceOptions = append(ifaceOptions, osl.WithLinkLocalAddresses(i.llAddrs))
		}
		if i.mac != nil {
			ifaceOptions = append(ifaceOptions, osl.WithMACAddress(i.mac))
		}
		if sysctls := ep.getSysctls(); len(sysctls) > 0 {
			ifaceOptions = append(ifaceOptions, osl.WithSysctls(sysctls))
		}
		if n := ep.getNetwork(); n != nil {
			if nMsgs, ok := n.advertiseAddrNMsgs(); ok {
				ifaceOptions = append(ifaceOptions, osl.WithAdvertiseAddrNMsgs(nMsgs))
			}
			if interval, ok := n.advertiseAddrInterval(); ok {
				ifaceOptions = append(ifaceOptions, osl.WithAdvertiseAddrInterval(interval))
			}
		}
		ifaceOptions = append(ifaceOptions, osl.WithCreatedInContainer(i.createdInContainer))

		if err := sb.osSbox.AddInterface(ctx, i.srcName, i.dstPrefix, i.dstName, ifaceOptions...); err != nil {
			return fmt.Errorf("failed to add interface %s to sandbox: %v", i.srcName, err)
		}

		if len(ep.virtualIP) > 0 && lbModeIsDSR {
			if sb.loadBalancerNID == "" {
				if err := sb.osSbox.DisableARPForVIP(i.srcName); err != nil {
					return fmt.Errorf("failed disable ARP for VIP: %v", err)
				}
			}
			ipNet := &net.IPNet{IP: ep.virtualIP, Mask: net.CIDRMask(32, 32)}
			if err := sb.osSbox.AddAliasIP(sb.osSbox.GetLoopbackIfaceName(), ipNet); err != nil {
				return fmt.Errorf("failed to add virtual ip %v to loopback: %v", ipNet, err)
			}
		}
	}

	if joinInfo != nil {
		// Set up non-interface routes.
		for _, r := range joinInfo.StaticRoutes {
			if err := sb.osSbox.AddStaticRoute(r); err != nil {
				return fmt.Errorf("failed to add static route %s: %v", r.Destination.String(), err)
			}
		}
	}

	// Make sure to add the endpoint to the populated endpoint set
	// before updating gateways or populating loadbalancers.
	sb.mu.Lock()
	sb.populatedEndpoints[ep.ID()] = struct{}{}
	sb.mu.Unlock()

	if gw4, gw6 := sb.getGatewayEndpoint(); ep == gw4 || ep == gw6 {
		if err := sb.updateGateway(gw4, gw6); err != nil {
			return fmt.Errorf("updating gateway endpoint: %w", err)
		}
	}

	// Populate load balancer only after updating all the other
	// information including gateway and other routes so that
	// loadbalancers are populated all the network state is in
	// place in the sandbox.
	sb.populateLoadBalancers(ep)

	// Only update the store if we did not come here as part of
	// sandbox delete. If we came here as part of delete then do
	// not bother updating the store. The sandbox object will be
	// deleted anyway
	if !inDelete {
		return sb.storeUpdate(ctx)
	}

	return nil
}
