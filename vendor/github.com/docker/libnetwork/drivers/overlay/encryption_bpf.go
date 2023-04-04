package overlay

import (
	"strconv"
)

// matchVXLANWithBPF returns an iptables rule fragment which matches VXLAN
// datagrams with the given destination port and VXLAN Network ID utilizing the
// xt_bpf netfilter kernel module. The returned slice's backing array is
// guaranteed not to alias any other slice's.
func matchVXLANWithBPF(port, vni uint32) []string {
	dport := strconv.FormatUint(uint64(port), 10)
	vniMatch := marshalXTBPF(vniMatchBPF(vni))

	// https://ipset.netfilter.org/iptables-extensions.man.html#lbAH
	return []string{"-p", "udp", "--dport", dport, "-m", "bpf", "--bytecode", vniMatch}
}
