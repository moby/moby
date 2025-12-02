package arm64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

// lowerConstant allocates a new VReg and inserts the instruction to load the constant value.
func (m *machine) lowerConstant(instr *ssa.Instruction) (vr regalloc.VReg) {
	val := instr.Return()
	valType := val.Type()

	vr = m.compiler.AllocateVReg(valType)
	v := instr.ConstantVal()
	m.insertLoadConstant(v, valType, vr)
	return
}

// InsertLoadConstantBlockArg implements backend.Machine.
func (m *machine) InsertLoadConstantBlockArg(instr *ssa.Instruction, vr regalloc.VReg) {
	val := instr.Return()
	valType := val.Type()
	v := instr.ConstantVal()
	load := m.allocateInstr()
	load.asLoadConstBlockArg(v, valType, vr)
	m.insert(load)
}

func (m *machine) lowerLoadConstantBlockArgAfterRegAlloc(i *instruction) {
	v, typ, dst := i.loadConstBlockArgData()
	m.insertLoadConstant(v, typ, dst)
}

func (m *machine) insertLoadConstant(v uint64, valType ssa.Type, vr regalloc.VReg) {
	if valType.Bits() < 64 { // Clear the redundant bits just in case it's unexpectedly sign-extended, etc.
		v = v & ((1 << valType.Bits()) - 1)
	}

	switch valType {
	case ssa.TypeF32:
		loadF := m.allocateInstr()
		loadF.asLoadFpuConst32(vr, v)
		m.insert(loadF)
	case ssa.TypeF64:
		loadF := m.allocateInstr()
		loadF.asLoadFpuConst64(vr, v)
		m.insert(loadF)
	case ssa.TypeI32:
		if v == 0 {
			m.InsertMove(vr, xzrVReg, ssa.TypeI32)
		} else {
			m.lowerConstantI32(vr, int32(v))
		}
	case ssa.TypeI64:
		if v == 0 {
			m.InsertMove(vr, xzrVReg, ssa.TypeI64)
		} else {
			m.lowerConstantI64(vr, int64(v))
		}
	default:
		panic("TODO")
	}
}

// The following logics are based on the old asm/arm64 package.
// https://github.com/tetratelabs/wazero/blob/39f2ff23a6d609e10c82b9cc0b981f6de5b87a9c/internal/asm/arm64/impl.go

func (m *machine) lowerConstantI32(dst regalloc.VReg, c int32) {
	// Following the logic here:
	// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L1637
	ic := int64(uint32(c))
	if ic >= 0 && (ic <= 0xfff || (ic&0xfff) == 0 && (uint64(ic>>12) <= 0xfff)) {
		if isBitMaskImmediate(uint64(c), false) {
			m.lowerConstViaBitMaskImmediate(uint64(uint32(c)), dst, false)
			return
		}
	}

	if t := const16bitAligned(int64(uint32(c))); t >= 0 {
		// If the const can fit within 16-bit alignment, for example, 0xffff, 0xffff_0000 or 0xffff_0000_0000_0000
		// We could load it into temporary with movk.
		m.insertMOVZ(dst, uint64(uint32(c)>>(16*t)), t, false)
	} else if t := const16bitAligned(int64(^c)); t >= 0 {
		// Also, if the inverse of the const can fit within 16-bit range, do the same ^^.
		m.insertMOVN(dst, uint64(^c>>(16*t)), t, false)
	} else if isBitMaskImmediate(uint64(uint32(c)), false) {
		m.lowerConstViaBitMaskImmediate(uint64(c), dst, false)
	} else {
		// Otherwise, we use MOVZ and MOVK to load it.
		c16 := uint16(c)
		m.insertMOVZ(dst, uint64(c16), 0, false)
		c16 = uint16(uint32(c) >> 16)
		m.insertMOVK(dst, uint64(c16), 1, false)
	}
}

func (m *machine) lowerConstantI64(dst regalloc.VReg, c int64) {
	// Following the logic here:
	// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L1798-L1852
	if c >= 0 && (c <= 0xfff || (c&0xfff) == 0 && (uint64(c>>12) <= 0xfff)) {
		if isBitMaskImmediate(uint64(c), true) {
			m.lowerConstViaBitMaskImmediate(uint64(c), dst, true)
			return
		}
	}

	if t := const16bitAligned(c); t >= 0 {
		// If the const can fit within 16-bit alignment, for example, 0xffff, 0xffff_0000 or 0xffff_0000_0000_0000
		// We could load it into temporary with movk.
		m.insertMOVZ(dst, uint64(c)>>(16*t), t, true)
	} else if t := const16bitAligned(^c); t >= 0 {
		// Also, if the reverse of the const can fit within 16-bit range, do the same ^^.
		m.insertMOVN(dst, uint64(^c)>>(16*t), t, true)
	} else if isBitMaskImmediate(uint64(c), true) {
		m.lowerConstViaBitMaskImmediate(uint64(c), dst, true)
	} else {
		m.load64bitConst(c, dst)
	}
}

func (m *machine) lowerConstViaBitMaskImmediate(c uint64, dst regalloc.VReg, b64 bool) {
	instr := m.allocateInstr()
	instr.asALUBitmaskImm(aluOpOrr, dst, xzrVReg, c, b64)
	m.insert(instr)
}

