package arm64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

var calleeSavedRegistersSorted = []regalloc.VReg{
	x19VReg, x20VReg, x21VReg, x22VReg, x23VReg, x24VReg, x25VReg, x26VReg, x28VReg,
	v18VReg, v19VReg, v20VReg, v21VReg, v22VReg, v23VReg, v24VReg, v25VReg, v26VReg, v27VReg, v28VReg, v29VReg, v30VReg, v31VReg,
}

// CompileGoFunctionTrampoline implements backend.Machine.
func (m *machine) CompileGoFunctionTrampoline(exitCode wazevoapi.ExitCode, sig *ssa.Signature, needModuleContextPtr bool) []byte {
	argBegin := 1 // Skips exec context by default.
	if needModuleContextPtr {
		argBegin++
	}

	abi := &backend.FunctionABI{}
	abi.Init(sig, intParamResultRegs, floatParamResultRegs)
	m.currentABI = abi

	cur := m.allocateInstr()
	cur.asNop0()
	m.rootInstr = cur

	// Execution context is always the first argument.
	execCtrPtr := x0VReg

	// In the following, we create the following stack layout:
	//
	//                   (high address)
	//     SP ------> +-----------------+  <----+
	//                |     .......     |       |
	//                |      ret Y      |       |
	//                |     .......     |       |
	//                |      ret 0      |       |
	//                |      arg X      |       |  size_of_arg_ret
	//                |     .......     |       |
	//                |      arg 1      |       |
	//                |      arg 0      |  <----+ <-------- originalArg0Reg
	//                | size_of_arg_ret |
	//                |  ReturnAddress  |
	//                +-----------------+ <----+
	//                |      xxxx       |      |  ;; might be padded to make it 16-byte aligned.
	//           +--->|  arg[N]/ret[M]  |      |
	//  sliceSize|    |   ............  |      | goCallStackSize
	//           |    |  arg[1]/ret[1]  |      |
	//           +--->|  arg[0]/ret[0]  | <----+ <-------- arg0ret0AddrReg
	//                |    sliceSize    |
	//                |   frame_size    |
	//                +-----------------+
	//                   (low address)
	//
	// where the region of "arg[0]/ret[0] ... arg[N]/ret[M]" is the stack used by the Go functions,
	// therefore will be accessed as the usual []uint64. So that's where we need to pass/receive
	// the arguments/return values.

	// First of all, to update the SP, and create "ReturnAddress + size_of_arg_ret".
	cur = m.createReturnAddrAndSizeOfArgRetSlot(cur)

	const frameInfoSize = 16 // == frame_size + sliceSize.

	// Next, we should allocate the stack for the Go function call if necessary.
	goCallStackSize, sliceSizeInBytes := backend.GoFunctionCallRequiredStackSize(sig, argBegin)
	cur = m.insertStackBoundsCheck(goCallStackSize+frameInfoSize, cur)

	originalArg0Reg := x17VReg // Caller save, so we can use it for whatever we want.
	if m.currentABI.AlignedArgResultStackSlotSize() > 0 {
		// At this point, SP points to `ReturnAddress`, so add 16 to get the original arg 0 slot.
		cur = m.addsAddOrSubStackPointer(cur, originalArg0Reg, frameInfoSize, true)
	}

	// Save the callee saved registers.
	cur = m.saveRegistersInExecutionContext(cur, calleeSavedRegistersSorted)

	if needModuleContextPtr {
		offset := wazevoapi.ExecutionContextOffsetGoFunctionCallCalleeModuleContextOpaque.I64()
		if !offsetFitsInAddressModeKindRegUnsignedImm12(64, offset) {
			panic("BUG: too large or un-aligned offset for goFunctionCallCalleeModuleContextOpaque in execution context")
		}

		// Module context is always the second argument.
		moduleCtrPtr := x1VReg
		store := m.allocateInstr()
		amode := m.amodePool.Allocate()
		*amode = addressMode{kind: addressModeKindRegUnsignedImm12, rn: execCtrPtr, imm: offset}
		store.asStore(operandNR(moduleCtrPtr), amode, 64)
		cur = linkInstr(cur, store)
	}

	// Advances the stack pointer.
	cur = m.addsAddOrSubStackPointer(cur, spVReg, goCallStackSize, false)

	// Copy the pointer to x15VReg.
	arg0ret0AddrReg := x15VReg // Caller save, so we can use it for whatever we want.
	copySp := m.allocateInstr()
	copySp.asMove64(arg0ret0AddrReg, spVReg)
	cur = linkInstr(cur, copySp)

	// Next, we need to store all the arguments to the stack in the typical Wasm stack style.
	for i := range abi.Args[argBegin:] {
		arg := &abi.Args[argBegin+i]
		store := m.allocateInstr()
		var v regalloc.VReg
		if arg.Kind == backend.ABIArgKindReg {
			v = arg.Reg
		} else {
			cur, v = m.goFunctionCallLoadStackArg(cur, originalArg0Reg, arg,
				// Caller save, so we can use it for whatever we want.
				x11VReg, v11VReg)
		}

		var sizeInBits byte
		if arg.Type == ssa.TypeV128 {
			sizeInBits = 128
		} else {
			sizeInBits = 64
		}
		amode := m.amodePool.Allocate()
		*amode = addressMode{kind: addressModeKindPostIndex, rn: arg0ret0AddrReg, imm: int64(sizeInBits / 8)}
		store.asStore(operandNR(v), amode, sizeInBits)
		cur = linkInstr(cur, store)
	}

	// Finally, now that we've advanced SP to arg[0]/ret[0], we allocate `frame_size + sliceSize`.
	var frameSizeReg, sliceSizeReg regalloc.VReg
	if goCallStackSize > 0 {
		cur = m.lowerConstantI64AndInsert(cur, tmpRegVReg, goCallStackSize)
		frameSizeReg = tmpRegVReg
		cur = m.lowerConstantI64AndInsert(cur, x16VReg, sliceSizeInBytes/8)
		sliceSizeReg = x16VReg
	} else {
		frameSizeReg = xzrVReg
		sliceSizeReg = xzrVReg
	}
	_amode := addressModePreOrPostIndex(m, spVReg, -16, true)
	storeP := m.allocateInstr()
	storeP.asStorePair64(frameSizeReg, sliceSizeReg, _amode)
	cur = linkInstr(cur, storeP)

	// Set the exit status on the execution context.
	cur = m.setExitCode(cur, x0VReg, exitCode)

	// Save the current stack pointer.
	cur = m.saveCurrentStackPointer(cur, x0VReg)

	// Exit the execution.
	cur = m.storeReturnAddressAndExit(cur)

	// After the call, we need to restore the callee saved registers.
	cur = m.restoreRegistersInExecutionContext(cur, calleeSavedRegistersSorted)

	// Get the pointer to the arg[0]/ret[0]: We need to skip `frame_size + sliceSize`.
	if len(abi.Rets) > 0 {
		cur = m.addsAddOrSubStackPointer(cur, arg0ret0AddrReg, frameInfoSize, true)
	}

	// Advances the SP so that it points to `ReturnAddress`.
	cur = m.addsAddOrSubStackPointer(cur, spVReg, frameInfoSize+goCallStackSize, true)
	ldr := m.allocateInstr()
	// And load the return address.
	amode := addressModePreOrPostIndex(m, spVReg, 16 /* stack pointer must be 16-byte aligned. */, false /* increment after loads */)
	ldr.asULoad(lrVReg, amode, 64)
	cur = linkInstr(cur, ldr)

	originalRet0Reg := x17VReg // Caller save, so we can use it for whatever we want.
	if m.currentABI.RetStackSize > 0 {
		cur = m.addsAddOrSubStackPointer(cur, originalRet0Reg, m.currentABI.ArgStackSize, true)
	}

	// Make the SP point to the original address (above the result slot).
	if s := int64(m.currentABI.AlignedArgResultStackSlotSize()); s > 0 {
		cur = m.addsAddOrSubStackPointer(cur, spVReg, s, true)
	}

	for i := range abi.Rets {
		r := &abi.Rets[i]
		if r.Kind == backend.ABIArgKindReg {
			loadIntoReg := m.allocateInstr()
			mode := m.amodePool.Allocate()
			*mode = addressMode{kind: addressModeKindPostIndex, rn: arg0ret0AddrReg}
			switch r.Type {
			case ssa.TypeI32:
				mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
				loadIntoReg.asULoad(r.Reg, mode, 32)
			case ssa.TypeI64:
				mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
				loadIntoReg.asULoad(r.Reg, mode, 64)
			case ssa.TypeF32:
				mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
				loadIntoReg.asFpuLoad(r.Reg, mode, 32)
			case ssa.TypeF64:
				mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
				loadIntoReg.asFpuLoad(r.Reg, mode, 64)
			case ssa.TypeV128:
				mode.imm = 16
				loadIntoReg.asFpuLoad(r.Reg, mode, 128)
			default:
				panic("TODO")
			}
			cur = linkInstr(cur, loadIntoReg)
		} else {
			// First we need to load the value to a temporary just like ^^.
			intTmp, floatTmp := x11VReg, v11VReg
			loadIntoTmpReg := m.allocateInstr()
			mode := m.amodePool.Allocate()
			*mode = addressMode{kind: addressModeKindPostIndex, rn: arg0ret0AddrReg}
			var resultReg regalloc.VReg
			switch r.Type {
			case ssa.TypeI32:
				mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
				loadIntoTmpReg.asULoad(intTmp, mode, 32)
				resultReg = intTmp
			case ssa.TypeI64:
				mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
				loadIntoTmpReg.asULoad(intTmp, mode, 64)
				resultReg = intTmp
			case ssa.TypeF32:
				mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
				loadIntoTmpReg.asFpuLoad(floatTmp, mode, 32)
				resultReg = floatTmp
			case ssa.TypeF64:
				mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
				loadIntoTmpReg.asFpuLoad(floatTmp, mode, 64)
				resultReg = floatTmp
			case ssa.TypeV128:
				mode.imm = 16
				loadIntoTmpReg.asFpuLoad(floatTmp, mode, 128)
				resultReg = floatTmp
			default:
				panic("TODO")
			}
			cur = linkInstr(cur, loadIntoTmpReg)
			cur = m.goFunctionCallStoreStackResult(cur, originalRet0Reg, r, resultReg)
		}
	}

	ret := m.allocateInstr()
	ret.asRet()
	linkInstr(cur, ret)

	m.encode(m.rootInstr)
	return m.compiler.Buf()
}

