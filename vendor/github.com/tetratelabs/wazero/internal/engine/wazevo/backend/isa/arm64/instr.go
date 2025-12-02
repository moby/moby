package arm64

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

type (
	// instruction represents either a real instruction in arm64, or the meta instructions
	// that are convenient for code generation. For example, inline constants are also treated
	// as instructions.
	//
	// Basically, each instruction knows how to get encoded in binaries. Hence, the final output of compilation
	// can be considered equivalent to the sequence of such instructions.
	//
	// Each field is interpreted depending on the kind.
	//
	// TODO: optimize the layout later once the impl settles.
	instruction struct {
		prev, next          *instruction
		u1, u2              uint64
		rd                  regalloc.VReg
		rm, rn              operand
		kind                instructionKind
		addedBeforeRegAlloc bool
	}

	// instructionKind represents the kind of instruction.
	// This controls how the instruction struct is interpreted.
	instructionKind byte
)

// IsCall implements regalloc.Instr IsCall.
func (i *instruction) IsCall() bool {
	return i.kind == call
}

// IsIndirectCall implements regalloc.Instr IsIndirectCall.
func (i *instruction) IsIndirectCall() bool {
	return i.kind == callInd
}

// IsReturn implements regalloc.Instr IsReturn.
func (i *instruction) IsReturn() bool {
	return i.kind == ret
}

type defKind byte

const (
	defKindNone defKind = iota + 1
	defKindRD
	defKindCall
)

var defKinds = [numInstructionKinds]defKind{
	adr:                  defKindRD,
	aluRRR:               defKindRD,
	aluRRRR:              defKindRD,
	aluRRImm12:           defKindRD,
	aluRRBitmaskImm:      defKindRD,
	aluRRRShift:          defKindRD,
	aluRRImmShift:        defKindRD,
	aluRRRExtend:         defKindRD,
	bitRR:                defKindRD,
	movZ:                 defKindRD,
	movK:                 defKindRD,
	movN:                 defKindRD,
	mov32:                defKindRD,
	mov64:                defKindRD,
	fpuMov64:             defKindRD,
	fpuMov128:            defKindRD,
	fpuRR:                defKindRD,
	fpuRRR:               defKindRD,
	nop0:                 defKindNone,
	call:                 defKindCall,
	callInd:              defKindCall,
	ret:                  defKindNone,
	store8:               defKindNone,
	store16:              defKindNone,
	store32:              defKindNone,
	store64:              defKindNone,
	exitSequence:         defKindNone,
	condBr:               defKindNone,
	br:                   defKindNone,
	brTableSequence:      defKindNone,
	cSet:                 defKindRD,
	extend:               defKindRD,
	fpuCmp:               defKindNone,
	uLoad8:               defKindRD,
	uLoad16:              defKindRD,
	uLoad32:              defKindRD,
	sLoad8:               defKindRD,
	sLoad16:              defKindRD,
	sLoad32:              defKindRD,
	uLoad64:              defKindRD,
	fpuLoad32:            defKindRD,
	fpuLoad64:            defKindRD,
	fpuLoad128:           defKindRD,
	vecLoad1R:            defKindRD,
	loadFpuConst32:       defKindRD,
	loadFpuConst64:       defKindRD,
	loadFpuConst128:      defKindRD,
	fpuStore32:           defKindNone,
	fpuStore64:           defKindNone,
	fpuStore128:          defKindNone,
	udf:                  defKindNone,
	cSel:                 defKindRD,
	fpuCSel:              defKindRD,
	movToVec:             defKindRD,
	movFromVec:           defKindRD,
	movFromVecSigned:     defKindRD,
	vecDup:               defKindRD,
	vecDupElement:        defKindRD,
	vecExtract:           defKindRD,
	vecMisc:              defKindRD,
	vecMovElement:        defKindRD,
	vecLanes:             defKindRD,
	vecShiftImm:          defKindRD,
	vecTbl:               defKindRD,
	vecTbl2:              defKindRD,
	vecPermute:           defKindRD,
	vecRRR:               defKindRD,
	vecRRRRewrite:        defKindNone,
	fpuToInt:             defKindRD,
	intToFpu:             defKindRD,
	cCmpImm:              defKindNone,
	movToFPSR:            defKindNone,
	movFromFPSR:          defKindRD,
	emitSourceOffsetInfo: defKindNone,
	atomicRmw:            defKindRD,
	atomicCas:            defKindNone,
	atomicLoad:           defKindRD,
	atomicStore:          defKindNone,
	dmb:                  defKindNone,
	loadConstBlockArg:    defKindRD,
}

// Defs returns the list of regalloc.VReg that are defined by the instruction.
// In order to reduce the number of allocations, the caller can pass the slice to be used.
func (i *instruction) Defs(regs *[]regalloc.VReg) []regalloc.VReg {
	*regs = (*regs)[:0]
	switch defKinds[i.kind] {
	case defKindNone:
	case defKindRD:
		*regs = append(*regs, i.rd)
	case defKindCall:
		_, _, retIntRealRegs, retFloatRealRegs, _ := backend.ABIInfoFromUint64(i.u2)
		for i := byte(0); i < retIntRealRegs; i++ {
			*regs = append(*regs, regInfo.RealRegToVReg[intParamResultRegs[i]])
		}
		for i := byte(0); i < retFloatRealRegs; i++ {
			*regs = append(*regs, regInfo.RealRegToVReg[floatParamResultRegs[i]])
		}
	default:
		panic(fmt.Sprintf("defKind for %v not defined", i))
	}
	return *regs
}

// AssignDef implements regalloc.Instr AssignDef.
func (i *instruction) AssignDef(reg regalloc.VReg) {
	switch defKinds[i.kind] {
	case defKindNone:
	case defKindRD:
		i.rd = reg
	case defKindCall:
		panic("BUG: call instructions shouldn't be assigned")
	default:
		panic(fmt.Sprintf("defKind for %v not defined", i))
	}
}

type useKind byte

const (
	useKindNone useKind = iota + 1
	useKindRN
	useKindRNRM
	useKindRNRMRA
	useKindRNRN1RM
	useKindCall
	useKindCallInd
	useKindAMode
	useKindRNAMode
	useKindCond
	// useKindRDRewrite indicates an instruction where RD is used both as a source and destination.
	// A temporary register for RD must be allocated explicitly with the source copied to this
	// register before the instruction and the value copied from this register to the instruction
	// return register.
	useKindRDRewrite
)

var useKinds = [numInstructionKinds]useKind{
	udf:                  useKindNone,
	aluRRR:               useKindRNRM,
	aluRRRR:              useKindRNRMRA,
	aluRRImm12:           useKindRN,
	aluRRBitmaskImm:      useKindRN,
	aluRRRShift:          useKindRNRM,
	aluRRImmShift:        useKindRN,
	aluRRRExtend:         useKindRNRM,
	bitRR:                useKindRN,
	movZ:                 useKindNone,
	movK:                 useKindNone,
	movN:                 useKindNone,
	mov32:                useKindRN,
	mov64:                useKindRN,
	fpuMov64:             useKindRN,
	fpuMov128:            useKindRN,
	fpuRR:                useKindRN,
	fpuRRR:               useKindRNRM,
	nop0:                 useKindNone,
	call:                 useKindCall,
	callInd:              useKindCallInd,
	ret:                  useKindNone,
	store8:               useKindRNAMode,
	store16:              useKindRNAMode,
	store32:              useKindRNAMode,
	store64:              useKindRNAMode,
	exitSequence:         useKindRN,
	condBr:               useKindCond,
	br:                   useKindNone,
	brTableSequence:      useKindRN,
	cSet:                 useKindNone,
	extend:               useKindRN,
	fpuCmp:               useKindRNRM,
	uLoad8:               useKindAMode,
	uLoad16:              useKindAMode,
	uLoad32:              useKindAMode,
	sLoad8:               useKindAMode,
	sLoad16:              useKindAMode,
	sLoad32:              useKindAMode,
	uLoad64:              useKindAMode,
	fpuLoad32:            useKindAMode,
	fpuLoad64:            useKindAMode,
	fpuLoad128:           useKindAMode,
	fpuStore32:           useKindRNAMode,
	fpuStore64:           useKindRNAMode,
	fpuStore128:          useKindRNAMode,
	loadFpuConst32:       useKindNone,
	loadFpuConst64:       useKindNone,
	loadFpuConst128:      useKindNone,
	vecLoad1R:            useKindRN,
	cSel:                 useKindRNRM,
	fpuCSel:              useKindRNRM,
	movToVec:             useKindRN,
	movFromVec:           useKindRN,
	movFromVecSigned:     useKindRN,
	vecDup:               useKindRN,
	vecDupElement:        useKindRN,
	vecExtract:           useKindRNRM,
	cCmpImm:              useKindRN,
	vecMisc:              useKindRN,
	vecMovElement:        useKindRN,
	vecLanes:             useKindRN,
	vecShiftImm:          useKindRN,
	vecTbl:               useKindRNRM,
	vecTbl2:              useKindRNRN1RM,
	vecRRR:               useKindRNRM,
	vecRRRRewrite:        useKindRDRewrite,
	vecPermute:           useKindRNRM,
	fpuToInt:             useKindRN,
	intToFpu:             useKindRN,
	movToFPSR:            useKindRN,
	movFromFPSR:          useKindNone,
	adr:                  useKindNone,
	emitSourceOffsetInfo: useKindNone,
	atomicRmw:            useKindRNRM,
	atomicCas:            useKindRDRewrite,
	atomicLoad:           useKindRN,
	atomicStore:          useKindRNRM,
	loadConstBlockArg:    useKindNone,
	dmb:                  useKindNone,
}

