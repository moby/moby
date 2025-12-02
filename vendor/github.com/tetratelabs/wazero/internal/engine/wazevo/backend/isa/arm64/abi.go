package arm64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

// References:
// * https://github.com/golang/go/blob/49d42128fd8594c172162961ead19ac95e247d24/src/cmd/compile/abi-internal.md#arm64-architecture
// * https://developer.arm.com/documentation/102374/0101/Procedure-Call-Standard

var (
	intParamResultRegs   = []regalloc.RealReg{x0, x1, x2, x3, x4, x5, x6, x7}
	floatParamResultRegs = []regalloc.RealReg{v0, v1, v2, v3, v4, v5, v6, v7}
)

var regInfo = &regalloc.RegisterInfo{
	AllocatableRegisters: [regalloc.NumRegType][]regalloc.RealReg{
		// We don't allocate:
		// - x18: Reserved by the macOS: https://developer.apple.com/documentation/xcode/writing-arm64-code-for-apple-platforms#Respect-the-purpose-of-specific-CPU-registers
		// - x28: Reserved by Go runtime.
		// - x27(=tmpReg): because of the reason described on tmpReg.
		regalloc.RegTypeInt: {
			x8, x9, x10, x11, x12, x13, x14, x15,
			x16, x17, x19, x20, x21, x22, x23, x24, x25,
			x26, x29, x30,
			// These are the argument/return registers. Less preferred in the allocation.
			x7, x6, x5, x4, x3, x2, x1, x0,
		},
		regalloc.RegTypeFloat: {
			v8, v9, v10, v11, v12, v13, v14, v15, v16, v17, v18, v19,
			v20, v21, v22, v23, v24, v25, v26, v27, v28, v29, v30,
			// These are the argument/return registers. Less preferred in the allocation.
			v7, v6, v5, v4, v3, v2, v1, v0,
		},
	},
	CalleeSavedRegisters: regalloc.NewRegSet(
		x19, x20, x21, x22, x23, x24, x25, x26, x28,
		v18, v19, v20, v21, v22, v23, v24, v25, v26, v27, v28, v29, v30, v31,
	),
	CallerSavedRegisters: regalloc.NewRegSet(
		x0, x1, x2, x3, x4, x5, x6, x7, x8, x9, x10, x11, x12, x13, x14, x15, x16, x17, x29, x30,
		v0, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12, v13, v14, v15, v16, v17,
	),
	RealRegToVReg: []regalloc.VReg{
		x0: x0VReg, x1: x1VReg, x2: x2VReg, x3: x3VReg, x4: x4VReg, x5: x5VReg, x6: x6VReg, x7: x7VReg, x8: x8VReg, x9: x9VReg, x10: x10VReg, x11: x11VReg, x12: x12VReg, x13: x13VReg, x14: x14VReg, x15: x15VReg, x16: x16VReg, x17: x17VReg, x18: x18VReg, x19: x19VReg, x20: x20VReg, x21: x21VReg, x22: x22VReg, x23: x23VReg, x24: x24VReg, x25: x25VReg, x26: x26VReg, x27: x27VReg, x28: x28VReg, x29: x29VReg, x30: x30VReg,
		v0: v0VReg, v1: v1VReg, v2: v2VReg, v3: v3VReg, v4: v4VReg, v5: v5VReg, v6: v6VReg, v7: v7VReg, v8: v8VReg, v9: v9VReg, v10: v10VReg, v11: v11VReg, v12: v12VReg, v13: v13VReg, v14: v14VReg, v15: v15VReg, v16: v16VReg, v17: v17VReg, v18: v18VReg, v19: v19VReg, v20: v20VReg, v21: v21VReg, v22: v22VReg, v23: v23VReg, v24: v24VReg, v25: v25VReg, v26: v26VReg, v27: v27VReg, v28: v28VReg, v29: v29VReg, v30: v30VReg, v31: v31VReg,
	},
	RealRegName: func(r regalloc.RealReg) string { return regNames[r] },
	RealRegType: func(r regalloc.RealReg) regalloc.RegType {
		if r < v0 {
			return regalloc.RegTypeInt
		}
		return regalloc.RegTypeFloat
	},
}

// ArgsResultsRegs implements backend.Machine.
func (m *machine) ArgsResultsRegs() (argResultInts, argResultFloats []regalloc.RealReg) {
	return intParamResultRegs, floatParamResultRegs
}

