//go:build linux
// +build linux

package overlay

import (
	"fmt"
	"net"
	"sync"
	"syscall"

	"github.com/docker/docker/libnetwork/internal/setmatrix"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/sirupsen/logrus"
)

const ovPeerTable = "overlay_peer_table"

type peerKey struct {
	peerIP  net.IP
	peerMac net.HardwareAddr
}

type peerEntry struct {
	eid        string
	vtep       net.IP
	peerIPMask net.IPMask
	isLocal    bool
}

func (p *peerEntry) MarshalDB() peerEntryDB {
	ones, bits := p.peerIPMask.Size()
	return peerEntryDB{
		eid:            p.eid,
		vtep:           p.vtep.String(),
		peerIPMaskOnes: ones,
		peerIPMaskBits: bits,
		isLocal:        p.isLocal,
	}
}

// This the structure saved into the set (SetMatrix), due to the implementation of it
// the value inserted in the set has to be Hashable so the []byte had to be converted into
// strings
type peerEntryDB struct {
	eid            string
	vtep           string
	peerIPMaskOnes int
	peerIPMaskBits int
	isLocal        bool
}

func (p *peerEntryDB) UnMarshalDB() peerEntry {
	return peerEntry{
		eid:        p.eid,
		vtep:       net.ParseIP(p.vtep),
		peerIPMask: net.CIDRMask(p.peerIPMaskOnes, p.peerIPMaskBits),
		isLocal:    p.isLocal,
	}
}

type peerMap struct {
	// set of peerEntry, note the values have to be objects and not pointers to maintain the proper equality checks
	mp setmatrix.SetMatrix[peerEntryDB]
	sync.Mutex
}

type peerNetworkMap struct {
	// map with key peerKey
	mp map[string]*peerMap
	sync.Mutex
}

func (pKey peerKey) String() string {
	return fmt.Sprintf("%s %s", pKey.peerIP, pKey.peerMac)
}

func (pKey *peerKey) Scan(state fmt.ScanState, verb rune) error {
	ipB, err := state.Token(true, nil)
	if err != nil {
		return err
	}

	pKey.peerIP = net.ParseIP(string(ipB))

	macB, err := state.Token(true, nil)
	if err != nil {
		return err
	}

	pKey.peerMac, err = net.ParseMAC(string(macB))
	return err
}

func (d *driver) peerDbWalk(f func(string, *peerKey, *peerEntry) bool) error {
	d.peerDb.Lock()
	nids := []string{}
	for nid := range d.peerDb.mp {
		nids = append(nids, nid)
	}
	d.peerDb.Unlock()

	for _, nid := range nids {
		d.peerDbNetworkWalk(nid, func(pKey *peerKey, pEntry *peerEntry) bool {
			return f(nid, pKey, pEntry)
		})
	}
	return nil
}

func (d *driver) peerDbNetworkWalk(nid string, f func(*peerKey, *peerEntry) bool) error {
	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	d.peerDb.Unlock()

	if !ok {
		return nil
	}

	mp := map[string]peerEntry{}
	pMap.Lock()
	for _, pKeyStr := range pMap.mp.Keys() {
		entryDBList, ok := pMap.mp.Get(pKeyStr)
		if ok {
			peerEntryDB := entryDBList[0]
			mp[pKeyStr] = peerEntryDB.UnMarshalDB()
		}
	}
	pMap.Unlock()

	for pKeyStr, pEntry := range mp {
		var pKey peerKey
		pEntry := pEntry
		if _, err := fmt.Sscan(pKeyStr, &pKey); err != nil {
			logrus.Warnf("Peer key scan on network %s failed: %v", nid, err)
		}
		if f(&pKey, &pEntry) {
			return nil
		}
	}

	return nil
}

func (d *driver) peerDbSearch(nid string, peerIP net.IP) (*peerKey, *peerEntry, error) {
	var pKeyMatched *peerKey
	var pEntryMatched *peerEntry
	err := d.peerDbNetworkWalk(nid, func(pKey *peerKey, pEntry *peerEntry) bool {
		if pKey.peerIP.Equal(peerIP) {
			pKeyMatched = pKey
			pEntryMatched = pEntry
			return true
		}

		return false
	})

	if err != nil {
		return nil, nil, fmt.Errorf("peerdb search for peer ip %q failed: %v", peerIP, err)
	}

	if pKeyMatched == nil || pEntryMatched == nil {
		return nil, nil, fmt.Errorf("peer ip %q not found in peerdb", peerIP)
	}

	return pKeyMatched, pEntryMatched, nil
}