// Uses returns the list of regalloc.VReg that are used by the instruction.
// In order to reduce the number of allocations, the caller can pass the slice to be used.
func (i *instruction) Uses(regs *[]regalloc.VReg) []regalloc.VReg {
	*regs = (*regs)[:0]
	switch useKinds[i.kind] {
	case useKindNone:
	case useKindRN:
		if rn := i.rn.reg(); rn.Valid() {
			*regs = append(*regs, rn)
		}
	case useKindRNRM:
		if rn := i.rn.reg(); rn.Valid() {
			*regs = append(*regs, rn)
		}
		if rm := i.rm.reg(); rm.Valid() {
			*regs = append(*regs, rm)
		}
	case useKindRNRMRA:
		if rn := i.rn.reg(); rn.Valid() {
			*regs = append(*regs, rn)
		}
		if rm := i.rm.reg(); rm.Valid() {
			*regs = append(*regs, rm)
		}
		if ra := regalloc.VReg(i.u2); ra.Valid() {
			*regs = append(*regs, ra)
		}
	case useKindRNRN1RM:
		if rn := i.rn.reg(); rn.Valid() && rn.IsRealReg() {
			rn1 := regalloc.FromRealReg(rn.RealReg()+1, rn.RegType())
			*regs = append(*regs, rn, rn1)
		}
		if rm := i.rm.reg(); rm.Valid() {
			*regs = append(*regs, rm)
		}
	case useKindAMode:
		amode := i.getAmode()
		if amodeRN := amode.rn; amodeRN.Valid() {
			*regs = append(*regs, amodeRN)
		}
		if amodeRM := amode.rm; amodeRM.Valid() {
			*regs = append(*regs, amodeRM)
		}
	case useKindRNAMode:
		*regs = append(*regs, i.rn.reg())
		amode := i.getAmode()
		if amodeRN := amode.rn; amodeRN.Valid() {
			*regs = append(*regs, amodeRN)
		}
		if amodeRM := amode.rm; amodeRM.Valid() {
			*regs = append(*regs, amodeRM)
		}
	case useKindCond:
		cnd := cond(i.u1)
		if cnd.kind() != condKindCondFlagSet {
			*regs = append(*regs, cnd.register())
		}
	case useKindCallInd:
		*regs = append(*regs, i.rn.nr())
		fallthrough
	case useKindCall:
		argIntRealRegs, argFloatRealRegs, _, _, _ := backend.ABIInfoFromUint64(i.u2)
		for i := byte(0); i < argIntRealRegs; i++ {
			*regs = append(*regs, regInfo.RealRegToVReg[intParamResultRegs[i]])
		}
		for i := byte(0); i < argFloatRealRegs; i++ {
			*regs = append(*regs, regInfo.RealRegToVReg[floatParamResultRegs[i]])
		}
	case useKindRDRewrite:
		*regs = append(*regs, i.rn.reg())
		*regs = append(*regs, i.rm.reg())
		*regs = append(*regs, i.rd)
	default:
		panic(fmt.Sprintf("useKind for %v not defined", i))
	}
	return *regs
}

func (i *instruction) AssignUse(index int, reg regalloc.VReg) {
	switch useKinds[i.kind] {
	case useKindNone:
	case useKindRN:
		if rn := i.rn.reg(); rn.Valid() {
			i.rn = i.rn.assignReg(reg)
		}
	case useKindRNRM:
		if index == 0 {
			if rn := i.rn.reg(); rn.Valid() {
				i.rn = i.rn.assignReg(reg)
			}
		} else {
			if rm := i.rm.reg(); rm.Valid() {
				i.rm = i.rm.assignReg(reg)
			}
		}
	case useKindRDRewrite:
		if index == 0 {
			if rn := i.rn.reg(); rn.Valid() {
				i.rn = i.rn.assignReg(reg)
			}
		} else if index == 1 {
			if rm := i.rm.reg(); rm.Valid() {
				i.rm = i.rm.assignReg(reg)
			}
		} else {
			if rd := i.rd; rd.Valid() {
				i.rd = reg
			}
		}
	case useKindRNRN1RM:
		if index == 0 {
			if rn := i.rn.reg(); rn.Valid() {
				i.rn = i.rn.assignReg(reg)
			}
			if rn1 := i.rn.reg() + 1; rn1.Valid() {
				i.rm = i.rm.assignReg(reg + 1)
			}
		} else {
			if rm := i.rm.reg(); rm.Valid() {
				i.rm = i.rm.assignReg(reg)
			}
		}
	case useKindRNRMRA:
		if index == 0 {
			if rn := i.rn.reg(); rn.Valid() {
				i.rn = i.rn.assignReg(reg)
			}
		} else if index == 1 {
			if rm := i.rm.reg(); rm.Valid() {
				i.rm = i.rm.assignReg(reg)
			}
		} else {
			if ra := regalloc.VReg(i.u2); ra.Valid() {
				i.u2 = uint64(reg)
			}
		}
	case useKindAMode:
		if index == 0 {
			amode := i.getAmode()
			if amodeRN := amode.rn; amodeRN.Valid() {
				amode.rn = reg
			}
		} else {
			amode := i.getAmode()
			if amodeRM := amode.rm; amodeRM.Valid() {
				amode.rm = reg
			}
		}
	case useKindRNAMode:
		if index == 0 {
			i.rn = i.rn.assignReg(reg)
		} else if index == 1 {
			amode := i.getAmode()
			if amodeRN := amode.rn; amodeRN.Valid() {
				amode.rn = reg
			} else {
				panic("BUG")
			}
		} else {
			amode := i.getAmode()
			if amodeRM := amode.rm; amodeRM.Valid() {
				amode.rm = reg
			} else {
				panic("BUG")
			}
		}
	case useKindCond:
		c := cond(i.u1)
		switch c.kind() {
		case condKindRegisterZero:
			i.u1 = uint64(registerAsRegZeroCond(reg))
		case condKindRegisterNotZero:
			i.u1 = uint64(registerAsRegNotZeroCond(reg))
		}
	case useKindCall:
		panic("BUG: call instructions shouldn't be assigned")
	case useKindCallInd:
		i.rn = i.rn.assignReg(reg)
	default:
		panic(fmt.Sprintf("useKind for %v not defined", i))
	}
}

func (i *instruction) asCall(ref ssa.FuncRef, abi *backend.FunctionABI) {
	i.kind = call
	i.u1 = uint64(ref)
	if abi != nil {
		i.u2 = abi.ABIInfoAsUint64()
	}
}

func (i *instruction) asCallIndirect(ptr regalloc.VReg, abi *backend.FunctionABI) {
	i.kind = callInd
	i.rn = operandNR(ptr)
	if abi != nil {
		i.u2 = abi.ABIInfoAsUint64()
	}
}

func (i *instruction) callFuncRef() ssa.FuncRef {
	return ssa.FuncRef(i.u1)
}

// shift must be divided by 16 and must be in range 0-3 (if dst64bit is true) or 0-1 (if dst64bit is false)
func (i *instruction) asMOVZ(dst regalloc.VReg, imm uint64, shift uint32, dst64bit bool) {
	i.kind = movZ
	i.rd = dst
	i.u1 = imm
	i.u2 = uint64(shift)
	if dst64bit {
		i.u2 |= 1 << 32
	}
}

// shift must be divided by 16 and must be in range 0-3 (if dst64bit is true) or 0-1 (if dst64bit is false)
func (i *instruction) asMOVK(dst regalloc.VReg, imm uint64, shift uint32, dst64bit bool) {
	i.kind = movK
	i.rd = dst
	i.u1 = imm
	i.u2 = uint64(shift)
	if dst64bit {
		i.u2 |= 1 << 32
	}
}

// shift must be divided by 16 and must be in range 0-3 (if dst64bit is true) or 0-1 (if dst64bit is false)
func (i *instruction) asMOVN(dst regalloc.VReg, imm uint64, shift uint32, dst64bit bool) {
	i.kind = movN
	i.rd = dst
	i.u1 = imm
	i.u2 = uint64(shift)
	if dst64bit {
		i.u2 |= 1 << 32
	}
}

func (i *instruction) asNop0() *instruction {
	i.kind = nop0
	return i
}

func (i *instruction) asNop0WithLabel(l label) {
	i.kind = nop0
	i.u1 = uint64(l)
}

func (i *instruction) nop0Label() label {
	return label(i.u1)
}

func (i *instruction) asRet() {
	i.kind = ret
}

func (i *instruction) asStorePair64(src1, src2 regalloc.VReg, amode *addressMode) {
	i.kind = storeP64
	i.rn = operandNR(src1)
	i.rm = operandNR(src2)
	i.setAmode(amode)
}

func (i *instruction) asLoadPair64(src1, src2 regalloc.VReg, amode *addressMode) {
	i.kind = loadP64
	i.rn = operandNR(src1)
	i.rm = operandNR(src2)
	i.setAmode(amode)
}

func (i *instruction) asStore(src operand, amode *addressMode, sizeInBits byte) {
	switch sizeInBits {
	case 8:
		i.kind = store8
	case 16:
		i.kind = store16
	case 32:
		if src.reg().RegType() == regalloc.RegTypeInt {
			i.kind = store32
		} else {
			i.kind = fpuStore32
		}
	case 64:
		if src.reg().RegType() == regalloc.RegTypeInt {
			i.kind = store64
		} else {
			i.kind = fpuStore64
		}
	case 128:
		i.kind = fpuStore128
	}
	i.rn = src
	i.setAmode(amode)
}

func (i *instruction) asSLoad(dst regalloc.VReg, amode *addressMode, sizeInBits byte) {
	switch sizeInBits {
	case 8:
		i.kind = sLoad8
	case 16:
		i.kind = sLoad16
	case 32:
		i.kind = sLoad32
	default:
		panic("BUG")
	}
	i.rd = dst
	i.setAmode(amode)
}

func (i *instruction) asULoad(dst regalloc.VReg, amode *addressMode, sizeInBits byte) {
	switch sizeInBits {
	case 8:
		i.kind = uLoad8
	case 16:
		i.kind = uLoad16
	case 32:
		i.kind = uLoad32
	case 64:
		i.kind = uLoad64
	}
	i.rd = dst
	i.setAmode(amode)
}

func (i *instruction) asFpuLoad(dst regalloc.VReg, amode *addressMode, sizeInBits byte) {
	switch sizeInBits {
	case 32:
		i.kind = fpuLoad32
	case 64:
		i.kind = fpuLoad64
	case 128:
		i.kind = fpuLoad128
	}
	i.rd = dst
	i.setAmode(amode)
}

func (i *instruction) getAmode() *addressMode {
	return wazevoapi.PtrFromUintptr[addressMode](uintptr(i.u1))
}

func (i *instruction) setAmode(a *addressMode) {
	i.u1 = uint64(uintptr(unsafe.Pointer(a)))
}

func (i *instruction) asVecLoad1R(rd regalloc.VReg, rn operand, arr vecArrangement) {
	// NOTE: currently only has support for no-offset loads, though it is suspicious that
	// we would need to support offset load (that is only available for post-index).
	i.kind = vecLoad1R
	i.rd = rd
	i.rn = rn
	i.u1 = uint64(arr)
}

func (i *instruction) asCSet(rd regalloc.VReg, mask bool, c condFlag) {
	i.kind = cSet
	i.rd = rd
	i.u1 = uint64(c)
	if mask {
		i.u2 = 1
	}
}

func (i *instruction) asCSel(rd regalloc.VReg, rn, rm operand, c condFlag, _64bit bool) {
	i.kind = cSel
	i.rd = rd
	i.rn = rn
	i.rm = rm
	i.u1 = uint64(c)
	if _64bit {
		i.u2 = 1
	}
}

func (i *instruction) asFpuCSel(rd regalloc.VReg, rn, rm operand, c condFlag, _64bit bool) {
	i.kind = fpuCSel
	i.rd = rd
	i.rn = rn
	i.rm = rm
	i.u1 = uint64(c)
	if _64bit {
		i.u2 = 1
	}
}

