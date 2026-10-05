package features

import (
	"errors"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/sys"
)

// HaveLargeInstructions probes the running kernel if more than 4096 instructions
// per program are supported.
//
// Upstream commit c04c0d2b968a ("bpf: increase complexity limit and maximum program size").
//
// See the package documentation for the meaning of the error return value.
func HaveLargeInstructions() error {
	return haveLargeInstructions()
}

var haveLargeInstructions = internal.NewFeatureTest(">4096 instructions", func() error {
	const maxInsns = 4096

	insns := make(asm.Instructions, maxInsns, maxInsns+1)
	for i := range insns {
		insns[i] = asm.Mov.Imm(asm.R0, 1)
	}
	insns = append(insns, asm.Return())

	return probeProgram(&ebpf.ProgramSpec{
		Type:         ebpf.SocketFilter,
		Instructions: insns,
	})
}, "5.2")

// HaveBoundedLoops probes the running kernel if bounded loops are supported.
//
// Upstream commit 2589726d12a1 ("bpf: introduce bounded loops").
//
// See the package documentation for the meaning of the error return value.
func HaveBoundedLoops() error {
	return haveBoundedLoops()
}

var haveBoundedLoops = internal.NewFeatureTest("bounded loops", func() error {
	return probeProgram(&ebpf.ProgramSpec{
		Type: ebpf.SocketFilter,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 10),
			asm.Sub.Imm(asm.R0, 1).WithSymbol("loop"),
			asm.JNE.Imm(asm.R0, 0, "loop"),
			asm.Return(),
		},
	})
}, "5.3")

// HaveV2ISA probes the running kernel if instructions of the v2 ISA are supported.
//
// Upstream commit 92b31a9af73b ("bpf: add BPF_J{LT,LE,SLT,SLE} instructions").
//
// See the package documentation for the meaning of the error return value.
func HaveV2ISA() error {
	return haveV2ISA()
}

var haveV2ISA = internal.NewFeatureTest("v2 ISA", func() error {
	err := probeProgram(&ebpf.ProgramSpec{
		Type: ebpf.SocketFilter,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.JLT.Imm(asm.R0, 0, "exit"),
			asm.Mov.Imm(asm.R0, 1),
			asm.Return().WithSymbol("exit"),
		},
	})
	// This sometimes bubbles up from the JIT on aarch64.
	if errors.Is(err, sys.ENOTSUPP) {
		return ebpf.ErrNotSupported
	}
	return err
}, "4.14")

// HaveV3ISA probes the running kernel if instructions of the v3 ISA are supported.
//
// Upstream commit 092ed0968bb6 ("bpf: verifier support JMP32").
//
// See the package documentation for the meaning of the error return value.
func HaveV3ISA() error {
	return haveV3ISA()
}

var haveV3ISA = internal.NewFeatureTest("v3 ISA", func() error {
	err := probeProgram(&ebpf.ProgramSpec{
		Type: ebpf.SocketFilter,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.JLT.Imm32(asm.R0, 0, "exit"),
			asm.Mov.Imm(asm.R0, 1),
			asm.Return().WithSymbol("exit"),
		},
	})
	// This sometimes bubbles up from the JIT on aarch64.
	if errors.Is(err, sys.ENOTSUPP) {
		return ebpf.ErrNotSupported
	}
	return err
}, "5.1")

// HaveV4ISA probes the running kernel if instructions of the v4 ISA are supported.
//
// Upstream commit 1f9a1ea821ff ("bpf: Support new sign-extension load insns").
//
// See the package documentation for the meaning of the error return value.
func HaveV4ISA() error {
	return haveV4ISA()
}

var haveV4ISA = internal.NewFeatureTest("v4 ISA", func() error {
	err := probeProgram(&ebpf.ProgramSpec{
		Type: ebpf.SocketFilter,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.JEq.Imm(asm.R0, 1, "error"),
			asm.LongJump("exit"),
			asm.Mov.Imm(asm.R0, 1).WithSymbol("error"),
			asm.Return().WithSymbol("exit"),
		},
	})
	// This sometimes bubbles up from the JIT on aarch64.
	if errors.Is(err, sys.ENOTSUPP) {
		return ebpf.ErrNotSupported
	}
	return err
}, "6.6")
