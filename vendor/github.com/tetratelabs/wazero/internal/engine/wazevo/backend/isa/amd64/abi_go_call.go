package amd64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

var calleeSavedVRegs = []regalloc.VReg{
	rdxVReg, r12VReg, r13VReg, r14VReg, r15VReg,
	xmm8VReg, xmm9VReg, xmm10VReg, xmm11VReg, xmm12VReg, xmm13VReg, xmm14VReg, xmm15VReg,
}

// CompileGoFunctionTrampoline implements backend.Machine.
func (m *machine) CompileGoFunctionTrampoline(exitCode wazevoapi.ExitCode, sig *ssa.Signature, needModuleContextPtr bool) []byte {
	argBegin := 1 // Skips exec context by default.
	if needModuleContextPtr {
		argBegin++
	}

	abi := &backend.FunctionABI{}
	abi.Init(sig, intArgResultRegs, floatArgResultRegs)
	m.currentABI = abi

	cur := m.allocateNop()
	m.rootInstr = cur

	// Execution context is always the first argument.
	execCtrPtr := raxVReg

	// First we update RBP and RSP just like the normal prologue.
	//
	//                   (high address)                     (high address)
	//       RBP ----> +-----------------+                +-----------------+
	//                 |     .......     |                |     .......     |
	//                 |      ret Y      |                |      ret Y      |
	//                 |     .......     |                |     .......     |
	//                 |      ret 0      |                |      ret 0      |
	//                 |      arg X      |                |      arg X      |
	//                 |     .......     |     ====>      |     .......     |
	//                 |      arg 1      |                |      arg 1      |
	//                 |      arg 0      |                |      arg 0      |
	//                 |   Return Addr   |                |   Return Addr   |
	//       RSP ----> +-----------------+                |    Caller_RBP   |
	//                    (low address)                   +-----------------+ <----- RSP, RBP
	//
	cur = m.setupRBPRSP(cur)

	goSliceSizeAligned, goSliceSizeAlignedUnaligned := backend.GoFunctionCallRequiredStackSize(sig, argBegin)
	cur = m.insertStackBoundsCheck(goSliceSizeAligned+8 /* size of the Go slice */, cur)

	// Save the callee saved registers.
	cur = m.saveRegistersInExecutionContext(cur, execCtrPtr, calleeSavedVRegs)

	if needModuleContextPtr {
		moduleCtrPtr := rbxVReg // Module context is always the second argument.
		mem := m.newAmodeImmReg(
			wazevoapi.ExecutionContextOffsetGoFunctionCallCalleeModuleContextOpaque.U32(),
			execCtrPtr)
		store := m.allocateInstr().asMovRM(moduleCtrPtr, newOperandMem(mem), 8)
		cur = linkInstr(cur, store)
	}

	// Now let's advance the RSP to the stack slot for the arguments.
	//
	//                (high address)                     (high address)
	//              +-----------------+               +-----------------+
	//              |     .......     |               |     .......     |
	//              |      ret Y      |               |      ret Y      |
	//              |     .......     |               |     .......     |
	//              |      ret 0      |               |      ret 0      |
	//              |      arg X      |               |      arg X      |
	//              |     .......     |   =======>    |     .......     |
	//              |      arg 1      |               |      arg 1      |
	//              |      arg 0      |               |      arg 0      |
	//              |   Return Addr   |               |   Return Addr   |
	//              |    Caller_RBP   |               |    Caller_RBP   |
	//  RBP,RSP --> +-----------------+               +-----------------+ <----- RBP
	//                 (low address)                  |  arg[N]/ret[M]  |
	//                                                |    ..........   |
	//                                                |  arg[1]/ret[1]  |
	//                                                |  arg[0]/ret[0]  |
	//                                                +-----------------+ <----- RSP
	//                                                   (low address)
	//
	// where the region of "arg[0]/ret[0] ... arg[N]/ret[M]" is the stack used by the Go functions,
	// therefore will be accessed as the usual []uint64. So that's where we need to pass/receive
	// the arguments/return values to/from Go function.
	cur = m.addRSP(-int32(goSliceSizeAligned), cur)

	// Next, we need to store all the arguments to the stack in the typical Wasm stack style.
	var offsetInGoSlice int32
	for i := range abi.Args[argBegin:] {
		arg := &abi.Args[argBegin+i]
		var v regalloc.VReg
		if arg.Kind == backend.ABIArgKindReg {
			v = arg.Reg
		} else {
			// We have saved callee saved registers, so we can use them.
			if arg.Type.IsInt() {
				v = r15VReg
			} else {
				v = xmm15VReg
			}
			mem := newOperandMem(m.newAmodeImmReg(uint32(arg.Offset+16 /* to skip caller_rbp and ret_addr */), rbpVReg))
			load := m.allocateInstr()
			switch arg.Type {
			case ssa.TypeI32:
				load.asMovzxRmR(extModeLQ, mem, v)
			case ssa.TypeI64:
				load.asMov64MR(mem, v)
			case ssa.TypeF32:
				load.asXmmUnaryRmR(sseOpcodeMovss, mem, v)
			case ssa.TypeF64:
				load.asXmmUnaryRmR(sseOpcodeMovsd, mem, v)
			case ssa.TypeV128:
				load.asXmmUnaryRmR(sseOpcodeMovdqu, mem, v)
			default:
				panic("BUG")
			}
			cur = linkInstr(cur, load)
		}

		store := m.allocateInstr()
		mem := newOperandMem(m.newAmodeImmReg(uint32(offsetInGoSlice), rspVReg))
		switch arg.Type {
		case ssa.TypeI32:
			store.asMovRM(v, mem, 4)
			offsetInGoSlice += 8 // always uint64 rep.
		case ssa.TypeI64:
			store.asMovRM(v, mem, 8)
			offsetInGoSlice += 8
		case ssa.TypeF32:
			store.asXmmMovRM(sseOpcodeMovss, v, mem)
			offsetInGoSlice += 8 // always uint64 rep.
		case ssa.TypeF64:
			store.asXmmMovRM(sseOpcodeMovsd, v, mem)
			offsetInGoSlice += 8
		case ssa.TypeV128:
			store.asXmmMovRM(sseOpcodeMovdqu, v, mem)
			offsetInGoSlice += 16
		default:
			panic("BUG")
		}
		cur = linkInstr(cur, store)
	}

	// Finally we push the size of the slice to the stack so the stack looks like:
	//
	//          (high address)
	//       +-----------------+
	//       |     .......     |
	//       |      ret Y      |
	//       |     .......     |
	//       |      ret 0      |
	//       |      arg X      |
	//       |     .......     |
	//       |      arg 1      |
	//       |      arg 0      |
	//       |   Return Addr   |
	//       |    Caller_RBP   |
	//       +-----------------+ <----- RBP
	//       |  arg[N]/ret[M]  |
	//       |    ..........   |
	//       |  arg[1]/ret[1]  |
	//       |  arg[0]/ret[0]  |
	//       |    slice size   |
	//       +-----------------+ <----- RSP
	//         (low address)
	//
	// 		push $sliceSize
	cur = linkInstr(cur, m.allocateInstr().asPush64(newOperandImm32(uint32(goSliceSizeAlignedUnaligned))))

	// Load the exitCode to the register.
	exitCodeReg := r12VReg // Callee saved which is already saved.
	cur = linkInstr(cur, m.allocateInstr().asImm(exitCodeReg, uint64(exitCode), false))

	saveRsp, saveRbp, setExitCode := m.allocateExitInstructions(execCtrPtr, exitCodeReg)
	cur = linkInstr(cur, setExitCode)
	cur = linkInstr(cur, saveRsp)
	cur = linkInstr(cur, saveRbp)

	// Ready to exit the execution.
	cur = m.storeReturnAddressAndExit(cur, execCtrPtr)

	// We don't need the slice size anymore, so pop it.
	cur = m.addRSP(8, cur)

	// Ready to set up the results.
	offsetInGoSlice = 0
	// To avoid overwriting with the execution context pointer by the result, we need to track the offset,
	// and defer the restoration of the result to the end of this function.
	var argOverlapWithExecCtxOffset int32 = -1
	for i := range abi.Rets {
		r := &abi.Rets[i]
		var v regalloc.VReg
		isRegResult := r.Kind == backend.ABIArgKindReg
		if isRegResult {
			v = r.Reg
			if v.RealReg() == execCtrPtr.RealReg() {
				argOverlapWithExecCtxOffset = offsetInGoSlice
				offsetInGoSlice += 8 // always uint64 rep.
				continue
			}
		} else {
			if r.Type.IsInt() {
				v = r15VReg
			} else {
				v = xmm15VReg
			}
		}

		load := m.allocateInstr()
		mem := newOperandMem(m.newAmodeImmReg(uint32(offsetInGoSlice), rspVReg))
		switch r.Type {
		case ssa.TypeI32:
			load.asMovzxRmR(extModeLQ, mem, v)
			offsetInGoSlice += 8 // always uint64 rep.
		case ssa.TypeI64:
			load.asMov64MR(mem, v)
			offsetInGoSlice += 8
		case ssa.TypeF32:
			load.asXmmUnaryRmR(sseOpcodeMovss, mem, v)
			offsetInGoSlice += 8 // always uint64 rep.
		case ssa.TypeF64:
			load.asXmmUnaryRmR(sseOpcodeMovsd, mem, v)
			offsetInGoSlice += 8
		case ssa.TypeV128:
			load.asXmmUnaryRmR(sseOpcodeMovdqu, mem, v)
			offsetInGoSlice += 16
		default:
			panic("BUG")
		}
		cur = linkInstr(cur, load)

		if !isRegResult {
			// We need to store it back to the result slot above rbp.
			store := m.allocateInstr()
			mem := newOperandMem(m.newAmodeImmReg(uint32(abi.ArgStackSize+r.Offset+16 /* to skip caller_rbp and ret_addr */), rbpVReg))
			switch r.Type {
			case ssa.TypeI32:
				store.asMovRM(v, mem, 4)
			case ssa.TypeI64:
				store.asMovRM(v, mem, 8)
			case ssa.TypeF32:
				store.asXmmMovRM(sseOpcodeMovss, v, mem)
			case ssa.TypeF64:
				store.asXmmMovRM(sseOpcodeMovsd, v, mem)
			case ssa.TypeV128:
				store.asXmmMovRM(sseOpcodeMovdqu, v, mem)
			default:
				panic("BUG")
			}
			cur = linkInstr(cur, store)
		}
	}

	// Before return, we need to restore the callee saved registers.
	cur = m.restoreRegistersInExecutionContext(cur, execCtrPtr, calleeSavedVRegs)

	if argOverlapWithExecCtxOffset >= 0 {
		// At this point execCtt is not used anymore, so we can finally store the
		// result to the register which overlaps with the execution context pointer.
		mem := newOperandMem(m.newAmodeImmReg(uint32(argOverlapWithExecCtxOffset), rspVReg))
		load := m.allocateInstr().asMov64MR(mem, execCtrPtr)
		cur = linkInstr(cur, load)
	}

	// Finally ready to return.
	cur = m.revertRBPRSP(cur)
	linkInstr(cur, m.allocateInstr().asRet())

	m.encodeWithoutSSA(m.rootInstr)
	return m.c.Buf()
}