func (i *instruction) asBr(target label) {
	if target == labelReturn {
		panic("BUG: call site should special case for returnLabel")
	}
	i.kind = br
	i.u1 = uint64(target)
}

func (i *instruction) asBrTableSequence(indexReg regalloc.VReg, targetIndex, targetCounts int) {
	i.kind = brTableSequence
	i.rn = operandNR(indexReg)
	i.u1 = uint64(targetIndex)
	i.u2 = uint64(targetCounts)
}

func (i *instruction) brTableSequenceOffsetsResolved() {
	i.rm.data = 1 // indicate that the offsets are resolved, for debugging.
}

func (i *instruction) brLabel() label {
	return label(i.u1)
}

// brOffsetResolved is called when the target label is resolved.
func (i *instruction) brOffsetResolve(offset int64) {
	i.u2 = uint64(offset)
	i.rm.data = 1 // indicate that the offset is resolved, for debugging.
}

func (i *instruction) brOffset() int64 {
	return int64(i.u2)
}

// asCondBr encodes a conditional branch instruction. is64bit is only needed when cond is not flag.
func (i *instruction) asCondBr(c cond, target label, is64bit bool) {
	i.kind = condBr
	i.u1 = c.asUint64()
	i.u2 = uint64(target)
	if is64bit {
		i.u2 |= 1 << 32
	}
}

func (i *instruction) setCondBrTargets(target label) {
	i.u2 = uint64(target)
}

func (i *instruction) condBrLabel() label {
	return label(i.u2)
}

// condBrOffsetResolve is called when the target label is resolved.
func (i *instruction) condBrOffsetResolve(offset int64) {
	i.rn.data = uint64(offset)
	i.rn.data2 = 1 // indicate that the offset is resolved, for debugging.
}

// condBrOffsetResolved returns true if condBrOffsetResolve is already called.
func (i *instruction) condBrOffsetResolved() bool {
	return i.rn.data2 == 1
}

func (i *instruction) condBrOffset() int64 {
	return int64(i.rn.data)
}

func (i *instruction) condBrCond() cond {
	return cond(i.u1)
}

func (i *instruction) condBr64bit() bool {
	return i.u2&(1<<32) != 0
}

func (i *instruction) asLoadFpuConst32(rd regalloc.VReg, raw uint64) {
	i.kind = loadFpuConst32
	i.u1 = raw
	i.rd = rd
}

func (i *instruction) asLoadFpuConst64(rd regalloc.VReg, raw uint64) {
	i.kind = loadFpuConst64
	i.u1 = raw
	i.rd = rd
}

func (i *instruction) asLoadFpuConst128(rd regalloc.VReg, lo, hi uint64) {
	i.kind = loadFpuConst128
	i.u1 = lo
	i.u2 = hi
	i.rd = rd
}

func (i *instruction) asFpuCmp(rn, rm operand, is64bit bool) {
	i.kind = fpuCmp
	i.rn, i.rm = rn, rm
	if is64bit {
		i.u1 = 1
	}
}

func (i *instruction) asCCmpImm(rn operand, imm uint64, c condFlag, flag byte, is64bit bool) {
	i.kind = cCmpImm
	i.rn = rn
	i.rm.data = imm
	i.u1 = uint64(c)
	i.u2 = uint64(flag)
	if is64bit {
		i.u2 |= 1 << 32
	}
}

// asALU setups a basic ALU instruction.
func (i *instruction) asALU(aluOp aluOp, rd regalloc.VReg, rn, rm operand, dst64bit bool) {
	switch rm.kind {
	case operandKindNR:
		i.kind = aluRRR
	case operandKindSR:
		i.kind = aluRRRShift
	case operandKindER:
		i.kind = aluRRRExtend
	case operandKindImm12:
		i.kind = aluRRImm12
	default:
		panic("BUG")
	}
	i.u1 = uint64(aluOp)
	i.rd, i.rn, i.rm = rd, rn, rm
	if dst64bit {
		i.u2 |= 1 << 32
	}
}

// asALU setups a basic ALU instruction.
func (i *instruction) asALURRRR(aluOp aluOp, rd regalloc.VReg, rn, rm operand, ra regalloc.VReg, dst64bit bool) {
	i.kind = aluRRRR
	i.u1 = uint64(aluOp)
	i.rd, i.rn, i.rm, i.u2 = rd, rn, rm, uint64(ra)
	if dst64bit {
		i.u1 |= 1 << 32
	}
}

// asALUShift setups a shift based ALU instruction.
func (i *instruction) asALUShift(aluOp aluOp, rd regalloc.VReg, rn, rm operand, dst64bit bool) {
	switch rm.kind {
	case operandKindNR:
		i.kind = aluRRR // If the shift amount op is a register, then the instruction is encoded as a normal ALU instruction with two register operands.
	case operandKindShiftImm:
		i.kind = aluRRImmShift
	default:
		panic("BUG")
	}
	i.u1 = uint64(aluOp)
	i.rd, i.rn, i.rm = rd, rn, rm
	if dst64bit {
		i.u2 |= 1 << 32
	}
}

func (i *instruction) asALUBitmaskImm(aluOp aluOp, rd, rn regalloc.VReg, imm uint64, dst64bit bool) {
	i.kind = aluRRBitmaskImm
	i.u1 = uint64(aluOp)
	i.rn, i.rd = operandNR(rn), rd
	i.u2 = imm
	if dst64bit {
		i.u1 |= 1 << 32
	}
}

func (i *instruction) asMovToFPSR(rn regalloc.VReg) {
	i.kind = movToFPSR
	i.rn = operandNR(rn)
}

func (i *instruction) asMovFromFPSR(rd regalloc.VReg) {
	i.kind = movFromFPSR
	i.rd = rd
}

func (i *instruction) asBitRR(bitOp bitOp, rd, rn regalloc.VReg, is64bit bool) {
	i.kind = bitRR
	i.rn, i.rd = operandNR(rn), rd
	i.u1 = uint64(bitOp)
	if is64bit {
		i.u2 = 1
	}
}

func (i *instruction) asFpuRRR(op fpuBinOp, rd regalloc.VReg, rn, rm operand, dst64bit bool) {
	i.kind = fpuRRR
	i.u1 = uint64(op)
	i.rd, i.rn, i.rm = rd, rn, rm
	if dst64bit {
		i.u2 = 1
	}
}

func (i *instruction) asFpuRR(op fpuUniOp, rd regalloc.VReg, rn operand, dst64bit bool) {
	i.kind = fpuRR
	i.u1 = uint64(op)
	i.rd, i.rn = rd, rn
	if dst64bit {
		i.u2 = 1
	}
}

func (i *instruction) asExtend(rd, rn regalloc.VReg, fromBits, toBits byte, signed bool) {
	i.kind = extend
	i.rn, i.rd = operandNR(rn), rd
	i.u1 = uint64(fromBits)
	i.u2 = uint64(toBits)
	if signed {
		i.u2 |= 1 << 32
	}
}

func (i *instruction) asMove32(rd, rn regalloc.VReg) {
	i.kind = mov32
	i.rn, i.rd = operandNR(rn), rd
}

func (i *instruction) asMove64(rd, rn regalloc.VReg) *instruction {
	i.kind = mov64
	i.rn, i.rd = operandNR(rn), rd
	return i
}

func (i *instruction) asFpuMov64(rd, rn regalloc.VReg) {
	i.kind = fpuMov64
	i.rn, i.rd = operandNR(rn), rd
}

func (i *instruction) asFpuMov128(rd, rn regalloc.VReg) *instruction {
	i.kind = fpuMov128
	i.rn, i.rd = operandNR(rn), rd
	return i
}

func (i *instruction) asMovToVec(rd regalloc.VReg, rn operand, arr vecArrangement, index vecIndex) {
	i.kind = movToVec
	i.rd = rd
	i.rn = rn
	i.u1, i.u2 = uint64(arr), uint64(index)
}

func (i *instruction) asMovFromVec(rd regalloc.VReg, rn operand, arr vecArrangement, index vecIndex, signed bool) {
	if signed {
		i.kind = movFromVecSigned
	} else {
		i.kind = movFromVec
	}
	i.rd = rd
	i.rn = rn
	i.u1, i.u2 = uint64(arr), uint64(index)
}

func (i *instruction) asVecDup(rd regalloc.VReg, rn operand, arr vecArrangement) {
	i.kind = vecDup
	i.u1 = uint64(arr)
	i.rn, i.rd = rn, rd
}

func (i *instruction) asVecDupElement(rd regalloc.VReg, rn operand, arr vecArrangement, index vecIndex) {
	i.kind = vecDupElement
	i.u1 = uint64(arr)
	i.rn, i.rd = rn, rd
	i.u2 = uint64(index)
}

func (i *instruction) asVecExtract(rd regalloc.VReg, rn, rm operand, arr vecArrangement, index uint32) {
	i.kind = vecExtract
	i.u1 = uint64(arr)
	i.rn, i.rm, i.rd = rn, rm, rd
	i.u2 = uint64(index)
}

func (i *instruction) asVecMovElement(rd regalloc.VReg, rn operand, arr vecArrangement, rdIndex, rnIndex vecIndex) {
	i.kind = vecMovElement
	i.u1 = uint64(arr)
	i.u2 = uint64(rdIndex) | uint64(rnIndex)<<32
	i.rn, i.rd = rn, rd
}

func (i *instruction) asVecMisc(op vecOp, rd regalloc.VReg, rn operand, arr vecArrangement) {
	i.kind = vecMisc
	i.u1 = uint64(op)
	i.rn, i.rd = rn, rd
	i.u2 = uint64(arr)
}

func (i *instruction) asVecLanes(op vecOp, rd regalloc.VReg, rn operand, arr vecArrangement) {
	i.kind = vecLanes
	i.u1 = uint64(op)
	i.rn, i.rd = rn, rd
	i.u2 = uint64(arr)
}

func (i *instruction) asVecShiftImm(op vecOp, rd regalloc.VReg, rn, rm operand, arr vecArrangement) *instruction {
	i.kind = vecShiftImm
	i.u1 = uint64(op)
	i.rn, i.rm, i.rd = rn, rm, rd
	i.u2 = uint64(arr)
	return i
}

func (i *instruction) asVecTbl(nregs byte, rd regalloc.VReg, rn, rm operand, arr vecArrangement) {
	switch nregs {
	case 0, 1:
		i.kind = vecTbl
	case 2:
		i.kind = vecTbl2
		if !rn.reg().IsRealReg() {
			panic("rn is not a RealReg")
		}
		if rn.realReg() == v31 {
			panic("rn cannot be v31")
		}
	default:
		panic(fmt.Sprintf("unsupported number of registers %d", nregs))
	}
	i.rn, i.rm, i.rd = rn, rm, rd
	i.u2 = uint64(arr)
}

