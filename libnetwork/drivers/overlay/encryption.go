//go:build linux

package overlay

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"net"
	"strconv"
	"sync"
	"syscall"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/drivers/overlay/overlayutils"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

/*
Encrypted overlay networks use IPsec in transport mode to encrypt and
authenticate the VXLAN UDP datagrams. This driver implements a bespoke control
plane which negotiates the security parameters for each peer-to-peer tunnel.

IPsec Terminology

 - ESP: IPSec Encapsulating Security Payload
 - SPI: Security Parameter Index
 - ICV: Integrity Check Value
 - SA: Security Association https://en.wikipedia.org/wiki/IPsec#Security_association


Developer documentation for Linux IPsec is rather sparse online. The following
slide deck provides a decent overview.
https://libreswan.org/wiki/images/e/e0/Netdev-0x12-ipsec-flow.pdf

The Linux IPsec stack is part of XFRM, the netlink packet transformation
interface.
https://man7.org/linux/man-pages/man8/ip-xfrm.8.html
*/

const (
	// Value used to mark outgoing packets which should have our IPsec
	// processing applied. It is also used as a label to identify XFRM
	// states (Security Associations) and policies (Security Policies)
	// programmed by us so we know which ones we can clean up without
	// disrupting other VPN connections on the system.
	mark = 0xD0C4E3

	pktExpansion = 26 // SPI(4) + SeqN(4) + IV(8) + PadLength(1) + NextHeader(1) + ICV(8)
)

const (
	forward = iota + 1
	reverse
	bidir
)

// Mark value for matching packets which should have our IPsec security policy
// applied.
var spMark = netlink.XfrmMark{Value: mark, Mask: 0xffffffff}

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

// Security Parameter Indices for the IPsec flows between local node and a
// remote peer, which identify the Security Associations (XFRM states) to be
// applied when encrypting and decrypting packets.
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

func (d *driver) checkEncryption(nid string, rIP net.IP, isLocal, add bool) error {
	log.G(context.TODO()).Debugf("checkEncryption(%.7s, %v, %t)", nid, rIP, isLocal)

	n := d.network(nid)
	if n == nil || !n.secure {
		return nil
	}

	if len(d.keys) == 0 {
		return types.ForbiddenErrorf("encryption key is not present")
	}

	lIP := net.ParseIP(d.bindAddress)
	aIP := net.ParseIP(d.advertiseAddress)
	nodes := map[string]net.IP{}

	switch {
	case isLocal:
		if err := d.peerDbNetworkWalk(nid, func(pKey *peerKey, pEntry *peerEntry) bool {
			if !aIP.Equal(pEntry.vtep) {
				nodes[pEntry.vtep.String()] = pEntry.vtep
			}
			return false
		}); err != nil {
			log.G(context.TODO()).Warnf("Failed to retrieve list of participating nodes in overlay network %.5s: %v", nid, err)
		}
	default:
		if len(d.network(nid).endpoints) > 0 {
			nodes[rIP.String()] = rIP
		}
	}

	log.G(context.TODO()).Debugf("List of nodes: %s", nodes)

	if add {
		for _, rIP := range nodes {
			if err := setupEncryption(lIP, aIP, rIP, d.secMap, d.keys); err != nil {
				log.G(context.TODO()).Warnf("Failed to program network encryption between %s and %s: %v", lIP, rIP, err)
			}
		}
	} else {
		if len(nodes) == 0 {
			if err := removeEncryption(lIP, rIP, d.secMap); err != nil {
				log.G(context.TODO()).Warnf("Failed to remove network encryption between %s and %s: %v", lIP, rIP, err)
			}
		}
	}

	return nil
}

// setupEncryption programs the encryption parameters for secure communication
// between the local node and a remote node.
func setupEncryption(localIP, advIP, remoteIP net.IP, em *encrMap, keys []*key) error {
	log.G(context.TODO()).Debugf("Programming encryption between %s and %s", localIP, remoteIP)
	rIPs := remoteIP.String()

	indices := make([]*spi, 0, len(keys))

	for i, k := range keys {
		spis := &spi{buildSPI(advIP, remoteIP, k.tag), buildSPI(remoteIP, advIP, k.tag)}
		dir := reverse
		if i == 0 {
			dir = bidir
		}
		fSA, rSA, err := programSA(localIP, remoteIP, spis, k, dir, true)
		if err != nil {
			log.G(context.TODO()).Warn(err)
		}
		indices = append(indices, spis)
		if i != 0 {
			continue
		}
		err = programSP(fSA, rSA, true)
		if err != nil {
			log.G(context.TODO()).Warn(err)
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
			log.G(context.TODO()).Warn(err)
		}
		if i != 0 {
			continue
		}
		err = programSP(fSA, rSA, false)
		if err != nil {
			log.G(context.TODO()).Warn(err)
		}
	}
	return nil
}

