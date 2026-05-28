package amd64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

var (
	executionContextPtrReg = raxVReg

	// Followings are callee saved registers. They can be used freely in the entry preamble
	// since the preamble is called via Go assembly function which has stack-based ABI.

	// savedExecutionContextPtr also must be a callee-saved reg so that they can be used in the prologue and epilogue.
	savedExecutionContextPtr = rdxVReg
	// paramResultSlicePtr must match with entrypoint function in abi_entry_amd64.s.
	paramResultSlicePtr = r12VReg
	// goAllocatedStackPtr must match with entrypoint function in abi_entry_amd64.s.
	goAllocatedStackPtr = r13VReg
	// functionExecutable must match with entrypoint function in abi_entry_amd64.s.
	functionExecutable = r14VReg
	tmpIntReg          = r15VReg
	tmpXmmReg          = xmm15VReg
)

// CompileEntryPreamble implements backend.Machine.
func (m *machine) CompileEntryPreamble(sig *ssa.Signature) []byte {
	root := m.compileEntryPreamble(sig)
	m.encodeWithoutSSA(root)
	buf := m.c.Buf()
	return buf
}

func (m *machine) compileEntryPreamble(sig *ssa.Signature) *instruction {
	abi := backend.FunctionABI{}
	abi.Init(sig, intArgResultRegs, floatArgResultRegs)

	root := m.allocateNop()

	//// ----------------------------------- prologue ----------------------------------- ////

	// First, we save executionContextPtrReg into a callee-saved register so that it can be used in epilogue as well.
	// 		mov %executionContextPtrReg, %savedExecutionContextPtr
	cur := m.move64(executionContextPtrReg, savedExecutionContextPtr, root)

	// Next is to save the original RBP and RSP into the execution context.
	cur = m.saveOriginalRSPRBP(cur)

	// Now set the RSP to the Go-allocated stack pointer.
	// 		mov %goAllocatedStackPtr, %rsp
	cur = m.move64(goAllocatedStackPtr, rspVReg, cur)

	if stackSlotSize := abi.AlignedArgResultStackSlotSize(); stackSlotSize > 0 {
		// Allocate stack slots for the arguments and return values.
		// 		sub $stackSlotSize, %rsp
		spDec := m.allocateInstr().asAluRmiR(aluRmiROpcodeSub, newOperandImm32(uint32(stackSlotSize)), rspVReg, true)
		cur = linkInstr(cur, spDec)
	}

	var offset uint32
	for i := range abi.Args {
		if i < 2 {
			// module context ptr and execution context ptr are passed in rax and rbx by the Go assembly function.
			continue
		}
		arg := &abi.Args[i]
		cur = m.goEntryPreamblePassArg(cur, paramResultSlicePtr, offset, arg)
		if arg.Type == ssa.TypeV128 {
			offset += 16
		} else {
			offset += 8
		}
	}

	// Zero out RBP so that the unwind/stack growth code can correctly detect the end of the stack.
	zerosRbp := m.allocateInstr().asAluRmiR(aluRmiROpcodeXor, newOperandReg(rbpVReg), rbpVReg, true)
	cur = linkInstr(cur, zerosRbp)

	// Now ready to call the real function. Note that at this point stack pointer is already set to the Go-allocated,
	// which is aligned to 16 bytes.
	call := m.allocateInstr().asCallIndirect(newOperandReg(functionExecutable), &abi)
	cur = linkInstr(cur, call)

	//// ----------------------------------- epilogue ----------------------------------- ////

	// Read the results from regs and the stack, and set them correctly into the paramResultSlicePtr.
	offset = 0
	for i := range abi.Rets {
		r := &abi.Rets[i]
		cur = m.goEntryPreamblePassResult(cur, paramResultSlicePtr, offset, r, uint32(abi.ArgStackSize))
		if r.Type == ssa.TypeV128 {
			offset += 16
		} else {
			offset += 8
		}
	}

	// Finally, restore the original RBP and RSP.
	cur = m.restoreOriginalRSPRBP(cur)

	ret := m.allocateInstr().asRet()
	linkInstr(cur, ret)
	return root
}

// saveOriginalRSPRBP saves the original RSP and RBP into the execution context.
func (m *machine) saveOriginalRSPRBP(cur *instruction) *instruction {
	// 		mov %rbp, wazevoapi.ExecutionContextOffsetOriginalFramePointer(%executionContextPtrReg)
	// 		mov %rsp, wazevoapi.ExecutionContextOffsetOriginalStackPointer(%executionContextPtrReg)
	cur = m.loadOrStore64AtExecutionCtx(executionContextPtrReg, wazevoapi.ExecutionContextOffsetOriginalFramePointer, rbpVReg, true, cur)
	cur = m.loadOrStore64AtExecutionCtx(executionContextPtrReg, wazevoapi.ExecutionContextOffsetOriginalStackPointer, rspVReg, true, cur)
	return cur
}

// restoreOriginalRSPRBP restores the original RSP and RBP from the execution context.
func (m *machine) restoreOriginalRSPRBP(cur *instruction) *instruction {
	// 		mov wazevoapi.ExecutionContextOffsetOriginalFramePointer(%executionContextPtrReg), %rbp
	// 		mov wazevoapi.ExecutionContextOffsetOriginalStackPointer(%executionContextPtrReg), %rsp
	cur = m.loadOrStore64AtExecutionCtx(savedExecutionContextPtr, wazevoapi.ExecutionContextOffsetOriginalFramePointer, rbpVReg, false, cur)
	cur = m.loadOrStore64AtExecutionCtx(savedExecutionContextPtr, wazevoapi.ExecutionContextOffsetOriginalStackPointer, rspVReg, false, cur)
	return cur
}