func (m *machine) saveRegistersInExecutionContext(cur *instruction, execCtx regalloc.VReg, regs []regalloc.VReg) *instruction {
	offset := wazevoapi.ExecutionContextOffsetSavedRegistersBegin.I64()
	for _, v := range regs {
		store := m.allocateInstr()
		mem := newOperandMem(m.newAmodeImmReg(uint32(offset), execCtx))
		switch v.RegType() {
		case regalloc.RegTypeInt:
			store.asMovRM(v, mem, 8)
		case regalloc.RegTypeFloat:
			store.asXmmMovRM(sseOpcodeMovdqu, v, mem)
		default:
			panic("BUG")
		}
		cur = linkInstr(cur, store)
		offset += 16 // See execution context struct. Each register is 16 bytes-aligned unconditionally.
	}
	return cur
}

func (m *machine) restoreRegistersInExecutionContext(cur *instruction, execCtx regalloc.VReg, regs []regalloc.VReg) *instruction {
	offset := wazevoapi.ExecutionContextOffsetSavedRegistersBegin.I64()
	for _, v := range regs {
		load := m.allocateInstr()
		mem := newOperandMem(m.newAmodeImmReg(uint32(offset), execCtx))
		switch v.RegType() {
		case regalloc.RegTypeInt:
			load.asMov64MR(mem, v)
		case regalloc.RegTypeFloat:
			load.asXmmUnaryRmR(sseOpcodeMovdqu, mem, v)
		default:
			panic("BUG")
		}
		cur = linkInstr(cur, load)
		offset += 16 // See execution context struct. Each register is 16 bytes-aligned unconditionally.
	}
	return cur
}

