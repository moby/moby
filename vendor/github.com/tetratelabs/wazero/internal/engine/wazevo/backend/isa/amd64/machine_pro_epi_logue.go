package amd64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
)

// PostRegAlloc implements backend.Machine.
func (m *machine) PostRegAlloc() {
	m.setupPrologue()
	m.postRegAlloc()
}

func (m *machine) setupPrologue() {
	cur := m.rootInstr
	prevInitInst := cur.next

	// At this point, we have the stack layout as follows:
	//
	//                   (high address)
	//                 +-----------------+ <----- RBP (somewhere in the middle of the stack)
	//                 |     .......     |
	//                 |      ret Y      |
	//                 |     .......     |
	//                 |      ret 0      |
	//                 |      arg X      |
	//                 |     .......     |
	//                 |      arg 1      |
	//                 |      arg 0      |
	//                 |   Return Addr   |
	//       RSP ----> +-----------------+
	//                    (low address)

	// First, we push the RBP, and update the RBP to the current RSP.
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

	if !m.stackBoundsCheckDisabled {
		cur = m.insertStackBoundsCheck(m.requiredStackSize(), cur)
	}

	//
	//            (high address)
	//          +-----------------+                  +-----------------+
	//          |     .......     |                  |     .......     |
	//          |      ret Y      |                  |      ret Y      |
	//          |     .......     |                  |     .......     |
	//          |      ret 0      |                  |      ret 0      |
	//          |      arg X      |                  |      arg X      |
	//          |     .......     |                  |     .......     |
	//          |      arg 1      |                  |      arg 1      |
	//          |      arg 0      |                  |      arg 0      |
	//          |      xxxxx      |                  |      xxxxx      |
	//          |   Return Addr   |                  |   Return Addr   |
	//          |    Caller_RBP   |      ====>       |    Caller_RBP   |
	// RBP,RSP->+-----------------+                  +-----------------+ <----- RBP
	//             (low address)                     |   clobbered M   |
	//                                               |   clobbered 1   |
	//                                               |   ...........   |
	//                                               |   clobbered 0   |
	//                                               +-----------------+ <----- RSP
	//
	if regs := m.clobberedRegs; len(regs) > 0 {
		for i := range regs {
			r := regs[len(regs)-1-i] // Reverse order.
			if r.RegType() == regalloc.RegTypeInt {
				cur = linkInstr(cur, m.allocateInstr().asPush64(newOperandReg(r)))
			} else {
				// Push the XMM register is not supported by the PUSH instruction.
				cur = m.addRSP(-16, cur)
				push := m.allocateInstr().asXmmMovRM(
					sseOpcodeMovdqu, r, newOperandMem(m.newAmodeImmReg(0, rspVReg)),
				)
				cur = linkInstr(cur, push)
			}
		}
	}

	if size := m.spillSlotSize; size > 0 {
		// Simply decrease the RSP to allocate the spill slots.
		// 		sub $size, %rsp
		cur = linkInstr(cur, m.allocateInstr().asAluRmiR(aluRmiROpcodeSub, newOperandImm32(uint32(size)), rspVReg, true))

		// At this point, we have the stack layout as follows:
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
		//          |   Caller_RBP    |
		//          +-----------------+ <--- RBP
		//          |    clobbered M  |
		//          |   ............  |
		//          |    clobbered 1  |
		//          |    clobbered 0  |
		//          |   spill slot N  |
		//          |   ............  |
		//          |   spill slot 0  |
		//          +-----------------+ <--- RSP
		//             (low address)
	}

	linkInstr(cur, prevInitInst)
}