func (i *instruction) asVecPermute(op vecOp, rd regalloc.VReg, rn, rm operand, arr vecArrangement) {
	i.kind = vecPermute
	i.u1 = uint64(op)
	i.rn, i.rm, i.rd = rn, rm, rd
	i.u2 = uint64(arr)
}

func (i *instruction) asVecRRR(op vecOp, rd regalloc.VReg, rn, rm operand, arr vecArrangement) *instruction {
	i.kind = vecRRR
	i.u1 = uint64(op)
	i.rn, i.rd, i.rm = rn, rd, rm
	i.u2 = uint64(arr)
	return i
}

// asVecRRRRewrite encodes a vector instruction that rewrites the destination register.
// IMPORTANT: the destination register must be already defined before this instruction.
func (i *instruction) asVecRRRRewrite(op vecOp, rd regalloc.VReg, rn, rm operand, arr vecArrangement) {
	i.kind = vecRRRRewrite
	i.u1 = uint64(op)
	i.rn, i.rd, i.rm = rn, rd, rm
	i.u2 = uint64(arr)
}

func (i *instruction) IsCopy() bool {
	op := i.kind
	// We do not include mov32 as it is not a copy instruction in the sense that it does not preserve the upper 32 bits,
	// and it is only used in the translation of IReduce, not the actual copy indeed.
	return op == mov64 || op == fpuMov64 || op == fpuMov128
}

