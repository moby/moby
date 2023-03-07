//go:build libnetwork_overlay_bpf
// +build libnetwork_overlay_bpf

package overlay

import (
	"strconv"
)

// matchVXLAN returns an iptables rule fragment which matches VXLAN datagrams
// with the given destination port and VXLAN Network ID utilizing the xt_u32
// netfilter kernel module. The returned slice's backing array is guaranteed not
// to alias any other slice's.
func matchVXLAN(port, vni uint32) []string {
	dport := strconv.FormatUint(uint64(port), 10)
	vniMatch := marshalXTBPF(vniMatchBPF(vni))

	// https://ipset.netfilter.org/iptables-extensions.man.html#lbAH
	return []string{"-p", "udp", "--dport", dport, "-m", "bpf", "--bytecode", vniMatch}
}
