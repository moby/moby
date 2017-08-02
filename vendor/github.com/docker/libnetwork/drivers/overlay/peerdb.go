package overlay

import (
	"context"
	"fmt"
	"net"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/common"
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
	inSandbox  bool
	isLocal    bool
}

type peerMap struct {
	mp map[string]peerEntry
	sync.Mutex
}

type peerNetworkMap struct {
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
	if err != nil {
		return err
	}

	return nil
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
	for pKeyStr, pEntry := range pMap.mp {
		mp[pKeyStr] = pEntry
	}
	pMap.Unlock()

	for pKeyStr, pEntry := range mp {
		var pKey peerKey
		if _, err := fmt.Sscan(pKeyStr, &pKey); err != nil {
			logrus.Warnf("Peer key scan on network %s failed: %v", nid, err)
		}
		if f(&pKey, &pEntry) {
			return nil
		}
	}

	return nil
}

func (d *driver) peerDbSearch(nid string, peerIP net.IP) (net.HardwareAddr, net.IPMask, net.IP, error) {
	var (
		peerMac    net.HardwareAddr
		vtep       net.IP
		peerIPMask net.IPMask
		found      bool
	)

	err := d.peerDbNetworkWalk(nid, func(pKey *peerKey, pEntry *peerEntry) bool {
		if pKey.peerIP.Equal(peerIP) {
			peerMac = pKey.peerMac
			peerIPMask = pEntry.peerIPMask
			vtep = pEntry.vtep
			found = true
			return found
		}

		return found
	})

	if err != nil {
		return nil, nil, nil, fmt.Errorf("peerdb search for peer ip %q failed: %v", peerIP, err)
	}

	if !found {
		return nil, nil, nil, fmt.Errorf("peer ip %q not found in peerdb", peerIP)
	}

	return peerMac, peerIPMask, vtep, nil
}

func (d *driver) peerDbAdd(nid, eid string, peerIP net.IP, peerIPMask net.IPMask,
	peerMac net.HardwareAddr, vtep net.IP, isLocal bool) {

	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		d.peerDb.mp[nid] = &peerMap{
			mp: make(map[string]peerEntry),
		}

		pMap = d.peerDb.mp[nid]
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
	pMap.mp[pKey.String()] = pEntry
	pMap.Unlock()
}

func (d *driver) peerDbDelete(nid, eid string, peerIP net.IP, peerIPMask net.IPMask,
	peerMac net.HardwareAddr, vtep net.IP) peerEntry {

	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		d.peerDb.Unlock()
		return peerEntry{}
	}
	d.peerDb.Unlock()

	pKey := peerKey{
		peerIP:  peerIP,
		peerMac: peerMac,
	}

	pMap.Lock()

	pEntry, ok := pMap.mp[pKey.String()]
	if ok {
		// Mismatched endpoint ID(possibly outdated). Do not
		// delete peerdb
		if pEntry.eid != eid {
			pMap.Unlock()
			return pEntry
		}
	}

	delete(pMap.mp, pKey.String())
	pMap.Unlock()

	return pEntry
}

func (d *driver) peerDbUpdateSandbox(nid string) {
	d.peerDb.Lock()
	pMap, ok := d.peerDb.mp[nid]
	if !ok {
		d.peerDb.Unlock()
		return
	}
	d.peerDb.Unlock()

	pMap.Lock()
	for pKeyStr, pEntry := range pMap.mp {
		var pKey peerKey
		if _, err := fmt.Sscan(pKeyStr, &pKey); err != nil {
			fmt.Printf("peer key scan failed: %v", err)
		}

		if pEntry.isLocal {
			continue
		}

		// Go captures variables by reference. The pEntry could be
		// pointing to the same memory location for every iteration. Make
		// a copy of pEntry before capturing it in the following closure.
		entry := pEntry

		d.peerAdd(nid, entry.eid, pKey.peerIP, entry.peerIPMask, pKey.peerMac, entry.vtep, false, false, false, false)
	}
	pMap.Unlock()
}

type peerOperation struct {
	isAdd      bool
	networkID  string
	endpointID string
	peerIP     net.IP
	peerIPMask net.IPMask
	peerMac    net.HardwareAddr
	vtepIP     net.IP
	updateDB   bool
	l2Miss     bool
	l3Miss     bool
	localPeer  bool
	callerName string
}

func (d *driver) peerOpRoutine(ctx context.Context, ch chan *peerOperation) {
	var err error
	for {
		select {
		case <-ctx.Done():
			return
		case op := <-ch:
			if op.isAdd {
				err = d.peerAddOp(op.networkID, op.endpointID, op.peerIP, op.peerIPMask, op.peerMac, op.vtepIP, op.updateDB, op.l2Miss, op.l3Miss, op.localPeer)
			} else {
				err = d.peerDeleteOp(op.networkID, op.endpointID, op.peerIP, op.peerIPMask, op.peerMac, op.vtepIP, op.localPeer)
			}
			if err != nil {
				logrus.Warnf("Peer operation failed:%s op:%v", err, op)
			}
		}
	}
}

