//go:build linux

package overlay

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type netlinkMonitor struct {
	network *network
	ch      chan netlink.NeighborUpdate
	done    chan struct{}
	stopCh  chan struct{}
}

func (n *network) startNetlinkMonitor() error {
	nlh := ns.NlHandle()
	if nlh == nil {
		return fmt.Errorf("failed to get netlink handle")
	}

	monitor := &netlinkMonitor{
		network: n,
		ch:      make(chan netlink.NeighborUpdate),
		done:    make(chan struct{}),
		stopCh:  make(chan struct{}),
	}

	if err := netlink.NeighborSubscribe(monitor.ch, monitor.done); err != nil {
		return fmt.Errorf("failed to subscribe to neighbor events: %v", err)
	}

	// Store monitor for cleanup
	n.netlinkMonitor = monitor

	go monitor.run()

	return nil
}

func (monitor *netlinkMonitor) run() {
	defer func() {
		// Cleanup on exit
		close(monitor.done)
		close(monitor.ch)
		log.G(context.TODO()).Debugf("Netlink monitor for network %s stopped", monitor.network.id)
	}()

	for {
		select {
		case event, ok := <-monitor.ch:
			if !ok {
				log.G(context.TODO()).Debugf("Netlink monitor channel closed for network %s", monitor.network.id)
				return
			}
			monitor.handleNeighborEvent(&event)

		case <-monitor.stopCh:
			log.G(context.TODO()).Debugf("Netlink monitor stop requested for network %s", monitor.network.id)
			return

		case <-monitor.network.stopCh:
			log.G(context.TODO()).Debugf("Network stop requested for network %s", monitor.network.id)
			return
		}
	}
}

func (monitor *netlinkMonitor) stop() {
	select {
	case <-monitor.stopCh:
		// Already stopped
		return
	default:
		close(monitor.stopCh)
	}
}

func (monitor *netlinkMonitor) handleNeighborEvent(event *netlink.NeighborUpdate) {
	monitor.network.handleNeighborMiss(event)
}


func (n *network) handleNeighborMiss(event *netlink.NeighborUpdate) {
	if event.Type != unix.RTM_GETNEIGH {
		return
	}
	
	if event.Neigh.Flags&netlink.NTF_PROXY == 0 {
		return
	}

	link, err := ns.NlHandle().LinkByIndex(event.Neigh.LinkIndex)
	if err != nil {
		log.G(context.TODO()).Debugf("Failed to get link for neighbor miss: %v", err)
		return
	}

	vxlan, ok := link.(*netlink.Vxlan)
	if !ok {
		return
	}

	var s *subnet
	for _, subnet := range n.subnets {
		if subnet.vxlanName == vxlan.Name {
			s = subnet
			break
		}
	}

	if s == nil {
		return
	}

	dstIP, ok := netip.AddrFromSlice(event.Neigh.IP)
	if !ok {
		return
	}

	if !dstIP.IsMulticast() {
		n.handleUnicastMiss(s, dstIP, event.Neigh.HardwareAddr)
		return
	}

	n.handleMulticastMiss(s, dstIP, event.Neigh.HardwareAddr)
}

func (n *network) handleUnicastMiss(s *subnet, dstIP netip.Addr, dstMAC net.HardwareAddr) {
	peerIP, peerMAC, peerEntry, err := n.driver.peerDbSearch(n.id, dstIP)
	if err != nil {
		log.G(context.TODO()).Debugf("Neighbor miss for unknown peer %s: %v", dstIP, err)
		return
	}

	if peerEntry.isLocal {
		return
	}

	log.G(context.TODO()).Debugf("Adding FDB entry for unicast miss: IP=%s MAC=%s VTEP=%s", peerIP, peerMAC, peerEntry.vtep)

	vxlan, err := netlink.LinkByName(s.vxlanName)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to find vxlan interface %s: %v", s.vxlanName, err)
		return
	}

	neigh := &netlink.Neigh{
		LinkIndex:    vxlan.Attrs().Index,
		Family:       syscall.AF_BRIDGE,
		State:        netlink.NUD_PERMANENT,
		Flags:        netlink.NTF_SELF,
		IP:           peerEntry.vtep.AsSlice(),
		HardwareAddr: peerMAC,
	}

	if err := ns.NlHandle().NeighSet(neigh); err != nil {
		log.G(context.TODO()).Warnf("Failed to add FDB entry for unicast miss: %v", err)
	}
}

func (n *network) handleMulticastMiss(s *subnet, dstIP netip.Addr, dstMAC net.HardwareAddr) {
	log.G(context.TODO()).Debugf("Handling multicast miss for IP=%s", dstIP)

	groupMAC := multicastIPToMAC(dstIP)
	if groupMAC == nil {
		return
	}

	err := n.driver.peerDbNetworkWalk(n.id, func(peerIP netip.Addr, peerMAC net.HardwareAddr, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			return false
		}

		vxlan, err := netlink.LinkByName(s.vxlanName)
		if err != nil {
			return false
		}

		neigh := &netlink.Neigh{
			LinkIndex:    vxlan.Attrs().Index,
			Family:       syscall.AF_BRIDGE,
			State:        netlink.NUD_PERMANENT,
			Flags:        netlink.NTF_SELF,
			IP:           pEntry.vtep.AsSlice(),
			HardwareAddr: groupMAC,
		}

		if err := ns.NlHandle().NeighSet(neigh); err != nil {
			log.G(context.TODO()).Debugf("Failed to add multicast FDB entry for VTEP %s: %v", pEntry.vtep, err)
		} else {
			log.G(context.TODO()).Debugf("Added multicast FDB entry: MAC=%s VTEP=%s", groupMAC, pEntry.vtep)
		}

		return false
	})

	if err != nil {
		log.G(context.TODO()).Warnf("Failed to walk peer DB for multicast miss: %v", err)
	}
}