// postRegAlloc does multiple things while walking through the instructions:
// 1. Inserts the epilogue code.
// 2. Removes the redundant copy instruction.
// 3. Inserts the dec/inc RSP instruction right before/after the call instruction.
// 4. Lowering that is supposed to be done after regalloc.
func (m *machine) postRegAlloc() {
	for cur := m.rootInstr; cur != nil; cur = cur.next {
		switch k := cur.kind; k {
		case ret:
			m.setupEpilogueAfter(cur.prev)
			continue
		case fcvtToSintSequence, fcvtToUintSequence:
			m.pendingInstructions = m.pendingInstructions[:0]
			if k == fcvtToSintSequence {
				m.lowerFcvtToSintSequenceAfterRegalloc(cur)
			} else {
				m.lowerFcvtToUintSequenceAfterRegalloc(cur)
			}
			prev := cur.prev
			next := cur.next
			cur := prev
			for _, instr := range m.pendingInstructions {
				cur = linkInstr(cur, instr)
			}
			linkInstr(cur, next)
			continue
		case xmmCMov:
			m.pendingInstructions = m.pendingInstructions[:0]
			m.lowerXmmCmovAfterRegAlloc(cur)
			prev := cur.prev
			next := cur.next
			cur := prev
			for _, instr := range m.pendingInstructions {
				cur = linkInstr(cur, instr)
			}
			linkInstr(cur, next)
			continue
		case idivRemSequence:
			m.pendingInstructions = m.pendingInstructions[:0]
			m.lowerIDivRemSequenceAfterRegAlloc(cur)
			prev := cur.prev
			next := cur.next
			cur := prev
			for _, instr := range m.pendingInstructions {
				cur = linkInstr(cur, instr)
			}
			linkInstr(cur, next)
			continue
		case call, callIndirect:
			// At this point, reg alloc is done, therefore we can safely insert dec/inc RPS instruction
			// right before/after the call instruction. If this is done before reg alloc, the stack slot
			// can point to the wrong location and therefore results in a wrong value.
			call := cur
			next := call.next
			_, _, _, _, size := backend.ABIInfoFromUint64(call.u2)
			if size > 0 {
				dec := m.allocateInstr().asAluRmiR(aluRmiROpcodeSub, newOperandImm32(size), rspVReg, true)
				linkInstr(call.prev, dec)
				linkInstr(dec, call)
				inc := m.allocateInstr().asAluRmiR(aluRmiROpcodeAdd, newOperandImm32(size), rspVReg, true)
				linkInstr(call, inc)
				linkInstr(inc, next)
			}
			continue
		}

		// Removes the redundant copy instruction.
		if cur.IsCopy() && cur.op1.reg().RealReg() == cur.op2.reg().RealReg() {
			prev, next := cur.prev, cur.next
			// Remove the copy instruction.
			prev.next = next
			if next != nil {
				next.prev = prev
			}
		}
	}
}

func (m *machine) setupEpilogueAfter(cur *instruction) {
	prevNext := cur.next

	// At this point, we have the stack layout as follows:
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
	//          |   Caller_RBP    |
	//          +-----------------+ <--- RBP
	//          |    clobbered M  |
	//          |   ............  |
	//          |    clobbered 1  |
	//          |    clobbered 0  |
	//          |   spill slot N  |
	//          |   ............  |
	//          |   spill slot 0  |
	//          +-----------------+ <--- RSP
	//             (low address)

	if size := m.spillSlotSize; size > 0 {
		// Simply increase the RSP to free the spill slots.
		// 		add $size, %rsp
		cur = linkInstr(cur, m.allocateInstr().asAluRmiR(aluRmiROpcodeAdd, newOperandImm32(uint32(size)), rspVReg, true))
	}

	//
	//             (high address)
	//            +-----------------+                     +-----------------+
	//            |     .......     |                     |     .......     |
	//            |      ret Y      |                     |      ret Y      |
	//            |     .......     |                     |     .......     |
	//            |      ret 0      |                     |      ret 0      |
	//            |      arg X      |                     |      arg X      |
	//            |     .......     |                     |     .......     |
	//            |      arg 1      |                     |      arg 1      |
	//            |      arg 0      |                     |      arg 0      |
	//            |   ReturnAddress |                     |   ReturnAddress |
	//            |    Caller_RBP   |                     |    Caller_RBP   |
	//   RBP ---> +-----------------+      ========>      +-----------------+ <---- RSP, RBP
	//            |    clobbered M  |
	//            |   ............  |
	//            |    clobbered 1  |
	//            |    clobbered 0  |
	//   RSP ---> +-----------------+
	//               (low address)
	//
	if regs := m.clobberedRegs; len(regs) > 0 {
		for _, r := range regs {
			if r.RegType() == regalloc.RegTypeInt {
				cur = linkInstr(cur, m.allocateInstr().asPop64(r))
			} else {
				// Pop the XMM register is not supported by the POP instruction.
				pop := m.allocateInstr().asXmmUnaryRmR(
					sseOpcodeMovdqu, newOperandMem(m.newAmodeImmReg(0, rspVReg)), r,
				)
				cur = linkInstr(cur, pop)
				cur = m.addRSP(16, cur)
			}
		}
	}

	// Now roll back the RSP to RBP, and pop the caller's RBP.
	cur = m.revertRBPRSP(cur)

	linkInstr(cur, prevNext)
}

func (m *machine) addRSP(offset int32, cur *instruction) *instruction {
	if offset == 0 {
		return cur
	}
	opcode := aluRmiROpcodeAdd
	if offset < 0 {
		opcode = aluRmiROpcodeSub
		offset = -offset
	}
	return linkInstr(cur, m.allocateInstr().asAluRmiR(opcode, newOperandImm32(uint32(offset)), rspVReg, true))
}

func (m *machine) setupRBPRSP(cur *instruction) *instruction {
	cur = linkInstr(cur, m.allocateInstr().asPush64(newOperandReg(rbpVReg)))
	cur = linkInstr(cur, m.allocateInstr().asMovRR(rspVReg, rbpVReg, true))
	return cur
}

func (m *machine) revertRBPRSP(cur *instruction) *instruction {
	cur = linkInstr(cur, m.allocateInstr().asMovRR(rbpVReg, rspVReg, true))
	cur = linkInstr(cur, m.allocateInstr().asPop64(rbpVReg))
	return cur
}