func (m *machine) move64(src, dst regalloc.VReg, prev *instruction) *instruction {
	mov := m.allocateInstr().asMovRR(src, dst, true)
	return linkInstr(prev, mov)
}

func (m *machine) loadOrStore64AtExecutionCtx(execCtx regalloc.VReg, offset wazevoapi.Offset, r regalloc.VReg, store bool, prev *instruction) *instruction {
	mem := newOperandMem(m.newAmodeImmReg(offset.U32(), execCtx))
	instr := m.allocateInstr()
	if store {
		instr.asMovRM(r, mem, 8)
	} else {
		instr.asMov64MR(mem, r)
	}
	return linkInstr(prev, instr)
}

// This is for debugging.
func (m *machine) linkUD2(cur *instruction) *instruction { //nolint
	return linkInstr(cur, m.allocateInstr().asUD2())
}

func (m *machine) goEntryPreamblePassArg(cur *instruction, paramSlicePtr regalloc.VReg, offsetInParamSlice uint32, arg *backend.ABIArg) *instruction {
	var dst regalloc.VReg
	argTyp := arg.Type
	if arg.Kind == backend.ABIArgKindStack {
		// Caller saved registers ca
		switch argTyp {
		case ssa.TypeI32, ssa.TypeI64:
			dst = tmpIntReg
		case ssa.TypeF32, ssa.TypeF64, ssa.TypeV128:
			dst = tmpXmmReg
		default:
			panic("BUG")
		}
	} else {
		dst = arg.Reg
	}

	load := m.allocateInstr()
	a := newOperandMem(m.newAmodeImmReg(offsetInParamSlice, paramSlicePtr))
	switch arg.Type {
	case ssa.TypeI32:
		load.asMovzxRmR(extModeLQ, a, dst)
	case ssa.TypeI64:
		load.asMov64MR(a, dst)
	case ssa.TypeF32:
		load.asXmmUnaryRmR(sseOpcodeMovss, a, dst)
	case ssa.TypeF64:
		load.asXmmUnaryRmR(sseOpcodeMovsd, a, dst)
	case ssa.TypeV128:
		load.asXmmUnaryRmR(sseOpcodeMovdqu, a, dst)
	}

	cur = linkInstr(cur, load)
	if arg.Kind == backend.ABIArgKindStack {
		// Store back to the stack.
		store := m.allocateInstr()
		a := newOperandMem(m.newAmodeImmReg(uint32(arg.Offset), rspVReg))
		switch arg.Type {
		case ssa.TypeI32:
			store.asMovRM(dst, a, 4)
		case ssa.TypeI64:
			store.asMovRM(dst, a, 8)
		case ssa.TypeF32:
			store.asXmmMovRM(sseOpcodeMovss, dst, a)
		case ssa.TypeF64:
			store.asXmmMovRM(sseOpcodeMovsd, dst, a)
		case ssa.TypeV128:
			store.asXmmMovRM(sseOpcodeMovdqu, dst, a)
		}
		cur = linkInstr(cur, store)
	}
	return cur
}

func (m *machine) goEntryPreamblePassResult(cur *instruction, resultSlicePtr regalloc.VReg, offsetInResultSlice uint32, result *backend.ABIArg, resultStackSlotBeginOffset uint32) *instruction {
	var r regalloc.VReg
	if result.Kind == backend.ABIArgKindStack {
		// Load the value to the temporary.
		load := m.allocateInstr()
		offset := resultStackSlotBeginOffset + uint32(result.Offset)
		a := newOperandMem(m.newAmodeImmReg(offset, rspVReg))
		switch result.Type {
		case ssa.TypeI32:
			r = tmpIntReg
			load.asMovzxRmR(extModeLQ, a, r)
		case ssa.TypeI64:
			r = tmpIntReg
			load.asMov64MR(a, r)
		case ssa.TypeF32:
			r = tmpXmmReg
			load.asXmmUnaryRmR(sseOpcodeMovss, a, r)
		case ssa.TypeF64:
			r = tmpXmmReg
			load.asXmmUnaryRmR(sseOpcodeMovsd, a, r)
		case ssa.TypeV128:
			r = tmpXmmReg
			load.asXmmUnaryRmR(sseOpcodeMovdqu, a, r)
		default:
			panic("BUG")
		}
		cur = linkInstr(cur, load)
	} else {
		r = result.Reg
	}

	store := m.allocateInstr()
	a := newOperandMem(m.newAmodeImmReg(offsetInResultSlice, resultSlicePtr))
	switch result.Type {
	case ssa.TypeI32:
		store.asMovRM(r, a, 4)
	case ssa.TypeI64:
		store.asMovRM(r, a, 8)
	case ssa.TypeF32:
		store.asXmmMovRM(sseOpcodeMovss, r, a)
	case ssa.TypeF64:
		store.asXmmMovRM(sseOpcodeMovsd, r, a)
	case ssa.TypeV128:
		store.asXmmMovRM(sseOpcodeMovdqu, r, a)
	}

	return linkInstr(cur, store)
}
