//go:build linux

package overlay

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"syscall"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/hashable"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/setmatrix"
	"github.com/moby/moby/v2/daemon/libnetwork/osl"
)

type peerEntry struct {
	eid  string
	mac  hashable.MACAddr
	vtep netip.Addr
}

func (p *peerEntry) isLocal() bool {
	return !p.vtep.IsValid()
}

type peerMap struct {
	mp setmatrix.SetMatrix[netip.Prefix, peerEntry]
}

func (pm *peerMap) Walk(f func(netip.Prefix, peerEntry)) {
	for _, peerAddr := range pm.mp.Keys() {
		entry, ok := pm.Get(peerAddr)
		if ok {
			f(peerAddr, entry)
		}
	}
}

func (pm *peerMap) Get(peerIP netip.Prefix) (peerEntry, bool) {
	c, _ := pm.mp.Get(peerIP)
	if len(c) == 0 {
		return peerEntry{}, false
	}
	return c[0], true
}

func (pm *peerMap) Add(eid string, peerIP netip.Prefix, peerMac hashable.MACAddr, vtep netip.Addr) (bool, int) {
	pEntry := peerEntry{
		eid:  eid,
		mac:  peerMac,
		vtep: vtep,
	}
	b, i := pm.mp.Insert(peerIP, pEntry)
	if i != 1 {
		// Transient case, there is more than one endpoint that is using the same IP
		s, _ := pm.mp.String(peerIP)
		log.G(context.TODO()).Warnf("peerDbAdd transient condition - Key:%s cardinality:%d db state:%s", peerIP, i, s)
	}
	return b, i
}

func (pm *peerMap) Delete(eid string, peerIP netip.Prefix, peerMac hashable.MACAddr, vtep netip.Addr) (bool, int) {
	pEntry := peerEntry{
		eid:  eid,
		mac:  peerMac,
		vtep: vtep,
	}

	b, i := pm.mp.Remove(peerIP, pEntry)
	if i != 0 {
		// Transient case, there is more than one endpoint that is using the same IP
		s, _ := pm.mp.String(peerIP)
		log.G(context.TODO()).Warnf("peerDbDelete transient condition - Key:%s cardinality:%d db state:%s", peerIP, i, s)
	}
	return b, i
}

// The overlay uses a lazy initialization approach, this means that when a network is created
// and the driver registered the overlay does not allocate resources till the moment that a
// sandbox is actually created.
// At the moment of this call, that happens when a sandbox is initialized, is possible that
// networkDB has already delivered some events of peers already available on remote nodes,
// these peers are saved into the peerDB and this function is used to properly configure
// the network sandbox with all those peers that got previously notified.
//
// The caller is responsible for ensuring that peerAdd and peerDelete are not
// called concurrently with this function to guarantee consistency.
func (n *network) initSandboxPeerDB() error {
	var errs []error
	n.peerdb.Walk(func(peerIP netip.Prefix, pEntry peerEntry) {
		if !pEntry.isLocal() {
			if err := n.addNeighbor(peerIP, pEntry.mac, pEntry.vtep); err != nil {
				errs = append(errs, fmt.Errorf("failed to add neighbor entries for %s: %w", peerIP, err))
			}
		}
	})
	return errors.Join(errs...)
}

// peerAdd adds a new entry to the peer database.
//
// Local peers are signified by an invalid vtep (i.e. netip.Addr{}).
func (n *network) peerAdd(eid string, peerIP netip.Prefix, peerMac hashable.MACAddr, vtep netip.Addr) error {
	if eid == "" {
		return errors.New("invalid endpoint id")
	}

	inserted, dbEntries := n.peerdb.Add(eid, peerIP, peerMac, vtep)
	if !inserted {
		log.G(context.TODO()).Warnf("Entry already present in db: nid:%s eid:%s peerIP:%v peerMac:%v vtep:%v",
			n.id, eid, peerIP, peerMac, vtep)
	}
	if vtep.IsValid() {
		err := n.addNeighbor(peerIP, peerMac, vtep)
		if err != nil {
			if dbEntries > 1 && errors.As(err, &osl.NeighborSearchError{}) {
				// Conflicting neighbor entries are already programmed into the kernel and we are in the transient case.
				// Upon deletion if the active configuration is deleted the next one from the database will be restored.
				return nil
			}
			return fmt.Errorf("peer add operation failed: %w", err)
		}
	}
	return nil
}