func (d *driver) peerDbAdd(nid, eid string, peerIP net.IP, peerIPMask net.IPMask, peerMac net.HardwareAddr, vtep net.IP, isLocal bool) (bool, int) {
	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		pMap = &peerMap{}
		d.peerDb.mp[nid] = pMap
	}
	d.peerDb.Unlock()

	pKey := peerKey{
		peerIP:  peerIP,
		peerMac: peerMac,
	}

	pEntry := peerEntry{
		eid:        eid,
		vtep:       vtep,
		peerIPMask: peerIPMask,
		isLocal:    isLocal,
	}

	pMap.Lock()
	defer pMap.Unlock()
	b, i := pMap.mp.Insert(pKey.String(), pEntry.MarshalDB())
	if i != 1 {
		// Transient case, there is more than one endpoint that is using the same IP,MAC pair
		s, _ := pMap.mp.String(pKey.String())
		logrus.Warnf("peerDbAdd transient condition - Key:%s cardinality:%d db state:%s", pKey.String(), i, s)
	}
	return b, i
}

func (d *driver) peerDbDelete(nid, eid string, peerIP net.IP, peerIPMask net.IPMask, peerMac net.HardwareAddr, vtep net.IP, isLocal bool) (bool, int) {
	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		d.peerDb.Unlock()
		return false, 0
	}
	d.peerDb.Unlock()

	pKey := peerKey{
		peerIP:  peerIP,
		peerMac: peerMac,
	}

	pEntry := peerEntry{
		eid:        eid,
		vtep:       vtep,
		peerIPMask: peerIPMask,
		isLocal:    isLocal,
	}

	pMap.Lock()
	defer pMap.Unlock()
	b, i := pMap.mp.Remove(pKey.String(), pEntry.MarshalDB())
	if i != 0 {
		// Transient case, there is more than one endpoint that is using the same IP,MAC pair
		s, _ := pMap.mp.String(pKey.String())
		logrus.Warnf("peerDbDelete transient condition - Key:%s cardinality:%d db state:%s", pKey.String(), i, s)
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
		logrus.WithError(err).Warn("Peer init operation failed")
	}
}

func (d *driver) peerInitOp(nid string) error {
	return d.peerDbNetworkWalk(nid, func(pKey *peerKey, pEntry *peerEntry) bool {
		// Local entries do not need to be added
		if pEntry.isLocal {
			return false
		}

		d.peerAddOp(nid, pEntry.eid, pKey.peerIP, pEntry.peerIPMask, pKey.peerMac, pEntry.vtep, false, false, false, pEntry.isLocal)
		// return false to loop on all entries
		return false
	})
}

func (d *driver) peerAdd(nid, eid string, peerIP net.IP, peerIPMask net.IPMask,
	peerMac net.HardwareAddr, vtep net.IP, l2Miss, l3Miss, localPeer bool) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	err := d.peerAddOp(nid, eid, peerIP, peerIPMask, peerMac, vtep, l2Miss, l3Miss, true, localPeer)
	if err != nil {
		logrus.WithError(err).Warn("Peer add operation failed")
	}
}

func (d *driver) peerAddOp(nid, eid string, peerIP net.IP, peerIPMask net.IPMask, peerMac net.HardwareAddr, vtep net.IP, l2Miss, l3Miss, updateDB, localPeer bool) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	var dbEntries int
	var inserted bool
	if updateDB {
		inserted, dbEntries = d.peerDbAdd(nid, eid, peerIP, peerIPMask, peerMac, vtep, localPeer)
		if !inserted {
			logrus.Warnf("Entry already present in db: nid:%s eid:%s peerIP:%v peerMac:%v isLocal:%t vtep:%v",
				nid, eid, peerIP, peerMac, localPeer, vtep)
		}
	}

	// Local peers do not need any further configuration
	if localPeer {
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

	IP := &net.IPNet{
		IP:   peerIP,
		Mask: peerIPMask,
	}

	s := n.getSubnetforIP(IP)
	if s == nil {
		return fmt.Errorf("couldn't find the subnet %q in network %q", IP.String(), n.id)
	}

	if err := n.joinSandbox(s, false); err != nil {
		return fmt.Errorf("subnet sandbox join failed for %q: %v", s.subnetIP.String(), err)
	}

	if err := d.checkEncryption(nid, vtep, false, true); err != nil {
		logrus.Warn(err)
	}

	// Add neighbor entry for the peer IP
	if err := sbox.AddNeighbor(peerIP, peerMac, l3Miss, sbox.NeighborOptions().LinkName(s.vxlanName)); err != nil {
		if _, ok := err.(osl.NeighborSearchError); ok && dbEntries > 1 {
			// We are in the transient case so only the first configuration is programmed into the kernel
			// Upon deletion if the active configuration is deleted the next one from the database will be restored
			// Note we are skipping also the next configuration
			return nil
		}
		return fmt.Errorf("could not add neighbor entry for nid:%s eid:%s into the sandbox:%v", nid, eid, err)
	}

	// Add fdb entry to the bridge for the peer mac
	if err := sbox.AddNeighbor(vtep, peerMac, l2Miss, sbox.NeighborOptions().LinkName(s.vxlanName),
		sbox.NeighborOptions().Family(syscall.AF_BRIDGE)); err != nil {
		return fmt.Errorf("could not add fdb entry for nid:%s eid:%s into the sandbox:%v", nid, eid, err)
	}

	return nil
}