// LowerParams implements backend.FunctionABI.
func (m *machine) LowerParams(args []ssa.Value) {
	a := m.currentABI

	for i, ssaArg := range args {
		if !ssaArg.Valid() {
			continue
		}
		reg := m.compiler.VRegOf(ssaArg)
		arg := &a.Args[i]
		if arg.Kind == backend.ABIArgKindReg {
			m.InsertMove(reg, arg.Reg, arg.Type)
		} else {
			// TODO: we could use pair load if there's consecutive loads for the same type.
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
			//          |      arg 0      |    <-|
			//          |   ReturnAddress |      |
			//          +-----------------+      |
			//          |   ...........   |      |
			//          |   clobbered  M  |      |   argStackOffset: is unknown at this point of compilation.
			//          |   ............  |      |
			//          |   clobbered  0  |      |
			//          |   spill slot N  |      |
			//          |   ...........   |      |
			//          |   spill slot 0  |      |
			//   SP---> +-----------------+    <-+
			//             (low address)

			bits := arg.Type.Bits()
			// At this point of compilation, we don't yet know how much space exist below the return address.
			// So we instruct the address mode to add the `argStackOffset` to the offset at the later phase of compilation.
			amode := m.amodePool.Allocate()
			*amode = addressMode{imm: arg.Offset, rn: spVReg, kind: addressModeKindArgStackSpace}
			load := m.allocateInstr()
			switch arg.Type {
			case ssa.TypeI32, ssa.TypeI64:
				load.asULoad(reg, amode, bits)
			case ssa.TypeF32, ssa.TypeF64, ssa.TypeV128:
				load.asFpuLoad(reg, amode, bits)
			default:
				panic("BUG")
			}
			m.insert(load)
			m.unresolvedAddressModes = append(m.unresolvedAddressModes, load)
		}
	}
}

// LowerReturns lowers the given returns.
func (m *machine) LowerReturns(rets []ssa.Value) {
	a := m.currentABI

	l := len(rets) - 1
	for i := range rets {
		// Reverse order in order to avoid overwriting the stack returns existing in the return registers.
		ret := rets[l-i]
		r := &a.Rets[l-i]
		reg := m.compiler.VRegOf(ret)
		if def := m.compiler.ValueDefinition(ret); def.IsFromInstr() {
			// Constant instructions are inlined.
			if inst := def.Instr; inst.Constant() {
				val := inst.Return()
				valType := val.Type()
				v := inst.ConstantVal()
				m.insertLoadConstant(v, valType, reg)
			}
		}
		if r.Kind == backend.ABIArgKindReg {
			m.InsertMove(r.Reg, reg, ret.Type())
		} else {
			// TODO: we could use pair store if there's consecutive stores for the same type.
			//
			//            (high address)
			//          +-----------------+
			//          |     .......     |
			//          |      ret Y      |
			//          |     .......     |
			//          |      ret 0      |    <-+
			//          |      arg X      |      |
			//          |     .......     |      |
			//          |      arg 1      |      |
			//          |      arg 0      |      |
			//          |   ReturnAddress |      |
			//          +-----------------+      |
			//          |   ...........   |      |
			//          |   spill slot M  |      |   retStackOffset: is unknown at this point of compilation.
			//          |   ............  |      |
			//          |   spill slot 2  |      |
			//          |   spill slot 1  |      |
			//          |   clobbered 0   |      |
			//          |   clobbered 1   |      |
			//          |   ...........   |      |
			//          |   clobbered N   |      |
			//   SP---> +-----------------+    <-+
			//             (low address)

			bits := r.Type.Bits()

			// At this point of compilation, we don't yet know how much space exist below the return address.
			// So we instruct the address mode to add the `retStackOffset` to the offset at the later phase of compilation.
			amode := m.amodePool.Allocate()
			*amode = addressMode{imm: r.Offset, rn: spVReg, kind: addressModeKindResultStackSpace}
			store := m.allocateInstr()
			store.asStore(operandNR(reg), amode, bits)
			m.insert(store)
			m.unresolvedAddressModes = append(m.unresolvedAddressModes, store)
		}
	}
}

// callerGenVRegToFunctionArg is the opposite of GenFunctionArgToVReg, which is used to generate the
// caller side of the function call.
func (m *machine) callerGenVRegToFunctionArg(a *backend.FunctionABI, argIndex int, reg regalloc.VReg, def backend.SSAValueDefinition, slotBegin int64) {
	arg := &a.Args[argIndex]
	if def.IsFromInstr() {
		// Constant instructions are inlined.
		if inst := def.Instr; inst.Constant() {
			val := inst.Return()
			valType := val.Type()
			v := inst.ConstantVal()
			m.insertLoadConstant(v, valType, reg)
		}
	}
	if arg.Kind == backend.ABIArgKindReg {
		m.InsertMove(arg.Reg, reg, arg.Type)
	} else {
		// TODO: we could use pair store if there's consecutive stores for the same type.
		//
		// Note that at this point, stack pointer is already adjusted.
		bits := arg.Type.Bits()
		amode := m.resolveAddressModeForOffset(arg.Offset-slotBegin, bits, spVReg, false)
		store := m.allocateInstr()
		store.asStore(operandNR(reg), amode, bits)
		m.insert(store)
	}
}

