package overlay

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/bpf"
)

// vniMatchBPF returns a BPF program suitable for passing to the iptables and
// ip6tables bpf match which matches on the VXAN Network ID of encapsulated
// packets. The program assumes that it will be used in a rule which only
// matches UDP datagrams.
func vniMatchBPF(vni uint32) []bpf.RawInstruction {
	asm, err := bpf.Assemble([]bpf.Instruction{
		// Load offset of UDP payload into X.
		bpf.LoadExtension{Num: bpf.ExtPayloadOffset}, // ld poff
		bpf.TAX{}, // tax

		bpf.LoadIndirect{Off: 4, Size: 4},                      // ld [x + 4] ; Load VXLAN ID into top 24 bits of A
		bpf.ALUOpConstant{Op: bpf.ALUOpShiftRight, Val: 8},     // rsh #8     ; A >>= 8
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: vni, SkipTrue: 1}, // jeq $vni, match
		bpf.RetConstant{Val: 0},                                // ret #0
		bpf.RetConstant{Val: ^uint32(0)},                       // match: ret #-1
	})
	// bpf.Assemble() only errors if an instruction is invalid. As the only variable
	// part of the program is an instruction value for which the entire range is
	// valid, whether the program can be successfully assembled is independent of
	// the input. Given that the only recourse is to fix this function and
	// recompile, there's little value in bubbling the error up to the caller.
	if err != nil {
		panic(err)
	}
	return asm
}

// marshalXTBPF marshals a BPF program into the "decimal" byte code format
// which is suitable for passing to the [iptables bpf match].
//
//	iptables -m bpf --bytecode
//
// [iptables bpf match]: https://ipset.netfilter.org/iptables-extensions.man.html#lbAH
func marshalXTBPF(prog []bpf.RawInstruction) string { //nolint:unused
	var b strings.Builder
	fmt.Fprintf(&b, "%d", len(prog))
	for _, ins := range prog {
		fmt.Fprintf(&b, ",%d %d %d %d", ins.Op, ins.Jt, ins.Jf, ins.K)
	}
	return b.String()
}

// matchVXLAN returns an iptables rule fragment which matches VXLAN datagrams
// with the given destination port and VXLAN Network ID utilizing the xt_bpf
// netfilter kernel module. The returned slice's backing array is guaranteed not
// to alias any other slice's.
func matchVXLAN(port, vni uint32) []string {
	dport := strconv.FormatUint(uint64(port), 10)
	vniMatch := marshalXTBPF(vniMatchBPF(vni))

	// https://ipset.netfilter.org/iptables-extensions.man.html#lbAH
	return []string{"-p", "udp", "--dport", dport, "-m", "bpf", "--bytecode", vniMatch}
}