// String implements fmt.Stringer.
func (i *instruction) String() (str string) {
	is64SizeBitToSize := func(v uint64) byte {
		if v == 0 {
			return 32
		}
		return 64
	}

	switch i.kind {
	case nop0:
		if i.u1 != 0 {
			l := label(i.u1)
			str = fmt.Sprintf("%s:", l)
		} else {
			str = "nop0"
		}
	case aluRRR:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("%s %s, %s, %s", aluOp(i.u1).String(),
			formatVRegSized(i.rd, size), formatVRegSized(i.rn.nr(), size),
			i.rm.format(size))
	case aluRRRR:
		size := is64SizeBitToSize(i.u1 >> 32)
		str = fmt.Sprintf("%s %s, %s, %s, %s", aluOp(i.u1).String(),
			formatVRegSized(i.rd, size), formatVRegSized(i.rn.nr(), size), formatVRegSized(i.rm.nr(), size), formatVRegSized(regalloc.VReg(i.u2), size))
	case aluRRImm12:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("%s %s, %s, %s", aluOp(i.u1).String(),
			formatVRegSized(i.rd, size), formatVRegSized(i.rn.nr(), size), i.rm.format(size))
	case aluRRBitmaskImm:
		size := is64SizeBitToSize(i.u1 >> 32)
		rd, rn := formatVRegSized(i.rd, size), formatVRegSized(i.rn.nr(), size)
		if size == 32 {
			str = fmt.Sprintf("%s %s, %s, #%#x", aluOp(i.u1).String(), rd, rn, uint32(i.u2))
		} else {
			str = fmt.Sprintf("%s %s, %s, #%#x", aluOp(i.u1).String(), rd, rn, i.u2)
		}
	case aluRRImmShift:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("%s %s, %s, %#x",
			aluOp(i.u1).String(),
			formatVRegSized(i.rd, size),
			formatVRegSized(i.rn.nr(), size),
			i.rm.shiftImm(),
		)
	case aluRRRShift:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("%s %s, %s, %s",
			aluOp(i.u1).String(),
			formatVRegSized(i.rd, size),
			formatVRegSized(i.rn.nr(), size),
			i.rm.format(size),
		)
	case aluRRRExtend:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("%s %s, %s, %s", aluOp(i.u1).String(),
			formatVRegSized(i.rd, size),
			formatVRegSized(i.rn.nr(), size),
			// Regardless of the source size, the register is formatted in 32-bit.
			i.rm.format(32),
		)
	case bitRR:
		size := is64SizeBitToSize(i.u2)
		str = fmt.Sprintf("%s %s, %s",
			bitOp(i.u1),
			formatVRegSized(i.rd, size),
			formatVRegSized(i.rn.nr(), size),
		)
	case uLoad8:
		str = fmt.Sprintf("ldrb %s, %s", formatVRegSized(i.rd, 32), i.getAmode().format(32))
	case sLoad8:
		str = fmt.Sprintf("ldrsb %s, %s", formatVRegSized(i.rd, 32), i.getAmode().format(32))
	case uLoad16:
		str = fmt.Sprintf("ldrh %s, %s", formatVRegSized(i.rd, 32), i.getAmode().format(32))
	case sLoad16:
		str = fmt.Sprintf("ldrsh %s, %s", formatVRegSized(i.rd, 32), i.getAmode().format(32))
	case uLoad32:
		str = fmt.Sprintf("ldr %s, %s", formatVRegSized(i.rd, 32), i.getAmode().format(32))
	case sLoad32:
		str = fmt.Sprintf("ldrs %s, %s", formatVRegSized(i.rd, 32), i.getAmode().format(32))
	case uLoad64:
		str = fmt.Sprintf("ldr %s, %s", formatVRegSized(i.rd, 64), i.getAmode().format(64))
	case store8:
		str = fmt.Sprintf("strb %s, %s", formatVRegSized(i.rn.nr(), 32), i.getAmode().format(8))
	case store16:
		str = fmt.Sprintf("strh %s, %s", formatVRegSized(i.rn.nr(), 32), i.getAmode().format(16))
	case store32:
		str = fmt.Sprintf("str %s, %s", formatVRegSized(i.rn.nr(), 32), i.getAmode().format(32))
	case store64:
		str = fmt.Sprintf("str %s, %s", formatVRegSized(i.rn.nr(), 64), i.getAmode().format(64))
	case storeP64:
		str = fmt.Sprintf("stp %s, %s, %s",
			formatVRegSized(i.rn.nr(), 64), formatVRegSized(i.rm.nr(), 64), i.getAmode().format(64))
	case loadP64:
		str = fmt.Sprintf("ldp %s, %s, %s",
			formatVRegSized(i.rn.nr(), 64), formatVRegSized(i.rm.nr(), 64), i.getAmode().format(64))
	case mov64:
		str = fmt.Sprintf("mov %s, %s",
			formatVRegSized(i.rd, 64),
			formatVRegSized(i.rn.nr(), 64))
	case mov32:
		str = fmt.Sprintf("mov %s, %s", formatVRegSized(i.rd, 32), formatVRegSized(i.rn.nr(), 32))
	case movZ:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("movz %s, #%#x, lsl %d", formatVRegSized(i.rd, size), uint16(i.u1), uint32(i.u2)*16)
	case movN:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("movn %s, #%#x, lsl %d", formatVRegSized(i.rd, size), uint16(i.u1), uint32(i.u2)*16)
	case movK:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("movk %s, #%#x, lsl %d", formatVRegSized(i.rd, size), uint16(i.u1), uint32(i.u2)*16)
	case extend:
		fromBits, toBits := byte(i.u1), byte(i.u2)

		var signedStr string
		if i.u2>>32 == 1 {
			signedStr = "s"
		} else {
			signedStr = "u"
		}
		var fromStr string
		switch fromBits {
		case 8:
			fromStr = "b"
		case 16:
			fromStr = "h"
		case 32:
			fromStr = "w"
		}
		str = fmt.Sprintf("%sxt%s %s, %s", signedStr, fromStr, formatVRegSized(i.rd, toBits), formatVRegSized(i.rn.nr(), 32))
	case cSel:
		size := is64SizeBitToSize(i.u2)
		str = fmt.Sprintf("csel %s, %s, %s, %s",
			formatVRegSized(i.rd, size),
			formatVRegSized(i.rn.nr(), size),
			formatVRegSized(i.rm.nr(), size),
			condFlag(i.u1),
		)
	case cSet:
		if i.u2 != 0 {
			str = fmt.Sprintf("csetm %s, %s", formatVRegSized(i.rd, 64), condFlag(i.u1))
		} else {
			str = fmt.Sprintf("cset %s, %s", formatVRegSized(i.rd, 64), condFlag(i.u1))
		}
	case cCmpImm:
		size := is64SizeBitToSize(i.u2 >> 32)
		str = fmt.Sprintf("ccmp %s, #%#x, #%#x, %s",
			formatVRegSized(i.rn.nr(), size), i.rm.data,
			i.u2&0b1111,
			condFlag(i.u1))
	case fpuMov64:
		str = fmt.Sprintf("mov %s, %s",
			formatVRegVec(i.rd, vecArrangement8B, vecIndexNone),
			formatVRegVec(i.rn.nr(), vecArrangement8B, vecIndexNone))
	case fpuMov128:
		str = fmt.Sprintf("mov %s, %s",
			formatVRegVec(i.rd, vecArrangement16B, vecIndexNone),
			formatVRegVec(i.rn.nr(), vecArrangement16B, vecIndexNone))
	case fpuMovFromVec:
		panic("TODO")
	case fpuRR:
		dstSz := is64SizeBitToSize(i.u2)
		srcSz := dstSz
		op := fpuUniOp(i.u1)
		switch op {
		case fpuUniOpCvt32To64:
			srcSz = 32
		case fpuUniOpCvt64To32:
			srcSz = 64
		}
		str = fmt.Sprintf("%s %s, %s", op.String(),
			formatVRegSized(i.rd, dstSz), formatVRegSized(i.rn.nr(), srcSz))
	case fpuRRR:
		size := is64SizeBitToSize(i.u2)
		str = fmt.Sprintf("%s %s, %s, %s", fpuBinOp(i.u1).String(),
			formatVRegSized(i.rd, size), formatVRegSized(i.rn.nr(), size), formatVRegSized(i.rm.nr(), size))
	case fpuRRI:
		panic("TODO")
	case fpuRRRR:
		panic("TODO")
	case fpuCmp:
		size := is64SizeBitToSize(i.u1)
		str = fmt.Sprintf("fcmp %s, %s",
			formatVRegSized(i.rn.nr(), size), formatVRegSized(i.rm.nr(), size))
	case fpuLoad32:
		str = fmt.Sprintf("ldr %s, %s", formatVRegSized(i.rd, 32), i.getAmode().format(32))
	case fpuStore32:
		str = fmt.Sprintf("str %s, %s", formatVRegSized(i.rn.nr(), 32), i.getAmode().format(64))
	case fpuLoad64:
		str = fmt.Sprintf("ldr %s, %s", formatVRegSized(i.rd, 64), i.getAmode().format(64))
	case fpuStore64:
		str = fmt.Sprintf("str %s, %s", formatVRegSized(i.rn.nr(), 64), i.getAmode().format(64))
	case fpuLoad128:
		str = fmt.Sprintf("ldr %s, %s", formatVRegSized(i.rd, 128), i.getAmode().format(64))
	case fpuStore128:
		str = fmt.Sprintf("str %s, %s", formatVRegSized(i.rn.nr(), 128), i.getAmode().format(64))
	case loadFpuConst32:
		str = fmt.Sprintf("ldr %s, #8; b 8; data.f32 %f", formatVRegSized(i.rd, 32), math.Float32frombits(uint32(i.u1)))
	case loadFpuConst64:
		str = fmt.Sprintf("ldr %s, #8; b 16; data.f64 %f", formatVRegSized(i.rd, 64), math.Float64frombits(i.u1))
	case loadFpuConst128:
		str = fmt.Sprintf("ldr %s, #8; b 32; data.v128  %016x %016x",
			formatVRegSized(i.rd, 128), i.u1, i.u2)
	case fpuToInt:
		var op, src, dst string
		if signed := i.u1 == 1; signed {
			op = "fcvtzs"
		} else {
			op = "fcvtzu"
		}
		if src64 := i.u2&1 != 0; src64 {
			src = formatVRegWidthVec(i.rn.nr(), vecArrangementD)
		} else {
			src = formatVRegWidthVec(i.rn.nr(), vecArrangementS)
		}
		if dst64 := i.u2&2 != 0; dst64 {
			dst = formatVRegSized(i.rd, 64)
		} else {
			dst = formatVRegSized(i.rd, 32)
		}
		str = fmt.Sprintf("%s %s, %s", op, dst, src)

	case intToFpu:
		var op, src, dst string
		if signed := i.u1 == 1; signed {
			op = "scvtf"
		} else {
			op = "ucvtf"
		}
		if src64 := i.u2&1 != 0; src64 {
			src = formatVRegSized(i.rn.nr(), 64)
		} else {
			src = formatVRegSized(i.rn.nr(), 32)
		}
		if dst64 := i.u2&2 != 0; dst64 {
			dst = formatVRegWidthVec(i.rd, vecArrangementD)
		} else {
			dst = formatVRegWidthVec(i.rd, vecArrangementS)
		}
		str = fmt.Sprintf("%s %s, %s", op, dst, src)
	case fpuCSel:
		size := is64SizeBitToSize(i.u2)
		str = fmt.Sprintf("fcsel %s, %s, %s, %s",
			formatVRegSized(i.rd, size),
			formatVRegSized(i.rn.nr(), size),
			formatVRegSized(i.rm.nr(), size),
			condFlag(i.u1),
		)
	case movToVec:
		var size byte
		arr := vecArrangement(i.u1)
		switch arr {
		case vecArrangementB, vecArrangementH, vecArrangementS:
			size = 32
		case vecArrangementD:
			size = 64
		default:
			panic("unsupported arrangement " + arr.String())
		}
		str = fmt.Sprintf("ins %s, %s", formatVRegVec(i.rd, arr, vecIndex(i.u2)), formatVRegSized(i.rn.nr(), size))
	case movFromVec, movFromVecSigned:
		var size byte
		var opcode string
		arr := vecArrangement(i.u1)
		signed := i.kind == movFromVecSigned
		switch arr {
		case vecArrangementB, vecArrangementH, vecArrangementS:
			size = 32
			if signed {
				opcode = "smov"
			} else {
				opcode = "umov"
			}
		case vecArrangementD:
			size = 64
			if signed {
				opcode = "smov"
			} else {
				opcode = "mov"
			}
		default:
			panic("unsupported arrangement " + arr.String())
		}
		str = fmt.Sprintf("%s %s, %s", opcode, formatVRegSized(i.rd, size), formatVRegVec(i.rn.nr(), arr, vecIndex(i.u2)))
	case vecDup:
		str = fmt.Sprintf("dup %s, %s",
			formatVRegVec(i.rd, vecArrangement(i.u1), vecIndexNone),
			formatVRegSized(i.rn.nr(), 64),
		)
	case vecDupElement:
		arr := vecArrangement(i.u1)
		str = fmt.Sprintf("dup %s, %s",
			formatVRegVec(i.rd, arr, vecIndexNone),
			formatVRegVec(i.rn.nr(), arr, vecIndex(i.u2)),
		)
	case vecDupFromFpu:
		panic("TODO")
	case vecExtract:
		str = fmt.Sprintf("ext %s, %s, %s, #%d",
			formatVRegVec(i.rd, vecArrangement(i.u1), vecIndexNone),
			formatVRegVec(i.rn.nr(), vecArrangement(i.u1), vecIndexNone),
			formatVRegVec(i.rm.nr(), vecArrangement(i.u1), vecIndexNone),
			uint32(i.u2),
		)
	case vecExtend:
		panic("TODO")
	case vecMovElement:
		str = fmt.Sprintf("mov %s, %s",
			formatVRegVec(i.rd, vecArrangement(i.u1), vecIndex(i.u2&0xffffffff)),
			formatVRegVec(i.rn.nr(), vecArrangement(i.u1), vecIndex(i.u2>>32)),
		)
	case vecMiscNarrow:
		panic("TODO")
	case vecRRR, vecRRRRewrite:
		str = fmt.Sprintf("%s %s, %s, %s",
			vecOp(i.u1),
			formatVRegVec(i.rd, vecArrangement(i.u2), vecIndexNone),
			formatVRegVec(i.rn.nr(), vecArrangement(i.u2), vecIndexNone),
			formatVRegVec(i.rm.nr(), vecArrangement(i.u2), vecIndexNone),
		)
	case vecMisc:
		vop := vecOp(i.u1)
		if vop == vecOpCmeq0 {
			str = fmt.Sprintf("cmeq %s, %s, #0",
				formatVRegVec(i.rd, vecArrangement(i.u2), vecIndexNone),
				formatVRegVec(i.rn.nr(), vecArrangement(i.u2), vecIndexNone))
		} else {
			str = fmt.Sprintf("%s %s, %s",
				vop,
				formatVRegVec(i.rd, vecArrangement(i.u2), vecIndexNone),
				formatVRegVec(i.rn.nr(), vecArrangement(i.u2), vecIndexNone))
		}
	case vecLanes:
		arr := vecArrangement(i.u2)
		var destArr vecArrangement
		switch arr {
		case vecArrangement8B, vecArrangement16B:
			destArr = vecArrangementH
		case vecArrangement4H, vecArrangement8H:
			destArr = vecArrangementS
		case vecArrangement4S:
			destArr = vecArrangementD
		default:
			panic("invalid arrangement " + arr.String())
		}
		str = fmt.Sprintf("%s %s, %s",
			vecOp(i.u1),
			formatVRegWidthVec(i.rd, destArr),
			formatVRegVec(i.rn.nr(), arr, vecIndexNone))
	case vecShiftImm:
		arr := vecArrangement(i.u2)
		str = fmt.Sprintf("%s %s, %s, #%d",
			vecOp(i.u1),
			formatVRegVec(i.rd, arr, vecIndexNone),
			formatVRegVec(i.rn.nr(), arr, vecIndexNone),
			i.rm.shiftImm())
	case vecTbl:
		arr := vecArrangement(i.u2)
		str = fmt.Sprintf("tbl %s, { %s }, %s",
			formatVRegVec(i.rd, arr, vecIndexNone),
			formatVRegVec(i.rn.nr(), vecArrangement16B, vecIndexNone),
			formatVRegVec(i.rm.nr(), arr, vecIndexNone))
	case vecTbl2:
		arr := vecArrangement(i.u2)
		rd, rn, rm := i.rd, i.rn.nr(), i.rm.nr()
		rn1 := regalloc.FromRealReg(rn.RealReg()+1, rn.RegType())
		str = fmt.Sprintf("tbl %s, { %s, %s }, %s",
			formatVRegVec(rd, arr, vecIndexNone),
			formatVRegVec(rn, vecArrangement16B, vecIndexNone),
			formatVRegVec(rn1, vecArrangement16B, vecIndexNone),
			formatVRegVec(rm, arr, vecIndexNone))
	case vecPermute:
		arr := vecArrangement(i.u2)
		str = fmt.Sprintf("%s %s, %s, %s",
			vecOp(i.u1),
			formatVRegVec(i.rd, arr, vecIndexNone),
			formatVRegVec(i.rn.nr(), arr, vecIndexNone),
			formatVRegVec(i.rm.nr(), arr, vecIndexNone))
	case movToFPSR:
		str = fmt.Sprintf("msr fpsr, %s", formatVRegSized(i.rn.nr(), 64))
	case movFromFPSR:
		str = fmt.Sprintf("mrs %s fpsr", formatVRegSized(i.rd, 64))
	case call:
		str = fmt.Sprintf("bl %s", ssa.FuncRef(i.u1))
	case callInd:
		str = fmt.Sprintf("bl %s", formatVRegSized(i.rn.nr(), 64))
	case ret:
		str = "ret"
	case br:
		target := label(i.u1)
		if i.rm.data != 0 {
			str = fmt.Sprintf("b #%#x (%s)", i.brOffset(), target.String())
		} else {
			str = fmt.Sprintf("b %s", target.String())
		}
	case condBr:
		size := is64SizeBitToSize(i.u2 >> 32)
		c := cond(i.u1)
		target := label(i.u2 & 0xffffffff)
		switch c.kind() {
		case condKindRegisterZero:
			if !i.condBrOffsetResolved() {
				str = fmt.Sprintf("cbz %s, (%s)", formatVRegSized(c.register(), size), target.String())
			} else {
				str = fmt.Sprintf("cbz %s, #%#x %s", formatVRegSized(c.register(), size), i.condBrOffset(), target.String())
			}
		case condKindRegisterNotZero:
			if offset := i.condBrOffset(); offset != 0 {
				str = fmt.Sprintf("cbnz %s, #%#x (%s)", formatVRegSized(c.register(), size), offset, target.String())
			} else {
				str = fmt.Sprintf("cbnz %s, %s", formatVRegSized(c.register(), size), target.String())
			}
		case condKindCondFlagSet:
			if offset := i.condBrOffset(); offset != 0 {
				if target == labelInvalid {
					str = fmt.Sprintf("b.%s #%#x", c.flag(), offset)
				} else {
					str = fmt.Sprintf("b.%s #%#x, (%s)", c.flag(), offset, target.String())
				}
			} else {
				str = fmt.Sprintf("b.%s %s", c.flag(), target.String())
			}
		}
	case adr:
		str = fmt.Sprintf("adr %s, #%#x", formatVRegSized(i.rd, 64), int64(i.u1))
	case brTableSequence:
		targetIndex := i.u1
		str = fmt.Sprintf("br_table_sequence %s, table_index=%d", formatVRegSized(i.rn.nr(), 64), targetIndex)
	case exitSequence:
		str = fmt.Sprintf("exit_sequence %s", formatVRegSized(i.rn.nr(), 64))
	case atomicRmw:
		m := atomicRmwOp(i.u1).String()
		size := byte(32)
		switch i.u2 {
		case 8:
			size = 64
		case 2:
			m = m + "h"
		case 1:
			m = m + "b"
		}
		str = fmt.Sprintf("%s %s, %s, %s", m, formatVRegSized(i.rm.nr(), size), formatVRegSized(i.rd, size), formatVRegSized(i.rn.nr(), 64))
	case atomicCas:
		m := "casal"
		size := byte(32)
		switch i.u2 {
		case 8:
			size = 64
		case 2:
			m = m + "h"
		case 1:
			m = m + "b"
		}
		str = fmt.Sprintf("%s %s, %s, %s", m, formatVRegSized(i.rd, size), formatVRegSized(i.rm.nr(), size), formatVRegSized(i.rn.nr(), 64))
	case atomicLoad:
		m := "ldar"
		size := byte(32)
		switch i.u2 {
		case 8:
			size = 64
		case 2:
			m = m + "h"
		case 1:
			m = m + "b"
		}
		str = fmt.Sprintf("%s %s, %s", m, formatVRegSized(i.rd, size), formatVRegSized(i.rn.nr(), 64))
	case atomicStore:
		m := "stlr"
		size := byte(32)
		switch i.u2 {
		case 8:
			size = 64
		case 2:
			m = m + "h"
		case 1:
			m = m + "b"
		}
		str = fmt.Sprintf("%s %s, %s", m, formatVRegSized(i.rm.nr(), size), formatVRegSized(i.rn.nr(), 64))
	case dmb:
		str = "dmb"
	case udf:
		str = "udf"
	case emitSourceOffsetInfo:
		str = fmt.Sprintf("source_offset_info %d", ssa.SourceOffset(i.u1))
	case vecLoad1R:
		str = fmt.Sprintf("ld1r {%s}, [%s]", formatVRegVec(i.rd, vecArrangement(i.u1), vecIndexNone), formatVRegSized(i.rn.nr(), 64))
	case loadConstBlockArg:
		str = fmt.Sprintf("load_const_block_arg %s, %#x", formatVRegSized(i.rd, 64), i.u1)
	default:
		panic(i.kind)
	}
	return
}