func (m *machine) callerGenFunctionReturnVReg(a *backend.FunctionABI, retIndex int, reg regalloc.VReg, slotBegin int64) {
	r := &a.Rets[retIndex]
	if r.Kind == backend.ABIArgKindReg {
		m.InsertMove(reg, r.Reg, r.Type)
	} else {
		// TODO: we could use pair load if there's consecutive loads for the same type.
		amode := m.resolveAddressModeForOffset(a.ArgStackSize+r.Offset-slotBegin, r.Type.Bits(), spVReg, false)
		ldr := m.allocateInstr()
		switch r.Type {
		case ssa.TypeI32, ssa.TypeI64:
			ldr.asULoad(reg, amode, r.Type.Bits())
		case ssa.TypeF32, ssa.TypeF64, ssa.TypeV128:
			ldr.asFpuLoad(reg, amode, r.Type.Bits())
		default:
			panic("BUG")
		}
		m.insert(ldr)
	}
}

func (m *machine) resolveAddressModeForOffsetAndInsert(cur *instruction, offset int64, dstBits byte, rn regalloc.VReg, allowTmpRegUse bool) (*instruction, *addressMode) {
	m.pendingInstructions = m.pendingInstructions[:0]
	mode := m.resolveAddressModeForOffset(offset, dstBits, rn, allowTmpRegUse)
	for _, instr := range m.pendingInstructions {
		cur = linkInstr(cur, instr)
	}
	return cur, mode
}

func (m *machine) resolveAddressModeForOffset(offset int64, dstBits byte, rn regalloc.VReg, allowTmpRegUse bool) *addressMode {
	if rn.RegType() != regalloc.RegTypeInt {
		panic("BUG: rn should be a pointer: " + formatVRegSized(rn, 64))
	}
	amode := m.amodePool.Allocate()
	if offsetFitsInAddressModeKindRegUnsignedImm12(dstBits, offset) {
		*amode = addressMode{kind: addressModeKindRegUnsignedImm12, rn: rn, imm: offset}
	} else if offsetFitsInAddressModeKindRegSignedImm9(offset) {
		*amode = addressMode{kind: addressModeKindRegSignedImm9, rn: rn, imm: offset}
	} else {
		var indexReg regalloc.VReg
		if allowTmpRegUse {
			m.lowerConstantI64(tmpRegVReg, offset)
			indexReg = tmpRegVReg
		} else {
			indexReg = m.compiler.AllocateVReg(ssa.TypeI64)
			m.lowerConstantI64(indexReg, offset)
		}
		*amode = addressMode{kind: addressModeKindRegReg, rn: rn, rm: indexReg, extOp: extendOpUXTX /* indicates index rm is 64-bit */}
	}
	return amode
}

func (m *machine) lowerCall(si *ssa.Instruction) {
	isDirectCall := si.Opcode() == ssa.OpcodeCall
	var indirectCalleePtr ssa.Value
	var directCallee ssa.FuncRef
	var sigID ssa.SignatureID
	var args []ssa.Value
	if isDirectCall {
		directCallee, sigID, args = si.CallData()
	} else {
		indirectCalleePtr, sigID, args, _ /* on arm64, the calling convention is compatible with the Go runtime */ = si.CallIndirectData()
	}
	calleeABI := m.compiler.GetFunctionABI(m.compiler.SSABuilder().ResolveSignature(sigID))

	stackSlotSize := int64(calleeABI.AlignedArgResultStackSlotSize())
	if m.maxRequiredStackSizeForCalls < stackSlotSize+16 {
		m.maxRequiredStackSizeForCalls = stackSlotSize + 16 // return address frame.
	}

	for i, arg := range args {
		reg := m.compiler.VRegOf(arg)
		def := m.compiler.ValueDefinition(arg)
		m.callerGenVRegToFunctionArg(calleeABI, i, reg, def, stackSlotSize)
	}

	if isDirectCall {
		call := m.allocateInstr()
		call.asCall(directCallee, calleeABI)
		m.insert(call)
	} else {
		ptr := m.compiler.VRegOf(indirectCalleePtr)
		callInd := m.allocateInstr()
		callInd.asCallIndirect(ptr, calleeABI)
		m.insert(callInd)
	}

	var index int
	r1, rs := si.Returns()
	if r1.Valid() {
		m.callerGenFunctionReturnVReg(calleeABI, 0, m.compiler.VRegOf(r1), stackSlotSize)
		index++
	}

	for _, r := range rs {
		m.callerGenFunctionReturnVReg(calleeABI, index, m.compiler.VRegOf(r), stackSlotSize)
		index++
	}
}

func (m *machine) insertAddOrSubStackPointer(rd regalloc.VReg, diff int64, add bool) {
	if imm12Operand, ok := asImm12Operand(uint64(diff)); ok {
		alu := m.allocateInstr()
		var ao aluOp
		if add {
			ao = aluOpAdd
		} else {
			ao = aluOpSub
		}
		alu.asALU(ao, rd, operandNR(spVReg), imm12Operand, true)
		m.insert(alu)
	} else {
		m.lowerConstantI64(tmpRegVReg, diff)
		alu := m.allocateInstr()
		var ao aluOp
		if add {
			ao = aluOpAdd
		} else {
			ao = aluOpSub
		}
		alu.asALU(ao, rd, operandNR(spVReg), operandNR(tmpRegVReg), true)
		m.insert(alu)
	}
}
