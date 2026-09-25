package arm64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// CompileEntryPreamble implements backend.Machine. This assumes `entrypoint` function (in abi_go_entry_arm64.s) passes:
//
//  1. First (execution context ptr) and Second arguments are already passed in x0, and x1.
//  2. param/result slice ptr in x19; the pointer to []uint64{} which is used to pass arguments and accept return values.
//  3. Go-allocated stack slice ptr in x26.
//  4. Function executable in x24.
//
// also SP and FP are correct Go-runtime-based values, and LR is the return address to the Go-side caller.
func (m *machine) CompileEntryPreamble(signature *ssa.Signature) []byte {
	root := m.constructEntryPreamble(signature)
	m.encode(root)
	return m.compiler.Buf()
}

var (
	executionContextPtrReg = x0VReg
	// callee-saved regs so that they can be used in the prologue and epilogue.
	paramResultSlicePtr      = x19VReg
	savedExecutionContextPtr = x20VReg
	// goAllocatedStackPtr is not used in the epilogue.
	goAllocatedStackPtr = x26VReg
	// paramResultSliceCopied is not used in the epilogue.
	paramResultSliceCopied = x25VReg
	// tmpRegVReg is not used in the epilogue.
	functionExecutable = x24VReg
)

func (m *machine) goEntryPreamblePassArg(cur *instruction, paramSlicePtr regalloc.VReg, arg *backend.ABIArg, argStartOffsetFromSP int64) *instruction {
	typ := arg.Type
	bits := typ.Bits()
	isStackArg := arg.Kind == backend.ABIArgKindStack

	var loadTargetReg operand
	if !isStackArg {
		loadTargetReg = operandNR(arg.Reg)
	} else {
		switch typ {
		case ssa.TypeI32, ssa.TypeI64:
			loadTargetReg = operandNR(x15VReg)
		case ssa.TypeF32, ssa.TypeF64, ssa.TypeV128:
			loadTargetReg = operandNR(v15VReg)
		default:
			panic("TODO?")
		}
	}

	var postIndexImm int64
	if typ == ssa.TypeV128 {
		postIndexImm = 16 // v128 is represented as 2x64-bit in Go slice.
	} else {
		postIndexImm = 8
	}
	loadMode := m.amodePool.Allocate()
	*loadMode = addressMode{kind: addressModeKindPostIndex, rn: paramSlicePtr, imm: postIndexImm}

	instr := m.allocateInstr()
	switch typ {
	case ssa.TypeI32:
		instr.asULoad(loadTargetReg.reg(), loadMode, 32)
	case ssa.TypeI64:
		instr.asULoad(loadTargetReg.reg(), loadMode, 64)
	case ssa.TypeF32:
		instr.asFpuLoad(loadTargetReg.reg(), loadMode, 32)
	case ssa.TypeF64:
		instr.asFpuLoad(loadTargetReg.reg(), loadMode, 64)
	case ssa.TypeV128:
		instr.asFpuLoad(loadTargetReg.reg(), loadMode, 128)
	}
	cur = linkInstr(cur, instr)

	if isStackArg {
		var storeMode *addressMode
		cur, storeMode = m.resolveAddressModeForOffsetAndInsert(cur, argStartOffsetFromSP+arg.Offset, bits, spVReg, true)
		toStack := m.allocateInstr()
		toStack.asStore(loadTargetReg, storeMode, bits)
		cur = linkInstr(cur, toStack)
	}
	return cur
}

func (m *machine) goEntryPreamblePassResult(cur *instruction, resultSlicePtr regalloc.VReg, result *backend.ABIArg, resultStartOffsetFromSP int64) *instruction {
	isStackArg := result.Kind == backend.ABIArgKindStack
	typ := result.Type
	bits := typ.Bits()

	var storeTargetReg operand
	if !isStackArg {
		storeTargetReg = operandNR(result.Reg)
	} else {
		switch typ {
		case ssa.TypeI32, ssa.TypeI64:
			storeTargetReg = operandNR(x15VReg)
		case ssa.TypeF32, ssa.TypeF64, ssa.TypeV128:
			storeTargetReg = operandNR(v15VReg)
		default:
			panic("TODO?")
		}
	}

	var postIndexImm int64
	if typ == ssa.TypeV128 {
		postIndexImm = 16 // v128 is represented as 2x64-bit in Go slice.
	} else {
		postIndexImm = 8
	}

	if isStackArg {
		var loadMode *addressMode
		cur, loadMode = m.resolveAddressModeForOffsetAndInsert(cur, resultStartOffsetFromSP+result.Offset, bits, spVReg, true)
		toReg := m.allocateInstr()
		switch typ {
		case ssa.TypeI32, ssa.TypeI64:
			toReg.asULoad(storeTargetReg.reg(), loadMode, bits)
		case ssa.TypeF32, ssa.TypeF64, ssa.TypeV128:
			toReg.asFpuLoad(storeTargetReg.reg(), loadMode, bits)
		default:
			panic("TODO?")
		}
		cur = linkInstr(cur, toReg)
	}

	mode := m.amodePool.Allocate()
	*mode = addressMode{kind: addressModeKindPostIndex, rn: resultSlicePtr, imm: postIndexImm}
	instr := m.allocateInstr()
	instr.asStore(storeTargetReg, mode, bits)
	cur = linkInstr(cur, instr)
	return cur
}