func (i *instruction) asAdr(rd regalloc.VReg, offset int64) {
	i.kind = adr
	i.rd = rd
	i.u1 = uint64(offset)
}

func (i *instruction) asAtomicRmw(op atomicRmwOp, rn, rs, rt regalloc.VReg, size uint64) {
	i.kind = atomicRmw
	i.rd, i.rn, i.rm = rt, operandNR(rn), operandNR(rs)
	i.u1 = uint64(op)
	i.u2 = size
}

func (i *instruction) asAtomicCas(rn, rs, rt regalloc.VReg, size uint64) {
	i.kind = atomicCas
	i.rm, i.rn, i.rd = operandNR(rt), operandNR(rn), rs
	i.u2 = size
}

func (i *instruction) asAtomicLoad(rn, rt regalloc.VReg, size uint64) {
	i.kind = atomicLoad
	i.rn, i.rd = operandNR(rn), rt
	i.u2 = size
}

func (i *instruction) asAtomicStore(rn, rt operand, size uint64) {
	i.kind = atomicStore
	i.rn, i.rm = rn, rt
	i.u2 = size
}

func (i *instruction) asDMB() {
	i.kind = dmb
}

// TODO: delete unnecessary things.
const (
	// nop0 represents a no-op of zero size.
	nop0 instructionKind = iota + 1
	// aluRRR represents an ALU operation with two register sources and a register destination.
	aluRRR
	// aluRRRR represents an ALU operation with three register sources and a register destination.
	aluRRRR
	// aluRRImm12 represents an ALU operation with a register source and an immediate-12 source, with a register destination.
	aluRRImm12
	// aluRRBitmaskImm represents an ALU operation with a register source and a bitmask immediate, with a register destination.
	aluRRBitmaskImm
	// aluRRImmShift represents an ALU operation with a register source and an immediate-shifted source, with a register destination.
	aluRRImmShift
	// aluRRRShift represents an ALU operation with two register sources, one of which can be shifted, with a register destination.
	aluRRRShift
	// aluRRRExtend represents an ALU operation with two register sources, one of which can be extended, with a register destination.
	aluRRRExtend
	// bitRR represents a bit op instruction with a single register source.
	bitRR
	// uLoad8 represents an unsigned 8-bit load.
	uLoad8
	// sLoad8 represents a signed 8-bit load into 64-bit register.
	sLoad8
	// uLoad16 represents an unsigned 16-bit load into 64-bit register.
	uLoad16
	// sLoad16 represents a signed 16-bit load into 64-bit register.
	sLoad16
	// uLoad32 represents an unsigned 32-bit load into 64-bit register.
	uLoad32
	// sLoad32 represents a signed 32-bit load into 64-bit register.
	sLoad32
	// uLoad64 represents a 64-bit load.
	uLoad64
	// store8 represents an 8-bit store.
	store8
	// store16 represents a 16-bit store.
	store16
	// store32 represents a 32-bit store.
	store32
	// store64 represents a 64-bit store.
	store64
	// storeP64 represents a store of a pair of registers.
	storeP64
	// loadP64 represents a load of a pair of registers.
	loadP64
	// mov64 represents a MOV instruction. These are encoded as ORR's but we keep them separate for better handling.
	mov64
	// mov32 represents a 32-bit MOV. This zeroes the top 32 bits of the destination.
	mov32
	// movZ represents a MOVZ with a 16-bit immediate.
	movZ
	// movN represents a MOVN with a 16-bit immediate.
	movN
	// movK represents a MOVK with a 16-bit immediate.
	movK
	// extend represents a sign- or zero-extend operation.
	extend
	// cSel represents a conditional-select operation.
	cSel
	// cSet represents a conditional-set operation.
	cSet
	// cCmpImm represents a conditional comparison with an immediate.
	cCmpImm
	// fpuMov64 represents a FPU move. Distinct from a vector-register move; moving just 64 bits appears to be significantly faster.
	fpuMov64
	// fpuMov128 represents a vector register move.
	fpuMov128
	// fpuMovFromVec represents a move to scalar from a vector element.
	fpuMovFromVec
	// fpuRR represents a 1-op FPU instruction.
	fpuRR
	// fpuRRR represents a 2-op FPU instruction.
	fpuRRR
	// fpuRRI represents a 2-op FPU instruction with immediate value.
	fpuRRI
	// fpuRRRR represents a 3-op FPU instruction.
	fpuRRRR
	// fpuCmp represents a FPU comparison, either 32 or 64 bit.
	fpuCmp
	// fpuLoad32 represents a floating-point load, single-precision (32 bit).
	fpuLoad32
	// fpuStore32 represents a floating-point store, single-precision (32 bit).
	fpuStore32
	// fpuLoad64 represents a floating-point load, double-precision (64 bit).
	fpuLoad64
	// fpuStore64 represents a floating-point store, double-precision (64 bit).
	fpuStore64
	// fpuLoad128 represents a floating-point/vector load, 128 bit.
	fpuLoad128
	// fpuStore128 represents a floating-point/vector store, 128 bit.
	fpuStore128
	// loadFpuConst32 represents a load of a 32-bit floating-point constant.
	loadFpuConst32
	// loadFpuConst64 represents a load of a 64-bit floating-point constant.
	loadFpuConst64
	// loadFpuConst128 represents a load of a 128-bit floating-point constant.
	loadFpuConst128
	// vecLoad1R represents a load of a one single-element structure that replicates to all lanes of a vector.
	vecLoad1R
	// fpuToInt represents a conversion from FP to integer.
	fpuToInt
	// intToFpu represents a conversion from integer to FP.
	intToFpu
	// fpuCSel represents a 32/64-bit FP conditional select.
	fpuCSel
	// movToVec represents a move to a vector element from a GPR.
	movToVec
	// movFromVec represents an unsigned move from a vector element to a GPR.
	movFromVec
	// movFromVecSigned represents a signed move from a vector element to a GPR.
	movFromVecSigned
	// vecDup represents a duplication of general-purpose register to vector.
	vecDup
	// vecDupElement represents a duplication of a vector element to vector or scalar.
	vecDupElement
	// vecDupFromFpu represents a duplication of scalar to vector.
	vecDupFromFpu
	// vecExtract represents a vector extraction operation.
	vecExtract
	// vecExtend represents a vector extension operation.
	vecExtend
	// vecMovElement represents a move vector element to another vector element operation.
	vecMovElement
	// vecMiscNarrow represents a vector narrowing operation.
	vecMiscNarrow
	// vecRRR represents a vector ALU operation.
	vecRRR
	// vecRRRRewrite is exactly the same as vecRRR except that this rewrites the destination register.
	// For example, BSL instruction rewrites the destination register, and the existing value influences the result.
	// Therefore, the "destination" register in vecRRRRewrite will be treated as "use" which makes the register outlive
	// the instruction while this instruction doesn't have "def" in the context of register allocation.
	vecRRRRewrite
	// vecMisc represents a vector two register miscellaneous instruction.
	vecMisc
	// vecLanes represents a vector instruction across lanes.
	vecLanes
	// vecShiftImm represents a SIMD scalar shift by immediate instruction.
	vecShiftImm
	// vecTbl represents a table vector lookup - single register table.
	vecTbl
	// vecTbl2 represents a table vector lookup - two register table.
	vecTbl2
	// vecPermute represents a vector permute instruction.
	vecPermute
	// movToNZCV represents a move to the FPSR.
	movToFPSR
	// movFromNZCV represents a move from the FPSR.
	movFromFPSR
	// call represents a machine call instruction.
	call
	// callInd represents a machine indirect-call instruction.
	callInd
	// ret represents a machine return instruction.
	ret
	// br represents an unconditional branch.
	br
	// condBr represents a conditional branch.
	condBr
	// adr represents a compute the address (using a PC-relative offset) of a memory location.
	adr
	// brTableSequence represents a jump-table sequence.
	brTableSequence
	// exitSequence consists of multiple instructions, and exits the execution immediately.
	// See encodeExitSequence.
	exitSequence
	// atomicRmw represents an atomic read-modify-write operation with two register sources and a register destination.
	atomicRmw
	// atomicCas represents an atomic compare-and-swap operation with three register sources. The value is loaded to
	// the source register containing the comparison value.
	atomicCas
	// atomicLoad represents an atomic load with one source register and a register destination.
	atomicLoad
	// atomicStore represents an atomic store with two source registers and no destination.
	atomicStore
	// dmb represents the data memory barrier instruction in inner-shareable (ish) mode.
	dmb
	// UDF is the undefined instruction. For debugging only.
	udf
	// loadConstBlockArg represents a load of a constant block argument.
	loadConstBlockArg

	// emitSourceOffsetInfo is a dummy instruction to emit source offset info.
	// The existence of this instruction does not affect the execution.
	emitSourceOffsetInfo

	// ------------------- do not define below this line -------------------
	numInstructionKinds
)

