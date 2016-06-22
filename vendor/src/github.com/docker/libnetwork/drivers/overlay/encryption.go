package overlay

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/ns"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	"strconv"
)

const (
	mark    = uint32(0xD0C4E3)
	timeout = 30
)

const (
	forward = iota + 1
	reverse
	bidir
)

type key struct {
	value []byte
	tag   uint32
}

func (k *key) String() string {
	if k != nil {
		return fmt.Sprintf("(key: %s, tag: 0x%x)", hex.EncodeToString(k.value)[0:5], k.tag)
	}
	return ""
}

type spi struct {
	forward int
	reverse int
}

func (s *spi) String() string {
	return fmt.Sprintf("SPI(FWD: 0x%x, REV: 0x%x)", uint32(s.forward), uint32(s.reverse))
}

type encrMap struct {
	nodes map[string][]*spi
	sync.Mutex
}

func (e *encrMap) String() string {
	e.Lock()
	defer e.Unlock()
	b := new(bytes.Buffer)
	for k, v := range e.nodes {
		b.WriteString("\n")
		b.WriteString(k)
		b.WriteString(":")
		b.WriteString("[")
		for _, s := range v {
			b.WriteString(s.String())
			b.WriteString(",")
		}
		b.WriteString("]")

	}
	return b.String()
}

func (d *driver) checkEncryption(nid string, rIP net.IP, vxlanID uint32, isLocal, add bool) error {
	log.Debugf("checkEncryption(%s, %v, %d, %t)", nid[0:7], rIP, vxlanID, isLocal)

	n := d.network(nid)
	if n == nil || !n.secure {
		return nil
	}

	if len(d.keys) == 0 {
		return types.ForbiddenErrorf("encryption key is not present")
	}

	lIP := types.GetMinimalIP(net.ParseIP(d.bindAddress))
	nodes := map[string]net.IP{}

	switch {
	case isLocal:
		if err := d.peerDbNetworkWalk(nid, func(pKey *peerKey, pEntry *peerEntry) bool {
			if !lIP.Equal(pEntry.vtep) {
				nodes[pEntry.vtep.String()] = types.GetMinimalIP(pEntry.vtep)
			}
			return false
		}); err != nil {
			log.Warnf("Failed to retrieve list of participating nodes in overlay network %s: %v", nid[0:5], err)
		}
	default:
		if len(d.network(nid).endpoints) > 0 {
			nodes[rIP.String()] = types.GetMinimalIP(rIP)
		}
	}

	log.Debugf("List of nodes: %s", nodes)

	if add {
		for _, rIP := range nodes {
			if err := setupEncryption(lIP, rIP, vxlanID, d.secMap, d.keys); err != nil {
				log.Warnf("Failed to program network encryption between %s and %s: %v", lIP, rIP, err)
			}
		}
	} else {
		if len(nodes) == 0 {
			if err := removeEncryption(lIP, rIP, d.secMap); err != nil {
				log.Warnf("Failed to remove network encryption between %s and %s: %v", lIP, rIP, err)
			}
		}
	}

	return nil
}

func setupEncryption(localIP, remoteIP net.IP, vni uint32, em *encrMap, keys []*key) error {
	log.Debugf("Programming encryption for vxlan %d between %s and %s", vni, localIP, remoteIP)
	rIPs := remoteIP.String()

	indices := make([]*spi, 0, len(keys))

	err := programMangle(vni, true)
	if err != nil {
		log.Warn(err)
	}

	for i, k := range keys {
		spis := &spi{buildSPI(localIP, remoteIP, k.tag), buildSPI(remoteIP, localIP, k.tag)}
		dir := reverse
		if i == 0 {
			dir = bidir
		}
		fSA, rSA, err := programSA(localIP, remoteIP, spis, k, dir, true)
		if err != nil {
			log.Warn(err)
		}
		indices = append(indices, spis)
		if i != 0 {
			continue
		}
		err = programSP(fSA, rSA, true)
		if err != nil {
			log.Warn(err)
		}
	}

	em.Lock()
	em.nodes[rIPs] = indices
	em.Unlock()

	return nil
}