func (d *driver) peerAdd(nid, eid string, peerIP net.IP, peerIPMask net.IPMask,
	peerMac net.HardwareAddr, vtep net.IP, updateDb, l2Miss, l3Miss, localPeer bool) {
	callerName := common.CallerName(1)
	d.peerOpCh <- &peerOperation{
		isAdd:      true,
		networkID:  nid,
		endpointID: eid,
		peerIP:     peerIP,
		peerIPMask: peerIPMask,
		peerMac:    peerMac,
		vtepIP:     vtep,
		updateDB:   updateDb,
		l2Miss:     l2Miss,
		l3Miss:     l3Miss,
		localPeer:  localPeer,
		callerName: callerName,
	}
}

func (d *driver) peerAddOp(nid, eid string, peerIP net.IP, peerIPMask net.IPMask,
	peerMac net.HardwareAddr, vtep net.IP, updateDb, l2Miss, l3Miss, updateOnlyDB bool) error {

	if err := validateID(nid, eid); err != nil {
		return err
	}

	if updateDb {
		d.peerDbAdd(nid, eid, peerIP, peerIPMask, peerMac, vtep, false)
		if updateOnlyDB {
			return nil
		}
	}

	n := d.network(nid)
	if n == nil {
		return nil
	}

	sbox := n.sandbox()
	if sbox == nil {
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

	if err := n.obtainVxlanID(s); err != nil {
		return fmt.Errorf("couldn't get vxlan id for %q: %v", s.subnetIP.String(), err)
	}

	if err := n.joinSubnetSandbox(s, false); err != nil {
		return fmt.Errorf("subnet sandbox join failed for %q: %v", s.subnetIP.String(), err)
	}

	if err := d.checkEncryption(nid, vtep, n.vxlanID(s), false, true); err != nil {
		logrus.Warn(err)
	}

	// Add neighbor entry for the peer IP
	if err := sbox.AddNeighbor(peerIP, peerMac, l3Miss, sbox.NeighborOptions().LinkName(s.vxlanName)); err != nil {
		return fmt.Errorf("could not add neighbor entry into the sandbox: %v", err)
	}

	// Add fdb entry to the bridge for the peer mac
	if err := sbox.AddNeighbor(vtep, peerMac, l2Miss, sbox.NeighborOptions().LinkName(s.vxlanName),
		sbox.NeighborOptions().Family(syscall.AF_BRIDGE)); err != nil {
		return fmt.Errorf("could not add fdb entry into the sandbox: %v", err)
	}

	return nil
}

func (d *driver) peerDelete(nid, eid string, peerIP net.IP, peerIPMask net.IPMask,
	peerMac net.HardwareAddr, vtep net.IP, updateDb bool) {
	callerName := common.CallerName(1)
	d.peerOpCh <- &peerOperation{
		isAdd:      false,
		networkID:  nid,
		endpointID: eid,
		peerIP:     peerIP,
		peerIPMask: peerIPMask,
		peerMac:    peerMac,
		vtepIP:     vtep,
		updateDB:   updateDb,
		callerName: callerName,
	}
}

func (d *driver) peerDeleteOp(nid, eid string, peerIP net.IP, peerIPMask net.IPMask,
	peerMac net.HardwareAddr, vtep net.IP, updateDb bool) error {

	if err := validateID(nid, eid); err != nil {
		return err
	}

	var pEntry peerEntry
	if updateDb {
		pEntry = d.peerDbDelete(nid, eid, peerIP, peerIPMask, peerMac, vtep)
	}

	n := d.network(nid)
	if n == nil {
		return nil
	}

	sbox := n.sandbox()
	if sbox == nil {
		return nil
	}

	// Delete fdb entry to the bridge for the peer mac only if the
	// entry existed in local peerdb. If it is a stale delete
	// request, still call DeleteNeighbor but only to cleanup any
	// leftover sandbox neighbor cache and not actually delete the
	// kernel state.
	if (eid == pEntry.eid && vtep.Equal(pEntry.vtep)) ||
		(eid != pEntry.eid && !vtep.Equal(pEntry.vtep)) {
		if err := sbox.DeleteNeighbor(vtep, peerMac,
			eid == pEntry.eid && vtep.Equal(pEntry.vtep)); err != nil {
			return fmt.Errorf("could not delete fdb entry into the sandbox: %v", err)
		}
	}

	// Delete neighbor entry for the peer IP
	if eid == pEntry.eid {
		if err := sbox.DeleteNeighbor(peerIP, peerMac, true); err != nil {
			return fmt.Errorf("could not delete neighbor entry into the sandbox: %v", err)
		}
	}

	if err := d.checkEncryption(nid, vtep, 0, false, false); err != nil {
		logrus.Warn(err)
	}

	return nil
}

func (d *driver) pushLocalDb() {
	d.peerDbWalk(func(nid string, pKey *peerKey, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			d.pushLocalEndpointEvent("join", nid, pEntry.eid)
		}
		return false
	})
}

func (d *driver) peerDBUpdateSelf() {
	d.peerDbWalk(func(nid string, pkey *peerKey, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			pEntry.vtep = net.ParseIP(d.advertiseAddress)
		}
		return false
	})
}
