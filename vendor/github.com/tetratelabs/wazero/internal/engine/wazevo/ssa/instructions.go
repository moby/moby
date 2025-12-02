package ssa

import (
	"fmt"
	"math"
	"strings"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// Opcode represents a SSA instruction.
type Opcode uint32

// Instruction represents an instruction whose opcode is specified by
// Opcode. Since Go doesn't have union type, we use this flattened type
// for all instructions, and therefore each field has different meaning
// depending on Opcode.
type Instruction struct {
	// id is the unique ID of this instruction which ascends from 0 following the order of program.
	id         int
	opcode     Opcode
	u1, u2     uint64
	v          Value
	v2         Value
	v3         Value
	vs         Values
	typ        Type
	prev, next *Instruction

	// rValue is the (first) return value of this instruction.
	// For branching instructions except for OpcodeBrTable, they hold BlockID to jump cast to Value.
	rValue Value
	// rValues are the rest of the return values of this instruction.
	// For OpcodeBrTable, it holds the list of BlockID to jump cast to Value.
	rValues        Values
	gid            InstructionGroupID
	sourceOffset   SourceOffset
	live           bool
	alreadyLowered bool
}

// SourceOffset represents the offset of the source of an instruction.
type SourceOffset int64

const sourceOffsetUnknown = -1

// Valid returns true if this source offset is valid.
func (l SourceOffset) Valid() bool {
	return l != sourceOffsetUnknown
}

func (i *Instruction) annotateSourceOffset(line SourceOffset) {
	i.sourceOffset = line
}

// SourceOffset returns the source offset of this instruction.
func (i *Instruction) SourceOffset() SourceOffset {
	return i.sourceOffset
}

// Opcode returns the opcode of this instruction.
func (i *Instruction) Opcode() Opcode {
	return i.opcode
}

// GroupID returns the InstructionGroupID of this instruction.
func (i *Instruction) GroupID() InstructionGroupID {
	return i.gid
}

// MarkLowered marks this instruction as already lowered.
func (i *Instruction) MarkLowered() {
	i.alreadyLowered = true
}

// Lowered returns true if this instruction is already lowered.
func (i *Instruction) Lowered() bool {
	return i.alreadyLowered
}

// resetInstruction resets this instruction to the initial state.
func resetInstruction(i *Instruction) {
	*i = Instruction{}
	i.v = ValueInvalid
	i.v2 = ValueInvalid
	i.v3 = ValueInvalid
	i.rValue = ValueInvalid
	i.typ = typeInvalid
	i.vs = ValuesNil
	i.sourceOffset = sourceOffsetUnknown
}

// InstructionGroupID is assigned to each instruction and represents a group of instructions
// where each instruction is interchangeable with others except for the last instruction
// in the group which has side effects. In short, InstructionGroupID is determined by the side effects of instructions.
// That means, if there's an instruction with side effect between two instructions, then these two instructions
// will have different instructionGroupID. Note that each block always ends with branching, which is with side effects,
// therefore, instructions in different blocks always have different InstructionGroupID(s).
//
// The notable application of this is used in lowering SSA-level instruction to a ISA specific instruction,
// where we eagerly try to merge multiple instructions into single operation etc. Such merging cannot be done
// if these instruction have different InstructionGroupID since it will change the semantics of a program.
//
// See passDeadCodeElimination.
type InstructionGroupID uint32

// Returns Value(s) produced by this instruction if any.
// The `first` is the first return value, and `rest` is the rest of the values.
func (i *Instruction) Returns() (first Value, rest []Value) {
	if i.IsBranching() {
		return ValueInvalid, nil
	}
	return i.rValue, i.rValues.View()
}

// Return returns a Value(s) produced by this instruction if any.
// If there's multiple return values, only the first one is returned.
func (i *Instruction) Return() (first Value) {
	return i.rValue
}

// Args returns the arguments to this instruction.
func (i *Instruction) Args() (v1, v2, v3 Value, vs []Value) {
	return i.v, i.v2, i.v3, i.vs.View()
}

// Arg returns the first argument to this instruction.
func (i *Instruction) Arg() Value {
	return i.v
}

// Arg2 returns the first two arguments to this instruction.
func (i *Instruction) Arg2() (Value, Value) {
	return i.v, i.v2
}

// ArgWithLane returns the first argument to this instruction, and the lane type.
func (i *Instruction) ArgWithLane() (Value, VecLane) {
	return i.v, VecLane(i.u1)
}

// Arg2WithLane returns the first two arguments to this instruction, and the lane type.
func (i *Instruction) Arg2WithLane() (Value, Value, VecLane) {
	return i.v, i.v2, VecLane(i.u1)
}

// ShuffleData returns the first two arguments to this instruction and 2 uint64s `lo`, `hi`.
//
// Note: Each uint64 encodes a sequence of 8 bytes where each byte encodes a VecLane,
// so that the 128bit integer `hi<<64|lo` packs a slice `[16]VecLane`,
// where `lane[0]` is the least significant byte, and `lane[n]` is shifted to offset `n*8`.
func (i *Instruction) ShuffleData() (v Value, v2 Value, lo uint64, hi uint64) {
	return i.v, i.v2, i.u1, i.u2
}

// Arg3 returns the first three arguments to this instruction.
func (i *Instruction) Arg3() (Value, Value, Value) {
	return i.v, i.v2, i.v3
}

// Next returns the next instruction laid out next to itself.
func (i *Instruction) Next() *Instruction {
	return i.next
}

// Prev returns the previous instruction laid out prior to itself.
func (i *Instruction) Prev() *Instruction {
	return i.prev
}

// IsBranching returns true if this instruction is a branching instruction.
func (i *Instruction) IsBranching() bool {
	switch i.opcode {
	case OpcodeJump, OpcodeBrz, OpcodeBrnz, OpcodeBrTable:
		return true
	default:
		return false
	}
}

// TODO: complete opcode comments.
const (
	OpcodeInvalid Opcode = iota

	// OpcodeUndefined is a placeholder for undefined opcode. This can be used for debugging to intentionally
	// cause a crash at certain point.
	OpcodeUndefined

	// OpcodeJump takes the list of args to the `block` and unconditionally jumps to it.
	OpcodeJump

	// OpcodeBrz branches into `blk` with `args`  if the value `c` equals zero: `Brz c, blk, args`.
	OpcodeBrz

	// OpcodeBrnz branches into `blk` with `args`  if the value `c` is not zero: `Brnz c, blk, args`.
	OpcodeBrnz

	// OpcodeBrTable takes the index value `index`, and branches into `labelX`. If the `index` is out of range,
	// it branches into the last labelN: `BrTable index, [label1, label2, ... labelN]`.
	OpcodeBrTable

	// OpcodeExitWithCode exit the execution immediately.
	OpcodeExitWithCode

	// OpcodeExitIfTrueWithCode exits the execution immediately if the value `c` is not zero.
	OpcodeExitIfTrueWithCode

	// OpcodeReturn returns from the function: `return rvalues`.
	OpcodeReturn

	// OpcodeCall calls a function specified by the symbol FN with arguments `args`: `returnvals = Call FN, args...`
	// This is a "near" call, which means the call target is known at compile time, and the target is relatively close
	// to this function. If the target cannot be reached by near call, the backend fails to compile.
	OpcodeCall

	// OpcodeCallIndirect calls a function specified by `callee` which is a function address: `returnvals = call_indirect SIG, callee, args`.
	// Note that this is different from call_indirect in Wasm, which also does type checking, etc.
	OpcodeCallIndirect

	// OpcodeSplat performs a vector splat operation: `v = Splat.lane x`.
	OpcodeSplat

	// OpcodeSwizzle performs a vector swizzle operation: `v = Swizzle.lane x, y`.
	OpcodeSwizzle

	// OpcodeInsertlane inserts a lane value into a vector: `v = InsertLane x, y, Idx`.
	OpcodeInsertlane

	// OpcodeExtractlane extracts a lane value from a vector: `v = ExtractLane x, Idx`.
	OpcodeExtractlane

	// OpcodeLoad loads a Type value from the [base + offset] address: `v = Load base, offset`.
	OpcodeLoad

	// OpcodeStore stores a Type value to the [base + offset] address: `Store v, base, offset`.
	OpcodeStore

	// OpcodeUload8 loads the 8-bit value from the [base + offset] address, zero-extended to 64 bits: `v = Uload8 base, offset`.
	OpcodeUload8

	// OpcodeSload8 loads the 8-bit value from the [base + offset] address, sign-extended to 64 bits: `v = Sload8 base, offset`.
	OpcodeSload8

	// OpcodeIstore8 stores the 8-bit value to the [base + offset] address, sign-extended to 64 bits: `Istore8 v, base, offset`.
	OpcodeIstore8

	// OpcodeUload16 loads the 16-bit value from the [base + offset] address, zero-extended to 64 bits: `v = Uload16 base, offset`.
	OpcodeUload16

	// OpcodeSload16 loads the 16-bit value from the [base + offset] address, sign-extended to 64 bits: `v = Sload16 base, offset`.
	OpcodeSload16

	// OpcodeIstore16 stores the 16-bit value to the [base + offset] address, zero-extended to 64 bits: `Istore16 v, base, offset`.
	OpcodeIstore16

	// OpcodeUload32 loads the 32-bit value from the [base + offset] address, zero-extended to 64 bits: `v = Uload32 base, offset`.
	OpcodeUload32

	// OpcodeSload32 loads the 32-bit value from the [base + offset] address, sign-extended to 64 bits: `v = Sload32 base, offset`.
	OpcodeSload32

	// OpcodeIstore32 stores the 32-bit value to the [base + offset] address, zero-extended to 64 bits: `Istore16 v, base, offset`.
	OpcodeIstore32

	// OpcodeLoadSplat represents a load that replicates the loaded value to all lanes `v = LoadSplat.lane p, Offset`.
	OpcodeLoadSplat

	// OpcodeVZeroExtLoad loads a scalar single/double precision floating point value from the [p + Offset] address,
	// and zero-extend it to the V128 value: `v = VExtLoad  p, Offset`.
	OpcodeVZeroExtLoad

	// OpcodeIconst represents the integer const.
	OpcodeIconst

	// OpcodeF32const represents the single-precision const.
	OpcodeF32const

	// OpcodeF64const represents the double-precision const.
	OpcodeF64const

	// OpcodeVconst represents the 128bit vector const.
	OpcodeVconst

	// OpcodeVbor computes binary or between two 128bit vectors: `v = bor x, y`.
	OpcodeVbor

	// OpcodeVbxor computes binary xor between two 128bit vectors: `v = bxor x, y`.
	OpcodeVbxor

	// OpcodeVband computes binary and between two 128bit vectors: `v = band x, y`.
	OpcodeVband

	// OpcodeVbandnot computes binary and-not between two 128bit vectors: `v = bandnot x, y`.
	OpcodeVbandnot

	// OpcodeVbnot negates a 128bit vector: `v = bnot x`.
	OpcodeVbnot

	// OpcodeVbitselect uses the bits in the control mask c to select the corresponding bit from x when 1
	// and y when 0: `v = bitselect c, x, y`.
	OpcodeVbitselect

	// OpcodeShuffle shuffles two vectors using the given 128-bit immediate: `v = shuffle imm, x, y`.
	// For each byte in the immediate, a value i in [0, 15] selects the i-th byte in vector x;
	// i in [16, 31] selects the (i-16)-th byte in vector y.
	OpcodeShuffle

	// OpcodeSelect chooses between two values based on a condition `c`: `v = Select c, x, y`.
	OpcodeSelect

	// OpcodeVanyTrue performs a any true operation: `s = VanyTrue a`.
	OpcodeVanyTrue

	// OpcodeVallTrue performs a lane-wise all true operation: `s = VallTrue.lane a`.
	OpcodeVallTrue

	// OpcodeVhighBits performs a lane-wise extract of the high bits: `v = VhighBits.lane a`.
	OpcodeVhighBits

	// OpcodeIcmp compares two integer values with the given condition: `v = icmp Cond, x, y`.
	OpcodeIcmp

	// OpcodeVIcmp compares two integer values with the given condition: `v = vicmp Cond, x, y` on vector.
	OpcodeVIcmp

	// OpcodeIcmpImm compares an integer value with the immediate value on the given condition: `v = icmp_imm Cond, x, Y`.
	OpcodeIcmpImm

	// OpcodeIadd performs an integer addition: `v = Iadd x, y`.
	OpcodeIadd

	// OpcodeVIadd performs an integer addition: `v = VIadd.lane x, y` on vector.
	OpcodeVIadd

	// OpcodeVSaddSat performs a signed saturating vector addition: `v = VSaddSat.lane x, y` on vector.
	OpcodeVSaddSat

	// OpcodeVUaddSat performs an unsigned saturating vector addition: `v = VUaddSat.lane x, y` on vector.
	OpcodeVUaddSat

	// OpcodeIsub performs an integer subtraction: `v = Isub x, y`.
	OpcodeIsub

	// OpcodeVIsub performs an integer subtraction: `v = VIsub.lane x, y` on vector.
	OpcodeVIsub

	// OpcodeVSsubSat performs a signed saturating vector subtraction: `v = VSsubSat.lane x, y` on vector.
	OpcodeVSsubSat

	// OpcodeVUsubSat performs an unsigned saturating vector subtraction: `v = VUsubSat.lane x, y` on vector.
	OpcodeVUsubSat

	// OpcodeVImin performs a signed integer min: `v = VImin.lane x, y` on vector.
	OpcodeVImin

	// OpcodeVUmin performs an unsigned integer min: `v = VUmin.lane x, y` on vector.
	OpcodeVUmin

	// OpcodeVImax performs a signed integer max: `v = VImax.lane x, y` on vector.
	OpcodeVImax

	// OpcodeVUmax performs an unsigned integer max: `v = VUmax.lane x, y` on vector.
	OpcodeVUmax

	// OpcodeVAvgRound performs an unsigned integer avg, truncating to zero: `v = VAvgRound.lane x, y` on vector.
	OpcodeVAvgRound

	// OpcodeVImul performs an integer multiplication: `v = VImul.lane x, y` on vector.
	OpcodeVImul

	// OpcodeVIneg negates the given integer vector value: `v = VIneg x`.
	OpcodeVIneg

	// OpcodeVIpopcnt counts the number of 1-bits in the given vector: `v = VIpopcnt x`.
	OpcodeVIpopcnt

	// OpcodeVIabs returns the absolute value for the given vector value: `v = VIabs.lane x`.
	OpcodeVIabs

	// OpcodeVIshl shifts x left by (y mod lane-width): `v = VIshl.lane x, y` on vector.
	OpcodeVIshl

	// OpcodeVUshr shifts x right by (y mod lane-width), unsigned: `v = VUshr.lane x, y` on vector.
	OpcodeVUshr

	// OpcodeVSshr shifts x right by (y mod lane-width), signed: `v = VSshr.lane x, y` on vector.
	OpcodeVSshr

	// OpcodeVFabs takes the absolute value of a floating point value: `v = VFabs.lane x on vector.
	OpcodeVFabs

	// OpcodeVFmax takes the maximum of two floating point values: `v = VFmax.lane x, y on vector.
	OpcodeVFmax

	// OpcodeVFmin takes the minimum of two floating point values: `v = VFmin.lane x, y on vector.
	OpcodeVFmin

	// OpcodeVFneg negates the given floating point vector value: `v = VFneg x`.
	OpcodeVFneg

	// OpcodeVFadd performs a floating point addition: `v = VFadd.lane x, y` on vector.
	OpcodeVFadd

	// OpcodeVFsub performs a floating point subtraction: `v = VFsub.lane x, y` on vector.
	OpcodeVFsub

	// OpcodeVFmul performs a floating point multiplication: `v = VFmul.lane x, y` on vector.
	OpcodeVFmul

	// OpcodeVFdiv performs a floating point division: `v = VFdiv.lane x, y` on vector.
	OpcodeVFdiv

	// OpcodeVFcmp compares two float values with the given condition: `v = VFcmp.lane Cond, x, y` on float.
	OpcodeVFcmp

	// OpcodeVCeil takes the ceiling of the given floating point value: `v = ceil.lane x` on vector.
	OpcodeVCeil

	// OpcodeVFloor takes the floor of the given floating point value: `v = floor.lane x` on vector.
	OpcodeVFloor

	// OpcodeVTrunc takes the truncation of the given floating point value: `v = trunc.lane x` on vector.
	OpcodeVTrunc

	// OpcodeVNearest takes the nearest integer of the given floating point value: `v = nearest.lane x` on vector.
	OpcodeVNearest

	// OpcodeVMaxPseudo computes the lane-wise maximum value `v = VMaxPseudo.lane x, y` on vector defined as `x < y ? x : y`.
	OpcodeVMaxPseudo

	// OpcodeVMinPseudo computes the lane-wise minimum value `v = VMinPseudo.lane x, y` on vector defined as `y < x ? x : y`.
	OpcodeVMinPseudo

	// OpcodeVSqrt takes the minimum of two floating point values: `v = VFmin.lane x, y` on vector.
	OpcodeVSqrt

	// OpcodeVFcvtToUintSat converts a floating point value to an unsigned integer: `v = FcvtToUintSat.lane x` on vector.
	OpcodeVFcvtToUintSat

	// OpcodeVFcvtToSintSat converts a floating point value to a signed integer: `v = VFcvtToSintSat.lane x` on vector.
	OpcodeVFcvtToSintSat

	// OpcodeVFcvtFromUint converts a floating point value from an unsigned integer: `v = FcvtFromUint.lane x` on vector.
	// x is always a 32-bit integer lane, and the result is either a 32-bit or 64-bit floating point-sized vector.
	OpcodeVFcvtFromUint

	// OpcodeVFcvtFromSint converts a floating point value from a signed integer: `v = VFcvtFromSint.lane x` on vector.
	// x is always a 32-bit integer lane, and the result is either a 32-bit or 64-bit floating point-sized vector.
	OpcodeVFcvtFromSint

	// OpcodeImul performs an integer multiplication: `v = Imul x, y`.
	OpcodeImul

	// OpcodeUdiv performs the unsigned integer division `v = Udiv x, y`.
	OpcodeUdiv

	// OpcodeSdiv performs the signed integer division `v = Sdiv x, y`.
	OpcodeSdiv

	// OpcodeUrem computes the remainder of the unsigned integer division `v = Urem x, y`.
	OpcodeUrem

	// OpcodeSrem computes the remainder of the signed integer division `v = Srem x, y`.
	OpcodeSrem

	// OpcodeBand performs a binary and: `v = Band x, y`.
	OpcodeBand

	// OpcodeBor performs a binary or: `v = Bor x, y`.
	OpcodeBor

	// OpcodeBxor performs a binary xor: `v = Bxor x, y`.
	OpcodeBxor

	// OpcodeBnot performs a binary not: `v = Bnot x`.
	OpcodeBnot

	// OpcodeRotl rotates the given integer value to the left: `v = Rotl x, y`.
	OpcodeRotl

	// OpcodeRotr rotates the given integer value to the right: `v = Rotr x, y`.
	OpcodeRotr

	// OpcodeIshl does logical shift left: `v = Ishl x, y`.
	OpcodeIshl

	// OpcodeUshr does logical shift right: `v = Ushr x, y`.
	OpcodeUshr

	// OpcodeSshr does arithmetic shift right: `v = Sshr x, y`.
	OpcodeSshr

	// OpcodeClz counts the number of leading zeros: `v = clz x`.
	OpcodeClz

	// OpcodeCtz counts the number of trailing zeros: `v = ctz x`.
	OpcodeCtz

	// OpcodePopcnt counts the number of 1-bits: `v = popcnt x`.
	OpcodePopcnt

	// OpcodeFcmp compares two floating point values: `v = fcmp Cond, x, y`.
	OpcodeFcmp

	// OpcodeFadd performs a floating point addition: / `v = Fadd x, y`.
	OpcodeFadd

	// OpcodeFsub performs a floating point subtraction: `v = Fsub x, y`.
	OpcodeFsub

	// OpcodeFmul performs a floating point multiplication: `v = Fmul x, y`.
	OpcodeFmul

	// OpcodeSqmulRoundSat performs a lane-wise saturating rounding multiplication
	// in Q15 format: `v = SqmulRoundSat.lane x,y` on vector.
	OpcodeSqmulRoundSat

	// OpcodeFdiv performs a floating point division: `v = Fdiv x, y`.
	OpcodeFdiv

	// OpcodeSqrt takes the square root of the given floating point value: `v = sqrt x`.
	OpcodeSqrt

	// OpcodeFneg negates the given floating point value: `v = Fneg x`.
	OpcodeFneg

	// OpcodeFabs takes the absolute value of the given floating point value: `v = fabs x`.
	OpcodeFabs

	// OpcodeFcopysign copies the sign of the second floating point value to the first floating point value:
	// `v = Fcopysign x, y`.
	OpcodeFcopysign

	// OpcodeFmin takes the minimum of two floating point values: `v = fmin x, y`.
	OpcodeFmin

	// OpcodeFmax takes the maximum of two floating point values: `v = fmax x, y`.
	OpcodeFmax

	// OpcodeCeil takes the ceiling of the given floating point value: `v = ceil x`.
	OpcodeCeil

	// OpcodeFloor takes the floor of the given floating point value: `v = floor x`.
	OpcodeFloor

	// OpcodeTrunc takes the truncation of the given floating point value: `v = trunc x`.
	OpcodeTrunc

	// OpcodeNearest takes the nearest integer of the given floating point value: `v = nearest x`.
	OpcodeNearest

	// OpcodeBitcast is a bitcast operation: `v = bitcast x`.
	OpcodeBitcast

	// OpcodeIreduce narrow the given integer: `v = Ireduce x`.
	OpcodeIreduce

	// OpcodeSnarrow converts two input vectors x, y into a smaller lane vector by narrowing each lane, signed `v = Snarrow.lane x, y`.
	OpcodeSnarrow

	// OpcodeUnarrow converts two input vectors x, y into a smaller lane vector by narrowing each lane, unsigned `v = Unarrow.lane x, y`.
	OpcodeUnarrow

	// OpcodeSwidenLow converts low half of the smaller lane vector to a larger lane vector, sign extended: `v = SwidenLow.lane x`.
	OpcodeSwidenLow

	// OpcodeSwidenHigh converts high half of the smaller lane vector to a larger lane vector, sign extended: `v = SwidenHigh.lane x`.
	OpcodeSwidenHigh

	// OpcodeUwidenLow converts low half of the smaller lane vector to a larger lane vector, zero (unsigned) extended: `v = UwidenLow.lane x`.
	OpcodeUwidenLow

	// OpcodeUwidenHigh converts high half of the smaller lane vector to a larger lane vector, zero (unsigned) extended: `v = UwidenHigh.lane x`.
	OpcodeUwidenHigh

	// OpcodeExtIaddPairwise is a lane-wise integer extended pairwise addition producing extended results (twice wider results than the inputs): `v = extiadd_pairwise x, y` on vector.
	OpcodeExtIaddPairwise

	// OpcodeWideningPairwiseDotProductS is a lane-wise widening pairwise dot product with signed saturation: `v = WideningPairwiseDotProductS x, y` on vector.
	// Currently, the only lane is i16, and the result is i32.
	OpcodeWideningPairwiseDotProductS

	// OpcodeUExtend zero-extends the given integer: `v = UExtend x, from->to`.
	OpcodeUExtend

	// OpcodeSExtend sign-extends the given integer: `v = SExtend x, from->to`.
	OpcodeSExtend

	// OpcodeFpromote promotes the given floating point value: `v = Fpromote x`.
	OpcodeFpromote

	// OpcodeFvpromoteLow converts the two lower single-precision floating point lanes
	// to the two double-precision lanes of the result: `v = FvpromoteLow.lane x` on vector.
	OpcodeFvpromoteLow

	// OpcodeFdemote demotes the given float point value: `v = Fdemote x`.
	OpcodeFdemote

	// OpcodeFvdemote converts the two double-precision floating point lanes
	// to two lower single-precision lanes of the result `v = Fvdemote.lane x`.
	OpcodeFvdemote

	// OpcodeFcvtToUint converts a floating point value to an unsigned integer: `v = FcvtToUint x`.
	OpcodeFcvtToUint

	// OpcodeFcvtToSint converts a floating point value to a signed integer: `v = FcvtToSint x`.
	OpcodeFcvtToSint

	// OpcodeFcvtToUintSat converts a floating point value to an unsigned integer: `v = FcvtToUintSat x` which saturates on overflow.
	OpcodeFcvtToUintSat

	// OpcodeFcvtToSintSat converts a floating point value to a signed integer: `v = FcvtToSintSat x` which saturates on overflow.
	OpcodeFcvtToSintSat

	// OpcodeFcvtFromUint converts an unsigned integer to a floating point value: `v = FcvtFromUint x`.
	OpcodeFcvtFromUint

	// OpcodeFcvtFromSint converts a signed integer to a floating point value: `v = FcvtFromSint x`.
	OpcodeFcvtFromSint

	// OpcodeAtomicRmw is atomic read-modify-write operation: `v = atomic_rmw op, p, offset, value`.
	OpcodeAtomicRmw

	// OpcodeAtomicCas is atomic compare-and-swap operation.
	OpcodeAtomicCas

	// OpcodeAtomicLoad is atomic load operation.
	OpcodeAtomicLoad

	// OpcodeAtomicStore is atomic store operation.
	OpcodeAtomicStore

	// OpcodeFence is a memory fence operation.
	OpcodeFence

	// opcodeEnd marks the end of the opcode list.
	opcodeEnd
)

// AtomicRmwOp represents the atomic read-modify-write operation.
type AtomicRmwOp byte

const (
	// AtomicRmwOpAdd is an atomic add operation.
	AtomicRmwOpAdd AtomicRmwOp = iota
	// AtomicRmwOpSub is an atomic sub operation.
	AtomicRmwOpSub
	// AtomicRmwOpAnd is an atomic and operation.
	AtomicRmwOpAnd
	// AtomicRmwOpOr is an atomic or operation.
	AtomicRmwOpOr
	// AtomicRmwOpXor is an atomic xor operation.
	AtomicRmwOpXor
	// AtomicRmwOpXchg is an atomic swap operation.
	AtomicRmwOpXchg
)

// String implements the fmt.Stringer.
func (op AtomicRmwOp) String() string {
	switch op {
	case AtomicRmwOpAdd:
		return "add"
	case AtomicRmwOpSub:
		return "sub"
	case AtomicRmwOpAnd:
		return "and"
	case AtomicRmwOpOr:
		return "or"
	case AtomicRmwOpXor:
		return "xor"
	case AtomicRmwOpXchg:
		return "xchg"
	}
	panic(fmt.Sprintf("unknown AtomicRmwOp: %d", op))
}

// returnTypesFn provides the info to determine the type of instruction.
// t1 is the type of the first result, ts are the types of the remaining results.
type returnTypesFn func(b *builder, instr *Instruction) (t1 Type, ts []Type)

var (
	returnTypesFnNoReturns returnTypesFn = func(b *builder, instr *Instruction) (t1 Type, ts []Type) { return typeInvalid, nil }
	returnTypesFnSingle                  = func(b *builder, instr *Instruction) (t1 Type, ts []Type) { return instr.typ, nil }
	returnTypesFnI32                     = func(b *builder, instr *Instruction) (t1 Type, ts []Type) { return TypeI32, nil }
	returnTypesFnF32                     = func(b *builder, instr *Instruction) (t1 Type, ts []Type) { return TypeF32, nil }
	returnTypesFnF64                     = func(b *builder, instr *Instruction) (t1 Type, ts []Type) { return TypeF64, nil }
	returnTypesFnV128                    = func(b *builder, instr *Instruction) (t1 Type, ts []Type) { return TypeV128, nil }
)

// sideEffect provides the info to determine if an instruction has side effects which
// is used to determine if it can be optimized out, interchanged with others, etc.
type sideEffect byte

const (
	sideEffectUnknown sideEffect = iota
	// sideEffectStrict represents an instruction with side effects, and should be always alive plus cannot be reordered.
	sideEffectStrict
	// sideEffectTraps represents an instruction that can trap, and should be always alive but can be reordered within the group.
	sideEffectTraps
	// sideEffectNone represents an instruction without side effects, and can be eliminated if the result is not used, plus can be reordered within the group.
	sideEffectNone
)

// instructionSideEffects provides the info to determine if an instruction has side effects.
// Instructions with side effects must not be eliminated regardless whether the result is used or not.
var instructionSideEffects = [opcodeEnd]sideEffect{
	OpcodeUndefined:                   sideEffectStrict,
	OpcodeJump:                        sideEffectStrict,
	OpcodeIconst:                      sideEffectNone,
	OpcodeCall:                        sideEffectStrict,
	OpcodeCallIndirect:                sideEffectStrict,
	OpcodeIadd:                        sideEffectNone,
	OpcodeImul:                        sideEffectNone,
	OpcodeIsub:                        sideEffectNone,
	OpcodeIcmp:                        sideEffectNone,
	OpcodeExtractlane:                 sideEffectNone,
	OpcodeInsertlane:                  sideEffectNone,
	OpcodeBand:                        sideEffectNone,
	OpcodeBor:                         sideEffectNone,
	OpcodeBxor:                        sideEffectNone,
	OpcodeRotl:                        sideEffectNone,
	OpcodeRotr:                        sideEffectNone,
	OpcodeFcmp:                        sideEffectNone,
	OpcodeFadd:                        sideEffectNone,
	OpcodeClz:                         sideEffectNone,
	OpcodeCtz:                         sideEffectNone,
	OpcodePopcnt:                      sideEffectNone,
	OpcodeLoad:                        sideEffectNone,
	OpcodeLoadSplat:                   sideEffectNone,
	OpcodeUload8:                      sideEffectNone,
	OpcodeUload16:                     sideEffectNone,
	OpcodeUload32:                     sideEffectNone,
	OpcodeSload8:                      sideEffectNone,
	OpcodeSload16:                     sideEffectNone,
	OpcodeSload32:                     sideEffectNone,
	OpcodeSExtend:                     sideEffectNone,
	OpcodeUExtend:                     sideEffectNone,
	OpcodeSwidenLow:                   sideEffectNone,
	OpcodeUwidenLow:                   sideEffectNone,
	OpcodeSwidenHigh:                  sideEffectNone,
	OpcodeUwidenHigh:                  sideEffectNone,
	OpcodeSnarrow:                     sideEffectNone,
	OpcodeUnarrow:                     sideEffectNone,
	OpcodeSwizzle:                     sideEffectNone,
	OpcodeShuffle:                     sideEffectNone,
	OpcodeSplat:                       sideEffectNone,
	OpcodeFsub:                        sideEffectNone,
	OpcodeF32const:                    sideEffectNone,
	OpcodeF64const:                    sideEffectNone,
	OpcodeIshl:                        sideEffectNone,
	OpcodeSshr:                        sideEffectNone,
	OpcodeUshr:                        sideEffectNone,
	OpcodeStore:                       sideEffectStrict,
	OpcodeIstore8:                     sideEffectStrict,
	OpcodeIstore16:                    sideEffectStrict,
	OpcodeIstore32:                    sideEffectStrict,
	OpcodeExitWithCode:                sideEffectStrict,
	OpcodeExitIfTrueWithCode:          sideEffectStrict,
	OpcodeReturn:                      sideEffectStrict,
	OpcodeBrz:                         sideEffectStrict,
	OpcodeBrnz:                        sideEffectStrict,
	OpcodeBrTable:                     sideEffectStrict,
	OpcodeFdiv:                        sideEffectNone,
	OpcodeFmul:                        sideEffectNone,
	OpcodeFmax:                        sideEffectNone,
	OpcodeSqmulRoundSat:               sideEffectNone,
	OpcodeSelect:                      sideEffectNone,
	OpcodeFmin:                        sideEffectNone,
	OpcodeFneg:                        sideEffectNone,
	OpcodeFcvtToSint:                  sideEffectTraps,
	OpcodeFcvtToUint:                  sideEffectTraps,
	OpcodeFcvtFromSint:                sideEffectNone,
	OpcodeFcvtFromUint:                sideEffectNone,
	OpcodeFcvtToSintSat:               sideEffectNone,
	OpcodeFcvtToUintSat:               sideEffectNone,
	OpcodeVFcvtFromUint:               sideEffectNone,
	OpcodeVFcvtFromSint:               sideEffectNone,
	OpcodeFdemote:                     sideEffectNone,
	OpcodeFvpromoteLow:                sideEffectNone,
	OpcodeFvdemote:                    sideEffectNone,
	OpcodeFpromote:                    sideEffectNone,
	OpcodeBitcast:                     sideEffectNone,
	OpcodeIreduce:                     sideEffectNone,
	OpcodeSqrt:                        sideEffectNone,
	OpcodeCeil:                        sideEffectNone,
	OpcodeFloor:                       sideEffectNone,
	OpcodeTrunc:                       sideEffectNone,
	OpcodeNearest:                     sideEffectNone,
	OpcodeSdiv:                        sideEffectTraps,
	OpcodeSrem:                        sideEffectTraps,
	OpcodeUdiv:                        sideEffectTraps,
	OpcodeUrem:                        sideEffectTraps,
	OpcodeFabs:                        sideEffectNone,
	OpcodeFcopysign:                   sideEffectNone,
	OpcodeExtIaddPairwise:             sideEffectNone,
	OpcodeVconst:                      sideEffectNone,
	OpcodeVbor:                        sideEffectNone,
	OpcodeVbxor:                       sideEffectNone,
	OpcodeVband:                       sideEffectNone,
	OpcodeVbandnot:                    sideEffectNone,
	OpcodeVbnot:                       sideEffectNone,
	OpcodeVbitselect:                  sideEffectNone,
	OpcodeVanyTrue:                    sideEffectNone,
	OpcodeVallTrue:                    sideEffectNone,
	OpcodeVhighBits:                   sideEffectNone,
	OpcodeVIadd:                       sideEffectNone,
	OpcodeVSaddSat:                    sideEffectNone,
	OpcodeVUaddSat:                    sideEffectNone,
	OpcodeVIsub:                       sideEffectNone,
	OpcodeVSsubSat:                    sideEffectNone,
	OpcodeVUsubSat:                    sideEffectNone,
	OpcodeVIcmp:                       sideEffectNone,
	OpcodeVImin:                       sideEffectNone,
	OpcodeVUmin:                       sideEffectNone,
	OpcodeVImax:                       sideEffectNone,
	OpcodeVUmax:                       sideEffectNone,
	OpcodeVAvgRound:                   sideEffectNone,
	OpcodeVImul:                       sideEffectNone,
	OpcodeVIabs:                       sideEffectNone,
	OpcodeVIneg:                       sideEffectNone,
	OpcodeVIpopcnt:                    sideEffectNone,
	OpcodeVIshl:                       sideEffectNone,
	OpcodeVSshr:                       sideEffectNone,
	OpcodeVUshr:                       sideEffectNone,
	OpcodeVSqrt:                       sideEffectNone,
	OpcodeVFabs:                       sideEffectNone,
	OpcodeVFmin:                       sideEffectNone,
	OpcodeVFmax:                       sideEffectNone,
	OpcodeVFneg:                       sideEffectNone,
	OpcodeVFadd:                       sideEffectNone,
	OpcodeVFsub:                       sideEffectNone,
	OpcodeVFmul:                       sideEffectNone,
	OpcodeVFdiv:                       sideEffectNone,
	OpcodeVFcmp:                       sideEffectNone,
	OpcodeVCeil:                       sideEffectNone,
	OpcodeVFloor:                      sideEffectNone,
	OpcodeVTrunc:                      sideEffectNone,
	OpcodeVNearest:                    sideEffectNone,
	OpcodeVMaxPseudo:                  sideEffectNone,
	OpcodeVMinPseudo:                  sideEffectNone,
	OpcodeVFcvtToUintSat:              sideEffectNone,
	OpcodeVFcvtToSintSat:              sideEffectNone,
	OpcodeVZeroExtLoad:                sideEffectNone,
	OpcodeAtomicRmw:                   sideEffectStrict,
	OpcodeAtomicLoad:                  sideEffectStrict,
	OpcodeAtomicStore:                 sideEffectStrict,
	OpcodeAtomicCas:                   sideEffectStrict,
	OpcodeFence:                       sideEffectStrict,
	OpcodeWideningPairwiseDotProductS: sideEffectNone,
}

// sideEffect returns true if this instruction has side effects.
func (i *Instruction) sideEffect() sideEffect {
	if e := instructionSideEffects[i.opcode]; e == sideEffectUnknown {
		panic("BUG: side effect info not registered for " + i.opcode.String())
	} else {
		return e
	}
}

// instructionReturnTypes provides the function to determine the return types of an instruction.
var instructionReturnTypes = [opcodeEnd]returnTypesFn{
	OpcodeExtIaddPairwise: returnTypesFnV128,
	OpcodeVbor:            returnTypesFnV128,
	OpcodeVbxor:           returnTypesFnV128,
	OpcodeVband:           returnTypesFnV128,
	OpcodeVbnot:           returnTypesFnV128,
	OpcodeVbandnot:        returnTypesFnV128,
	OpcodeVbitselect:      returnTypesFnV128,
	OpcodeVanyTrue:        returnTypesFnI32,
	OpcodeVallTrue:        returnTypesFnI32,
	OpcodeVhighBits:       returnTypesFnI32,
	OpcodeVIadd:           returnTypesFnV128,
	OpcodeVSaddSat:        returnTypesFnV128,
	OpcodeVUaddSat:        returnTypesFnV128,
	OpcodeVIsub:           returnTypesFnV128,
	OpcodeVSsubSat:        returnTypesFnV128,
	OpcodeVUsubSat:        returnTypesFnV128,
	OpcodeVIcmp:           returnTypesFnV128,
	OpcodeVImin:           returnTypesFnV128,
	OpcodeVUmin:           returnTypesFnV128,
	OpcodeVImax:           returnTypesFnV128,
	OpcodeVUmax:           returnTypesFnV128,
	OpcodeVImul:           returnTypesFnV128,
	OpcodeVAvgRound:       returnTypesFnV128,
	OpcodeVIabs:           returnTypesFnV128,
	OpcodeVIneg:           returnTypesFnV128,
	OpcodeVIpopcnt:        returnTypesFnV128,
	OpcodeVIshl:           returnTypesFnV128,
	OpcodeVSshr:           returnTypesFnV128,
	OpcodeVUshr:           returnTypesFnV128,
	OpcodeExtractlane:     returnTypesFnSingle,
	OpcodeInsertlane:      returnTypesFnV128,
	OpcodeBand:            returnTypesFnSingle,
	OpcodeFcopysign:       returnTypesFnSingle,
	OpcodeBitcast:         returnTypesFnSingle,
	OpcodeBor:             returnTypesFnSingle,
	OpcodeBxor:            returnTypesFnSingle,
	OpcodeRotl:            returnTypesFnSingle,
	OpcodeRotr:            returnTypesFnSingle,
	OpcodeIshl:            returnTypesFnSingle,
	OpcodeSshr:            returnTypesFnSingle,
	OpcodeSdiv:            returnTypesFnSingle,
	OpcodeSrem:            returnTypesFnSingle,
	OpcodeUdiv:            returnTypesFnSingle,
	OpcodeUrem:            returnTypesFnSingle,
	OpcodeUshr:            returnTypesFnSingle,
	OpcodeJump:            returnTypesFnNoReturns,
	OpcodeUndefined:       returnTypesFnNoReturns,
	OpcodeIconst:          returnTypesFnSingle,
	OpcodeSelect:          returnTypesFnSingle,
	OpcodeSExtend:         returnTypesFnSingle,
	OpcodeUExtend:         returnTypesFnSingle,
	OpcodeSwidenLow:       returnTypesFnV128,
	OpcodeUwidenLow:       returnTypesFnV128,
	OpcodeSwidenHigh:      returnTypesFnV128,
	OpcodeUwidenHigh:      returnTypesFnV128,
	OpcodeSnarrow:         returnTypesFnV128,
	OpcodeUnarrow:         returnTypesFnV128,
	OpcodeSwizzle:         returnTypesFnSingle,
	OpcodeShuffle:         returnTypesFnV128,
	OpcodeSplat:           returnTypesFnV128,
	OpcodeIreduce:         returnTypesFnSingle,
	OpcodeFabs:            returnTypesFnSingle,
	OpcodeSqrt:            returnTypesFnSingle,
	OpcodeCeil:            returnTypesFnSingle,
	OpcodeFloor:           returnTypesFnSingle,
	OpcodeTrunc:           returnTypesFnSingle,
	OpcodeNearest:         returnTypesFnSingle,
	OpcodeCallIndirect: func(b *builder, instr *Instruction) (t1 Type, ts []Type) {
		sigID := SignatureID(instr.u1)
		sig, ok := b.signatures[sigID]
		if !ok {
			panic("BUG")
		}
		switch len(sig.Results) {
		case 0:
			t1 = typeInvalid
		case 1:
			t1 = sig.Results[0]
		default:
			t1, ts = sig.Results[0], sig.Results[1:]
		}
		return
	},
	OpcodeCall: func(b *builder, instr *Instruction) (t1 Type, ts []Type) {
		sigID := SignatureID(instr.u2)
		sig, ok := b.signatures[sigID]
		if !ok {
			panic("BUG")
		}
		switch len(sig.Results) {
		case 0:
			t1 = typeInvalid
		case 1:
			t1 = sig.Results[0]
		default:
			t1, ts = sig.Results[0], sig.Results[1:]
		}
		return
	},
	OpcodeLoad:                        returnTypesFnSingle,
	OpcodeVZeroExtLoad:                returnTypesFnV128,
	OpcodeLoadSplat:                   returnTypesFnV128,
	OpcodeIadd:                        returnTypesFnSingle,
	OpcodeIsub:                        returnTypesFnSingle,
	OpcodeImul:                        returnTypesFnSingle,
	OpcodeIcmp:                        returnTypesFnI32,
	OpcodeFcmp:                        returnTypesFnI32,
	OpcodeFadd:                        returnTypesFnSingle,
	OpcodeFsub:                        returnTypesFnSingle,
	OpcodeFdiv:                        returnTypesFnSingle,
	OpcodeFmul:                        returnTypesFnSingle,
	OpcodeFmax:                        returnTypesFnSingle,
	OpcodeFmin:                        returnTypesFnSingle,
	OpcodeSqmulRoundSat:               returnTypesFnV128,
	OpcodeF32const:                    returnTypesFnF32,
	OpcodeF64const:                    returnTypesFnF64,
	OpcodeClz:                         returnTypesFnSingle,
	OpcodeCtz:                         returnTypesFnSingle,
	OpcodePopcnt:                      returnTypesFnSingle,
	OpcodeStore:                       returnTypesFnNoReturns,
	OpcodeIstore8:                     returnTypesFnNoReturns,
	OpcodeIstore16:                    returnTypesFnNoReturns,
	OpcodeIstore32:                    returnTypesFnNoReturns,
	OpcodeExitWithCode:                returnTypesFnNoReturns,
	OpcodeExitIfTrueWithCode:          returnTypesFnNoReturns,
	OpcodeReturn:                      returnTypesFnNoReturns,
	OpcodeBrz:                         returnTypesFnNoReturns,
	OpcodeBrnz:                        returnTypesFnNoReturns,
	OpcodeBrTable:                     returnTypesFnNoReturns,
	OpcodeUload8:                      returnTypesFnSingle,
	OpcodeUload16:                     returnTypesFnSingle,
	OpcodeUload32:                     returnTypesFnSingle,
	OpcodeSload8:                      returnTypesFnSingle,
	OpcodeSload16:                     returnTypesFnSingle,
	OpcodeSload32:                     returnTypesFnSingle,
	OpcodeFcvtToSint:                  returnTypesFnSingle,
	OpcodeFcvtToUint:                  returnTypesFnSingle,
	OpcodeFcvtFromSint:                returnTypesFnSingle,
	OpcodeFcvtFromUint:                returnTypesFnSingle,
	OpcodeFcvtToSintSat:               returnTypesFnSingle,
	OpcodeFcvtToUintSat:               returnTypesFnSingle,
	OpcodeVFcvtFromUint:               returnTypesFnV128,
	OpcodeVFcvtFromSint:               returnTypesFnV128,
	OpcodeFneg:                        returnTypesFnSingle,
	OpcodeFdemote:                     returnTypesFnF32,
	OpcodeFvdemote:                    returnTypesFnV128,
	OpcodeFvpromoteLow:                returnTypesFnV128,
	OpcodeFpromote:                    returnTypesFnF64,
	OpcodeVconst:                      returnTypesFnV128,
	OpcodeVFabs:                       returnTypesFnV128,
	OpcodeVSqrt:                       returnTypesFnV128,
	OpcodeVFmax:                       returnTypesFnV128,
	OpcodeVFmin:                       returnTypesFnV128,
	OpcodeVFneg:                       returnTypesFnV128,
	OpcodeVFadd:                       returnTypesFnV128,
	OpcodeVFsub:                       returnTypesFnV128,
	OpcodeVFmul:                       returnTypesFnV128,
	OpcodeVFdiv:                       returnTypesFnV128,
	OpcodeVFcmp:                       returnTypesFnV128,
	OpcodeVCeil:                       returnTypesFnV128,
	OpcodeVFloor:                      returnTypesFnV128,
	OpcodeVTrunc:                      returnTypesFnV128,
	OpcodeVNearest:                    returnTypesFnV128,
	OpcodeVMaxPseudo:                  returnTypesFnV128,
	OpcodeVMinPseudo:                  returnTypesFnV128,
	OpcodeVFcvtToUintSat:              returnTypesFnV128,
	OpcodeVFcvtToSintSat:              returnTypesFnV128,
	OpcodeAtomicRmw:                   returnTypesFnSingle,
	OpcodeAtomicLoad:                  returnTypesFnSingle,
	OpcodeAtomicStore:                 returnTypesFnNoReturns,
	OpcodeAtomicCas:                   returnTypesFnSingle,
	OpcodeFence:                       returnTypesFnNoReturns,
	OpcodeWideningPairwiseDotProductS: returnTypesFnV128,
}

// AsLoad initializes this instruction as a store instruction with OpcodeLoad.
func (i *Instruction) AsLoad(ptr Value, offset uint32, typ Type) *Instruction {
	i.opcode = OpcodeLoad
	i.v = ptr
	i.u1 = uint64(offset)
	i.typ = typ
	return i
}

// AsExtLoad initializes this instruction as a store instruction with OpcodeLoad.
func (i *Instruction) AsExtLoad(op Opcode, ptr Value, offset uint32, dst64bit bool) *Instruction {
	i.opcode = op
	i.v = ptr
	i.u1 = uint64(offset)
	if dst64bit {
		i.typ = TypeI64
	} else {
		i.typ = TypeI32
	}
	return i
}

// AsVZeroExtLoad initializes this instruction as a store instruction with OpcodeVExtLoad.
func (i *Instruction) AsVZeroExtLoad(ptr Value, offset uint32, scalarType Type) *Instruction {
	i.opcode = OpcodeVZeroExtLoad
	i.v = ptr
	i.u1 = uint64(offset)
	i.u2 = uint64(scalarType)
	i.typ = TypeV128
	return i
}

// VZeroExtLoadData returns the operands for a load instruction. The returned `typ` is the scalar type of the load target.
func (i *Instruction) VZeroExtLoadData() (ptr Value, offset uint32, typ Type) {
	return i.v, uint32(i.u1), Type(i.u2)
}

// AsLoadSplat initializes this instruction as a store instruction with OpcodeLoadSplat.
func (i *Instruction) AsLoadSplat(ptr Value, offset uint32, lane VecLane) *Instruction {
	i.opcode = OpcodeLoadSplat
	i.v = ptr
	i.u1 = uint64(offset)
	i.u2 = uint64(lane)
	i.typ = TypeV128
	return i
}

// LoadData returns the operands for a load instruction.
func (i *Instruction) LoadData() (ptr Value, offset uint32, typ Type) {
	return i.v, uint32(i.u1), i.typ
}

// LoadSplatData returns the operands for a load splat instruction.
func (i *Instruction) LoadSplatData() (ptr Value, offset uint32, lane VecLane) {
	return i.v, uint32(i.u1), VecLane(i.u2)
}

// AsStore initializes this instruction as a store instruction with OpcodeStore.
func (i *Instruction) AsStore(storeOp Opcode, value, ptr Value, offset uint32) *Instruction {
	i.opcode = storeOp
	i.v = value
	i.v2 = ptr

	var dstSize uint64
	switch storeOp {
	case OpcodeStore:
		dstSize = uint64(value.Type().Bits())
	case OpcodeIstore8:
		dstSize = 8
	case OpcodeIstore16:
		dstSize = 16
	case OpcodeIstore32:
		dstSize = 32
	default:
		panic("invalid store opcode" + storeOp.String())
	}
	i.u1 = uint64(offset) | dstSize<<32
	return i
}

// StoreData returns the operands for a store instruction.
func (i *Instruction) StoreData() (value, ptr Value, offset uint32, storeSizeInBits byte) {
	return i.v, i.v2, uint32(i.u1), byte(i.u1 >> 32)
}

// AsIconst64 initializes this instruction as a 64-bit integer constant instruction with OpcodeIconst.
func (i *Instruction) AsIconst64(v uint64) *Instruction {
	i.opcode = OpcodeIconst
	i.typ = TypeI64
	i.u1 = v
	return i
}

// AsIconst32 initializes this instruction as a 32-bit integer constant instruction with OpcodeIconst.
func (i *Instruction) AsIconst32(v uint32) *Instruction {
	i.opcode = OpcodeIconst
	i.typ = TypeI32
	i.u1 = uint64(v)
	return i
}

// AsIadd initializes this instruction as an integer addition instruction with OpcodeIadd.
func (i *Instruction) AsIadd(x, y Value) *Instruction {
	i.opcode = OpcodeIadd
	i.v = x
	i.v2 = y
	i.typ = x.Type()
	return i
}

// AsVIadd initializes this instruction as an integer addition instruction with OpcodeVIadd on a vector.
func (i *Instruction) AsVIadd(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVIadd
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsWideningPairwiseDotProductS initializes this instruction as a lane-wise integer extended pairwise addition instruction
// with OpcodeIaddPairwise on a vector.
func (i *Instruction) AsWideningPairwiseDotProductS(x, y Value) *Instruction {
	i.opcode = OpcodeWideningPairwiseDotProductS
	i.v = x
	i.v2 = y
	i.typ = TypeV128
	return i
}

// AsExtIaddPairwise initializes this instruction as a lane-wise integer extended pairwise addition instruction
// with OpcodeIaddPairwise on a vector.
func (i *Instruction) AsExtIaddPairwise(x Value, srcLane VecLane, signed bool) *Instruction {
	i.opcode = OpcodeExtIaddPairwise
	i.v = x
	i.u1 = uint64(srcLane)
	if signed {
		i.u2 = 1
	}
	i.typ = TypeV128
	return i
}

// ExtIaddPairwiseData returns the operands for a lane-wise integer extended pairwise addition instruction.
func (i *Instruction) ExtIaddPairwiseData() (x Value, srcLane VecLane, signed bool) {
	return i.v, VecLane(i.u1), i.u2 != 0
}

// AsVSaddSat initializes this instruction as a vector addition with saturation instruction with OpcodeVSaddSat on a vector.
func (i *Instruction) AsVSaddSat(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVSaddSat
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVUaddSat initializes this instruction as a vector addition with saturation instruction with OpcodeVUaddSat on a vector.
func (i *Instruction) AsVUaddSat(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVUaddSat
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVIsub initializes this instruction as an integer subtraction instruction with OpcodeVIsub on a vector.
func (i *Instruction) AsVIsub(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVIsub
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVSsubSat initializes this instruction as a vector addition with saturation instruction with OpcodeVSsubSat on a vector.
func (i *Instruction) AsVSsubSat(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVSsubSat
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVUsubSat initializes this instruction as a vector addition with saturation instruction with OpcodeVUsubSat on a vector.
func (i *Instruction) AsVUsubSat(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVUsubSat
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVImin initializes this instruction as a signed integer min instruction with OpcodeVImin on a vector.
func (i *Instruction) AsVImin(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVImin
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVUmin initializes this instruction as an unsigned integer min instruction with OpcodeVUmin on a vector.
func (i *Instruction) AsVUmin(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVUmin
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVImax initializes this instruction as a signed integer max instruction with OpcodeVImax on a vector.
func (i *Instruction) AsVImax(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVImax
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVUmax initializes this instruction as an unsigned integer max instruction with OpcodeVUmax on a vector.
func (i *Instruction) AsVUmax(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVUmax
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVAvgRound initializes this instruction as an unsigned integer avg instruction, truncating to zero with OpcodeVAvgRound on a vector.
func (i *Instruction) AsVAvgRound(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVAvgRound
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVImul initializes this instruction as an integer multiplication with OpcodeVImul on a vector.
func (i *Instruction) AsVImul(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVImul
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsSqmulRoundSat initializes this instruction as a lane-wise saturating rounding multiplication
// in Q15 format with OpcodeSqmulRoundSat on a vector.
func (i *Instruction) AsSqmulRoundSat(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeSqmulRoundSat
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVIabs initializes this instruction as a vector absolute value with OpcodeVIabs.
func (i *Instruction) AsVIabs(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVIabs
	i.v = x
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVIneg initializes this instruction as a vector negation with OpcodeVIneg.
func (i *Instruction) AsVIneg(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVIneg
	i.v = x
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVIpopcnt initializes this instruction as a Population Count instruction with OpcodeVIpopcnt on a vector.
func (i *Instruction) AsVIpopcnt(x Value, lane VecLane) *Instruction {
	if lane != VecLaneI8x16 {
		panic("Unsupported lane type " + lane.String())
	}
	i.opcode = OpcodeVIpopcnt
	i.v = x
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVSqrt initializes this instruction as a sqrt instruction with OpcodeVSqrt on a vector.
func (i *Instruction) AsVSqrt(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVSqrt
	i.v = x
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFabs initializes this instruction as a float abs instruction with OpcodeVFabs on a vector.
func (i *Instruction) AsVFabs(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFabs
	i.v = x
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFneg initializes this instruction as a float neg instruction with OpcodeVFneg on a vector.
func (i *Instruction) AsVFneg(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFneg
	i.v = x
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFmax initializes this instruction as a float max instruction with OpcodeVFmax on a vector.
func (i *Instruction) AsVFmax(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFmax
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFmin initializes this instruction as a float min instruction with OpcodeVFmin on a vector.
func (i *Instruction) AsVFmin(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFmin
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFadd initializes this instruction as a floating point add instruction with OpcodeVFadd on a vector.
func (i *Instruction) AsVFadd(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFadd
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFsub initializes this instruction as a floating point subtraction instruction with OpcodeVFsub on a vector.
func (i *Instruction) AsVFsub(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFsub
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFmul initializes this instruction as a floating point multiplication instruction with OpcodeVFmul on a vector.
func (i *Instruction) AsVFmul(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFmul
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFdiv initializes this instruction as a floating point division instruction with OpcodeVFdiv on a vector.
func (i *Instruction) AsVFdiv(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFdiv
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsImul initializes this instruction as an integer addition instruction with OpcodeImul.
func (i *Instruction) AsImul(x, y Value) *Instruction {
	i.opcode = OpcodeImul
	i.v = x
	i.v2 = y
	i.typ = x.Type()
	return i
}

func (i *Instruction) Insert(b Builder) *Instruction {
	b.InsertInstruction(i)
	return i
}

// AsIsub initializes this instruction as an integer subtraction instruction with OpcodeIsub.
func (i *Instruction) AsIsub(x, y Value) *Instruction {
	i.opcode = OpcodeIsub
	i.v = x
	i.v2 = y
	i.typ = x.Type()
	return i
}

// AsIcmp initializes this instruction as an integer comparison instruction with OpcodeIcmp.
func (i *Instruction) AsIcmp(x, y Value, c IntegerCmpCond) *Instruction {
	i.opcode = OpcodeIcmp
	i.v = x
	i.v2 = y
	i.u1 = uint64(c)
	i.typ = TypeI32
	return i
}

// AsFcmp initializes this instruction as an integer comparison instruction with OpcodeFcmp.
func (i *Instruction) AsFcmp(x, y Value, c FloatCmpCond) {
	i.opcode = OpcodeFcmp
	i.v = x
	i.v2 = y
	i.u1 = uint64(c)
	i.typ = TypeI32
}

// AsVIcmp initializes this instruction as an integer vector comparison instruction with OpcodeVIcmp.
func (i *Instruction) AsVIcmp(x, y Value, c IntegerCmpCond, lane VecLane) *Instruction {
	i.opcode = OpcodeVIcmp
	i.v = x
	i.v2 = y
	i.u1 = uint64(c)
	i.u2 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsVFcmp initializes this instruction as a float comparison instruction with OpcodeVFcmp on Vector.
func (i *Instruction) AsVFcmp(x, y Value, c FloatCmpCond, lane VecLane) *Instruction {
	i.opcode = OpcodeVFcmp
	i.v = x
	i.v2 = y
	i.u1 = uint64(c)
	i.typ = TypeV128
	i.u2 = uint64(lane)
	return i
}

// AsVCeil initializes this instruction as an instruction with OpcodeCeil.
func (i *Instruction) AsVCeil(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVCeil
	i.v = x
	i.typ = x.Type()
	i.u1 = uint64(lane)
	return i
}

// AsVFloor initializes this instruction as an instruction with OpcodeFloor.
func (i *Instruction) AsVFloor(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVFloor
	i.v = x
	i.typ = x.Type()
	i.u1 = uint64(lane)
	return i
}

// AsVTrunc initializes this instruction as an instruction with OpcodeTrunc.
func (i *Instruction) AsVTrunc(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVTrunc
	i.v = x
	i.typ = x.Type()
	i.u1 = uint64(lane)
	return i
}

// AsVNearest initializes this instruction as an instruction with OpcodeNearest.
func (i *Instruction) AsVNearest(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVNearest
	i.v = x
	i.typ = x.Type()
	i.u1 = uint64(lane)
	return i
}

// AsVMaxPseudo initializes this instruction as an instruction with OpcodeVMaxPseudo.
func (i *Instruction) AsVMaxPseudo(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVMaxPseudo
	i.typ = x.Type()
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	return i
}

// AsVMinPseudo initializes this instruction as an instruction with OpcodeVMinPseudo.
func (i *Instruction) AsVMinPseudo(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVMinPseudo
	i.typ = x.Type()
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	return i
}

// AsSDiv initializes this instruction as an integer bitwise and instruction with OpcodeSdiv.
func (i *Instruction) AsSDiv(x, y, ctx Value) *Instruction {
	i.opcode = OpcodeSdiv
	i.v = x
	i.v2 = y
	i.v3 = ctx
	i.typ = x.Type()
	return i
}

// AsUDiv initializes this instruction as an integer bitwise and instruction with OpcodeUdiv.
func (i *Instruction) AsUDiv(x, y, ctx Value) *Instruction {
	i.opcode = OpcodeUdiv
	i.v = x
	i.v2 = y
	i.v3 = ctx
	i.typ = x.Type()
	return i
}

// AsSRem initializes this instruction as an integer bitwise and instruction with OpcodeSrem.
func (i *Instruction) AsSRem(x, y, ctx Value) *Instruction {
	i.opcode = OpcodeSrem
	i.v = x
	i.v2 = y
	i.v3 = ctx
	i.typ = x.Type()
	return i
}

// AsURem initializes this instruction as an integer bitwise and instruction with OpcodeUrem.
func (i *Instruction) AsURem(x, y, ctx Value) *Instruction {
	i.opcode = OpcodeUrem
	i.v = x
	i.v2 = y
	i.v3 = ctx
	i.typ = x.Type()
	return i
}

// AsBand initializes this instruction as an integer bitwise and instruction with OpcodeBand.
func (i *Instruction) AsBand(x, amount Value) *Instruction {
	i.opcode = OpcodeBand
	i.v = x
	i.v2 = amount
	i.typ = x.Type()
	return i
}

// AsBor initializes this instruction as an integer bitwise or instruction with OpcodeBor.
func (i *Instruction) AsBor(x, amount Value) {
	i.opcode = OpcodeBor
	i.v = x
	i.v2 = amount
	i.typ = x.Type()
}

// AsBxor initializes this instruction as an integer bitwise xor instruction with OpcodeBxor.
func (i *Instruction) AsBxor(x, amount Value) {
	i.opcode = OpcodeBxor
	i.v = x
	i.v2 = amount
	i.typ = x.Type()
}

// AsIshl initializes this instruction as an integer shift left instruction with OpcodeIshl.
func (i *Instruction) AsIshl(x, amount Value) *Instruction {
	i.opcode = OpcodeIshl
	i.v = x
	i.v2 = amount
	i.typ = x.Type()
	return i
}

// AsVIshl initializes this instruction as an integer shift left instruction with OpcodeVIshl on vector.
func (i *Instruction) AsVIshl(x, amount Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVIshl
	i.v = x
	i.v2 = amount
	i.u1 = uint64(lane)
	i.typ = x.Type()
	return i
}

// AsUshr initializes this instruction as an integer unsigned shift right (logical shift right) instruction with OpcodeUshr.
func (i *Instruction) AsUshr(x, amount Value) *Instruction {
	i.opcode = OpcodeUshr
	i.v = x
	i.v2 = amount
	i.typ = x.Type()
	return i
}

// AsVUshr initializes this instruction as an integer unsigned shift right (logical shift right) instruction with OpcodeVUshr on vector.
func (i *Instruction) AsVUshr(x, amount Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVUshr
	i.v = x
	i.v2 = amount
	i.u1 = uint64(lane)
	i.typ = x.Type()
	return i
}

// AsSshr initializes this instruction as an integer signed shift right (arithmetic shift right) instruction with OpcodeSshr.
func (i *Instruction) AsSshr(x, amount Value) *Instruction {
	i.opcode = OpcodeSshr
	i.v = x
	i.v2 = amount
	i.typ = x.Type()
	return i
}

// AsVSshr initializes this instruction as an integer signed shift right (arithmetic shift right) instruction with OpcodeVSshr on vector.
func (i *Instruction) AsVSshr(x, amount Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVSshr
	i.v = x
	i.v2 = amount
	i.u1 = uint64(lane)
	i.typ = x.Type()
	return i
}

// AsExtractlane initializes this instruction as an extract lane instruction with OpcodeExtractlane on vector.
func (i *Instruction) AsExtractlane(x Value, index byte, lane VecLane, signed bool) *Instruction {
	i.opcode = OpcodeExtractlane
	i.v = x
	// We do not have a field for signedness, but `index` is a byte,
	// so we just encode the flag in the high bits of `u1`.
	i.u1 = uint64(index)
	if signed {
		i.u1 = i.u1 | 1<<32
	}
	i.u2 = uint64(lane)
	switch lane {
	case VecLaneI8x16, VecLaneI16x8, VecLaneI32x4:
		i.typ = TypeI32
	case VecLaneI64x2:
		i.typ = TypeI64
	case VecLaneF32x4:
		i.typ = TypeF32
	case VecLaneF64x2:
		i.typ = TypeF64
	}
	return i
}

// AsInsertlane initializes this instruction as an insert lane instruction with OpcodeInsertlane on vector.
func (i *Instruction) AsInsertlane(x, y Value, index byte, lane VecLane) *Instruction {
	i.opcode = OpcodeInsertlane
	i.v = x
	i.v2 = y
	i.u1 = uint64(index)
	i.u2 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsShuffle initializes this instruction as a shuffle instruction with OpcodeShuffle on vector.
func (i *Instruction) AsShuffle(x, y Value, lane []byte) *Instruction {
	i.opcode = OpcodeShuffle
	i.v = x
	i.v2 = y
	// Encode the 16 bytes as 8 bytes in u1, and 8 bytes in u2.
	i.u1 = uint64(lane[7])<<56 | uint64(lane[6])<<48 | uint64(lane[5])<<40 | uint64(lane[4])<<32 | uint64(lane[3])<<24 | uint64(lane[2])<<16 | uint64(lane[1])<<8 | uint64(lane[0])
	i.u2 = uint64(lane[15])<<56 | uint64(lane[14])<<48 | uint64(lane[13])<<40 | uint64(lane[12])<<32 | uint64(lane[11])<<24 | uint64(lane[10])<<16 | uint64(lane[9])<<8 | uint64(lane[8])
	i.typ = TypeV128
	return i
}

// AsSwizzle initializes this instruction as an insert lane instruction with OpcodeSwizzle on vector.
func (i *Instruction) AsSwizzle(x, y Value, lane VecLane) *Instruction {
	i.opcode = OpcodeSwizzle
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsSplat initializes this instruction as an insert lane instruction with OpcodeSplat on vector.
func (i *Instruction) AsSplat(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeSplat
	i.v = x
	i.u1 = uint64(lane)
	i.typ = TypeV128
	return i
}

// AsRotl initializes this instruction as a word rotate left instruction with OpcodeRotl.
func (i *Instruction) AsRotl(x, amount Value) {
	i.opcode = OpcodeRotl
	i.v = x
	i.v2 = amount
	i.typ = x.Type()
}

// AsRotr initializes this instruction as a word rotate right instruction with OpcodeRotr.
func (i *Instruction) AsRotr(x, amount Value) {
	i.opcode = OpcodeRotr
	i.v = x
	i.v2 = amount
	i.typ = x.Type()
}

// IcmpData returns the operands and comparison condition of this integer comparison instruction.
func (i *Instruction) IcmpData() (x, y Value, c IntegerCmpCond) {
	return i.v, i.v2, IntegerCmpCond(i.u1)
}

// FcmpData returns the operands and comparison condition of this floating-point comparison instruction.
func (i *Instruction) FcmpData() (x, y Value, c FloatCmpCond) {
	return i.v, i.v2, FloatCmpCond(i.u1)
}

// VIcmpData returns the operands and comparison condition of this integer comparison instruction on vector.
func (i *Instruction) VIcmpData() (x, y Value, c IntegerCmpCond, l VecLane) {
	return i.v, i.v2, IntegerCmpCond(i.u1), VecLane(i.u2)
}

// VFcmpData returns the operands and comparison condition of this float comparison instruction on vector.
func (i *Instruction) VFcmpData() (x, y Value, c FloatCmpCond, l VecLane) {
	return i.v, i.v2, FloatCmpCond(i.u1), VecLane(i.u2)
}

// ExtractlaneData returns the operands and sign flag of Extractlane on vector.
func (i *Instruction) ExtractlaneData() (x Value, index byte, signed bool, l VecLane) {
	x = i.v
	index = byte(0b00001111 & i.u1)
	signed = i.u1>>32 != 0
	l = VecLane(i.u2)
	return
}

// InsertlaneData returns the operands and sign flag of Insertlane on vector.
func (i *Instruction) InsertlaneData() (x, y Value, index byte, l VecLane) {
	x = i.v
	y = i.v2
	index = byte(i.u1)
	l = VecLane(i.u2)
	return
}

// AsFadd initializes this instruction as a floating-point addition instruction with OpcodeFadd.
func (i *Instruction) AsFadd(x, y Value) {
	i.opcode = OpcodeFadd
	i.v = x
	i.v2 = y
	i.typ = x.Type()
}

// AsFsub initializes this instruction as a floating-point subtraction instruction with OpcodeFsub.
func (i *Instruction) AsFsub(x, y Value) {
	i.opcode = OpcodeFsub
	i.v = x
	i.v2 = y
	i.typ = x.Type()
}

// AsFmul initializes this instruction as a floating-point multiplication instruction with OpcodeFmul.
func (i *Instruction) AsFmul(x, y Value) {
	i.opcode = OpcodeFmul
	i.v = x
	i.v2 = y
	i.typ = x.Type()
}

// AsFdiv initializes this instruction as a floating-point division instruction with OpcodeFdiv.
func (i *Instruction) AsFdiv(x, y Value) {
	i.opcode = OpcodeFdiv
	i.v = x
	i.v2 = y
	i.typ = x.Type()
}

// AsFmin initializes this instruction to take the minimum of two floating-points with OpcodeFmin.
func (i *Instruction) AsFmin(x, y Value) {
	i.opcode = OpcodeFmin
	i.v = x
	i.v2 = y
	i.typ = x.Type()
}

// AsFmax initializes this instruction to take the maximum of two floating-points with OpcodeFmax.
func (i *Instruction) AsFmax(x, y Value) {
	i.opcode = OpcodeFmax
	i.v = x
	i.v2 = y
	i.typ = x.Type()
}

// AsF32const initializes this instruction as a 32-bit floating-point constant instruction with OpcodeF32const.
func (i *Instruction) AsF32const(f float32) *Instruction {
	i.opcode = OpcodeF32const
	i.typ = TypeF64
	i.u1 = uint64(math.Float32bits(f))
	return i
}

// AsF64const initializes this instruction as a 64-bit floating-point constant instruction with OpcodeF64const.
func (i *Instruction) AsF64const(f float64) *Instruction {
	i.opcode = OpcodeF64const
	i.typ = TypeF64
	i.u1 = math.Float64bits(f)
	return i
}

// AsVconst initializes this instruction as a vector constant instruction with OpcodeVconst.
func (i *Instruction) AsVconst(lo, hi uint64) *Instruction {
	i.opcode = OpcodeVconst
	i.typ = TypeV128
	i.u1 = lo
	i.u2 = hi
	return i
}

// AsVbnot initializes this instruction as a vector negation instruction with OpcodeVbnot.
func (i *Instruction) AsVbnot(v Value) *Instruction {
	i.opcode = OpcodeVbnot
	i.typ = TypeV128
	i.v = v
	return i
}

// AsVband initializes this instruction as an and vector instruction with OpcodeVband.
func (i *Instruction) AsVband(x, y Value) *Instruction {
	i.opcode = OpcodeVband
	i.typ = TypeV128
	i.v = x
	i.v2 = y
	return i
}

// AsVbor initializes this instruction as an or vector instruction with OpcodeVbor.
func (i *Instruction) AsVbor(x, y Value) *Instruction {
	i.opcode = OpcodeVbor
	i.typ = TypeV128
	i.v = x
	i.v2 = y
	return i
}

// AsVbxor initializes this instruction as a xor vector instruction with OpcodeVbxor.
func (i *Instruction) AsVbxor(x, y Value) *Instruction {
	i.opcode = OpcodeVbxor
	i.typ = TypeV128
	i.v = x
	i.v2 = y
	return i
}

// AsVbandnot initializes this instruction as an and-not vector instruction with OpcodeVbandnot.
func (i *Instruction) AsVbandnot(x, y Value) *Instruction {
	i.opcode = OpcodeVbandnot
	i.typ = TypeV128
	i.v = x
	i.v2 = y
	return i
}

// AsVbitselect initializes this instruction as a bit select vector instruction with OpcodeVbitselect.
func (i *Instruction) AsVbitselect(c, x, y Value) *Instruction {
	i.opcode = OpcodeVbitselect
	i.typ = TypeV128
	i.v = c
	i.v2 = x
	i.v3 = y
	return i
}

// AsVanyTrue initializes this instruction as an anyTrue vector instruction with OpcodeVanyTrue.
func (i *Instruction) AsVanyTrue(x Value) *Instruction {
	i.opcode = OpcodeVanyTrue
	i.typ = TypeI32
	i.v = x
	return i
}

// AsVallTrue initializes this instruction as an allTrue vector instruction with OpcodeVallTrue.
func (i *Instruction) AsVallTrue(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVallTrue
	i.typ = TypeI32
	i.v = x
	i.u1 = uint64(lane)
	return i
}

// AsVhighBits initializes this instruction as a highBits vector instruction with OpcodeVhighBits.
func (i *Instruction) AsVhighBits(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeVhighBits
	i.typ = TypeI32
	i.v = x
	i.u1 = uint64(lane)
	return i
}

// VconstData returns the operands of this vector constant instruction.
func (i *Instruction) VconstData() (lo, hi uint64) {
	return i.u1, i.u2
}

// AsReturn initializes this instruction as a return instruction with OpcodeReturn.
func (i *Instruction) AsReturn(vs wazevoapi.VarLength[Value]) *Instruction {
	i.opcode = OpcodeReturn
	i.vs = vs
	return i
}

// AsIreduce initializes this instruction as a reduction instruction with OpcodeIreduce.
func (i *Instruction) AsIreduce(v Value, dstType Type) *Instruction {
	i.opcode = OpcodeIreduce
	i.v = v
	i.typ = dstType
	return i
}

// AsWiden initializes this instruction as a signed or unsigned widen instruction
// on low half or high half of the given vector with OpcodeSwidenLow, OpcodeUwidenLow, OpcodeSwidenHigh, OpcodeUwidenHigh.
func (i *Instruction) AsWiden(v Value, lane VecLane, signed, low bool) *Instruction {
	switch {
	case signed && low:
		i.opcode = OpcodeSwidenLow
	case !signed && low:
		i.opcode = OpcodeUwidenLow
	case signed && !low:
		i.opcode = OpcodeSwidenHigh
	case !signed && !low:
		i.opcode = OpcodeUwidenHigh
	}
	i.v = v
	i.u1 = uint64(lane)
	return i
}

// AsAtomicLoad initializes this instruction as an atomic load.
// The size is in bytes and must be 1, 2, 4, or 8.
func (i *Instruction) AsAtomicLoad(addr Value, size uint64, typ Type) *Instruction {
	i.opcode = OpcodeAtomicLoad
	i.u1 = size
	i.v = addr
	i.typ = typ
	return i
}

// AsAtomicLoad initializes this instruction as an atomic store.
// The size is in bytes and must be 1, 2, 4, or 8.
func (i *Instruction) AsAtomicStore(addr, val Value, size uint64) *Instruction {
	i.opcode = OpcodeAtomicStore
	i.u1 = size
	i.v = addr
	i.v2 = val
	i.typ = val.Type()
	return i
}

// AsAtomicRmw initializes this instruction as an atomic read-modify-write.
// The size is in bytes and must be 1, 2, 4, or 8.
func (i *Instruction) AsAtomicRmw(op AtomicRmwOp, addr, val Value, size uint64) *Instruction {
	i.opcode = OpcodeAtomicRmw
	i.u1 = uint64(op)
	i.u2 = size
	i.v = addr
	i.v2 = val
	i.typ = val.Type()
	return i
}

// AsAtomicCas initializes this instruction as an atomic compare-and-swap.
// The size is in bytes and must be 1, 2, 4, or 8.
func (i *Instruction) AsAtomicCas(addr, exp, repl Value, size uint64) *Instruction {
	i.opcode = OpcodeAtomicCas
	i.u1 = size
	i.v = addr
	i.v2 = exp
	i.v3 = repl
	i.typ = repl.Type()
	return i
}

// AsFence initializes this instruction as a memory fence.
// A single byte immediate may be used to indicate fence ordering in the future
// but is currently always 0 and ignored.
func (i *Instruction) AsFence(order byte) *Instruction {
	i.opcode = OpcodeFence
	i.u1 = uint64(order)
	return i
}

// AtomicRmwData returns the data for this atomic read-modify-write instruction.
func (i *Instruction) AtomicRmwData() (op AtomicRmwOp, size uint64) {
	return AtomicRmwOp(i.u1), i.u2
}

// AtomicTargetSize returns the target memory size of the atomic instruction.
func (i *Instruction) AtomicTargetSize() (size uint64) {
	return i.u1
}

// ReturnVals returns the return values of OpcodeReturn.
func (i *Instruction) ReturnVals() []Value {
	return i.vs.View()
}

// AsExitWithCode initializes this instruction as a trap instruction with OpcodeExitWithCode.
func (i *Instruction) AsExitWithCode(ctx Value, code wazevoapi.ExitCode) {
	i.opcode = OpcodeExitWithCode
	i.v = ctx
	i.u1 = uint64(code)
}

// AsExitIfTrueWithCode initializes this instruction as a trap instruction with OpcodeExitIfTrueWithCode.
func (i *Instruction) AsExitIfTrueWithCode(ctx, c Value, code wazevoapi.ExitCode) *Instruction {
	i.opcode = OpcodeExitIfTrueWithCode
	i.v = ctx
	i.v2 = c
	i.u1 = uint64(code)
	return i
}

// ExitWithCodeData returns the context and exit code of OpcodeExitWithCode.
func (i *Instruction) ExitWithCodeData() (ctx Value, code wazevoapi.ExitCode) {
	return i.v, wazevoapi.ExitCode(i.u1)
}

// ExitIfTrueWithCodeData returns the context and exit code of OpcodeExitWithCode.
func (i *Instruction) ExitIfTrueWithCodeData() (ctx, c Value, code wazevoapi.ExitCode) {
	return i.v, i.v2, wazevoapi.ExitCode(i.u1)
}

// InvertBrx inverts either OpcodeBrz or OpcodeBrnz to the other.
func (i *Instruction) InvertBrx() {
	switch i.opcode {
	case OpcodeBrz:
		i.opcode = OpcodeBrnz
	case OpcodeBrnz:
		i.opcode = OpcodeBrz
	default:
		panic("BUG")
	}
}

// BranchData returns the branch data for this instruction necessary for backends.
func (i *Instruction) BranchData() (condVal Value, blockArgs []Value, target BasicBlockID) {
	switch i.opcode {
	case OpcodeJump:
		condVal = ValueInvalid
	case OpcodeBrz, OpcodeBrnz:
		condVal = i.v
	default:
		panic("BUG")
	}
	blockArgs = i.vs.View()
	target = BasicBlockID(i.rValue)
	return
}

// BrTableData returns the branch table data for this instruction necessary for backends.
func (i *Instruction) BrTableData() (index Value, targets Values) {
	if i.opcode != OpcodeBrTable {
		panic("BUG: BrTableData only available for OpcodeBrTable")
	}
	index = i.v
	targets = i.rValues
	return
}

// AsJump initializes this instruction as a jump instruction with OpcodeJump.
func (i *Instruction) AsJump(vs Values, target BasicBlock) *Instruction {
	i.opcode = OpcodeJump
	i.vs = vs
	i.rValue = Value(target.ID())
	return i
}

// IsFallthroughJump returns true if this instruction is a fallthrough jump.
func (i *Instruction) IsFallthroughJump() bool {
	if i.opcode != OpcodeJump {
		panic("BUG: IsFallthrough only available for OpcodeJump")
	}
	return i.opcode == OpcodeJump && i.u1 != 0
}

// AsFallthroughJump marks this instruction as a fallthrough jump.
func (i *Instruction) AsFallthroughJump() {
	if i.opcode != OpcodeJump {
		panic("BUG: AsFallthroughJump only available for OpcodeJump")
	}
	i.u1 = 1
}

// AsBrz initializes this instruction as a branch-if-zero instruction with OpcodeBrz.
func (i *Instruction) AsBrz(v Value, args Values, target BasicBlock) {
	i.opcode = OpcodeBrz
	i.v = v
	i.vs = args
	i.rValue = Value(target.ID())
}

// AsBrnz initializes this instruction as a branch-if-not-zero instruction with OpcodeBrnz.
func (i *Instruction) AsBrnz(v Value, args Values, target BasicBlock) *Instruction {
	i.opcode = OpcodeBrnz
	i.v = v
	i.vs = args
	i.rValue = Value(target.ID())
	return i
}

// AsBrTable initializes this instruction as a branch-table instruction with OpcodeBrTable.
// targets is a list of basic block IDs cast to Values.
func (i *Instruction) AsBrTable(index Value, targets Values) {
	i.opcode = OpcodeBrTable
	i.v = index
	i.rValues = targets
}

// AsCall initializes this instruction as a call instruction with OpcodeCall.
func (i *Instruction) AsCall(ref FuncRef, sig *Signature, args Values) {
	i.opcode = OpcodeCall
	i.u1 = uint64(ref)
	i.vs = args
	i.u2 = uint64(sig.ID)
	sig.used = true
}

// CallData returns the call data for this instruction necessary for backends.
func (i *Instruction) CallData() (ref FuncRef, sigID SignatureID, args []Value) {
	if i.opcode != OpcodeCall {
		panic("BUG: CallData only available for OpcodeCall")
	}
	ref = FuncRef(i.u1)
	sigID = SignatureID(i.u2)
	args = i.vs.View()
	return
}

// AsCallIndirect initializes this instruction as a call-indirect instruction with OpcodeCallIndirect.
func (i *Instruction) AsCallIndirect(funcPtr Value, sig *Signature, args Values) *Instruction {
	i.opcode = OpcodeCallIndirect
	i.typ = TypeF64
	i.vs = args
	i.v = funcPtr
	i.u1 = uint64(sig.ID)
	sig.used = true
	return i
}

// AsCallGoRuntimeMemmove is the same as AsCallIndirect, but with a special flag set to indicate that it is a call to the Go runtime memmove function.
func (i *Instruction) AsCallGoRuntimeMemmove(funcPtr Value, sig *Signature, args Values) *Instruction {
	i.AsCallIndirect(funcPtr, sig, args)
	i.u2 = 1
	return i
}

// CallIndirectData returns the call indirect data for this instruction necessary for backends.
func (i *Instruction) CallIndirectData() (funcPtr Value, sigID SignatureID, args []Value, isGoMemmove bool) {
	if i.opcode != OpcodeCallIndirect {
		panic("BUG: CallIndirectData only available for OpcodeCallIndirect")
	}
	funcPtr = i.v
	sigID = SignatureID(i.u1)
	args = i.vs.View()
	isGoMemmove = i.u2 == 1
	return
}

// AsClz initializes this instruction as a Count Leading Zeroes instruction with OpcodeClz.
func (i *Instruction) AsClz(x Value) {
	i.opcode = OpcodeClz
	i.v = x
	i.typ = x.Type()
}

// AsCtz initializes this instruction as a Count Trailing Zeroes instruction with OpcodeCtz.
func (i *Instruction) AsCtz(x Value) {
	i.opcode = OpcodeCtz
	i.v = x
	i.typ = x.Type()
}

// AsPopcnt initializes this instruction as a Population Count instruction with OpcodePopcnt.
func (i *Instruction) AsPopcnt(x Value) {
	i.opcode = OpcodePopcnt
	i.v = x
	i.typ = x.Type()
}

// AsFneg initializes this instruction as an instruction with OpcodeFneg.
func (i *Instruction) AsFneg(x Value) *Instruction {
	i.opcode = OpcodeFneg
	i.v = x
	i.typ = x.Type()
	return i
}

// AsSqrt initializes this instruction as an instruction with OpcodeSqrt.
func (i *Instruction) AsSqrt(x Value) *Instruction {
	i.opcode = OpcodeSqrt
	i.v = x
	i.typ = x.Type()
	return i
}

// AsFabs initializes this instruction as an instruction with OpcodeFabs.
func (i *Instruction) AsFabs(x Value) *Instruction {
	i.opcode = OpcodeFabs
	i.v = x
	i.typ = x.Type()
	return i
}

// AsFcopysign initializes this instruction as an instruction with OpcodeFcopysign.
func (i *Instruction) AsFcopysign(x, y Value) *Instruction {
	i.opcode = OpcodeFcopysign
	i.v = x
	i.v2 = y
	i.typ = x.Type()
	return i
}

// AsCeil initializes this instruction as an instruction with OpcodeCeil.
func (i *Instruction) AsCeil(x Value) *Instruction {
	i.opcode = OpcodeCeil
	i.v = x
	i.typ = x.Type()
	return i
}

// AsFloor initializes this instruction as an instruction with OpcodeFloor.
func (i *Instruction) AsFloor(x Value) *Instruction {
	i.opcode = OpcodeFloor
	i.v = x
	i.typ = x.Type()
	return i
}

// AsTrunc initializes this instruction as an instruction with OpcodeTrunc.
func (i *Instruction) AsTrunc(x Value) *Instruction {
	i.opcode = OpcodeTrunc
	i.v = x
	i.typ = x.Type()
	return i
}

// AsNearest initializes this instruction as an instruction with OpcodeNearest.
func (i *Instruction) AsNearest(x Value) *Instruction {
	i.opcode = OpcodeNearest
	i.v = x
	i.typ = x.Type()
	return i
}

// AsBitcast initializes this instruction as an instruction with OpcodeBitcast.
func (i *Instruction) AsBitcast(x Value, dstType Type) *Instruction {
	i.opcode = OpcodeBitcast
	i.v = x
	i.typ = dstType
	return i
}

// BitcastData returns the operands for a bitcast instruction.
func (i *Instruction) BitcastData() (x Value, dstType Type) {
	return i.v, i.typ
}

// AsFdemote initializes this instruction as an instruction with OpcodeFdemote.
func (i *Instruction) AsFdemote(x Value) {
	i.opcode = OpcodeFdemote
	i.v = x
	i.typ = TypeF32
}

// AsFpromote initializes this instruction as an instruction with OpcodeFpromote.
func (i *Instruction) AsFpromote(x Value) {
	i.opcode = OpcodeFpromote
	i.v = x
	i.typ = TypeF64
}

// AsFcvtFromInt initializes this instruction as an instruction with either OpcodeFcvtFromUint or OpcodeFcvtFromSint
func (i *Instruction) AsFcvtFromInt(x Value, signed bool, dst64bit bool) *Instruction {
	if signed {
		i.opcode = OpcodeFcvtFromSint
	} else {
		i.opcode = OpcodeFcvtFromUint
	}
	i.v = x
	if dst64bit {
		i.typ = TypeF64
	} else {
		i.typ = TypeF32
	}
	return i
}

// AsFcvtToInt initializes this instruction as an instruction with either OpcodeFcvtToUint or OpcodeFcvtToSint
func (i *Instruction) AsFcvtToInt(x, ctx Value, signed bool, dst64bit bool, sat bool) *Instruction {
	switch {
	case signed && !sat:
		i.opcode = OpcodeFcvtToSint
	case !signed && !sat:
		i.opcode = OpcodeFcvtToUint
	case signed && sat:
		i.opcode = OpcodeFcvtToSintSat
	case !signed && sat:
		i.opcode = OpcodeFcvtToUintSat
	}
	i.v = x
	i.v2 = ctx
	if dst64bit {
		i.typ = TypeI64
	} else {
		i.typ = TypeI32
	}
	return i
}

// AsVFcvtToIntSat initializes this instruction as an instruction with either OpcodeVFcvtToSintSat or OpcodeVFcvtToUintSat
func (i *Instruction) AsVFcvtToIntSat(x Value, lane VecLane, signed bool) *Instruction {
	if signed {
		i.opcode = OpcodeVFcvtToSintSat
	} else {
		i.opcode = OpcodeVFcvtToUintSat
	}
	i.v = x
	i.u1 = uint64(lane)
	return i
}

// AsVFcvtFromInt initializes this instruction as an instruction with either OpcodeVFcvtToSintSat or OpcodeVFcvtToUintSat
func (i *Instruction) AsVFcvtFromInt(x Value, lane VecLane, signed bool) *Instruction {
	if signed {
		i.opcode = OpcodeVFcvtFromSint
	} else {
		i.opcode = OpcodeVFcvtFromUint
	}
	i.v = x
	i.u1 = uint64(lane)
	return i
}

// AsNarrow initializes this instruction as an instruction with either OpcodeSnarrow or OpcodeUnarrow
func (i *Instruction) AsNarrow(x, y Value, lane VecLane, signed bool) *Instruction {
	if signed {
		i.opcode = OpcodeSnarrow
	} else {
		i.opcode = OpcodeUnarrow
	}
	i.v = x
	i.v2 = y
	i.u1 = uint64(lane)
	return i
}

// AsFvpromoteLow initializes this instruction as an instruction with OpcodeFvpromoteLow
func (i *Instruction) AsFvpromoteLow(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeFvpromoteLow
	i.v = x
	i.u1 = uint64(lane)
	return i
}

// AsFvdemote initializes this instruction as an instruction with OpcodeFvdemote
func (i *Instruction) AsFvdemote(x Value, lane VecLane) *Instruction {
	i.opcode = OpcodeFvdemote
	i.v = x
	i.u1 = uint64(lane)
	return i
}

// AsSExtend initializes this instruction as a sign extension instruction with OpcodeSExtend.
func (i *Instruction) AsSExtend(v Value, from, to byte) *Instruction {
	i.opcode = OpcodeSExtend
	i.v = v
	i.u1 = uint64(from)<<8 | uint64(to)
	if to == 64 {
		i.typ = TypeI64
	} else {
		i.typ = TypeI32
	}
	return i
}

// AsUExtend initializes this instruction as an unsigned extension instruction with OpcodeUExtend.
func (i *Instruction) AsUExtend(v Value, from, to byte) *Instruction {
	i.opcode = OpcodeUExtend
	i.v = v
	i.u1 = uint64(from)<<8 | uint64(to)
	if to == 64 {
		i.typ = TypeI64
	} else {
		i.typ = TypeI32
	}
	return i
}

func (i *Instruction) ExtendData() (from, to byte, signed bool) {
	if i.opcode != OpcodeSExtend && i.opcode != OpcodeUExtend {
		panic("BUG: ExtendData only available for OpcodeSExtend and OpcodeUExtend")
	}
	from = byte(i.u1 >> 8)
	to = byte(i.u1)
	signed = i.opcode == OpcodeSExtend
	return
}

// AsSelect initializes this instruction as an unsigned extension instruction with OpcodeSelect.
func (i *Instruction) AsSelect(c, x, y Value) *Instruction {
	i.opcode = OpcodeSelect
	i.v = c
	i.v2 = x
	i.v3 = y
	i.typ = x.Type()
	return i
}

// SelectData returns the select data for this instruction necessary for backends.
func (i *Instruction) SelectData() (c, x, y Value) {
	c = i.v
	x = i.v2
	y = i.v3
	return
}

// ExtendFromToBits returns the from and to bit size for the extension instruction.
func (i *Instruction) ExtendFromToBits() (from, to byte) {
	from = byte(i.u1 >> 8)
	to = byte(i.u1)
	return
}

// Format returns a string representation of this instruction with the given builder.
// For debugging purposes only.
func (i *Instruction) Format(b Builder) string {
	var instSuffix string
	switch i.opcode {
	case OpcodeExitWithCode:
		instSuffix = fmt.Sprintf(" %s, %s", i.v.Format(b), wazevoapi.ExitCode(i.u1))
	case OpcodeExitIfTrueWithCode:
		instSuffix = fmt.Sprintf(" %s, %s, %s", i.v2.Format(b), i.v.Format(b), wazevoapi.ExitCode(i.u1))
	case OpcodeIadd, OpcodeIsub, OpcodeImul, OpcodeFadd, OpcodeFsub, OpcodeFmin, OpcodeFmax, OpcodeFdiv, OpcodeFmul:
		instSuffix = fmt.Sprintf(" %s, %s", i.v.Format(b), i.v2.Format(b))
	case OpcodeIcmp:
		instSuffix = fmt.Sprintf(" %s, %s, %s", IntegerCmpCond(i.u1), i.v.Format(b), i.v2.Format(b))
	case OpcodeFcmp:
		instSuffix = fmt.Sprintf(" %s, %s, %s", FloatCmpCond(i.u1), i.v.Format(b), i.v2.Format(b))
	case OpcodeSExtend, OpcodeUExtend:
		instSuffix = fmt.Sprintf(" %s, %d->%d", i.v.Format(b), i.u1>>8, i.u1&0xff)
	case OpcodeCall, OpcodeCallIndirect:
		view := i.vs.View()
		vs := make([]string, len(view))
		for idx := range vs {
			vs[idx] = view[idx].Format(b)
		}
		if i.opcode == OpcodeCallIndirect {
			instSuffix = fmt.Sprintf(" %s:%s, %s", i.v.Format(b), SignatureID(i.u1), strings.Join(vs, ", "))
		} else {
			instSuffix = fmt.Sprintf(" %s:%s, %s", FuncRef(i.u1), SignatureID(i.u2), strings.Join(vs, ", "))
		}
	case OpcodeStore, OpcodeIstore8, OpcodeIstore16, OpcodeIstore32:
		instSuffix = fmt.Sprintf(" %s, %s, %#x", i.v.Format(b), i.v2.Format(b), uint32(i.u1))
	case OpcodeLoad, OpcodeVZeroExtLoad:
		instSuffix = fmt.Sprintf(" %s, %#x", i.v.Format(b), int32(i.u1))
	case OpcodeLoadSplat:
		instSuffix = fmt.Sprintf(".%s %s, %#x", VecLane(i.u2), i.v.Format(b), int32(i.u1))
	case OpcodeUload8, OpcodeUload16, OpcodeUload32, OpcodeSload8, OpcodeSload16, OpcodeSload32:
		instSuffix = fmt.Sprintf(" %s, %#x", i.v.Format(b), int32(i.u1))
	case OpcodeSelect, OpcodeVbitselect:
		instSuffix = fmt.Sprintf(" %s, %s, %s", i.v.Format(b), i.v2.Format(b), i.v3.Format(b))
	case OpcodeIconst:
		switch i.typ {
		case TypeI32:
			instSuffix = fmt.Sprintf("_32 %#x", uint32(i.u1))
		case TypeI64:
			instSuffix = fmt.Sprintf("_64 %#x", i.u1)
		}
	case OpcodeVconst:
		instSuffix = fmt.Sprintf(" %016x %016x", i.u1, i.u2)
	case OpcodeF32const:
		instSuffix = fmt.Sprintf(" %f", math.Float32frombits(uint32(i.u1)))
	case OpcodeF64const:
		instSuffix = fmt.Sprintf(" %f", math.Float64frombits(i.u1))
	case OpcodeReturn:
		view := i.vs.View()
		if len(view) == 0 {
			break
		}
		vs := make([]string, len(view))
		for idx := range vs {
			vs[idx] = view[idx].Format(b)
		}
		instSuffix = fmt.Sprintf(" %s", strings.Join(vs, ", "))
	case OpcodeJump:
		view := i.vs.View()
		vs := make([]string, len(view)+1)
		if i.IsFallthroughJump() {
			vs[0] = " fallthrough"
		} else {
			blockId := BasicBlockID(i.rValue)
			vs[0] = " " + b.BasicBlock(blockId).Name()
		}
		for idx := range view {
			vs[idx+1] = view[idx].Format(b)
		}

		instSuffix = strings.Join(vs, ", ")
	case OpcodeBrz, OpcodeBrnz:
		view := i.vs.View()
		vs := make([]string, len(view)+2)
		vs[0] = " " + i.v.Format(b)
		blockId := BasicBlockID(i.rValue)
		vs[1] = b.BasicBlock(blockId).Name()
		for idx := range view {
			vs[idx+2] = view[idx].Format(b)
		}
		instSuffix = strings.Join(vs, ", ")
	case OpcodeBrTable:
		// `BrTable index, [label1, label2, ... labelN]`
		instSuffix = fmt.Sprintf(" %s", i.v.Format(b))
		instSuffix += ", ["
		for i, target := range i.rValues.View() {
			blk := b.BasicBlock(BasicBlockID(target))
			if i == 0 {
				instSuffix += blk.Name()
			} else {
				instSuffix += ", " + blk.Name()
			}
		}
		instSuffix += "]"
	case OpcodeBand, OpcodeBor, OpcodeBxor, OpcodeRotr, OpcodeRotl, OpcodeIshl, OpcodeSshr, OpcodeUshr,
		OpcodeSdiv, OpcodeUdiv, OpcodeFcopysign, OpcodeSrem, OpcodeUrem,
		OpcodeVbnot, OpcodeVbxor, OpcodeVbor, OpcodeVband, OpcodeVbandnot, OpcodeVIcmp, OpcodeVFcmp:
		instSuffix = fmt.Sprintf(" %s, %s", i.v.Format(b), i.v2.Format(b))
	case OpcodeUndefined:
	case OpcodeClz, OpcodeCtz, OpcodePopcnt, OpcodeFneg, OpcodeFcvtToSint, OpcodeFcvtToUint, OpcodeFcvtFromSint,
		OpcodeFcvtFromUint, OpcodeFcvtToSintSat, OpcodeFcvtToUintSat, OpcodeFdemote, OpcodeFpromote, OpcodeIreduce, OpcodeBitcast, OpcodeSqrt, OpcodeFabs,
		OpcodeCeil, OpcodeFloor, OpcodeTrunc, OpcodeNearest:
		instSuffix = " " + i.v.Format(b)
	case OpcodeVIadd, OpcodeExtIaddPairwise, OpcodeVSaddSat, OpcodeVUaddSat, OpcodeVIsub, OpcodeVSsubSat, OpcodeVUsubSat,
		OpcodeVImin, OpcodeVUmin, OpcodeVImax, OpcodeVUmax, OpcodeVImul, OpcodeVAvgRound,
		OpcodeVFadd, OpcodeVFsub, OpcodeVFmul, OpcodeVFdiv,
		OpcodeVIshl, OpcodeVSshr, OpcodeVUshr,
		OpcodeVFmin, OpcodeVFmax, OpcodeVMinPseudo, OpcodeVMaxPseudo,
		OpcodeSnarrow, OpcodeUnarrow, OpcodeSwizzle, OpcodeSqmulRoundSat:
		instSuffix = fmt.Sprintf(".%s %s, %s", VecLane(i.u1), i.v.Format(b), i.v2.Format(b))
	case OpcodeVIabs, OpcodeVIneg, OpcodeVIpopcnt, OpcodeVhighBits, OpcodeVallTrue, OpcodeVanyTrue,
		OpcodeVFabs, OpcodeVFneg, OpcodeVSqrt, OpcodeVCeil, OpcodeVFloor, OpcodeVTrunc, OpcodeVNearest,
		OpcodeVFcvtToUintSat, OpcodeVFcvtToSintSat, OpcodeVFcvtFromUint, OpcodeVFcvtFromSint,
		OpcodeFvpromoteLow, OpcodeFvdemote, OpcodeSwidenLow, OpcodeUwidenLow, OpcodeSwidenHigh, OpcodeUwidenHigh,
		OpcodeSplat:
		instSuffix = fmt.Sprintf(".%s %s", VecLane(i.u1), i.v.Format(b))
	case OpcodeExtractlane:
		var signedness string
		if i.u1 != 0 {
			signedness = "signed"
		} else {
			signedness = "unsigned"
		}
		instSuffix = fmt.Sprintf(".%s %d, %s (%s)", VecLane(i.u2), 0x0000FFFF&i.u1, i.v.Format(b), signedness)
	case OpcodeInsertlane:
		instSuffix = fmt.Sprintf(".%s %d, %s, %s", VecLane(i.u2), i.u1, i.v.Format(b), i.v2.Format(b))
	case OpcodeShuffle:
		lanes := make([]byte, 16)
		for idx := 0; idx < 8; idx++ {
			lanes[idx] = byte(i.u1 >> (8 * idx))
		}
		for idx := 0; idx < 8; idx++ {
			lanes[idx+8] = byte(i.u2 >> (8 * idx))
		}
		// Prints Shuffle.[0 1 2 3 4 5 6 7 ...] v2, v3
		instSuffix = fmt.Sprintf(".%v %s, %s", lanes, i.v.Format(b), i.v2.Format(b))
	case OpcodeAtomicRmw:
		instSuffix = fmt.Sprintf(" %s_%d, %s, %s", AtomicRmwOp(i.u1), 8*i.u2, i.v.Format(b), i.v2.Format(b))
	case OpcodeAtomicLoad:
		instSuffix = fmt.Sprintf("_%d, %s", 8*i.u1, i.v.Format(b))
	case OpcodeAtomicStore:
		instSuffix = fmt.Sprintf("_%d, %s, %s", 8*i.u1, i.v.Format(b), i.v2.Format(b))
	case OpcodeAtomicCas:
		instSuffix = fmt.Sprintf("_%d, %s, %s, %s", 8*i.u1, i.v.Format(b), i.v2.Format(b), i.v3.Format(b))
	case OpcodeFence:
		instSuffix = fmt.Sprintf(" %d", i.u1)
	case OpcodeWideningPairwiseDotProductS:
		instSuffix = fmt.Sprintf(" %s, %s", i.v.Format(b), i.v2.Format(b))
	default:
		panic(fmt.Sprintf("TODO: format for %s", i.opcode))
	}

	instr := i.opcode.String() + instSuffix

	var rvs []string
	r1, rs := i.Returns()
	if r1.Valid() {
		rvs = append(rvs, r1.formatWithType(b))
	}

	for _, v := range rs {
		rvs = append(rvs, v.formatWithType(b))
	}

	if len(rvs) > 0 {
		return fmt.Sprintf("%s = %s", strings.Join(rvs, ", "), instr)
	} else {
		return instr
	}
}

// addArgumentBranchInst adds an argument to this instruction.
func (i *Instruction) addArgumentBranchInst(b *builder, v Value) {
	switch i.opcode {
	case OpcodeJump, OpcodeBrz, OpcodeBrnz:
		i.vs = i.vs.Append(&b.varLengthPool, v)
	default:
		panic("BUG: " + i.opcode.String())
	}
}

// Constant returns true if this instruction is a constant instruction.
func (i *Instruction) Constant() bool {
	switch i.opcode {
	case OpcodeIconst, OpcodeF32const, OpcodeF64const:
		return true
	}
	return false
}

// ConstantVal returns the constant value of this instruction.
// How to interpret the return value depends on the opcode.
func (i *Instruction) ConstantVal() (ret uint64) {
	switch i.opcode {
	case OpcodeIconst, OpcodeF32const, OpcodeF64const:
		ret = i.u1
	default:
		panic("TODO")
	}
	return
}

// String implements fmt.Stringer.
func (o Opcode) String() (ret string) {
	switch o {
	case OpcodeInvalid:
		return "invalid"
	case OpcodeUndefined:
		return "Undefined"
	case OpcodeJump:
		return "Jump"
	case OpcodeBrz:
		return "Brz"
	case OpcodeBrnz:
		return "Brnz"
	case OpcodeBrTable:
		return "BrTable"
	case OpcodeExitWithCode:
		return "Exit"
	case OpcodeExitIfTrueWithCode:
		return "ExitIfTrue"
	case OpcodeReturn:
		return "Return"
	case OpcodeCall:
		return "Call"
	case OpcodeCallIndirect:
		return "CallIndirect"
	case OpcodeSplat:
		return "Splat"
	case OpcodeSwizzle:
		return "Swizzle"
	case OpcodeInsertlane:
		return "Insertlane"
	case OpcodeExtractlane:
		return "Extractlane"
	case OpcodeLoad:
		return "Load"
	case OpcodeLoadSplat:
		return "LoadSplat"
	case OpcodeStore:
		return "Store"
	case OpcodeUload8:
		return "Uload8"
	case OpcodeSload8:
		return "Sload8"
	case OpcodeIstore8:
		return "Istore8"
	case OpcodeUload16:
		return "Uload16"
	case OpcodeSload16:
		return "Sload16"
	case OpcodeIstore16:
		return "Istore16"
	case OpcodeUload32:
		return "Uload32"
	case OpcodeSload32:
		return "Sload32"
	case OpcodeIstore32:
		return "Istore32"
	case OpcodeIconst:
		return "Iconst"
	case OpcodeF32const:
		return "F32const"
	case OpcodeF64const:
		return "F64const"
	case OpcodeVconst:
		return "Vconst"
	case OpcodeShuffle:
		return "Shuffle"
	case OpcodeSelect:
		return "Select"
	case OpcodeVanyTrue:
		return "VanyTrue"
	case OpcodeVallTrue:
		return "VallTrue"
	case OpcodeVhighBits:
		return "VhighBits"
	case OpcodeIcmp:
		return "Icmp"
	case OpcodeIcmpImm:
		return "IcmpImm"
	case OpcodeVIcmp:
		return "VIcmp"
	case OpcodeIadd:
		return "Iadd"
	case OpcodeIsub:
		return "Isub"
	case OpcodeImul:
		return "Imul"
	case OpcodeUdiv:
		return "Udiv"
	case OpcodeSdiv:
		return "Sdiv"
	case OpcodeUrem:
		return "Urem"
	case OpcodeSrem:
		return "Srem"
	case OpcodeBand:
		return "Band"
	case OpcodeBor:
		return "Bor"
	case OpcodeBxor:
		return "Bxor"
	case OpcodeBnot:
		return "Bnot"
	case OpcodeRotl:
		return "Rotl"
	case OpcodeRotr:
		return "Rotr"
	case OpcodeIshl:
		return "Ishl"
	case OpcodeUshr:
		return "Ushr"
	case OpcodeSshr:
		return "Sshr"
	case OpcodeClz:
		return "Clz"
	case OpcodeCtz:
		return "Ctz"
	case OpcodePopcnt:
		return "Popcnt"
	case OpcodeFcmp:
		return "Fcmp"
	case OpcodeFadd:
		return "Fadd"
	case OpcodeFsub:
		return "Fsub"
	case OpcodeFmul:
		return "Fmul"
	case OpcodeFdiv:
		return "Fdiv"
	case OpcodeSqmulRoundSat:
		return "SqmulRoundSat"
	case OpcodeSqrt:
		return "Sqrt"
	case OpcodeFneg:
		return "Fneg"
	case OpcodeFabs:
		return "Fabs"
	case OpcodeFcopysign:
		return "Fcopysign"
	case OpcodeFmin:
		return "Fmin"
	case OpcodeFmax:
		return "Fmax"
	case OpcodeCeil:
		return "Ceil"
	case OpcodeFloor:
		return "Floor"
	case OpcodeTrunc:
		return "Trunc"
	case OpcodeNearest:
		return "Nearest"
	case OpcodeBitcast:
		return "Bitcast"
	case OpcodeIreduce:
		return "Ireduce"
	case OpcodeSnarrow:
		return "Snarrow"
	case OpcodeUnarrow:
		return "Unarrow"
	case OpcodeSwidenLow:
		return "SwidenLow"
	case OpcodeSwidenHigh:
		return "SwidenHigh"
	case OpcodeUwidenLow:
		return "UwidenLow"
	case OpcodeUwidenHigh:
		return "UwidenHigh"
	case OpcodeExtIaddPairwise:
		return "IaddPairwise"
	case OpcodeWideningPairwiseDotProductS:
		return "WideningPairwiseDotProductS"
	case OpcodeUExtend:
		return "UExtend"
	case OpcodeSExtend:
		return "SExtend"
	case OpcodeFpromote:
		return "Fpromote"
	case OpcodeFdemote:
		return "Fdemote"
	case OpcodeFvdemote:
		return "Fvdemote"
	case OpcodeFcvtToUint:
		return "FcvtToUint"
	case OpcodeFcvtToSint:
		return "FcvtToSint"
	case OpcodeFcvtToUintSat:
		return "FcvtToUintSat"
	case OpcodeFcvtToSintSat:
		return "FcvtToSintSat"
	case OpcodeFcvtFromUint:
		return "FcvtFromUint"
	case OpcodeFcvtFromSint:
		return "FcvtFromSint"
	case OpcodeAtomicRmw:
		return "AtomicRmw"
	case OpcodeAtomicCas:
		return "AtomicCas"
	case OpcodeAtomicLoad:
		return "AtomicLoad"
	case OpcodeAtomicStore:
		return "AtomicStore"
	case OpcodeFence:
		return "Fence"
	case OpcodeVbor:
		return "Vbor"
	case OpcodeVbxor:
		return "Vbxor"
	case OpcodeVband:
		return "Vband"
	case OpcodeVbandnot:
		return "Vbandnot"
	case OpcodeVbnot:
		return "Vbnot"
	case OpcodeVbitselect:
		return "Vbitselect"
	case OpcodeVIadd:
		return "VIadd"
	case OpcodeVSaddSat:
		return "VSaddSat"
	case OpcodeVUaddSat:
		return "VUaddSat"
	case OpcodeVSsubSat:
		return "VSsubSat"
	case OpcodeVUsubSat:
		return "VUsubSat"
	case OpcodeVAvgRound:
		return "OpcodeVAvgRound"
	case OpcodeVIsub:
		return "VIsub"
	case OpcodeVImin:
		return "VImin"
	case OpcodeVUmin:
		return "VUmin"
	case OpcodeVImax:
		return "VImax"
	case OpcodeVUmax:
		return "VUmax"
	case OpcodeVImul:
		return "VImul"
	case OpcodeVIabs:
		return "VIabs"
	case OpcodeVIneg:
		return "VIneg"
	case OpcodeVIpopcnt:
		return "VIpopcnt"
	case OpcodeVIshl:
		return "VIshl"
	case OpcodeVUshr:
		return "VUshr"
	case OpcodeVSshr:
		return "VSshr"
	case OpcodeVFabs:
		return "VFabs"
	case OpcodeVFmax:
		return "VFmax"
	case OpcodeVFmin:
		return "VFmin"
	case OpcodeVFneg:
		return "VFneg"
	case OpcodeVFadd:
		return "VFadd"
	case OpcodeVFsub:
		return "VFsub"
	case OpcodeVFmul:
		return "VFmul"
	case OpcodeVFdiv:
		return "VFdiv"
	case OpcodeVFcmp:
		return "VFcmp"
	case OpcodeVCeil:
		return "VCeil"
	case OpcodeVFloor:
		return "VFloor"
	case OpcodeVTrunc:
		return "VTrunc"
	case OpcodeVNearest:
		return "VNearest"
	case OpcodeVMaxPseudo:
		return "VMaxPseudo"
	case OpcodeVMinPseudo:
		return "VMinPseudo"
	case OpcodeVSqrt:
		return "VSqrt"
	case OpcodeVFcvtToUintSat:
		return "VFcvtToUintSat"
	case OpcodeVFcvtToSintSat:
		return "VFcvtToSintSat"
	case OpcodeVFcvtFromUint:
		return "VFcvtFromUint"
	case OpcodeVFcvtFromSint:
		return "VFcvtFromSint"
	case OpcodeFvpromoteLow:
		return "FvpromoteLow"
	case OpcodeVZeroExtLoad:
		return "VZeroExtLoad"
	}
	panic(fmt.Sprintf("unknown opcode %d", o))
}
