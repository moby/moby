// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23 && linux

package overlay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/internal/setmatrix"
	"github.com/docker/docker/libnetwork/osl"
)

const ovPeerTable = "overlay_peer_table"

type peerEntry struct {
	eid  string
	mac  macAddr
	vtep netip.Addr
}

func (p *peerEntry) isLocal() bool {
	return !p.vtep.IsValid()
}

type peerMap struct {
	mp setmatrix.SetMatrix[netip.Prefix, peerEntry]
	mu sync.Mutex
}

type peerNetworkMap struct {
	// map with key peerKey
	mp map[string]*peerMap
	mu sync.Mutex
}

func (d *driver) peerDbNetworkWalk(nid string, f func(netip.Prefix, peerEntry) bool) {
	d.peerDb.mu.Lock()
	pMap, ok := d.peerDb.mp[nid]
	d.peerDb.mu.Unlock()

	if !ok {
		return
	}

	mp := map[netip.Prefix]peerEntry{}
	pMap.mu.Lock()
	for _, pKey := range pMap.mp.Keys() {
		entryDBList, ok := pMap.mp.Get(pKey)
		if ok {
			mp[pKey] = entryDBList[0]
		}
	}
	pMap.mu.Unlock()

	for k, v := range mp {
		if f(k, v) {
			return
		}
	}
}

func (d *driver) peerDbGet(nid string, peerIP netip.Prefix) (peerEntry, bool) {
	d.peerDb.mu.Lock()
	pMap, ok := d.peerDb.mp[nid]
	d.peerDb.mu.Unlock()
	if !ok {
		return peerEntry{}, false
	}

	pMap.mu.Lock()
	defer pMap.mu.Unlock()
	c, _ := pMap.mp.Get(peerIP)
	if len(c) == 0 {
		return peerEntry{}, false
	}
	return c[0], true
}

func (d *driver) peerDbAdd(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) (bool, int) {
	d.peerDb.mu.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		pMap = &peerMap{}
		d.peerDb.mp[nid] = pMap
	}
	d.peerDb.mu.Unlock()

	pEntry := peerEntry{
		eid:  eid,
		mac:  macAddrOf(peerMac),
		vtep: vtep,
	}

	pMap.mu.Lock()
	defer pMap.mu.Unlock()
	b, i := pMap.mp.Insert(peerIP, pEntry)
	if i != 1 {
		// Transient case, there is more than one endpoint that is using the same IP
		s, _ := pMap.mp.String(peerIP)
		log.G(context.TODO()).Warnf("peerDbAdd transient condition - Key:%s cardinality:%d db state:%s", peerIP, i, s)
	}

	return b, i
}

func (d *driver) peerDbDelete(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) (bool, int) {
	d.peerDb.mu.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		d.peerDb.mu.Unlock()
		return false, 0
	}
	d.peerDb.mu.Unlock()

	pEntry := peerEntry{
		eid:  eid,
		mac:  macAddrOf(peerMac),
		vtep: vtep,
	}

	pMap.mu.Lock()
	defer pMap.mu.Unlock()
	b, i := pMap.mp.Remove(peerIP, pEntry)
	if i != 0 {
		// Transient case, there is more than one endpoint that is using the same IP
		s, _ := pMap.mp.String(peerIP)
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
// Note also that this method atomically loops on the whole table of peers
// and programs their state in one single atomic operation.
// This is fundamental to guarantee consistency, and avoid that
// new peerAdd or peerDelete gets reordered during the sandbox init.
func (d *driver) initSandboxPeerDB(nid string) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	d.peerDbNetworkWalk(nid, func(peerIP netip.Prefix, pEntry peerEntry) bool {
		if !pEntry.isLocal() {
			d.addNeighbor(nid, peerIP, pEntry.mac.HardwareAddr(), pEntry.vtep)
		}
		return false // walk all entries
	})
}

// peerAdd adds a new entry to the peer database.
//
// Local peers are signified by an invalid vtep (i.e. netip.Addr{}).
func (d *driver) peerAdd(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) {
	if err := validateID(nid, eid); err != nil {
		log.G(context.TODO()).WithError(err).Warn("Peer add operation failed")
		return
	}

	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()

	inserted, dbEntries := d.peerDbAdd(nid, eid, peerIP, peerMac, vtep)
	if !inserted {
		log.G(context.TODO()).Warnf("Entry already present in db: nid:%s eid:%s peerIP:%v peerMac:%v vtep:%v",
			nid, eid, peerIP, peerMac, vtep)
	}
	if vtep.IsValid() {
		err := d.addNeighbor(nid, peerIP, peerMac, vtep)
		if err != nil {
			if dbEntries > 1 && errors.As(err, &osl.NeighborSearchError{}) {
				// We are in the transient case so only the first configuration is programmed into the kernel.
				// Upon deletion if the active configuration is deleted the next one from the database will be restored.
				return
			}
			log.G(context.TODO()).WithFields(log.Fields{"nid": nid, "eid": eid}).WithError(err).Warn("Peer add operation failed")
		}
	}
}