func (m *machine) storeReturnAddressAndExit(cur *instruction, execCtx regalloc.VReg) *instruction {
	readRip := m.allocateInstr()
	cur = linkInstr(cur, readRip)

	ripReg := r12VReg // Callee saved which is already saved.
	saveRip := m.allocateInstr().asMovRM(
		ripReg,
		newOperandMem(m.newAmodeImmReg(wazevoapi.ExecutionContextOffsetGoCallReturnAddress.U32(), execCtx)),
		8,
	)
	cur = linkInstr(cur, saveRip)

	exit := m.allocateExitSeq(execCtx)
	cur = linkInstr(cur, exit)

	nop, l := m.allocateBrTarget()
	cur = linkInstr(cur, nop)
	readRip.asLEA(newOperandLabel(l), ripReg)
	return cur
}

// saveRequiredRegs is the set of registers that must be saved/restored during growing stack when there's insufficient
// stack space left. Basically this is the all allocatable registers except for RSP and RBP, and RAX which contains the
// execution context pointer. ExecCtx pointer is always the first argument so we don't need to save it.
var stackGrowSaveVRegs = []regalloc.VReg{
	rdxVReg, r12VReg, r13VReg, r14VReg, r15VReg,
	rcxVReg, rbxVReg, rsiVReg, rdiVReg, r8VReg, r9VReg, r10VReg, r11VReg,
	xmm8VReg, xmm9VReg, xmm10VReg, xmm11VReg, xmm12VReg, xmm13VReg, xmm14VReg, xmm15VReg,
	xmm0VReg, xmm1VReg, xmm2VReg, xmm3VReg, xmm4VReg, xmm5VReg, xmm6VReg, xmm7VReg,
}

