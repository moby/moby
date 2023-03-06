package overlay

import (
	"fmt"
	"strconv"
)

// matchVXLAN returns an iptables rule fragment which matches VXLAN datagrams
// with the given destination port and VXLAN Network ID utilizing the xt_u32
// netfilter kernel module. The returned slice's backing array is guaranteed not
// to alias any other slice's.
func matchVXLAN(port, vni uint32) []string {
	dport := strconv.FormatUint(uint64(port), 10)

	// The u32 expression language is documented in iptables-extensions(8).
	// https://ipset.netfilter.org/iptables-extensions.man.html#lbCK
	//
	// 0>>22&0x3C                ; Compute number of octets in IPv4 header
	//           @               ; Make this the new offset into the packet
	//                           ; (jump to start of UDP header)
	//            12&0xFFFFFF00  ; Read 32-bit value at offset 12 and mask off the bottom octet
	//                         = ; Test whether the value is equal to a constant
	//
	// A UDP header is eight octets long so offset 12 from the start of the
	// UDP header is four octets into the payload: the VNI field of the
	// VXLAN header.
	vniMatch := fmt.Sprintf("0>>22&0x3C@12&0xFFFFFF00=%d", int(vni)<<8)

	return []string{"-p", "udp", "--dport", dport, "-m", "u32", "--u32", vniMatch}
}