func (i *instruction) asLoadConstBlockArg(v uint64, typ ssa.Type, dst regalloc.VReg) *instruction {
	i.kind = loadConstBlockArg
	i.u1 = v
	i.u2 = uint64(typ)
	i.rd = dst
	return i
}

func (i *instruction) loadConstBlockArgData() (v uint64, typ ssa.Type, dst regalloc.VReg) {
	return i.u1, ssa.Type(i.u2), i.rd
}

func (i *instruction) asEmitSourceOffsetInfo(l ssa.SourceOffset) *instruction {
	i.kind = emitSourceOffsetInfo
	i.u1 = uint64(l)
	return i
}

func (i *instruction) sourceOffsetInfo() ssa.SourceOffset {
	return ssa.SourceOffset(i.u1)
}

func (i *instruction) asUDF() *instruction {
	i.kind = udf
	return i
}

func (i *instruction) asFpuToInt(rd regalloc.VReg, rn operand, rdSigned, src64bit, dst64bit bool) {
	i.kind = fpuToInt
	i.rn = rn
	i.rd = rd
	if rdSigned {
		i.u1 = 1
	}
	if src64bit {
		i.u2 = 1
	}
	if dst64bit {
		i.u2 |= 2
	}
}

func (i *instruction) asIntToFpu(rd regalloc.VReg, rn operand, rnSigned, src64bit, dst64bit bool) {
	i.kind = intToFpu
	i.rn = rn
	i.rd = rd
	if rnSigned {
		i.u1 = 1
	}
	if src64bit {
		i.u2 = 1
	}
	if dst64bit {
		i.u2 |= 2
	}
}

func (i *instruction) asExitSequence(ctx regalloc.VReg) *instruction {
	i.kind = exitSequence
	i.rn = operandNR(ctx)
	return i
}

// aluOp determines the type of ALU operation. Instructions whose kind is one of
// aluRRR, aluRRRR, aluRRImm12, aluRRBitmaskImm, aluRRImmShift, aluRRRShift and aluRRRExtend
// would use this type.
type aluOp uint32

func (a aluOp) String() string {
	switch a {
	case aluOpAdd:
		return "add"
	case aluOpSub:
		return "sub"
	case aluOpOrr:
		return "orr"
	case aluOpOrn:
		return "orn"
	case aluOpAnd:
		return "and"
	case aluOpAnds:
		return "ands"
	case aluOpBic:
		return "bic"
	case aluOpEor:
		return "eor"
	case aluOpAddS:
		return "adds"
	case aluOpSubS:
		return "subs"
	case aluOpSMulH:
		return "sMulH"
	case aluOpUMulH:
		return "uMulH"
	case aluOpSDiv:
		return "sdiv"
	case aluOpUDiv:
		return "udiv"
	case aluOpRotR:
		return "ror"
	case aluOpLsr:
		return "lsr"
	case aluOpAsr:
		return "asr"
	case aluOpLsl:
		return "lsl"
	case aluOpMAdd:
		return "madd"
	case aluOpMSub:
		return "msub"
	}
	panic(int(a))
}

const (
	// 32/64-bit Add.
	aluOpAdd aluOp = iota
	// 32/64-bit Subtract.
	aluOpSub
	// 32/64-bit Bitwise OR.
	aluOpOrr
	// 32/64-bit Bitwise OR NOT.
	aluOpOrn
	// 32/64-bit Bitwise AND.
	aluOpAnd
	// 32/64-bit Bitwise ANDS.
	aluOpAnds
	// 32/64-bit Bitwise AND NOT.
	aluOpBic
	// 32/64-bit Bitwise XOR (Exclusive OR).
	aluOpEor
	// 32/64-bit Add setting flags.
	aluOpAddS
	// 32/64-bit Subtract setting flags.
	aluOpSubS
	// Signed multiply, high-word result.
	aluOpSMulH
	// Unsigned multiply, high-word result.
	aluOpUMulH
	// 64-bit Signed divide.
	aluOpSDiv
	// 64-bit Unsigned divide.
	aluOpUDiv
	// 32/64-bit Rotate right.
	aluOpRotR
	// 32/64-bit Logical shift right.
	aluOpLsr
	// 32/64-bit Arithmetic shift right.
	aluOpAsr
	// 32/64-bit Logical shift left.
	aluOpLsl /// Multiply-add

	// MAdd and MSub are only applicable for aluRRRR.
	aluOpMAdd
	aluOpMSub
)

// vecOp determines the type of vector operation. Instructions whose kind is one of
// vecOpCnt would use this type.
type vecOp int

// String implements fmt.Stringer.
func (b vecOp) String() string {
	switch b {
	case vecOpCnt:
		return "cnt"
	case vecOpCmeq:
		return "cmeq"
	case vecOpCmgt:
		return "cmgt"
	case vecOpCmhi:
		return "cmhi"
	case vecOpCmge:
		return "cmge"
	case vecOpCmhs:
		return "cmhs"
	case vecOpFcmeq:
		return "fcmeq"
	case vecOpFcmgt:
		return "fcmgt"
	case vecOpFcmge:
		return "fcmge"
	case vecOpCmeq0:
		return "cmeq0"
	case vecOpUaddlv:
		return "uaddlv"
	case vecOpBit:
		return "bit"
	case vecOpBic:
		return "bic"
	case vecOpBsl:
		return "bsl"
	case vecOpNot:
		return "not"
	case vecOpAnd:
		return "and"
	case vecOpOrr:
		return "orr"
	case vecOpEOR:
		return "eor"
	case vecOpFadd:
		return "fadd"
	case vecOpAdd:
		return "add"
	case vecOpAddp:
		return "addp"
	case vecOpAddv:
		return "addv"
	case vecOpSub:
		return "sub"
	case vecOpFsub:
		return "fsub"
	case vecOpSmin:
		return "smin"
	case vecOpUmin:
		return "umin"
	case vecOpUminv:
		return "uminv"
	case vecOpSmax:
		return "smax"
	case vecOpUmax:
		return "umax"
	case vecOpUmaxp:
		return "umaxp"
	case vecOpUrhadd:
		return "urhadd"
	case vecOpFmul:
		return "fmul"
	case vecOpSqrdmulh:
		return "sqrdmulh"
	case vecOpMul:
		return "mul"
	case vecOpUmlal:
		return "umlal"
	case vecOpFdiv:
		return "fdiv"
	case vecOpFsqrt:
		return "fsqrt"
	case vecOpAbs:
		return "abs"
	case vecOpFabs:
		return "fabs"
	case vecOpNeg:
		return "neg"
	case vecOpFneg:
		return "fneg"
	case vecOpFrintp:
		return "frintp"
	case vecOpFrintm:
		return "frintm"
	case vecOpFrintn:
		return "frintn"
	case vecOpFrintz:
		return "frintz"
	case vecOpFcvtl:
		return "fcvtl"
	case vecOpFcvtn:
		return "fcvtn"
	case vecOpFcvtzu:
		return "fcvtzu"
	case vecOpFcvtzs:
		return "fcvtzs"
	case vecOpScvtf:
		return "scvtf"
	case vecOpUcvtf:
		return "ucvtf"
	case vecOpSqxtn:
		return "sqxtn"
	case vecOpUqxtn:
		return "uqxtn"
	case vecOpSqxtun:
		return "sqxtun"
	case vecOpRev64:
		return "rev64"
	case vecOpXtn:
		return "xtn"
	case vecOpShll:
		return "shll"
	case vecOpSshl:
		return "sshl"
	case vecOpSshll:
		return "sshll"
	case vecOpUshl:
		return "ushl"
	case vecOpUshll:
		return "ushll"
	case vecOpSshr:
		return "sshr"
	case vecOpZip1:
		return "zip1"
	case vecOpFmin:
		return "fmin"
	case vecOpFmax:
		return "fmax"
	case vecOpSmull:
		return "smull"
	case vecOpSmull2:
		return "smull2"
	}
	panic(int(b))
}

const (
	vecOpCnt vecOp = iota
	vecOpCmeq0
	vecOpCmeq
	vecOpCmgt
	vecOpCmhi
	vecOpCmge
	vecOpCmhs
	vecOpFcmeq
	vecOpFcmgt
	vecOpFcmge
	vecOpUaddlv
	vecOpBit
	vecOpBic
	vecOpBsl
	vecOpNot
	vecOpAnd
	vecOpOrr
	vecOpEOR
	vecOpAdd
	vecOpFadd
	vecOpAddv
	vecOpSqadd
	vecOpUqadd
	vecOpAddp
	vecOpSub
	vecOpFsub
	vecOpSqsub
	vecOpUqsub
	vecOpSmin
	vecOpUmin
	vecOpUminv
	vecOpFmin
	vecOpSmax
	vecOpUmax
	vecOpUmaxp
	vecOpFmax
	vecOpUrhadd
	vecOpMul
	vecOpFmul
	vecOpSqrdmulh
	vecOpUmlal
	vecOpFdiv
	vecOpFsqrt
	vecOpAbs
	vecOpFabs
	vecOpNeg
	vecOpFneg
	vecOpFrintm
	vecOpFrintn
	vecOpFrintp
	vecOpFrintz
	vecOpFcvtl
	vecOpFcvtn
	vecOpFcvtzs
	vecOpFcvtzu
	vecOpScvtf
	vecOpUcvtf
	vecOpSqxtn
	vecOpSqxtun
	vecOpUqxtn
	vecOpRev64
	vecOpXtn
	vecOpShll
	vecOpSshl
	vecOpSshll
	vecOpUshl
	vecOpUshll
	vecOpSshr
	vecOpZip1
	vecOpSmull
	vecOpSmull2
)

// bitOp determines the type of bitwise operation. Instructions whose kind is one of
// bitOpRbit and bitOpClz would use this type.
type bitOp int

// String implements fmt.Stringer.
func (b bitOp) String() string {
	switch b {
	case bitOpRbit:
		return "rbit"
	case bitOpClz:
		return "clz"
	}
	panic(int(b))
}

const (
	// 32/64-bit Rbit.
	bitOpRbit bitOp = iota
	// 32/64-bit Clz.
	bitOpClz
)

// fpuUniOp represents a unary floating-point unit (FPU) operation.
type fpuUniOp byte

const (
	fpuUniOpNeg fpuUniOp = iota
	fpuUniOpCvt32To64
	fpuUniOpCvt64To32
	fpuUniOpSqrt
	fpuUniOpRoundPlus
	fpuUniOpRoundMinus
	fpuUniOpRoundZero
	fpuUniOpRoundNearest
	fpuUniOpAbs
)

