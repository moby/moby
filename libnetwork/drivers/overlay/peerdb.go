// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23 && linux

package overlay

import (
	"context"
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
	eid        string
	vtep       netip.Addr // Virtual Tunnel End Point for non-local peers
	prefixBits int        // number of 1-bits in network mask of peerIP
}

func (p *peerEntry) isLocal() bool {
	return !p.vtep.IsValid()
}

type peerMap struct {
	// set of peerEntry, note the values have to be objects and not pointers to maintain the proper equality checks
	mp setmatrix.SetMatrix[ipmac, peerEntry]
	sync.Mutex
}

type peerNetworkMap struct {
	// map with key peerKey
	mp map[string]*peerMap
	sync.Mutex
}

func (d *driver) peerDbNetworkWalk(nid string, f func(netip.Addr, net.HardwareAddr, *peerEntry) bool) error {
	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	d.peerDb.Unlock()

	if !ok {
		return nil
	}

	mp := map[ipmac]peerEntry{}
	pMap.Lock()
	for _, pKey := range pMap.mp.Keys() {
		entryDBList, ok := pMap.mp.Get(pKey)
		if ok {
			mp[pKey] = entryDBList[0]
		}
	}
	pMap.Unlock()

	for pKey, pEntry := range mp {
		if f(pKey.ip, pKey.mac.HardwareAddr(), &pEntry) {
			return nil
		}
	}

	return nil
}

func (d *driver) peerDbSearch(nid string, peerIP netip.Addr) (netip.Addr, net.HardwareAddr, *peerEntry, error) {
	var peerIPMatched netip.Addr
	var peerMacMatched net.HardwareAddr
	var pEntryMatched *peerEntry
	err := d.peerDbNetworkWalk(nid, func(ip netip.Addr, mac net.HardwareAddr, pEntry *peerEntry) bool {
		if ip == peerIP {
			peerIPMatched = ip
			peerMacMatched = mac
			pEntryMatched = pEntry
			return true
		}

		return false
	})
	if err != nil {
		return netip.Addr{}, nil, nil, fmt.Errorf("peerdb search for peer ip %q failed: %v", peerIP, err)
	}

	if !peerIPMatched.IsValid() || pEntryMatched == nil {
		return netip.Addr{}, nil, nil, fmt.Errorf("peer ip %q not found in peerdb", peerIP)
	}

	return peerIPMatched, peerMacMatched, pEntryMatched, nil
}

func (d *driver) peerDbAdd(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) (bool, int) {
	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		pMap = &peerMap{}
		d.peerDb.mp[nid] = pMap
	}
	d.peerDb.Unlock()

	pKey := ipmacOf(peerIP.Addr(), peerMac)

	pEntry := peerEntry{
		eid:        eid,
		vtep:       vtep,
		prefixBits: peerIP.Bits(),
	}

	pMap.Lock()
	defer pMap.Unlock()
	b, i := pMap.mp.Insert(pKey, pEntry)
	if i != 1 {
		// Transient case, there is more than one endpoint that is using the same IP,MAC pair
		s, _ := pMap.mp.String(pKey)
		log.G(context.TODO()).Warnf("peerDbAdd transient condition - Key:%s cardinality:%d db state:%s", pKey.String(), i, s)
	}
	return b, i
}

func (d *driver) peerDbDelete(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) (bool, int) {
	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		d.peerDb.Unlock()
		return false, 0
	}
	d.peerDb.Unlock()

	pKey := ipmacOf(peerIP.Addr(), peerMac)

	pEntry := peerEntry{
		eid:        eid,
		vtep:       vtep,
		prefixBits: peerIP.Bits(),
	}

	pMap.Lock()
	defer pMap.Unlock()
	b, i := pMap.mp.Remove(pKey, pEntry)
	if i != 0 {
		// Transient case, there is more than one endpoint that is using the same IP,MAC pair
		s, _ := pMap.mp.String(pKey)
		log.G(context.TODO()).Warnf("peerDbDelete transient condition - Key:%s cardinality:%d db state:%s", pKey, i, s)
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
// Note also that this method sends a single message on the channel and the go routine on the
// other side, will atomically loop on the whole table of peers and will program their state
// in one single atomic operation. This is fundamental to guarantee consistency, and avoid that
// new peerAdd or peerDelete gets reordered during the sandbox init.
func (d *driver) initSandboxPeerDB(nid string) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	if err := d.peerInitOp(nid); err != nil {
		log.G(context.TODO()).WithError(err).Warn("Peer init operation failed")
	}
}

func (d *driver) peerInitOp(nid string) error {
	return d.peerDbNetworkWalk(nid, func(peerIP netip.Addr, peerMac net.HardwareAddr, pEntry *peerEntry) bool {
		// Local entries do not need to be added
		if pEntry.isLocal() {
			return false
		}

		d.peerAddOp(nid, pEntry.eid, netip.PrefixFrom(peerIP, pEntry.prefixBits), peerMac, pEntry.vtep, false)
		// return false to loop on all entries
		return false
	})
}

// peerAdd adds a new entry to the peer database.
//
// Local peers are signified by an invalid vtep (i.e. netip.Addr{}).
func (d *driver) peerAdd(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	err := d.peerAddOp(nid, eid, peerIP, peerMac, vtep, true)
	if err != nil {
		log.G(context.TODO()).WithError(err).Warn("Peer add operation failed")
	}
}