func programMangle(vni uint32, add bool) error {
	var (
		m      = strconv.FormatUint(mark, 10)
		chain  = "OUTPUT"
		rule   = append(matchVXLAN(overlayutils.VXLANUDPPort(), vni), "-j", "MARK", "--set-mark", m)
		a      = iptables.Append
		action = "install"
	)

	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)

	if !add {
		a = iptables.Delete
		action = "remove"
	}

	if err := iptable.ProgramRule(iptables.Mangle, chain, a, rule); err != nil {
		return fmt.Errorf("could not %s mangle rule: %w", action, err)
	}

	return nil
}

func programInput(vni uint32, add bool) error {
	var (
		plainVxlan = matchVXLAN(overlayutils.VXLANUDPPort(), vni)
		chain      = "INPUT"
		msg        = "add"
	)

	rule := func(policy, jump string) []string {
		args := append([]string{"-m", "policy", "--dir", "in", "--pol", policy}, plainVxlan...)
		return append(args, "-j", jump)
	}

	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)

	if !add {
		msg = "remove"
	}

	action := func(a iptables.Action) iptables.Action {
		if !add {
			return iptables.Delete
		}
		return a
	}

	// Drop incoming VXLAN datagrams for the VNI which were received in cleartext.
	// Insert at the top of the chain so the packets are dropped even if an
	// administrator-configured rule exists which would otherwise unconditionally
	// accept incoming VXLAN traffic.
	if err := iptable.ProgramRule(iptables.Filter, chain, action(iptables.Insert), rule("none", "DROP")); err != nil {
		return fmt.Errorf("could not %s input drop rule: %w", msg, err)
	}

	return nil
}