func (d *driver) peerDelete(nid, eid string, peerIP net.IP, peerIPMask net.IPMask,
	peerMac net.HardwareAddr, vtep net.IP, localPeer bool) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	err := d.peerDeleteOp(nid, eid, peerIP, peerIPMask, peerMac, vtep, localPeer)
	if err != nil {
		logrus.WithError(err).Warn("Peer delete operation failed")
	}
}

func (d *driver) peerDeleteOp(nid, eid string, peerIP net.IP, peerIPMask net.IPMask, peerMac net.HardwareAddr, vtep net.IP, localPeer bool) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	deleted, dbEntries := d.peerDbDelete(nid, eid, peerIP, peerIPMask, peerMac, vtep, localPeer)
	if !deleted {
		logrus.Warnf("Entry was not in db: nid:%s eid:%s peerIP:%v peerMac:%v isLocal:%t vtep:%v",
			nid, eid, peerIP, peerMac, localPeer, vtep)
	}

	n := d.network(nid)
	if n == nil {
		return nil
	}

	sbox := n.sandbox()
	if sbox == nil {
		return nil
	}

	if err := d.checkEncryption(nid, vtep, localPeer, false); err != nil {
		logrus.Warn(err)
	}

	// Local peers do not have any local configuration to delete
	if !localPeer {
		// Remove fdb entry to the bridge for the peer mac
		if err := sbox.DeleteNeighbor(vtep, peerMac, true); err != nil {
			if _, ok := err.(osl.NeighborSearchError); ok && dbEntries > 0 {
				// We fall in here if there is a transient state and if the neighbor that is being deleted
				// was never been configured into the kernel (we allow only 1 configuration at the time per <ip,mac> mapping)
				return nil
			}
			return fmt.Errorf("could not delete fdb entry for nid:%s eid:%s into the sandbox:%v", nid, eid, err)
		}

		// Delete neighbor entry for the peer IP
		if err := sbox.DeleteNeighbor(peerIP, peerMac, true); err != nil {
			return fmt.Errorf("could not delete neighbor entry for nid:%s eid:%s into the sandbox:%v", nid, eid, err)
		}
	}

	if dbEntries == 0 {
		return nil
	}

	// If there is still an entry into the database and the deletion went through without errors means that there is now no
	// configuration active in the kernel.
	// Restore one configuration for the <ip,mac> directly from the database, note that is guaranteed that there is one
	peerKey, peerEntry, err := d.peerDbSearch(nid, peerIP)
	if err != nil {
		logrus.Errorf("peerDeleteOp unable to restore a configuration for nid:%s ip:%v mac:%v err:%s", nid, peerIP, peerMac, err)
		return err
	}
	return d.peerAddOp(nid, peerEntry.eid, peerIP, peerEntry.peerIPMask, peerKey.peerMac, peerEntry.vtep, false, false, false, peerEntry.isLocal)
}

func (d *driver) peerFlush(nid string) {
	d.peerOpMu.Lock()
	defer d.peerOpMu.Unlock()
	if err := d.peerFlushOp(nid); err != nil {
		logrus.WithError(err).Warn("Peer flush operation failed")
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

func (d *driver) peerDBUpdateSelf() {
	d.peerDbWalk(func(nid string, pkey *peerKey, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			pEntry.vtep = net.ParseIP(d.advertiseAddress)
		}
		return false
	})
}