func (d *driver) peerAddOp(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr, updateDB bool) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	var dbEntries int
	var inserted bool
	if updateDB {
		inserted, dbEntries = d.peerDbAdd(nid, eid, peerIP, peerMac, vtep)
		if !inserted {
			log.G(context.TODO()).Warnf("Entry already present in db: nid:%s eid:%s peerIP:%v peerMac:%v vtep:%v",
				nid, eid, peerIP, peerMac, vtep)
		}
	}

	// Local peers do not need any further configuration
	if !vtep.IsValid() {
		return nil
	}

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

	if err := d.checkEncryption(nid, vtep, false, true); err != nil {
		log.G(context.TODO()).Warn(err)
	}

	// Add neighbor entry for the peer IP
	if err := sbox.AddNeighbor(peerIP.Addr().AsSlice(), peerMac, osl.WithLinkName(s.vxlanName)); err != nil {
		if _, ok := err.(osl.NeighborSearchError); ok && dbEntries > 1 {
			// We are in the transient case so only the first configuration is programmed into the kernel
			// Upon deletion if the active configuration is deleted the next one from the database will be restored
			// Note we are skipping also the next configuration
			return nil
		}
		return fmt.Errorf("could not add neighbor entry for nid:%s eid:%s into the sandbox:%v", nid, eid, err)
	}

	// Add fdb entry to the bridge for the peer mac
	if err := sbox.AddNeighbor(vtep.AsSlice(), peerMac, osl.WithLinkName(s.vxlanName), osl.WithFamily(syscall.AF_BRIDGE)); err != nil {
		return fmt.Errorf("could not add fdb entry for nid:%s eid:%s into the sandbox:%v", nid, eid, err)
	}

	return nil
}

// peerDelete removes an entry from the peer database.
//
// Local peers are signified by an invalid vtep (i.e. netip.Addr{}).
func (d *driver) peerDelete(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	err := d.peerDeleteOp(nid, eid, peerIP, peerMac, vtep)
	if err != nil {
		log.G(context.TODO()).WithError(err).Warn("Peer delete operation failed")
	}
}

func (d *driver) peerDeleteOp(nid, eid string, peerIP netip.Prefix, peerMac net.HardwareAddr, vtep netip.Addr) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	deleted, dbEntries := d.peerDbDelete(nid, eid, peerIP, peerMac, vtep)
	if !deleted {
		log.G(context.TODO()).Warnf("Entry was not in db: nid:%s eid:%s peerIP:%v peerMac:%v vtep:%v",
			nid, eid, peerIP, peerMac, vtep)
	}

	n := d.network(nid)
	if n == nil {
		return nil
	}

	sbox := n.sandbox()
	if sbox == nil {
		return nil
	}

	if err := d.checkEncryption(nid, vtep, !vtep.IsValid(), false); err != nil {
		log.G(context.TODO()).Warn(err)
	}

	// Local peers do not have any local configuration to delete
	if vtep.IsValid() {
		s := n.getSubnetforIP(peerIP)
		if s == nil {
			return fmt.Errorf("could not find the subnet %q in network %q", peerIP.String(), n.id)
		}
		// Remove fdb entry to the bridge for the peer mac
		if err := sbox.DeleteNeighbor(vtep.AsSlice(), peerMac, osl.WithLinkName(s.vxlanName), osl.WithFamily(syscall.AF_BRIDGE)); err != nil {
			if _, ok := err.(osl.NeighborSearchError); ok && dbEntries > 0 {
				// We fall in here if there is a transient state and if the neighbor that is being deleted
				// was never been configured into the kernel (we allow only 1 configuration at the time per <ip,mac> mapping)
				return nil
			}
			return fmt.Errorf("could not delete fdb entry for nid:%s eid:%s into the sandbox:%v", nid, eid, err)
		}

		// Delete neighbor entry for the peer IP
		if err := sbox.DeleteNeighbor(peerIP.Addr().AsSlice(), peerMac, osl.WithLinkName(s.vxlanName)); err != nil {
			return fmt.Errorf("could not delete neighbor entry for nid:%s eid:%s into the sandbox:%v", nid, eid, err)
		}
	}

	if dbEntries == 0 {
		return nil
	}

	// If there is still an entry into the database and the deletion went through without errors means that there is now no
	// configuration active in the kernel.
	// Restore one configuration for the <ip,mac> directly from the database, note that is guaranteed that there is one
	peerIPAddr, peerMac, peerEntry, err := d.peerDbSearch(nid, peerIP.Addr())
	if err != nil {
		log.G(context.TODO()).Errorf("peerDeleteOp unable to restore a configuration for nid:%s ip:%v mac:%v err:%s", nid, peerIP, peerMac, err)
		return err
	}
	return d.peerAddOp(nid, peerEntry.eid, netip.PrefixFrom(peerIPAddr, peerEntry.prefixBits), peerMac, peerEntry.vtep, false)
}

func (d *driver) peerFlush(nid string) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	if err := d.peerFlushOp(nid); err != nil {
		log.G(context.TODO()).WithError(err).Warn("Peer flush operation failed")
	}
}

func (d *driver) peerFlushOp(nid string) error {
	d.peerDb.Lock()
	defer d.peerDb.Unlock()
	_, ok := d.peerDb.mp[nid]
	if !ok {
		return fmt.Errorf("Unable to find the peerDB for nid:%s", nid)
	}
	delete(d.peerDb.mp, nid)
	return nil
}