func programSA(localIP, remoteIP net.IP, spi *spi, k *key, dir int, add bool) (fSA *netlink.XfrmState, rSA *netlink.XfrmState, err error) {
	var (
		action      = "Removing"
		xfrmProgram = ns.NlHandle().XfrmStateDel
	)

	if add {
		action = "Adding"
		xfrmProgram = ns.NlHandle().XfrmStateAdd
	}

	if dir&reverse > 0 {
		rSA = &netlink.XfrmState{
			Src:   remoteIP,
			Dst:   localIP,
			Proto: netlink.XFRM_PROTO_ESP,
			Spi:   spi.reverse,
			Mode:  netlink.XFRM_MODE_TRANSPORT,
			Reqid: mark,
		}
		if add {
			rSA.Aead = buildAeadAlgo(k, spi.reverse)
		}

		exists, err := saExists(rSA)
		if err != nil {
			exists = !add
		}

		if add != exists {
			log.G(context.TODO()).Debugf("%s: rSA{%s}", action, rSA)
			if err := xfrmProgram(rSA); err != nil {
				log.G(context.TODO()).Warnf("Failed %s rSA{%s}: %v", action, rSA, err)
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
			Reqid: mark,
		}
		if add {
			fSA.Aead = buildAeadAlgo(k, spi.forward)
		}

		exists, err := saExists(fSA)
		if err != nil {
			exists = !add
		}

		if add != exists {
			log.G(context.TODO()).Debugf("%s fSA{%s}", action, fSA)
			if err := xfrmProgram(fSA); err != nil {
				log.G(context.TODO()).Warnf("Failed %s fSA{%s}: %v.", action, fSA, err)
			}
		}
	}

	return
}

// getMinimalIP returns the address in its shortest form
// If ip contains an IPv4-mapped IPv6 address, the 4-octet form of the IPv4 address will be returned.
// Otherwise ip is returned unchanged.
func getMinimalIP(ip net.IP) net.IP {
	if ip != nil && ip.To4() != nil {
		return ip.To4()
	}
	return ip
}

func programSP(fSA *netlink.XfrmState, rSA *netlink.XfrmState, add bool) error {
	action := "Removing"
	xfrmProgram := ns.NlHandle().XfrmPolicyDel
	if add {
		action = "Adding"
		xfrmProgram = ns.NlHandle().XfrmPolicyAdd
	}

	// Create a congruent cidr
	s := getMinimalIP(fSA.Src)
	d := getMinimalIP(fSA.Dst)
	fullMask := net.CIDRMask(8*len(s), 8*len(s))

	fPol := &netlink.XfrmPolicy{
		Src:     &net.IPNet{IP: s, Mask: fullMask},
		Dst:     &net.IPNet{IP: d, Mask: fullMask},
		Dir:     netlink.XFRM_DIR_OUT,
		Proto:   syscall.IPPROTO_UDP,
		DstPort: int(overlayutils.VXLANUDPPort()),
		Mark:    &spMark,
		Tmpls: []netlink.XfrmPolicyTmpl{
			{
				Src:   fSA.Src,
				Dst:   fSA.Dst,
				Proto: netlink.XFRM_PROTO_ESP,
				Mode:  netlink.XFRM_MODE_TRANSPORT,
				Spi:   fSA.Spi,
				Reqid: mark,
			},
		},
	}

	exists, err := spExists(fPol)
	if err != nil {
		exists = !add
	}

	if add != exists {
		log.G(context.TODO()).Debugf("%s fSP{%s}", action, fPol)
		if err := xfrmProgram(fPol); err != nil {
			log.G(context.TODO()).Warnf("%s fSP{%s}: %v", action, fPol, err)
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
		log.G(context.TODO()).Warn(err)
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
		log.G(context.TODO()).Warn(err)
		return false, err
	}
}

func buildSPI(src, dst net.IP, st uint32) int {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, st)
	h := fnv.New32a()
	h.Write(src)
	h.Write(b)
	h.Write(dst)
	return int(binary.BigEndian.Uint32(h.Sum(nil)))
}

func buildAeadAlgo(k *key, s int) *netlink.XfrmStateAlgo {
	salt := make([]byte, 4)
	binary.BigEndian.PutUint32(salt, uint32(s))
	return &netlink.XfrmStateAlgo{
		Name:   "rfc4106(gcm(aes))",
		Key:    append(k.value, salt...),
		ICVLen: 64,
	}
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
	// Remove any stale policy, state
	clearEncryptionStates()
	// Accept the encryption keys and clear any stale encryption map
	d.Lock()
	d.keys = keys
	d.secMap = &encrMap{nodes: map[string][]*spi{}}
	d.Unlock()
	log.G(context.TODO()).Debugf("Initial encryption keys: %v", keys)
	return nil
}

// updateKeys allows to add a new key and/or change the primary key and/or prune an existing key
// The primary key is the key used in transmission and will go in first position in the list.
func (d *driver) updateKeys(newKey, primary, pruneKey *key) error {
	log.G(context.TODO()).Debugf("Updating Keys. New: %v, Primary: %v, Pruned: %v", newKey, primary, pruneKey)

	log.G(context.TODO()).Debugf("Current: %v", d.keys)

	var (
		newIdx = -1
		priIdx = -1
		delIdx = -1
		lIP    = net.ParseIP(d.bindAddress)
		aIP    = net.ParseIP(d.advertiseAddress)
	)

	d.Lock()
	defer d.Unlock()

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

	if (newKey != nil && newIdx == -1) ||
		(primary != nil && priIdx == -1) ||
		(pruneKey != nil && delIdx == -1) {
		return types.InvalidParameterErrorf("cannot find proper key indices while processing key update:"+
			"(newIdx,priIdx,delIdx):(%d, %d, %d)", newIdx, priIdx, delIdx)
	}

	if priIdx != -1 && priIdx == delIdx {
		return types.InvalidParameterErrorf("attempting to both make a key (index %d) primary and delete it", priIdx)
	}

	d.secMapWalk(func(rIPs string, spis []*spi) ([]*spi, bool) {
		rIP := net.ParseIP(rIPs)
		return updateNodeKey(lIP, aIP, rIP, spis, d.keys, newIdx, priIdx, delIdx), false
	})

	// swap primary
	if priIdx != -1 {
		d.keys[0], d.keys[priIdx] = d.keys[priIdx], d.keys[0]
	}
	// prune
	if delIdx != -1 {
		if delIdx == 0 {
			delIdx = priIdx
		}
		d.keys = append(d.keys[:delIdx], d.keys[delIdx+1:]...)
	}

	log.G(context.TODO()).Debugf("Updated: %v", d.keys)

	return nil
}

/********************************************************
 * Steady state: rSA0, rSA1, rSA2, fSA1, fSP1
 * Rotation --> -rSA0, +rSA3, +fSA2, +fSP2/-fSP1, -fSA1
 * Steady state: rSA1, rSA2, rSA3, fSA2, fSP2
 *********************************************************/

// Spis and keys are sorted in such away the one in position 0 is the primary
func updateNodeKey(lIP, aIP, rIP net.IP, idxs []*spi, curKeys []*key, newIdx, priIdx, delIdx int) []*spi {
	log.G(context.TODO()).Debugf("Updating keys for node: %s (%d,%d,%d)", rIP, newIdx, priIdx, delIdx)

	spis := idxs
	log.G(context.TODO()).Debugf("Current: %v", spis)

	// add new
	if newIdx != -1 {
		spis = append(spis, &spi{
			forward: buildSPI(aIP, rIP, curKeys[newIdx].tag),
			reverse: buildSPI(rIP, aIP, curKeys[newIdx].tag),
		})
	}

	if delIdx != -1 {
		// -rSA0
		programSA(lIP, rIP, spis[delIdx], nil, reverse, false)
	}

	if newIdx > -1 {
		// +rSA2
		programSA(lIP, rIP, spis[newIdx], curKeys[newIdx], reverse, true)
	}

	if priIdx > 0 {
		// +fSA2
		fSA2, _, _ := programSA(lIP, rIP, spis[priIdx], curKeys[priIdx], forward, true)

		// +fSP2, -fSP1
		s := getMinimalIP(fSA2.Src)
		d := getMinimalIP(fSA2.Dst)
		fullMask := net.CIDRMask(8*len(s), 8*len(s))

		fSP1 := &netlink.XfrmPolicy{
			Src:     &net.IPNet{IP: s, Mask: fullMask},
			Dst:     &net.IPNet{IP: d, Mask: fullMask},
			Dir:     netlink.XFRM_DIR_OUT,
			Proto:   syscall.IPPROTO_UDP,
			DstPort: int(overlayutils.VXLANUDPPort()),
			Mark:    &spMark,
			Tmpls: []netlink.XfrmPolicyTmpl{
				{
					Src:   fSA2.Src,
					Dst:   fSA2.Dst,
					Proto: netlink.XFRM_PROTO_ESP,
					Mode:  netlink.XFRM_MODE_TRANSPORT,
					Spi:   fSA2.Spi,
					Reqid: mark,
				},
			},
		}
		log.G(context.TODO()).Debugf("Updating fSP{%s}", fSP1)
		if err := ns.NlHandle().XfrmPolicyUpdate(fSP1); err != nil {
			log.G(context.TODO()).Warnf("Failed to update fSP{%s}: %v", fSP1, err)
		}

		// -fSA1
		programSA(lIP, rIP, spis[0], nil, forward, false)
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

	log.G(context.TODO()).Debugf("Updated: %v", spis)

	return spis
}

func (n *network) maxMTU() int {
	mtu := 1500
	if n.mtu != 0 {
		mtu = n.mtu
	}
	mtu -= vxlanEncap
	if n.secure {
		// In case of encryption account for the
		// esp packet expansion and padding
		mtu -= pktExpansion
		mtu -= (mtu % 4)
	}
	return mtu
}

func clearEncryptionStates() {
	nlh := ns.NlHandle()
	spList, err := nlh.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to retrieve SP list for cleanup: %v", err)
	}
	saList, err := nlh.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to retrieve SA list for cleanup: %v", err)
	}
	for _, sp := range spList {
		sp := sp
		if sp.Mark != nil && sp.Mark.Value == spMark.Value {
			if err := nlh.XfrmPolicyDel(&sp); err != nil {
				log.G(context.TODO()).Warnf("Failed to delete stale SP %s: %v", sp, err)
				continue
			}
			log.G(context.TODO()).Debugf("Removed stale SP: %s", sp)
		}
	}
	for _, sa := range saList {
		sa := sa
		if sa.Reqid == mark {
			if err := nlh.XfrmStateDel(&sa); err != nil {
				log.G(context.TODO()).Warnf("Failed to delete stale SA %s: %v", sa, err)
				continue
			}
			log.G(context.TODO()).Debugf("Removed stale SA: %s", sa)
		}
	}
}
