package overlay

import (
	"fmt"
	"strings"

	"golang.org/x/net/bpf"
)

// vniMatchBPF returns a BPF program suitable for passing to the iptables bpf
// match which matches on the VXAN Network ID of encapsulated packets. The
// program assumes that it will be used in a rule which only matches UDP
// datagrams.
func vniMatchBPF(vni uint32) []bpf.RawInstruction {
	asm, err := bpf.Assemble([]bpf.Instruction{
		bpf.LoadMemShift{Off: 0},                                    // ldx 4*([0] & 0xf) ; Load length of IPv4 header into X
		bpf.LoadIndirect{Off: 12, Size: 4},                          // ld [x + 12]       ; Load VXLAN ID (UDP header + 4 bytes) into A
		bpf.ALUOpConstant{Op: bpf.ALUOpAnd, Val: 0xffffff00},        // and #0xffffff00   ; VXLAN ID is in top 24 bits
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: vni << 8, SkipTrue: 1}, // jeq ($vni << 8), match
		bpf.RetConstant{Val: 0},                                     // ret #0
		bpf.RetConstant{Val: ^uint32(0)},                            // match: ret #-1
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