func removeEncryption(localIP, remoteIP net.IP, em *encrMap) error {
	em.Lock()
	indices, ok := em.nodes[remoteIP.String()]
	em.Unlock()
	if !ok {
		return nil
	}
	for i, idxs := range indices {
		dir := reverse
		if i == 0 {
			dir = bidir
		}
		fSA, rSA, err := programSA(localIP, remoteIP, idxs, nil, dir, false)
		if err != nil {
			log.Warn(err)
		}
		if i != 0 {
			continue
		}
		err = programSP(fSA, rSA, false)
		if err != nil {
			log.Warn(err)
		}
	}
	return nil
}

func programMangle(vni uint32, add bool) (err error) {
	var (
		p      = strconv.FormatUint(uint64(vxlanPort), 10)
		c      = fmt.Sprintf("0>>22&0x3C@12&0xFFFFFF00=%d", int(vni)<<8)
		m      = strconv.FormatUint(uint64(mark), 10)
		chain  = "OUTPUT"
		rule   = []string{"-p", "udp", "--dport", p, "-m", "u32", "--u32", c, "-j", "MARK", "--set-mark", m}
		a      = "-A"
		action = "install"
	)

	if add == iptables.Exists(iptables.Mangle, chain, rule...) {
		return
	}

	if !add {
		a = "-D"
		action = "remove"
	}

	if err = iptables.RawCombinedOutput(append([]string{"-t", string(iptables.Mangle), a, chain}, rule...)...); err != nil {
		log.Warnf("could not %s mangle rule: %v", action, err)
	}

	return
}

func programSA(localIP, remoteIP net.IP, spi *spi, k *key, dir int, add bool) (fSA *netlink.XfrmState, rSA *netlink.XfrmState, err error) {
	var (
		crypt       *netlink.XfrmStateAlgo
		action      = "Removing"
		xfrmProgram = ns.NlHandle().XfrmStateDel
	)

	if add {
		action = "Adding"
		xfrmProgram = ns.NlHandle().XfrmStateAdd
		crypt = &netlink.XfrmStateAlgo{Name: "cbc(aes)", Key: k.value}
	}

	if dir&reverse > 0 {
		rSA = &netlink.XfrmState{
			Src:   remoteIP,
			Dst:   localIP,
			Proto: netlink.XFRM_PROTO_ESP,
			Spi:   spi.reverse,
			Mode:  netlink.XFRM_MODE_TRANSPORT,
		}
		if add {
			rSA.Crypt = crypt
		}

		exists, err := saExists(rSA)
		if err != nil {
			exists = !add
		}

		if add != exists {
			log.Debugf("%s: rSA{%s}", action, rSA)
			if err := xfrmProgram(rSA); err != nil {
				log.Warnf("Failed %s rSA{%s}: %v", action, rSA, err)
			}
		}
	}

	if dir&forward > 0 {
		fSA = &netlink.XfrmState{
			Src:   localIP,
			Dst:   remoteIP,
			Proto: netlink.XFRM_PROTO_ESP,
			Spi:   spi.forward,
			Mode:  netlink.XFRM_MODE_TRANSPORT,
		}
		if add {
			fSA.Crypt = crypt
		}

		exists, err := saExists(fSA)
		if err != nil {
			exists = !add
		}

		if add != exists {
			log.Debugf("%s fSA{%s}", action, fSA)
			if err := xfrmProgram(fSA); err != nil {
				log.Warnf("Failed %s fSA{%s}: %v.", action, fSA, err)
			}
		}
	}

	return
}

func programSP(fSA *netlink.XfrmState, rSA *netlink.XfrmState, add bool) error {
	action := "Removing"
	xfrmProgram := ns.NlHandle().XfrmPolicyDel
	if add {
		action = "Adding"
		xfrmProgram = ns.NlHandle().XfrmPolicyAdd
	}

	fullMask := net.CIDRMask(8*len(fSA.Src), 8*len(fSA.Src))

	fPol := &netlink.XfrmPolicy{
		Src:     &net.IPNet{IP: fSA.Src, Mask: fullMask},
		Dst:     &net.IPNet{IP: fSA.Dst, Mask: fullMask},
		Dir:     netlink.XFRM_DIR_OUT,
		Proto:   17,
		DstPort: 4789,
		Mark: &netlink.XfrmMark{
			Value: mark,
		},
		Tmpls: []netlink.XfrmPolicyTmpl{
			{
				Src:   fSA.Src,
				Dst:   fSA.Dst,
				Proto: netlink.XFRM_PROTO_ESP,
				Mode:  netlink.XFRM_MODE_TRANSPORT,
				Spi:   fSA.Spi,
			},
		},
	}

	exists, err := spExists(fPol)
	if err != nil {
		exists = !add
	}

	if add != exists {
		log.Debugf("%s fSP{%s}", action, fPol)
		if err := xfrmProgram(fPol); err != nil {
			log.Warnf("%s fSP{%s}: %v", action, fPol, err)
		}
	}

	return nil
}