// addNeighbor programs the kernel so the given peer is reachable through the VXLAN tunnel.
func (n *network) addNeighbor(peerIP netip.Prefix, peerMac hashable.MACAddr, vtep netip.Addr) error {
	if n.sbox == nil {
		// We are hitting this case for all the events that are arriving before that the sandbox
		// is being created. The peer got already added into the database and the sandbox init will
		// call the peerDbUpdateSandbox that will configure all these peers from the database
		return nil
	}

	s := n.getSubnetforIP(peerIP)
	if s == nil {
		return fmt.Errorf("couldn't find the subnet %q in network %q", peerIP.String(), n.id)
	}

	if err := n.joinSandbox(s, false); err != nil {
		return fmt.Errorf("subnet sandbox join failed for %q: %v", s.subnetIP.String(), err)
	}

	if n.secure {
		if err := n.driver.setupEncryption(vtep); err != nil {
			log.G(context.TODO()).Warn(err)
		}
	}

	// Add neighbor entry for the peer IP
	if err := n.sbox.AddNeighbor(peerIP.Addr().AsSlice(), peerMac.AsSlice(), osl.WithLinkName(s.vxlanName)); err != nil {
		return fmt.Errorf("could not add neighbor entry into the sandbox: %w", err)
	}

	// Add fdb entry to the bridge for the peer mac
	if n.fdbCnt.Add(hashable.IPMACFrom(vtep, peerMac), 1) == 1 {
		if err := n.sbox.AddNeighbor(vtep.AsSlice(), peerMac.AsSlice(), osl.WithLinkName(s.vxlanName), osl.WithFamily(syscall.AF_BRIDGE)); err != nil {
			return fmt.Errorf("could not add fdb entry into the sandbox: %w", err)
		}
	}

	return nil
}

// peerDelete removes an entry from the peer database.
//
// Local peers are signified by an invalid vtep (i.e. netip.Addr{}).
func (n *network) peerDelete(eid string, peerIP netip.Prefix, peerMac hashable.MACAddr, vtep netip.Addr) error {
	if eid == "" {
		return errors.New("invalid endpoint id")
	}

	logger := log.G(context.TODO()).WithFields(log.Fields{
		"nid":  n.id,
		"eid":  eid,
		"ip":   peerIP,
		"mac":  peerMac,
		"vtep": vtep,
	})
	deleted, dbEntries := n.peerdb.Delete(eid, peerIP, peerMac, vtep)
	if !deleted {
		logger.Warn("Peer entry was not in db")
	}
	if vtep.IsValid() {
		err := n.deleteNeighbor(peerIP, peerMac, vtep)
		if err != nil {
			if dbEntries > 0 && errors.As(err, &osl.NeighborSearchError{}) {
				// We fall in here if there is a transient state and if the neighbor that is being deleted
				// was never been configured into the kernel (we allow only 1 configuration at the time per <ip,mac> mapping)
				return nil
			}
			logger.WithError(err).Warn("Peer delete operation failed")
		}
	}

	if dbEntries > 0 {
		// If there is still an entry into the database and the deletion went through without errors means that there is now no
		// configuration active in the kernel.
		// Restore one configuration for the ip directly from the database, note that is guaranteed that there is one
		peerEntry, ok := n.peerdb.Get(peerIP)
		if !ok {
			return fmt.Errorf("peerDelete: unable to restore a configuration: no entry for %v found in the database", peerIP)
		}
		err := n.addNeighbor(peerIP, peerEntry.mac, peerEntry.vtep)
		if err != nil {
			return fmt.Errorf("peer delete operation failed: %w", err)
		}
	}
	return nil
}

// deleteNeighbor removes programming from the kernel for the given peer to be
// reachable through the VXLAN tunnel. It is the inverse of [driver.addNeighbor].
func (n *network) deleteNeighbor(peerIP netip.Prefix, peerMac hashable.MACAddr, vtep netip.Addr) error {
	if n.sbox == nil {
		return nil
	}

	if n.secure {
		if err := n.driver.removeEncryption(vtep); err != nil {
			log.G(context.TODO()).Warn(err)
		}
	}

	s := n.getSubnetforIP(peerIP)
	if s == nil {
		return fmt.Errorf("could not find the subnet %q in network %q", peerIP.String(), n.id)
	}
	// Remove fdb entry to the bridge for the peer mac
	if n.fdbCnt.Add(hashable.IPMACFrom(vtep, peerMac), -1) == 0 {
		if err := n.sbox.DeleteNeighbor(vtep.AsSlice(), peerMac.AsSlice(), osl.WithLinkName(s.vxlanName), osl.WithFamily(syscall.AF_BRIDGE)); err != nil {
			return fmt.Errorf("could not delete fdb entry in the sandbox: %w", err)
		}
	}

	// Delete neighbor entry for the peer IP
	if err := n.sbox.DeleteNeighbor(peerIP.Addr().AsSlice(), peerMac.AsSlice(), osl.WithLinkName(s.vxlanName)); err != nil {
		return fmt.Errorf("could not delete neighbor entry in the sandbox:%v", err)
	}

	return nil
}