// addNeighbor programs the kernel so the given peer is reachable through the VXLAN tunnel.
func (d *driver) addNeighbor(nid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) error {
	n := d.network(nid)
	if n == nil {
		return nil
	}

	sbox := n.sandbox()
	if sbox == nil {
		// We are hitting this case for all the events that are arriving before that the sandbox
		// is being created. The peer got already added into the database and the sanbox init will
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
		if err := d.setupEncryption(vtep); err != nil {
			log.G(context.TODO()).Warn(err)
		}
	}

	// Add neighbor entry for the peer IP
	if err := sbox.AddNeighbor(peerIP.Addr().AsSlice(), peerMac, osl.WithLinkName(s.vxlanName)); err != nil {
		return fmt.Errorf("could not add neighbor entry into the sandbox: %w", err)
	}

	// Add fdb entry to the bridge for the peer mac
	if n.fdbCnt.Add(ipmacOf(vtep, peerMac), 1) == 1 {
		if err := sbox.AddNeighbor(vtep.AsSlice(), peerMac, osl.WithLinkName(s.vxlanName), osl.WithFamily(syscall.AF_BRIDGE)); err != nil {
			return fmt.Errorf("could not add fdb entry into the sandbox: %w", err)
		}
	}

	return nil
}

// peerDelete removes an entry from the peer database.
//
// Local peers are signified by an invalid vtep (i.e. netip.Addr{}).
func (d *driver) peerDelete(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) {
	logger := log.G(context.TODO()).WithFields(log.Fields{
		"nid":  nid,
		"eid":  eid,
		"ip":   peerIP,
		"mac":  peerMac,
		"vtep": vtep,
	})
	if err := validateID(nid, eid); err != nil {
		logger.WithError(err).Warn("Peer delete operation failed")
		return
	}

	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()

	deleted, dbEntries := d.peerDbDelete(nid, eid, peerIP, peerMac, vtep)
	if !deleted {
		logger.Warn("Peer entry was not in db")
	}

	if vtep.IsValid() {
		err := d.deleteNeighbor(nid, peerIP, peerMac, vtep)
		if err != nil {
			if dbEntries > 0 && errors.As(err, &osl.NeighborSearchError{}) {
				// We fall in here if there is a transient state and if the neighbor that is being deleted
				// was never been configured into the kernel (we allow only 1 configuration at the time per <ip,mac> mapping)
				return
			}
			logger.WithError(err).Warn("Peer delete operation failed")
		}

		if dbEntries > 0 {
			// If there is still an entry into the database and the deletion went through without errors means that there is now no
			// configuration active in the kernel.
			// Restore one configuration for the ip directly from the database, note that is guaranteed that there is one
			peerEntry, ok := d.peerDbGet(nid, peerIP)
			if !ok {
				log.G(context.TODO()).WithFields(log.Fields{
					"nid": nid,
					"ip":  peerIP,
				}).Error("peerDelete unable to restore a configuration: no entry found in the database")
				return
			}
			err = d.addNeighbor(nid, peerIP, peerEntry.mac.HardwareAddr(), peerEntry.vtep)
			if err != nil {
				log.G(context.TODO()).WithFields(log.Fields{
					"nid":  nid,
					"eid":  eid,
					"ip":   peerIP,
					"mac":  peerEntry.mac,
					"vtep": peerEntry.vtep,
				}).WithError(err).Error("Peer delete operation failed")
			}
		}
	}
}

// deleteNeighbor removes programming from the kernel for the given peer to be
// reachable through the VXLAN tunnel. It is the inverse of [driver.addNeighbor].
func (d *driver) deleteNeighbor(nid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) error {
	n := d.network(nid)
	if n == nil {
		return nil
	}

	sbox := n.sandbox()
	if sbox == nil {
		return nil
	}

	if n.secure {
		if err := d.removeEncryption(vtep); err != nil {
			log.G(context.TODO()).Warn(err)
		}
	}

	s := n.getSubnetforIP(peerIP)
	if s == nil {
		return fmt.Errorf("could not find the subnet %q in network %q", peerIP.String(), n.id)
	}
	// Remove fdb entry to the bridge for the peer mac
	if n.fdbCnt.Add(ipmacOf(vtep, peerMac), -1) == 0 {
		if err := sbox.DeleteNeighbor(vtep.AsSlice(), peerMac, osl.WithLinkName(s.vxlanName), osl.WithFamily(syscall.AF_BRIDGE)); err != nil {
			return fmt.Errorf("could not delete fdb entry in the sandbox: %w", err)
		}
	}

	// Delete neighbor entry for the peer IP
	if err := sbox.DeleteNeighbor(peerIP.Addr().AsSlice(), peerMac, osl.WithLinkName(s.vxlanName)); err != nil {
		return fmt.Errorf("could not delete neighbor entry in the sandbox:%v", err)
	}

	return nil
}

func (d *driver) peerFlush(nid string) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	d.peerDb.mu.Lock()
	defer d.peerDb.mu.Unlock()
	_, ok := d.peerDb.mp[nid]
	if !ok {
		log.G(context.TODO()).Warnf("Peer flush operation failed: unable to find the peerDB for nid:%s", nid)
		return
	}
	delete(d.peerDb.mp, nid)
}