// CompileStackGrowCallSequence implements backend.Machine.
func (m *machine) CompileStackGrowCallSequence() []byte {
	cur := m.allocateNop()
	m.rootInstr = cur

	cur = m.setupRBPRSP(cur)

	// Execution context is always the first argument.
	execCtrPtr := raxVReg

	// Save the callee saved and argument registers.
	cur = m.saveRegistersInExecutionContext(cur, execCtrPtr, stackGrowSaveVRegs)

	// Load the exitCode to the register.
	exitCodeReg := r12VReg // Already saved.
	cur = linkInstr(cur, m.allocateInstr().asImm(exitCodeReg, uint64(wazevoapi.ExitCodeGrowStack), false))

	saveRsp, saveRbp, setExitCode := m.allocateExitInstructions(execCtrPtr, exitCodeReg)
	cur = linkInstr(cur, setExitCode)
	cur = linkInstr(cur, saveRsp)
	cur = linkInstr(cur, saveRbp)

	// Ready to exit the execution.
	cur = m.storeReturnAddressAndExit(cur, execCtrPtr)

	// After the exit, restore the saved registers.
	cur = m.restoreRegistersInExecutionContext(cur, execCtrPtr, stackGrowSaveVRegs)

	// Finally ready to return.
	cur = m.revertRBPRSP(cur)
	linkInstr(cur, m.allocateInstr().asRet())

	m.encodeWithoutSSA(m.rootInstr)
	return m.c.Buf()
}