func (m *machine) saveRegistersInExecutionContext(cur *instruction, regs []regalloc.VReg) *instruction {
	offset := wazevoapi.ExecutionContextOffsetSavedRegistersBegin.I64()
	for _, v := range regs {
		store := m.allocateInstr()
		var sizeInBits byte
		switch v.RegType() {
		case regalloc.RegTypeInt:
			sizeInBits = 64
		case regalloc.RegTypeFloat:
			sizeInBits = 128
		}
		mode := m.amodePool.Allocate()
		*mode = addressMode{
			kind: addressModeKindRegUnsignedImm12,
			// Execution context is always the first argument.
			rn: x0VReg, imm: offset,
		}
		store.asStore(operandNR(v), mode, sizeInBits)
		store.prev = cur
		cur.next = store
		cur = store
		offset += 16 // Imm12 must be aligned 16 for vector regs, so we unconditionally store regs at the offset of multiple of 16.
	}
	return cur
}

func (m *machine) restoreRegistersInExecutionContext(cur *instruction, regs []regalloc.VReg) *instruction {
	offset := wazevoapi.ExecutionContextOffsetSavedRegistersBegin.I64()
	for _, v := range regs {
		load := m.allocateInstr()
		var as func(dst regalloc.VReg, amode *addressMode, sizeInBits byte)
		var sizeInBits byte
		switch v.RegType() {
		case regalloc.RegTypeInt:
			as = load.asULoad
			sizeInBits = 64
		case regalloc.RegTypeFloat:
			as = load.asFpuLoad
			sizeInBits = 128
		}
		mode := m.amodePool.Allocate()
		*mode = addressMode{
			kind: addressModeKindRegUnsignedImm12,
			// Execution context is always the first argument.
			rn: x0VReg, imm: offset,
		}
		as(v, mode, sizeInBits)
		cur = linkInstr(cur, load)
		offset += 16 // Imm12 must be aligned 16 for vector regs, so we unconditionally load regs at the offset of multiple of 16.
	}
	return cur
}