// isBitMaskImmediate determines if the value can be encoded as "bitmask immediate".
//
//	Such an immediate is a 32-bit or 64-bit pattern viewed as a vector of identical elements of size e = 2, 4, 8, 16, 32, or 64 bits.
//	Each element contains the same sub-pattern: a single run of 1 to e-1 non-zero bits, rotated by 0 to e-1 bits.
//
// See https://developer.arm.com/documentation/dui0802/b/A64-General-Instructions/MOV--bitmask-immediate-
func isBitMaskImmediate(x uint64, _64 bool) bool {
	// All zeros and ones are not "bitmask immediate" by definition.
	if x == 0 || (_64 && x == 0xffff_ffff_ffff_ffff) || (!_64 && x == 0xffff_ffff) {
		return false
	}

	switch {
	case x != x>>32|x<<32:
		// e = 64
	case x != x>>16|x<<48:
		// e = 32 (x == x>>32|x<<32).
		// e.g. 0x00ff_ff00_00ff_ff00
		x = uint64(int32(x))
	case x != x>>8|x<<56:
		// e = 16 (x == x>>16|x<<48).
		// e.g. 0x00ff_00ff_00ff_00ff
		x = uint64(int16(x))
	case x != x>>4|x<<60:
		// e = 8 (x == x>>8|x<<56).
		// e.g. 0x0f0f_0f0f_0f0f_0f0f
		x = uint64(int8(x))
	default:
		// e = 4 or 2.
		return true
	}
	return sequenceOfSetbits(x) || sequenceOfSetbits(^x)
}

// sequenceOfSetbits returns true if the number's binary representation is the sequence set bit (1).
// For example: 0b1110 -> true, 0b1010 -> false
func sequenceOfSetbits(x uint64) bool {
	y := getLowestBit(x)
	// If x is a sequence of set bit, this should results in the number
	// with only one set bit (i.e. power of two).
	y += x
	return (y-1)&y == 0
}

func getLowestBit(x uint64) uint64 {
	return x & (^x + 1)
}

// const16bitAligned check if the value is on the 16-bit alignment.
// If so, returns the shift num divided by 16, and otherwise -1.
func const16bitAligned(v int64) (ret int) {
	ret = -1
	for s := 0; s < 64; s += 16 {
		if (uint64(v) &^ (uint64(0xffff) << uint(s))) == 0 {
			ret = s / 16
			break
		}
	}
	return
}

// load64bitConst loads a 64-bit constant into the register, following the same logic to decide how to load large 64-bit
// consts as in the Go assembler.
//
// See https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L6632-L6759
func (m *machine) load64bitConst(c int64, dst regalloc.VReg) {
	var bits [4]uint64
	var zeros, negs int
	for i := 0; i < 4; i++ {
		bits[i] = uint64(c) >> uint(i*16) & 0xffff
		if v := bits[i]; v == 0 {
			zeros++
		} else if v == 0xffff {
			negs++
		}
	}

	if zeros == 3 {
		// one MOVZ instruction.
		for i, v := range bits {
			if v != 0 {
				m.insertMOVZ(dst, v, i, true)
			}
		}
	} else if negs == 3 {
		// one MOVN instruction.
		for i, v := range bits {
			if v != 0xffff {
				v = ^v
				m.insertMOVN(dst, v, i, true)
			}
		}
	} else if zeros == 2 {
		// one MOVZ then one OVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				m.insertMOVZ(dst, v, i, true)
				movz = true
			} else if v != 0 {
				m.insertMOVK(dst, v, i, true)
			}
		}

	} else if negs == 2 {
		// one MOVN then one or two MOVK.
		var movn bool
		for i, v := range bits { // Emit MOVN.
			if !movn && v != 0xffff {
				v = ^v
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVN
				m.insertMOVN(dst, v, i, true)
				movn = true
			} else if v != 0xffff {
				m.insertMOVK(dst, v, i, true)
			}
		}

	} else if zeros == 1 {
		// one MOVZ then two MOVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				m.insertMOVZ(dst, v, i, true)
				movz = true
			} else if v != 0 {
				m.insertMOVK(dst, v, i, true)
			}
		}

	} else if negs == 1 {
		// one MOVN then two MOVK.
		var movn bool
		for i, v := range bits { // Emit MOVN.
			if !movn && v != 0xffff {
				v = ^v
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVN
				m.insertMOVN(dst, v, i, true)
				movn = true
			} else if v != 0xffff {
				m.insertMOVK(dst, v, i, true)
			}
		}

	} else {
		// one MOVZ then up to three MOVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				m.insertMOVZ(dst, v, i, true)
				movz = true
			} else if v != 0 {
				m.insertMOVK(dst, v, i, true)
			}
		}
	}
}

func (m *machine) insertMOVZ(dst regalloc.VReg, v uint64, shift int, dst64 bool) {
	instr := m.allocateInstr()
	instr.asMOVZ(dst, v, uint32(shift), dst64)
	m.insert(instr)
}

func (m *machine) insertMOVK(dst regalloc.VReg, v uint64, shift int, dst64 bool) {
	instr := m.allocateInstr()
	instr.asMOVK(dst, v, uint32(shift), dst64)
	m.insert(instr)
}

func (m *machine) insertMOVN(dst regalloc.VReg, v uint64, shift int, dst64 bool) {
	instr := m.allocateInstr()
	instr.asMOVN(dst, v, uint32(shift), dst64)
	m.insert(instr)
}