func saExists(sa *netlink.XfrmState) (bool, error) {
	_, err := ns.NlHandle().XfrmStateGet(sa)
	switch err {
	case nil:
		return true, nil
	case syscall.ESRCH:
		return false, nil
	default:
		err = fmt.Errorf("Error while checking for SA existence: %v", err)
		log.Debug(err)
		return false, err
	}
}

func spExists(sp *netlink.XfrmPolicy) (bool, error) {
	_, err := ns.NlHandle().XfrmPolicyGet(sp)
	switch err {
	case nil:
		return true, nil
	case syscall.ENOENT:
		return false, nil
	default:
		err = fmt.Errorf("Error while checking for SP existence: %v", err)
		log.Debug(err)
		return false, err
	}
}

func buildSPI(src, dst net.IP, st uint32) int {
	spi := int(st)
	f := src[len(src)-4:]
	t := dst[len(dst)-4:]
	for i := 0; i < 4; i++ {
		spi = spi ^ (int(f[i])^int(t[3-i]))<<uint32(8*i)
	}
	return spi
}

func (d *driver) secMapWalk(f func(string, []*spi) ([]*spi, bool)) error {
	d.secMap.Lock()
	for node, indices := range d.secMap.nodes {
		idxs, stop := f(node, indices)
		if idxs != nil {
			d.secMap.nodes[node] = idxs
		}
		if stop {
			break
		}
	}
	d.secMap.Unlock()
	return nil
}

func (d *driver) setKeys(keys []*key) error {
	if d.keys != nil {
		return types.ForbiddenErrorf("initial keys are already present")
	}
	d.keys = keys
	log.Debugf("Initial encryption keys: %v", d.keys)
	return nil
}

// updateKeys allows to add a new key and/or change the primary key and/or prune an existing key
// The primary key is the key used in transmission and will go in first position in the list.
func (d *driver) updateKeys(newKey, primary, pruneKey *key) error {
	log.Debugf("Updating Keys. New: %v, Primary: %v, Pruned: %v", newKey, primary, pruneKey)

	log.Debugf("Current: %v", d.keys)

	var (
		newIdx = -1
		priIdx = -1
		delIdx = -1
		lIP    = types.GetMinimalIP(net.ParseIP(d.bindAddress))
	)

	d.Lock()
	// add new
	if newKey != nil {
		d.keys = append(d.keys, newKey)
		newIdx += len(d.keys)
	}
	for i, k := range d.keys {
		if primary != nil && k.tag == primary.tag {
			priIdx = i
		}
		if pruneKey != nil && k.tag == pruneKey.tag {
			delIdx = i
		}
	}
	d.Unlock()

	if (newKey != nil && newIdx == -1) ||
		(primary != nil && priIdx == -1) ||
		(pruneKey != nil && delIdx == -1) {
		err := types.BadRequestErrorf("cannot find proper key indices while processing key update:"+
			"(newIdx,priIdx,delIdx):(%d, %d, %d)", newIdx, priIdx, delIdx)
		log.Warn(err)
		return err
	}

	d.secMapWalk(func(rIPs string, spis []*spi) ([]*spi, bool) {
		rIP := types.GetMinimalIP(net.ParseIP(rIPs))
		return updateNodeKey(lIP, rIP, spis, d.keys, newIdx, priIdx, delIdx), false
	})

	d.Lock()
	// swap primary
	if priIdx != -1 {
		swp := d.keys[0]
		d.keys[0] = d.keys[priIdx]
		d.keys[priIdx] = swp
	}
	// prune
	if delIdx != -1 {
		if delIdx == 0 {
			delIdx = priIdx
		}
		d.keys = append(d.keys[:delIdx], d.keys[delIdx+1:]...)
	}
	d.Unlock()

	log.Debugf("Updated: %v", d.keys)

	return nil
}