func (m *machine) lowerConstantI64AndInsert(cur *instruction, dst regalloc.VReg, v int64) *instruction {
	m.pendingInstructions = m.pendingInstructions[:0]
	m.lowerConstantI64(dst, v)
	for _, instr := range m.pendingInstructions {
		cur = linkInstr(cur, instr)
	}
	return cur
}

func (m *machine) lowerConstantI32AndInsert(cur *instruction, dst regalloc.VReg, v int32) *instruction {
	m.pendingInstructions = m.pendingInstructions[:0]
	m.lowerConstantI32(dst, v)
	for _, instr := range m.pendingInstructions {
		cur = linkInstr(cur, instr)
	}
	return cur
}

func (m *machine) setExitCode(cur *instruction, execCtr regalloc.VReg, exitCode wazevoapi.ExitCode) *instruction {
	constReg := x17VReg // caller-saved, so we can use it.
	cur = m.lowerConstantI32AndInsert(cur, constReg, int32(exitCode))

	// Set the exit status on the execution context.
	setExistStatus := m.allocateInstr()
	mode := m.amodePool.Allocate()
	*mode = addressMode{kind: addressModeKindRegUnsignedImm12, rn: execCtr, imm: wazevoapi.ExecutionContextOffsetExitCodeOffset.I64()}
	setExistStatus.asStore(operandNR(constReg), mode, 32)
	cur = linkInstr(cur, setExistStatus)
	return cur
}