func (m *machine) constructEntryPreamble(sig *ssa.Signature) (root *instruction) {
	abi := backend.FunctionABI{}
	abi.Init(sig, intParamResultRegs, floatParamResultRegs)

	root = m.allocateNop()

	//// ----------------------------------- prologue ----------------------------------- ////

	// First, we save executionContextPtrReg into a callee-saved register so that it can be used in epilogue as well.
	// 		mov savedExecutionContextPtr, x0
	cur := m.move64(savedExecutionContextPtr, executionContextPtrReg, root)

	// Next, save the current FP, SP and LR into the wazevo.executionContext:
	// 		str fp, [savedExecutionContextPtr, #OriginalFramePointer]
	//      mov tmp, sp ;; sp cannot be str'ed directly.
	// 		str sp, [savedExecutionContextPtr, #OriginalStackPointer]
	// 		str lr, [savedExecutionContextPtr, #GoReturnAddress]
	cur = m.loadOrStoreAtExecutionContext(fpVReg, wazevoapi.ExecutionContextOffsetOriginalFramePointer, true, cur)
	cur = m.move64(tmpRegVReg, spVReg, cur)
	cur = m.loadOrStoreAtExecutionContext(tmpRegVReg, wazevoapi.ExecutionContextOffsetOriginalStackPointer, true, cur)
	cur = m.loadOrStoreAtExecutionContext(lrVReg, wazevoapi.ExecutionContextOffsetGoReturnAddress, true, cur)

	// Then, move the Go-allocated stack pointer to SP:
	// 		mov sp, goAllocatedStackPtr
	cur = m.move64(spVReg, goAllocatedStackPtr, cur)

	prReg := paramResultSlicePtr
	if len(abi.Args) > 2 && len(abi.Rets) > 0 {
		// paramResultSlicePtr is modified during the execution of goEntryPreamblePassArg,
		// so copy it to another reg.
		cur = m.move64(paramResultSliceCopied, paramResultSlicePtr, cur)
		prReg = paramResultSliceCopied
	}

	stackSlotSize := int64(abi.AlignedArgResultStackSlotSize())
	for i := range abi.Args {
		if i < 2 {
			// module context ptr and execution context ptr are passed in x0 and x1 by the Go assembly function.
			continue
		}
		arg := &abi.Args[i]
		cur = m.goEntryPreamblePassArg(cur, prReg, arg, -stackSlotSize)
	}

	// Call the real function.
	bl := m.allocateInstr()
	bl.asCallIndirect(functionExecutable, &abi)
	cur = linkInstr(cur, bl)

	///// ----------------------------------- epilogue ----------------------------------- /////

	// Store the register results into paramResultSlicePtr.
	for i := range abi.Rets {
		cur = m.goEntryPreamblePassResult(cur, paramResultSlicePtr, &abi.Rets[i], abi.ArgStackSize-stackSlotSize)
	}

	// Finally, restore the FP, SP and LR, and return to the Go code.
	// 		ldr fp, [savedExecutionContextPtr, #OriginalFramePointer]
	// 		ldr tmp, [savedExecutionContextPtr, #OriginalStackPointer]
	//      mov sp, tmp ;; sp cannot be str'ed directly.
	// 		ldr lr, [savedExecutionContextPtr, #GoReturnAddress]
	// 		ret ;; --> return to the Go code
	cur = m.loadOrStoreAtExecutionContext(fpVReg, wazevoapi.ExecutionContextOffsetOriginalFramePointer, false, cur)
	cur = m.loadOrStoreAtExecutionContext(tmpRegVReg, wazevoapi.ExecutionContextOffsetOriginalStackPointer, false, cur)
	cur = m.move64(spVReg, tmpRegVReg, cur)
	cur = m.loadOrStoreAtExecutionContext(lrVReg, wazevoapi.ExecutionContextOffsetGoReturnAddress, false, cur)
	retInst := m.allocateInstr()
	retInst.asRet()
	linkInstr(cur, retInst)
	return
}

func (m *machine) move64(dst, src regalloc.VReg, prev *instruction) *instruction {
	instr := m.allocateInstr()
	instr.asMove64(dst, src)
	return linkInstr(prev, instr)
}

func (m *machine) loadOrStoreAtExecutionContext(d regalloc.VReg, offset wazevoapi.Offset, store bool, prev *instruction) *instruction {
	instr := m.allocateInstr()
	mode := m.amodePool.Allocate()
	*mode = addressMode{kind: addressModeKindRegUnsignedImm12, rn: savedExecutionContextPtr, imm: offset.I64()}
	if store {
		instr.asStore(operandNR(d), mode, 64)
	} else {
		instr.asULoad(d, mode, 64)
	}
	return linkInstr(prev, instr)
}

func linkInstr(prev, next *instruction) *instruction {
	prev.next = next
	next.prev = prev
	return next
}