// String implements the fmt.Stringer.
func (f fpuUniOp) String() string {
	switch f {
	case fpuUniOpNeg:
		return "fneg"
	case fpuUniOpCvt32To64:
		return "fcvt"
	case fpuUniOpCvt64To32:
		return "fcvt"
	case fpuUniOpSqrt:
		return "fsqrt"
	case fpuUniOpRoundPlus:
		return "frintp"
	case fpuUniOpRoundMinus:
		return "frintm"
	case fpuUniOpRoundZero:
		return "frintz"
	case fpuUniOpRoundNearest:
		return "frintn"
	case fpuUniOpAbs:
		return "fabs"
	}
	panic(int(f))
}

// fpuBinOp represents a binary floating-point unit (FPU) operation.
type fpuBinOp byte

const (
	fpuBinOpAdd = iota
	fpuBinOpSub
	fpuBinOpMul
	fpuBinOpDiv
	fpuBinOpMax
	fpuBinOpMin
)

// String implements the fmt.Stringer.
func (f fpuBinOp) String() string {
	switch f {
	case fpuBinOpAdd:
		return "fadd"
	case fpuBinOpSub:
		return "fsub"
	case fpuBinOpMul:
		return "fmul"
	case fpuBinOpDiv:
		return "fdiv"
	case fpuBinOpMax:
		return "fmax"
	case fpuBinOpMin:
		return "fmin"
	}
	panic(int(f))
}

// extMode represents the mode of a register operand extension.
// For example, aluRRRExtend instructions need this info to determine the extensions.
type extMode byte

const (
	extModeNone extMode = iota
	// extModeZeroExtend64 suggests a zero-extension to 32 bits if the original bit size is less than 32.
	extModeZeroExtend32
	// extModeSignExtend64 stands for a sign-extension to 32 bits if the original bit size is less than 32.
	extModeSignExtend32
	// extModeZeroExtend64 suggests a zero-extension to 64 bits if the original bit size is less than 64.
	extModeZeroExtend64
	// extModeSignExtend64 stands for a sign-extension to 64 bits if the original bit size is less than 64.
	extModeSignExtend64
)

func (e extMode) bits() byte {
	switch e {
	case extModeZeroExtend32, extModeSignExtend32:
		return 32
	case extModeZeroExtend64, extModeSignExtend64:
		return 64
	default:
		return 0
	}
}

func (e extMode) signed() bool {
	switch e {
	case extModeSignExtend32, extModeSignExtend64:
		return true
	default:
		return false
	}
}

func extModeOf(t ssa.Type, signed bool) extMode {
	switch t.Bits() {
	case 32:
		if signed {
			return extModeSignExtend32
		}
		return extModeZeroExtend32
	case 64:
		if signed {
			return extModeSignExtend64
		}
		return extModeZeroExtend64
	default:
		panic("TODO? do we need narrower than 32 bits?")
	}
}

type extendOp byte

const (
	extendOpUXTB extendOp = 0b000
	extendOpUXTH extendOp = 0b001
	extendOpUXTW extendOp = 0b010
	// extendOpUXTX does nothing, but convenient symbol that officially exists. See:
	// https://stackoverflow.com/questions/72041372/what-do-the-uxtx-and-sxtx-extensions-mean-for-32-bit-aarch64-adds-instruct
	extendOpUXTX extendOp = 0b011
	extendOpSXTB extendOp = 0b100
	extendOpSXTH extendOp = 0b101
	extendOpSXTW extendOp = 0b110
	// extendOpSXTX does nothing, but convenient symbol that officially exists. See:
	// https://stackoverflow.com/questions/72041372/what-do-the-uxtx-and-sxtx-extensions-mean-for-32-bit-aarch64-adds-instruct
	extendOpSXTX extendOp = 0b111
	extendOpNone extendOp = 0xff
)

func (e extendOp) srcBits() byte {
	switch e {
	case extendOpUXTB, extendOpSXTB:
		return 8
	case extendOpUXTH, extendOpSXTH:
		return 16
	case extendOpUXTW, extendOpSXTW:
		return 32
	case extendOpUXTX, extendOpSXTX:
		return 64
	}
	panic(int(e))
}

func (e extendOp) String() string {
	switch e {
	case extendOpUXTB:
		return "UXTB"
	case extendOpUXTH:
		return "UXTH"
	case extendOpUXTW:
		return "UXTW"
	case extendOpUXTX:
		return "UXTX"
	case extendOpSXTB:
		return "SXTB"
	case extendOpSXTH:
		return "SXTH"
	case extendOpSXTW:
		return "SXTW"
	case extendOpSXTX:
		return "SXTX"
	}
	panic(int(e))
}

func extendOpFrom(signed bool, from byte) extendOp {
	switch from {
	case 8:
		if signed {
			return extendOpSXTB
		}
		return extendOpUXTB
	case 16:
		if signed {
			return extendOpSXTH
		}
		return extendOpUXTH
	case 32:
		if signed {
			return extendOpSXTW
		}
		return extendOpUXTW
	case 64:
		if signed {
			return extendOpSXTX
		}
		return extendOpUXTX
	}
	panic("invalid extendOpFrom")
}

type shiftOp byte

const (
	shiftOpLSL shiftOp = 0b00
	shiftOpLSR shiftOp = 0b01
	shiftOpASR shiftOp = 0b10
	shiftOpROR shiftOp = 0b11
)

func (s shiftOp) String() string {
	switch s {
	case shiftOpLSL:
		return "lsl"
	case shiftOpLSR:
		return "lsr"
	case shiftOpASR:
		return "asr"
	case shiftOpROR:
		return "ror"
	}
	panic(int(s))
}

const exitSequenceSize = 6 * 4 // 6 instructions as in encodeExitSequence.

// size returns the size of the instruction in encoded bytes.
func (i *instruction) size() int64 {
	switch i.kind {
	case exitSequence:
		return exitSequenceSize // 5 instructions as in encodeExitSequence.
	case nop0, loadConstBlockArg:
		return 0
	case emitSourceOffsetInfo:
		return 0
	case loadFpuConst32:
		if i.u1 == 0 {
			return 4 // zero loading can be encoded as a single instruction.
		}
		return 4 + 4 + 4
	case loadFpuConst64:
		if i.u1 == 0 {
			return 4 // zero loading can be encoded as a single instruction.
		}
		return 4 + 4 + 8
	case loadFpuConst128:
		if i.u1 == 0 && i.u2 == 0 {
			return 4 // zero loading can be encoded as a single instruction.
		}
		return 4 + 4 + 16
	case brTableSequence:
		return 4*4 + int64(i.u2)*4
	default:
		return 4
	}
}

// vecArrangement is the arrangement of data within a vector register.
type vecArrangement byte

const (
	// vecArrangementNone is an arrangement indicating no data is stored.
	vecArrangementNone vecArrangement = iota
	// vecArrangement8B is an arrangement of 8 bytes (64-bit vector)
	vecArrangement8B
	// vecArrangement16B is an arrangement of 16 bytes (128-bit vector)
	vecArrangement16B
	// vecArrangement4H is an arrangement of 4 half precisions (64-bit vector)
	vecArrangement4H
	// vecArrangement8H is an arrangement of 8 half precisions (128-bit vector)
	vecArrangement8H
	// vecArrangement2S is an arrangement of 2 single precisions (64-bit vector)
	vecArrangement2S
	// vecArrangement4S is an arrangement of 4 single precisions (128-bit vector)
	vecArrangement4S
	// vecArrangement1D is an arrangement of 1 double precision (64-bit vector)
	vecArrangement1D
	// vecArrangement2D is an arrangement of 2 double precisions (128-bit vector)
	vecArrangement2D

	// Assign each vector size specifier to a vector arrangement ID.
	// Instructions can only have an arrangement or a size specifier, but not both, so it
	// simplifies the internal representation of vector instructions by being able to
	// store either into the same field.

	// vecArrangementB is a size specifier of byte
	vecArrangementB
	// vecArrangementH is a size specifier of word (16-bit)
	vecArrangementH
	// vecArrangementS is a size specifier of double word (32-bit)
	vecArrangementS
	// vecArrangementD is a size specifier of quad word (64-bit)
	vecArrangementD
	// vecArrangementQ is a size specifier of the entire vector (128-bit)
	vecArrangementQ
)

// String implements fmt.Stringer
func (v vecArrangement) String() (ret string) {
	switch v {
	case vecArrangement8B:
		ret = "8B"
	case vecArrangement16B:
		ret = "16B"
	case vecArrangement4H:
		ret = "4H"
	case vecArrangement8H:
		ret = "8H"
	case vecArrangement2S:
		ret = "2S"
	case vecArrangement4S:
		ret = "4S"
	case vecArrangement1D:
		ret = "1D"
	case vecArrangement2D:
		ret = "2D"
	case vecArrangementB:
		ret = "B"
	case vecArrangementH:
		ret = "H"
	case vecArrangementS:
		ret = "S"
	case vecArrangementD:
		ret = "D"
	case vecArrangementQ:
		ret = "Q"
	case vecArrangementNone:
		ret = "none"
	default:
		panic(v)
	}
	return
}

// vecIndex is the index of an element of a vector register
type vecIndex byte

// vecIndexNone indicates no vector index specified.
const vecIndexNone = ^vecIndex(0)

func ssaLaneToArrangement(lane ssa.VecLane) vecArrangement {
	switch lane {
	case ssa.VecLaneI8x16:
		return vecArrangement16B
	case ssa.VecLaneI16x8:
		return vecArrangement8H
	case ssa.VecLaneI32x4:
		return vecArrangement4S
	case ssa.VecLaneI64x2:
		return vecArrangement2D
	case ssa.VecLaneF32x4:
		return vecArrangement4S
	case ssa.VecLaneF64x2:
		return vecArrangement2D
	default:
		panic(lane)
	}
}

// atomicRmwOp is the type of atomic read-modify-write operation.
type atomicRmwOp byte

const (
	// atomicRmwOpAdd is an atomic add operation.
	atomicRmwOpAdd atomicRmwOp = iota
	// atomicRmwOpClr is an atomic clear operation, i.e. AND NOT.
	atomicRmwOpClr
	// atomicRmwOpSet is an atomic set operation, i.e. OR.
	atomicRmwOpSet
	// atomicRmwOpEor is an atomic exclusive OR operation.
	atomicRmwOpEor
	// atomicRmwOpSwp is an atomic swap operation.
	atomicRmwOpSwp
)

// String implements fmt.Stringer
func (a atomicRmwOp) String() string {
	switch a {
	case atomicRmwOpAdd:
		return "ldaddal"
	case atomicRmwOpClr:
		return "ldclral"
	case atomicRmwOpSet:
		return "ldsetal"
	case atomicRmwOpEor:
		return "ldeoral"
	case atomicRmwOpSwp:
		return "swpal"
	}
	panic(fmt.Sprintf("unknown atomicRmwOp: %d", a))
}