func (m *machine) storeReturnAddressAndExit(cur *instruction) *instruction {
	// Read the return address into tmp, and store it in the execution context.
	adr := m.allocateInstr()
	adr.asAdr(tmpRegVReg, exitSequenceSize+8)
	cur = linkInstr(cur, adr)

	storeReturnAddr := m.allocateInstr()
	mode := m.amodePool.Allocate()
	*mode = addressMode{
		kind: addressModeKindRegUnsignedImm12,
		// Execution context is always the first argument.
		rn: x0VReg, imm: wazevoapi.ExecutionContextOffsetGoCallReturnAddress.I64(),
	}
	storeReturnAddr.asStore(operandNR(tmpRegVReg), mode, 64)
	cur = linkInstr(cur, storeReturnAddr)

	// Exit the execution.
	trapSeq := m.allocateInstr()
	trapSeq.asExitSequence(x0VReg)
	cur = linkInstr(cur, trapSeq)
	return cur
}

func (m *machine) saveCurrentStackPointer(cur *instruction, execCtr regalloc.VReg) *instruction {
	// Save the current stack pointer:
	// 	mov tmp, sp,
	// 	str tmp, [exec_ctx, #stackPointerBeforeGoCall]
	movSp := m.allocateInstr()
	movSp.asMove64(tmpRegVReg, spVReg)
	cur = linkInstr(cur, movSp)

	strSp := m.allocateInstr()
	mode := m.amodePool.Allocate()
	*mode = addressMode{
		kind: addressModeKindRegUnsignedImm12,
		rn:   execCtr, imm: wazevoapi.ExecutionContextOffsetStackPointerBeforeGoCall.I64(),
	}
	strSp.asStore(operandNR(tmpRegVReg), mode, 64)
	cur = linkInstr(cur, strSp)
	return cur
}

func (m *machine) goFunctionCallLoadStackArg(cur *instruction, originalArg0Reg regalloc.VReg, arg *backend.ABIArg, intVReg, floatVReg regalloc.VReg) (*instruction, regalloc.VReg) {
	load := m.allocateInstr()
	var result regalloc.VReg
	mode := m.amodePool.Allocate()
	*mode = addressMode{kind: addressModeKindPostIndex, rn: originalArg0Reg}
	switch arg.Type {
	case ssa.TypeI32:
		mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
		load.asULoad(intVReg, mode, 32)
		result = intVReg
	case ssa.TypeI64:
		mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
		load.asULoad(intVReg, mode, 64)
		result = intVReg
	case ssa.TypeF32:
		mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
		load.asFpuLoad(floatVReg, mode, 32)
		result = floatVReg
	case ssa.TypeF64:
		mode.imm = 8 // We use uint64 for all basic types, except SIMD v128.
		load.asFpuLoad(floatVReg, mode, 64)
		result = floatVReg
	case ssa.TypeV128:
		mode.imm = 16
		load.asFpuLoad(floatVReg, mode, 128)
		result = floatVReg
	default:
		panic("TODO")
	}

	cur = linkInstr(cur, load)
	return cur, result
}

func (m *machine) goFunctionCallStoreStackResult(cur *instruction, originalRet0Reg regalloc.VReg, result *backend.ABIArg, resultVReg regalloc.VReg) *instruction {
	store := m.allocateInstr()
	mode := m.amodePool.Allocate()
	*mode = addressMode{kind: addressModeKindPostIndex, rn: originalRet0Reg}
	var sizeInBits byte
	switch result.Type {
	case ssa.TypeI32, ssa.TypeF32:
		mode.imm = 8
		sizeInBits = 32
	case ssa.TypeI64, ssa.TypeF64:
		mode.imm = 8
		sizeInBits = 64
	case ssa.TypeV128:
		mode.imm = 16
		sizeInBits = 128
	default:
		panic("TODO")
	}
	store.asStore(operandNR(resultVReg), mode, sizeInBits)
	return linkInstr(cur, store)
}
