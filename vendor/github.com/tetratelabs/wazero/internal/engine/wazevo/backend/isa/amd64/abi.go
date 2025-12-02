package amd64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

// For the details of the ABI, see:
// https://github.com/golang/go/blob/49d42128fd8594c172162961ead19ac95e247d24/src/cmd/compile/abi-internal.md#amd64-architecture

var (
	intArgResultRegs   = []regalloc.RealReg{rax, rbx, rcx, rdi, rsi, r8, r9, r10, r11}
	floatArgResultRegs = []regalloc.RealReg{xmm0, xmm1, xmm2, xmm3, xmm4, xmm5, xmm6, xmm7}
)

var regInfo = &regalloc.RegisterInfo{
	AllocatableRegisters: [regalloc.NumRegType][]regalloc.RealReg{
		regalloc.RegTypeInt: {
			rax, rcx, rdx, rbx, rsi, rdi, r8, r9, r10, r11, r12, r13, r14, r15,
		},
		regalloc.RegTypeFloat: {
			xmm0, xmm1, xmm2, xmm3, xmm4, xmm5, xmm6, xmm7, xmm8, xmm9, xmm10, xmm11, xmm12, xmm13, xmm14, xmm15,
		},
	},
	CalleeSavedRegisters: regalloc.NewRegSet(
		rdx, r12, r13, r14, r15,
		xmm8, xmm9, xmm10, xmm11, xmm12, xmm13, xmm14, xmm15,
	),
	CallerSavedRegisters: regalloc.NewRegSet(
		rax, rcx, rbx, rsi, rdi, r8, r9, r10, r11,
		xmm0, xmm1, xmm2, xmm3, xmm4, xmm5, xmm6, xmm7,
	),
	RealRegToVReg: []regalloc.VReg{
		rax: raxVReg, rcx: rcxVReg, rdx: rdxVReg, rbx: rbxVReg, rsp: rspVReg, rbp: rbpVReg, rsi: rsiVReg, rdi: rdiVReg,
		r8: r8VReg, r9: r9VReg, r10: r10VReg, r11: r11VReg, r12: r12VReg, r13: r13VReg, r14: r14VReg, r15: r15VReg,
		xmm0: xmm0VReg, xmm1: xmm1VReg, xmm2: xmm2VReg, xmm3: xmm3VReg, xmm4: xmm4VReg, xmm5: xmm5VReg, xmm6: xmm6VReg,
		xmm7: xmm7VReg, xmm8: xmm8VReg, xmm9: xmm9VReg, xmm10: xmm10VReg, xmm11: xmm11VReg, xmm12: xmm12VReg,
		xmm13: xmm13VReg, xmm14: xmm14VReg, xmm15: xmm15VReg,
	},
	RealRegName: func(r regalloc.RealReg) string { return regNames[r] },
	RealRegType: func(r regalloc.RealReg) regalloc.RegType {
		if r < xmm0 {
			return regalloc.RegTypeInt
		}
		return regalloc.RegTypeFloat
	},
}

// ArgsResultsRegs implements backend.Machine.
func (m *machine) ArgsResultsRegs() (argResultInts, argResultFloats []regalloc.RealReg) {
	return intArgResultRegs, floatArgResultRegs
}

// LowerParams implements backend.Machine.
func (m *machine) LowerParams(args []ssa.Value) {
	a := m.currentABI

	for i, ssaArg := range args {
		if !ssaArg.Valid() {
			continue
		}
		reg := m.c.VRegOf(ssaArg)
		arg := &a.Args[i]
		if arg.Kind == backend.ABIArgKindReg {
			m.InsertMove(reg, arg.Reg, arg.Type)
		} else {
			//
			//            (high address)
			//          +-----------------+
			//          |     .......     |
			//          |      ret Y      |
			//          |     .......     |
			//          |      ret 0      |
			//          |      arg X      |
			//          |     .......     |
			//          |      arg 1      |
			//          |      arg 0      |
			//          |   ReturnAddress |
			//          |    Caller_RBP   |
			//          +-----------------+ <-- RBP
			//          |   ...........   |
			//          |   clobbered  M  |
			//          |   ............  |
			//          |   clobbered  0  |
			//          |   spill slot N  |
			//          |   ...........   |
			//          |   spill slot 0  |
			//   RSP--> +-----------------+
			//             (low address)

			// Load the value from the arg stack slot above the current RBP.
			load := m.allocateInstr()
			mem := newOperandMem(m.newAmodeImmRBPReg(uint32(arg.Offset + 16)))
			switch arg.Type {
			case ssa.TypeI32:
				load.asMovzxRmR(extModeLQ, mem, reg)
			case ssa.TypeI64:
				load.asMov64MR(mem, reg)
			case ssa.TypeF32:
				load.asXmmUnaryRmR(sseOpcodeMovss, mem, reg)
			case ssa.TypeF64:
				load.asXmmUnaryRmR(sseOpcodeMovsd, mem, reg)
			case ssa.TypeV128:
				load.asXmmUnaryRmR(sseOpcodeMovdqu, mem, reg)
			default:
				panic("BUG")
			}
			m.insert(load)
		}
	}
}

// LowerReturns implements backend.Machine.
func (m *machine) LowerReturns(rets []ssa.Value) {
	// Load the XMM registers first as it might need a temporary register to inline
	// constant return.
	a := m.currentABI
	for i, ret := range rets {
		r := &a.Rets[i]
		if !r.Type.IsInt() {
			m.LowerReturn(ret, r)
		}
	}
	// Then load the GPR registers.
	for i, ret := range rets {
		r := &a.Rets[i]
		if r.Type.IsInt() {
			m.LowerReturn(ret, r)
		}
	}
}

func (m *machine) LowerReturn(ret ssa.Value, r *backend.ABIArg) {
	reg := m.c.VRegOf(ret)
	if def := m.c.ValueDefinition(ret); def.IsFromInstr() {
		// Constant instructions are inlined.
		if inst := def.Instr; inst.Constant() {
			m.insertLoadConstant(inst, reg)
		}
	}
	if r.Kind == backend.ABIArgKindReg {
		m.InsertMove(r.Reg, reg, ret.Type())
	} else {
		//
		//            (high address)
		//          +-----------------+
		//          |     .......     |
		//          |      ret Y      |
		//          |     .......     |
		//          |      ret 0      |
		//          |      arg X      |
		//          |     .......     |
		//          |      arg 1      |
		//          |      arg 0      |
		//          |   ReturnAddress |
		//          |    Caller_RBP   |
		//          +-----------------+ <-- RBP
		//          |   ...........   |
		//          |   clobbered  M  |
		//          |   ............  |
		//          |   clobbered  0  |
		//          |   spill slot N  |
		//          |   ...........   |
		//          |   spill slot 0  |
		//   RSP--> +-----------------+
		//             (low address)

		// Store the value to the return stack slot above the current RBP.
		store := m.allocateInstr()
		mem := newOperandMem(m.newAmodeImmRBPReg(uint32(m.currentABI.ArgStackSize + 16 + r.Offset)))
		switch r.Type {
		case ssa.TypeI32:
			store.asMovRM(reg, mem, 4)
		case ssa.TypeI64:
			store.asMovRM(reg, mem, 8)
		case ssa.TypeF32:
			store.asXmmMovRM(sseOpcodeMovss, reg, mem)
		case ssa.TypeF64:
			store.asXmmMovRM(sseOpcodeMovsd, reg, mem)
		case ssa.TypeV128:
			store.asXmmMovRM(sseOpcodeMovdqu, reg, mem)
		}
		m.insert(store)
	}
}