// insertStackBoundsCheck will insert the instructions after `cur` to check the
// stack bounds, and if there's no sufficient spaces required for the function,
// exit the execution and try growing it in Go world.
func (m *machine) insertStackBoundsCheck(requiredStackSize int64, cur *instruction) *instruction {
	//		add $requiredStackSize, %rsp ;; Temporarily update the sp.
	// 		cmp ExecutionContextOffsetStackBottomPtr(%rax), %rsp ;; Compare the stack bottom and the sp.
	// 		ja .ok
	//		sub $requiredStackSize, %rsp ;; Reverse the temporary update.
	//      pushq r15 ;; save the temporary.
	//		mov $requiredStackSize, %r15
	//		mov %15, ExecutionContextOffsetStackGrowRequiredSize(%rax) ;; Set the required size in the execution context.
	//      popq r15 ;; restore the temporary.
	//		callq *ExecutionContextOffsetStackGrowCallTrampolineAddress(%rax) ;; Call the Go function to grow the stack.
	//		jmp .cont
	// .ok:
	//		sub $requiredStackSize, %rsp ;; Reverse the temporary update.
	// .cont:
	cur = m.addRSP(-int32(requiredStackSize), cur)
	cur = linkInstr(cur, m.allocateInstr().asCmpRmiR(true,
		newOperandMem(m.newAmodeImmReg(wazevoapi.ExecutionContextOffsetStackBottomPtr.U32(), raxVReg)),
		rspVReg, true))

	ja := m.allocateInstr()
	cur = linkInstr(cur, ja)

	cur = m.addRSP(int32(requiredStackSize), cur)

	// Save the temporary.

	cur = linkInstr(cur, m.allocateInstr().asPush64(newOperandReg(r15VReg)))
	// Load the required size to the temporary.
	cur = linkInstr(cur, m.allocateInstr().asImm(r15VReg, uint64(requiredStackSize), true))
	// Set the required size in the execution context.
	cur = linkInstr(cur, m.allocateInstr().asMovRM(r15VReg,
		newOperandMem(m.newAmodeImmReg(wazevoapi.ExecutionContextOffsetStackGrowRequiredSize.U32(), raxVReg)), 8))
	// Restore the temporary.
	cur = linkInstr(cur, m.allocateInstr().asPop64(r15VReg))
	// Call the Go function to grow the stack.
	cur = linkInstr(cur, m.allocateInstr().asCallIndirect(newOperandMem(m.newAmodeImmReg(
		wazevoapi.ExecutionContextOffsetStackGrowCallTrampolineAddress.U32(), raxVReg)), nil))
	// Jump to the continuation.
	jmpToCont := m.allocateInstr()
	cur = linkInstr(cur, jmpToCont)

	// .ok:
	okInstr, ok := m.allocateBrTarget()
	cur = linkInstr(cur, okInstr)
	ja.asJmpIf(condNBE, newOperandLabel(ok))
	// On the ok path, we only need to reverse the temporary update.
	cur = m.addRSP(int32(requiredStackSize), cur)

	// .cont:
	contInstr, cont := m.allocateBrTarget()
	cur = linkInstr(cur, contInstr)
	jmpToCont.asJmp(newOperandLabel(cont))

	return cur
}