/********************************************************
 * Steady state: rSA0, rSA1, fSA0, fSP0
 * Rotation --> %rSA0, +rSA2, +fSA1, +fSP1/-fSP0, -fSA0,
 * Half state:   rSA0, rSA1, rSA2, fSA1, fSP1
 * Steady state: rSA1, rSA2, fSA1, fSP1
 *********************************************************/

// Spis and keys are sorted in such away the one in position 0 is the primary
func updateNodeKey(lIP, rIP net.IP, idxs []*spi, curKeys []*key, newIdx, priIdx, delIdx int) []*spi {
	log.Debugf("Updating keys for node: %s (%d,%d,%d)", rIP, newIdx, priIdx, delIdx)

	spis := idxs
	log.Debugf("Current: %v", spis)

	// add new
	if newIdx != -1 {
		spis = append(spis, &spi{
			forward: buildSPI(lIP, rIP, curKeys[newIdx].tag),
			reverse: buildSPI(rIP, lIP, curKeys[newIdx].tag),
		})
	}

	if delIdx != -1 {
		// %rSA0
		rSA0 := &netlink.XfrmState{
			Src:    rIP,
			Dst:    lIP,
			Proto:  netlink.XFRM_PROTO_ESP,
			Spi:    spis[delIdx].reverse,
			Mode:   netlink.XFRM_MODE_TRANSPORT,
			Crypt:  &netlink.XfrmStateAlgo{Name: "cbc(aes)", Key: curKeys[delIdx].value},
			Limits: netlink.XfrmStateLimits{TimeSoft: timeout},
		}
		log.Debugf("Updating rSA0{%s}", rSA0)
		if err := ns.NlHandle().XfrmStateUpdate(rSA0); err != nil {
			log.Warnf("Failed to update rSA0{%s}: %v", rSA0, err)
		}
	}

	if newIdx > -1 {
		// +RSA2
		programSA(lIP, rIP, spis[newIdx], curKeys[newIdx], reverse, true)
	}

	if priIdx > 0 {
		// +fSA1
		fSA1, _, _ := programSA(lIP, rIP, spis[priIdx], curKeys[priIdx], forward, true)

		// +fSP1, -fSP0
		fullMask := net.CIDRMask(8*len(fSA1.Src), 8*len(fSA1.Src))
		fSP1 := &netlink.XfrmPolicy{
			Src:     &net.IPNet{IP: fSA1.Src, Mask: fullMask},
			Dst:     &net.IPNet{IP: fSA1.Dst, Mask: fullMask},
			Dir:     netlink.XFRM_DIR_OUT,
			Proto:   17,
			DstPort: 4789,
			Mark: &netlink.XfrmMark{
				Value: mark,
			},
			Tmpls: []netlink.XfrmPolicyTmpl{
				{
					Src:   fSA1.Src,
					Dst:   fSA1.Dst,
					Proto: netlink.XFRM_PROTO_ESP,
					Mode:  netlink.XFRM_MODE_TRANSPORT,
					Spi:   fSA1.Spi,
				},
			},
		}
		log.Debugf("Updating fSP{%s}", fSP1)
		if err := ns.NlHandle().XfrmPolicyUpdate(fSP1); err != nil {
			log.Warnf("Failed to update fSP{%s}: %v", fSP1, err)
		}

		// -fSA0
		fSA0 := &netlink.XfrmState{
			Src:    lIP,
			Dst:    rIP,
			Proto:  netlink.XFRM_PROTO_ESP,
			Spi:    spis[0].forward,
			Mode:   netlink.XFRM_MODE_TRANSPORT,
			Crypt:  &netlink.XfrmStateAlgo{Name: "cbc(aes)", Key: curKeys[0].value},
			Limits: netlink.XfrmStateLimits{TimeHard: timeout},
		}
		log.Debugf("Removing fSA0{%s}", fSA0)
		if err := ns.NlHandle().XfrmStateUpdate(fSA0); err != nil {
			log.Warnf("Failed to remove fSA0{%s}: %v", fSA0, err)
		}
	}

	// swap
	if priIdx > 0 {
		swp := spis[0]
		spis[0] = spis[priIdx]
		spis[priIdx] = swp
	}
	// prune
	if delIdx != -1 {
		if delIdx == 0 {
			delIdx = priIdx
		}
		spis = append(spis[:delIdx], spis[delIdx+1:]...)
	}

	log.Debugf("Updated: %v", spis)

	return spis
}
