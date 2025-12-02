package interpreter

import (
	"fmt"
	"math"
	"strings"
)

// unsignedInt represents unsigned 32-bit or 64-bit integers.
type unsignedInt byte

const (
	unsignedInt32 unsignedInt = iota
	unsignedInt64
)

// String implements fmt.Stringer.
func (s unsignedInt) String() (ret string) {
	switch s {
	case unsignedInt32:
		ret = "i32"
	case unsignedInt64:
		ret = "i64"
	}
	return
}

// signedInt represents signed or unsigned integers.
type signedInt byte

const (
	signedInt32 signedInt = iota
	signedInt64
	signedUint32
	signedUint64
)

// String implements fmt.Stringer.
func (s signedInt) String() (ret string) {
	switch s {
	case signedUint32:
		ret = "u32"
	case signedUint64:
		ret = "u64"
	case signedInt32:
		ret = "s32"
	case signedInt64:
		ret = "s64"
	}
	return
}

// float represents the scalar double or single precision floating points.
type float byte

const (
	f32 float = iota
	f64
)

// String implements fmt.Stringer.
func (s float) String() (ret string) {
	switch s {
	case f32:
		ret = "f32"
	case f64:
		ret = "f64"
	}
	return
}

// unsignedType is the union of unsignedInt, float and V128 vector type.
type unsignedType byte

const (
	unsignedTypeI32 unsignedType = iota
	unsignedTypeI64
	unsignedTypeF32
	unsignedTypeF64
	unsignedTypeV128
	unsignedTypeUnknown
)

// String implements fmt.Stringer.
func (s unsignedType) String() (ret string) {
	switch s {
	case unsignedTypeI32:
		ret = "i32"
	case unsignedTypeI64:
		ret = "i64"
	case unsignedTypeF32:
		ret = "f32"
	case unsignedTypeF64:
		ret = "f64"
	case unsignedTypeV128:
		ret = "v128"
	case unsignedTypeUnknown:
		ret = "unknown"
	}
	return
}

// signedType is the union of signedInt and float types.
type signedType byte

const (
	signedTypeInt32 signedType = iota
	signedTypeUint32
	signedTypeInt64
	signedTypeUint64
	signedTypeFloat32
	signedTypeFloat64
)

// String implements fmt.Stringer.
func (s signedType) String() (ret string) {
	switch s {
	case signedTypeInt32:
		ret = "s32"
	case signedTypeUint32:
		ret = "u32"
	case signedTypeInt64:
		ret = "s64"
	case signedTypeUint64:
		ret = "u64"
	case signedTypeFloat32:
		ret = "f32"
	case signedTypeFloat64:
		ret = "f64"
	}
	return
}

// operationKind is the Kind of each implementation of Operation interface.
type operationKind uint16

// String implements fmt.Stringer.
func (o operationKind) String() (ret string) {
	switch o {
	case operationKindUnreachable:
		ret = "Unreachable"
	case operationKindLabel:
		ret = "label"
	case operationKindBr:
		ret = "Br"
	case operationKindBrIf:
		ret = "BrIf"
	case operationKindBrTable:
		ret = "BrTable"
	case operationKindCall:
		ret = "Call"
	case operationKindCallIndirect:
		ret = "CallIndirect"
	case operationKindDrop:
		ret = "Drop"
	case operationKindSelect:
		ret = "Select"
	case operationKindPick:
		ret = "Pick"
	case operationKindSet:
		ret = "Swap"
	case operationKindGlobalGet:
		ret = "GlobalGet"
	case operationKindGlobalSet:
		ret = "GlobalSet"
	case operationKindLoad:
		ret = "Load"
	case operationKindLoad8:
		ret = "Load8"
	case operationKindLoad16:
		ret = "Load16"
	case operationKindLoad32:
		ret = "Load32"
	case operationKindStore:
		ret = "Store"
	case operationKindStore8:
		ret = "Store8"
	case operationKindStore16:
		ret = "Store16"
	case operationKindStore32:
		ret = "Store32"
	case operationKindMemorySize:
		ret = "MemorySize"
	case operationKindMemoryGrow:
		ret = "MemoryGrow"
	case operationKindConstI32:
		ret = "ConstI32"
	case operationKindConstI64:
		ret = "ConstI64"
	case operationKindConstF32:
		ret = "ConstF32"
	case operationKindConstF64:
		ret = "ConstF64"
	case operationKindEq:
		ret = "Eq"
	case operationKindNe:
		ret = "Ne"
	case operationKindEqz:
		ret = "Eqz"
	case operationKindLt:
		ret = "Lt"
	case operationKindGt:
		ret = "Gt"
	case operationKindLe:
		ret = "Le"
	case operationKindGe:
		ret = "Ge"
	case operationKindAdd:
		ret = "Add"
	case operationKindSub:
		ret = "Sub"
	case operationKindMul:
		ret = "Mul"
	case operationKindClz:
		ret = "Clz"
	case operationKindCtz:
		ret = "Ctz"
	case operationKindPopcnt:
		ret = "Popcnt"
	case operationKindDiv:
		ret = "Div"
	case operationKindRem:
		ret = "Rem"
	case operationKindAnd:
		ret = "And"
	case operationKindOr:
		ret = "Or"
	case operationKindXor:
		ret = "Xor"
	case operationKindShl:
		ret = "Shl"
	case operationKindShr:
		ret = "Shr"
	case operationKindRotl:
		ret = "Rotl"
	case operationKindRotr:
		ret = "Rotr"
	case operationKindAbs:
		ret = "Abs"
	case operationKindNeg:
		ret = "Neg"
	case operationKindCeil:
		ret = "Ceil"
	case operationKindFloor:
		ret = "Floor"
	case operationKindTrunc:
		ret = "Trunc"
	case operationKindNearest:
		ret = "Nearest"
	case operationKindSqrt:
		ret = "Sqrt"
	case operationKindMin:
		ret = "Min"
	case operationKindMax:
		ret = "Max"
	case operationKindCopysign:
		ret = "Copysign"
	case operationKindI32WrapFromI64:
		ret = "I32WrapFromI64"
	case operationKindITruncFromF:
		ret = "ITruncFromF"
	case operationKindFConvertFromI:
		ret = "FConvertFromI"
	case operationKindF32DemoteFromF64:
		ret = "F32DemoteFromF64"
	case operationKindF64PromoteFromF32:
		ret = "F64PromoteFromF32"
	case operationKindI32ReinterpretFromF32:
		ret = "I32ReinterpretFromF32"
	case operationKindI64ReinterpretFromF64:
		ret = "I64ReinterpretFromF64"
	case operationKindF32ReinterpretFromI32:
		ret = "F32ReinterpretFromI32"
	case operationKindF64ReinterpretFromI64:
		ret = "F64ReinterpretFromI64"
	case operationKindExtend:
		ret = "Extend"
	case operationKindMemoryInit:
		ret = "MemoryInit"
	case operationKindDataDrop:
		ret = "DataDrop"
	case operationKindMemoryCopy:
		ret = "MemoryCopy"
	case operationKindMemoryFill:
		ret = "MemoryFill"
	case operationKindTableInit:
		ret = "TableInit"
	case operationKindElemDrop:
		ret = "ElemDrop"
	case operationKindTableCopy:
		ret = "TableCopy"
	case operationKindRefFunc:
		ret = "RefFunc"
	case operationKindTableGet:
		ret = "TableGet"
	case operationKindTableSet:
		ret = "TableSet"
	case operationKindTableSize:
		ret = "TableSize"
	case operationKindTableGrow:
		ret = "TableGrow"
	case operationKindTableFill:
		ret = "TableFill"
	case operationKindV128Const:
		ret = "ConstV128"
	case operationKindV128Add:
		ret = "V128Add"
	case operationKindV128Sub:
		ret = "V128Sub"
	case operationKindV128Load:
		ret = "V128Load"
	case operationKindV128LoadLane:
		ret = "V128LoadLane"
	case operationKindV128Store:
		ret = "V128Store"
	case operationKindV128StoreLane:
		ret = "V128StoreLane"
	case operationKindV128ExtractLane:
		ret = "V128ExtractLane"
	case operationKindV128ReplaceLane:
		ret = "V128ReplaceLane"
	case operationKindV128Splat:
		ret = "V128Splat"
	case operationKindV128Shuffle:
		ret = "V128Shuffle"
	case operationKindV128Swizzle:
		ret = "V128Swizzle"
	case operationKindV128AnyTrue:
		ret = "V128AnyTrue"
	case operationKindV128AllTrue:
		ret = "V128AllTrue"
	case operationKindV128And:
		ret = "V128And"
	case operationKindV128Not:
		ret = "V128Not"
	case operationKindV128Or:
		ret = "V128Or"
	case operationKindV128Xor:
		ret = "V128Xor"
	case operationKindV128Bitselect:
		ret = "V128Bitselect"
	case operationKindV128AndNot:
		ret = "V128AndNot"
	case operationKindV128BitMask:
		ret = "V128BitMask"
	case operationKindV128Shl:
		ret = "V128Shl"
	case operationKindV128Shr:
		ret = "V128Shr"
	case operationKindV128Cmp:
		ret = "V128Cmp"
	case operationKindSignExtend32From8:
		ret = "SignExtend32From8"
	case operationKindSignExtend32From16:
		ret = "SignExtend32From16"
	case operationKindSignExtend64From8:
		ret = "SignExtend64From8"
	case operationKindSignExtend64From16:
		ret = "SignExtend64From16"
	case operationKindSignExtend64From32:
		ret = "SignExtend64From32"
	case operationKindV128AddSat:
		ret = "V128AddSat"
	case operationKindV128SubSat:
		ret = "V128SubSat"
	case operationKindV128Mul:
		ret = "V128Mul"
	case operationKindV128Div:
		ret = "V128Div"
	case operationKindV128Neg:
		ret = "V128Neg"
	case operationKindV128Sqrt:
		ret = "V128Sqrt"
	case operationKindV128Abs:
		ret = "V128Abs"
	case operationKindV128Popcnt:
		ret = "V128Popcnt"
	case operationKindV128Min:
		ret = "V128Min"
	case operationKindV128Max:
		ret = "V128Max"
	case operationKindV128AvgrU:
		ret = "V128AvgrU"
	case operationKindV128Ceil:
		ret = "V128Ceil"
	case operationKindV128Floor:
		ret = "V128Floor"
	case operationKindV128Trunc:
		ret = "V128Trunc"
	case operationKindV128Nearest:
		ret = "V128Nearest"
	case operationKindV128Pmin:
		ret = "V128Pmin"
	case operationKindV128Pmax:
		ret = "V128Pmax"
	case operationKindV128Extend:
		ret = "V128Extend"
	case operationKindV128ExtMul:
		ret = "V128ExtMul"
	case operationKindV128Q15mulrSatS:
		ret = "V128Q15mulrSatS"
	case operationKindV128ExtAddPairwise:
		ret = "V128ExtAddPairwise"
	case operationKindV128FloatPromote:
		ret = "V128FloatPromote"
	case operationKindV128FloatDemote:
		ret = "V128FloatDemote"
	case operationKindV128FConvertFromI:
		ret = "V128FConvertFromI"
	case operationKindV128Dot:
		ret = "V128Dot"
	case operationKindV128Narrow:
		ret = "V128Narrow"
	case operationKindV128ITruncSatFromF:
		ret = "V128ITruncSatFromF"
	case operationKindBuiltinFunctionCheckExitCode:
		ret = "BuiltinFunctionCheckExitCode"
	case operationKindAtomicMemoryWait:
		ret = "operationKindAtomicMemoryWait"
	case operationKindAtomicMemoryNotify:
		ret = "operationKindAtomicMemoryNotify"
	case operationKindAtomicFence:
		ret = "operationKindAtomicFence"
	case operationKindAtomicLoad:
		ret = "operationKindAtomicLoad"
	case operationKindAtomicLoad8:
		ret = "operationKindAtomicLoad8"
	case operationKindAtomicLoad16:
		ret = "operationKindAtomicLoad16"
	case operationKindAtomicStore:
		ret = "operationKindAtomicStore"
	case operationKindAtomicStore8:
		ret = "operationKindAtomicStore8"
	case operationKindAtomicStore16:
		ret = "operationKindAtomicStore16"
	case operationKindAtomicRMW:
		ret = "operationKindAtomicRMW"
	case operationKindAtomicRMW8:
		ret = "operationKindAtomicRMW8"
	case operationKindAtomicRMW16:
		ret = "operationKindAtomicRMW16"
	case operationKindAtomicRMWCmpxchg:
		ret = "operationKindAtomicRMWCmpxchg"
	case operationKindAtomicRMW8Cmpxchg:
		ret = "operationKindAtomicRMW8Cmpxchg"
	case operationKindAtomicRMW16Cmpxchg:
		ret = "operationKindAtomicRMW16Cmpxchg"
	default:
		panic(fmt.Errorf("unknown operation %d", o))
	}
	return
}

const (
	// operationKindUnreachable is the Kind for NewOperationUnreachable.
	operationKindUnreachable operationKind = iota
	// operationKindLabel is the Kind for NewOperationLabel.
	operationKindLabel
	// operationKindBr is the Kind for NewOperationBr.
	operationKindBr
	// operationKindBrIf is the Kind for NewOperationBrIf.
	operationKindBrIf
	// operationKindBrTable is the Kind for NewOperationBrTable.
	operationKindBrTable
	// operationKindCall is the Kind for NewOperationCall.
	operationKindCall
	// operationKindCallIndirect is the Kind for NewOperationCallIndirect.
	operationKindCallIndirect
	// operationKindDrop is the Kind for NewOperationDrop.
	operationKindDrop
	// operationKindSelect is the Kind for NewOperationSelect.
	operationKindSelect
	// operationKindPick is the Kind for NewOperationPick.
	operationKindPick
	// operationKindSet is the Kind for NewOperationSet.
	operationKindSet
	// operationKindGlobalGet is the Kind for NewOperationGlobalGet.
	operationKindGlobalGet
	// operationKindGlobalSet is the Kind for NewOperationGlobalSet.
	operationKindGlobalSet
	// operationKindLoad is the Kind for NewOperationLoad.
	operationKindLoad
	// operationKindLoad8 is the Kind for NewOperationLoad8.
	operationKindLoad8
	// operationKindLoad16 is the Kind for NewOperationLoad16.
	operationKindLoad16
	// operationKindLoad32 is the Kind for NewOperationLoad32.
	operationKindLoad32
	// operationKindStore is the Kind for NewOperationStore.
	operationKindStore
	// operationKindStore8 is the Kind for NewOperationStore8.
	operationKindStore8
	// operationKindStore16 is the Kind for NewOperationStore16.
	operationKindStore16
	// operationKindStore32 is the Kind for NewOperationStore32.
	operationKindStore32
	// operationKindMemorySize is the Kind for NewOperationMemorySize.
	operationKindMemorySize
	// operationKindMemoryGrow is the Kind for NewOperationMemoryGrow.
	operationKindMemoryGrow
	// operationKindConstI32 is the Kind for NewOperationConstI32.
	operationKindConstI32
	// operationKindConstI64 is the Kind for NewOperationConstI64.
	operationKindConstI64
	// operationKindConstF32 is the Kind for NewOperationConstF32.
	operationKindConstF32
	// operationKindConstF64 is the Kind for NewOperationConstF64.
	operationKindConstF64
	// operationKindEq is the Kind for NewOperationEq.
	operationKindEq
	// operationKindNe is the Kind for NewOperationNe.
	operationKindNe
	// operationKindEqz is the Kind for NewOperationEqz.
	operationKindEqz
	// operationKindLt is the Kind for NewOperationLt.
	operationKindLt
	// operationKindGt is the Kind for NewOperationGt.
	operationKindGt
	// operationKindLe is the Kind for NewOperationLe.
	operationKindLe
	// operationKindGe is the Kind for NewOperationGe.
	operationKindGe
	// operationKindAdd is the Kind for NewOperationAdd.
	operationKindAdd
	// operationKindSub is the Kind for NewOperationSub.
	operationKindSub
	// operationKindMul is the Kind for NewOperationMul.
	operationKindMul
	// operationKindClz is the Kind for NewOperationClz.
	operationKindClz
	// operationKindCtz is the Kind for NewOperationCtz.
	operationKindCtz
	// operationKindPopcnt is the Kind for NewOperationPopcnt.
	operationKindPopcnt
	// operationKindDiv is the Kind for NewOperationDiv.
	operationKindDiv
	// operationKindRem is the Kind for NewOperationRem.
	operationKindRem
	// operationKindAnd is the Kind for NewOperationAnd.
	operationKindAnd
	// operationKindOr is the Kind for NewOperationOr.
	operationKindOr
	// operationKindXor is the Kind for NewOperationXor.
	operationKindXor
	// operationKindShl is the Kind for NewOperationShl.
	operationKindShl
	// operationKindShr is the Kind for NewOperationShr.
	operationKindShr
	// operationKindRotl is the Kind for NewOperationRotl.
	operationKindRotl
	// operationKindRotr is the Kind for NewOperationRotr.
	operationKindRotr
	// operationKindAbs is the Kind for NewOperationAbs.
	operationKindAbs
	// operationKindNeg is the Kind for NewOperationNeg.
	operationKindNeg
	// operationKindCeil is the Kind for NewOperationCeil.
	operationKindCeil
	// operationKindFloor is the Kind for NewOperationFloor.
	operationKindFloor
	// operationKindTrunc is the Kind for NewOperationTrunc.
	operationKindTrunc
	// operationKindNearest is the Kind for NewOperationNearest.
	operationKindNearest
	// operationKindSqrt is the Kind for NewOperationSqrt.
	operationKindSqrt
	// operationKindMin is the Kind for NewOperationMin.
	operationKindMin
	// operationKindMax is the Kind for NewOperationMax.
	operationKindMax
	// operationKindCopysign is the Kind for NewOperationCopysign.
	operationKindCopysign
	// operationKindI32WrapFromI64 is the Kind for NewOperationI32WrapFromI64.
	operationKindI32WrapFromI64
	// operationKindITruncFromF is the Kind for NewOperationITruncFromF.
	operationKindITruncFromF
	// operationKindFConvertFromI is the Kind for NewOperationFConvertFromI.
	operationKindFConvertFromI
	// operationKindF32DemoteFromF64 is the Kind for NewOperationF32DemoteFromF64.
	operationKindF32DemoteFromF64
	// operationKindF64PromoteFromF32 is the Kind for NewOperationF64PromoteFromF32.
	operationKindF64PromoteFromF32
	// operationKindI32ReinterpretFromF32 is the Kind for NewOperationI32ReinterpretFromF32.
	operationKindI32ReinterpretFromF32
	// operationKindI64ReinterpretFromF64 is the Kind for NewOperationI64ReinterpretFromF64.
	operationKindI64ReinterpretFromF64
	// operationKindF32ReinterpretFromI32 is the Kind for NewOperationF32ReinterpretFromI32.
	operationKindF32ReinterpretFromI32
	// operationKindF64ReinterpretFromI64 is the Kind for NewOperationF64ReinterpretFromI64.
	operationKindF64ReinterpretFromI64
	// operationKindExtend is the Kind for NewOperationExtend.
	operationKindExtend
	// operationKindSignExtend32From8 is the Kind for NewOperationSignExtend32From8.
	operationKindSignExtend32From8
	// operationKindSignExtend32From16 is the Kind for NewOperationSignExtend32From16.
	operationKindSignExtend32From16
	// operationKindSignExtend64From8 is the Kind for NewOperationSignExtend64From8.
	operationKindSignExtend64From8
	// operationKindSignExtend64From16 is the Kind for NewOperationSignExtend64From16.
	operationKindSignExtend64From16
	// operationKindSignExtend64From32 is the Kind for NewOperationSignExtend64From32.
	operationKindSignExtend64From32
	// operationKindMemoryInit is the Kind for NewOperationMemoryInit.
	operationKindMemoryInit
	// operationKindDataDrop is the Kind for NewOperationDataDrop.
	operationKindDataDrop
	// operationKindMemoryCopy is the Kind for NewOperationMemoryCopy.
	operationKindMemoryCopy
	// operationKindMemoryFill is the Kind for NewOperationMemoryFill.
	operationKindMemoryFill
	// operationKindTableInit is the Kind for NewOperationTableInit.
	operationKindTableInit
	// operationKindElemDrop is the Kind for NewOperationElemDrop.
	operationKindElemDrop
	// operationKindTableCopy is the Kind for NewOperationTableCopy.
	operationKindTableCopy
	// operationKindRefFunc is the Kind for NewOperationRefFunc.
	operationKindRefFunc
	// operationKindTableGet is the Kind for NewOperationTableGet.
	operationKindTableGet
	// operationKindTableSet is the Kind for NewOperationTableSet.
	operationKindTableSet
	// operationKindTableSize is the Kind for NewOperationTableSize.
	operationKindTableSize
	// operationKindTableGrow is the Kind for NewOperationTableGrow.
	operationKindTableGrow
	// operationKindTableFill is the Kind for NewOperationTableFill.
	operationKindTableFill

	// Vector value related instructions are prefixed by V128.

	// operationKindV128Const is the Kind for NewOperationV128Const.
	operationKindV128Const
	// operationKindV128Add is the Kind for NewOperationV128Add.
	operationKindV128Add
	// operationKindV128Sub is the Kind for NewOperationV128Sub.
	operationKindV128Sub
	// operationKindV128Load is the Kind for NewOperationV128Load.
	operationKindV128Load
	// operationKindV128LoadLane is the Kind for NewOperationV128LoadLane.
	operationKindV128LoadLane
	// operationKindV128Store is the Kind for NewOperationV128Store.
	operationKindV128Store
	// operationKindV128StoreLane is the Kind for NewOperationV128StoreLane.
	operationKindV128StoreLane
	// operationKindV128ExtractLane is the Kind for NewOperationV128ExtractLane.
	operationKindV128ExtractLane
	// operationKindV128ReplaceLane is the Kind for NewOperationV128ReplaceLane.
	operationKindV128ReplaceLane
	// operationKindV128Splat is the Kind for NewOperationV128Splat.
	operationKindV128Splat
	// operationKindV128Shuffle is the Kind for NewOperationV128Shuffle.
	operationKindV128Shuffle
	// operationKindV128Swizzle is the Kind for NewOperationV128Swizzle.
	operationKindV128Swizzle
	// operationKindV128AnyTrue is the Kind for NewOperationV128AnyTrue.
	operationKindV128AnyTrue
	// operationKindV128AllTrue is the Kind for NewOperationV128AllTrue.
	operationKindV128AllTrue
	// operationKindV128BitMask is the Kind for NewOperationV128BitMask.
	operationKindV128BitMask
	// operationKindV128And is the Kind for NewOperationV128And.
	operationKindV128And
	// operationKindV128Not is the Kind for NewOperationV128Not.
	operationKindV128Not
	// operationKindV128Or is the Kind for NewOperationV128Or.
	operationKindV128Or
	// operationKindV128Xor is the Kind for NewOperationV128Xor.
	operationKindV128Xor
	// operationKindV128Bitselect is the Kind for NewOperationV128Bitselect.
	operationKindV128Bitselect
	// operationKindV128AndNot is the Kind for NewOperationV128AndNot.
	operationKindV128AndNot
	// operationKindV128Shl is the Kind for NewOperationV128Shl.
	operationKindV128Shl
	// operationKindV128Shr is the Kind for NewOperationV128Shr.
	operationKindV128Shr
	// operationKindV128Cmp is the Kind for NewOperationV128Cmp.
	operationKindV128Cmp
	// operationKindV128AddSat is the Kind for NewOperationV128AddSat.
	operationKindV128AddSat
	// operationKindV128SubSat is the Kind for NewOperationV128SubSat.
	operationKindV128SubSat
	// operationKindV128Mul is the Kind for NewOperationV128Mul.
	operationKindV128Mul
	// operationKindV128Div is the Kind for NewOperationV128Div.
	operationKindV128Div
	// operationKindV128Neg is the Kind for NewOperationV128Neg.
	operationKindV128Neg
	// operationKindV128Sqrt is the Kind for NewOperationV128Sqrt.
	operationKindV128Sqrt
	// operationKindV128Abs is the Kind for NewOperationV128Abs.
	operationKindV128Abs
	// operationKindV128Popcnt is the Kind for NewOperationV128Popcnt.
	operationKindV128Popcnt
	// operationKindV128Min is the Kind for NewOperationV128Min.
	operationKindV128Min
	// operationKindV128Max is the Kind for NewOperationV128Max.
	operationKindV128Max
	// operationKindV128AvgrU is the Kind for NewOperationV128AvgrU.
	operationKindV128AvgrU
	// operationKindV128Pmin is the Kind for NewOperationV128Pmin.
	operationKindV128Pmin
	// operationKindV128Pmax is the Kind for NewOperationV128Pmax.
	operationKindV128Pmax
	// operationKindV128Ceil is the Kind for NewOperationV128Ceil.
	operationKindV128Ceil
	// operationKindV128Floor is the Kind for NewOperationV128Floor.
	operationKindV128Floor
	// operationKindV128Trunc is the Kind for NewOperationV128Trunc.
	operationKindV128Trunc
	// operationKindV128Nearest is the Kind for NewOperationV128Nearest.
	operationKindV128Nearest
	// operationKindV128Extend is the Kind for NewOperationV128Extend.
	operationKindV128Extend
	// operationKindV128ExtMul is the Kind for NewOperationV128ExtMul.
	operationKindV128ExtMul
	// operationKindV128Q15mulrSatS is the Kind for NewOperationV128Q15mulrSatS.
	operationKindV128Q15mulrSatS
	// operationKindV128ExtAddPairwise is the Kind for NewOperationV128ExtAddPairwise.
	operationKindV128ExtAddPairwise
	// operationKindV128FloatPromote is the Kind for NewOperationV128FloatPromote.
	operationKindV128FloatPromote
	// operationKindV128FloatDemote is the Kind for NewOperationV128FloatDemote.
	operationKindV128FloatDemote
	// operationKindV128FConvertFromI is the Kind for NewOperationV128FConvertFromI.
	operationKindV128FConvertFromI
	// operationKindV128Dot is the Kind for NewOperationV128Dot.
	operationKindV128Dot
	// operationKindV128Narrow is the Kind for NewOperationV128Narrow.
	operationKindV128Narrow
	// operationKindV128ITruncSatFromF is the Kind for NewOperationV128ITruncSatFromF.
	operationKindV128ITruncSatFromF

	// operationKindBuiltinFunctionCheckExitCode is the Kind for NewOperationBuiltinFunctionCheckExitCode.
	operationKindBuiltinFunctionCheckExitCode

	// operationKindAtomicMemoryWait is the kind for NewOperationAtomicMemoryWait.
	operationKindAtomicMemoryWait
	// operationKindAtomicMemoryNotify is the kind for NewOperationAtomicMemoryNotify.
	operationKindAtomicMemoryNotify
	// operationKindAtomicFence is the kind for NewOperationAtomicFence.
	operationKindAtomicFence
	// operationKindAtomicLoad is the kind for NewOperationAtomicLoad.
	operationKindAtomicLoad
	// operationKindAtomicLoad8 is the kind for NewOperationAtomicLoad8.
	operationKindAtomicLoad8
	// operationKindAtomicLoad16 is the kind for NewOperationAtomicLoad16.
	operationKindAtomicLoad16
	// operationKindAtomicStore is the kind for NewOperationAtomicStore.
	operationKindAtomicStore
	// operationKindAtomicStore8 is the kind for NewOperationAtomicStore8.
	operationKindAtomicStore8
	// operationKindAtomicStore16 is the kind for NewOperationAtomicStore16.
	operationKindAtomicStore16

	// operationKindAtomicRMW is the kind for NewOperationAtomicRMW.
	operationKindAtomicRMW
	// operationKindAtomicRMW8 is the kind for NewOperationAtomicRMW8.
	operationKindAtomicRMW8
	// operationKindAtomicRMW16 is the kind for NewOperationAtomicRMW16.
	operationKindAtomicRMW16

	// operationKindAtomicRMWCmpxchg is the kind for NewOperationAtomicRMWCmpxchg.
	operationKindAtomicRMWCmpxchg
	// operationKindAtomicRMW8Cmpxchg is the kind for NewOperationAtomicRMW8Cmpxchg.
	operationKindAtomicRMW8Cmpxchg
	// operationKindAtomicRMW16Cmpxchg is the kind for NewOperationAtomicRMW16Cmpxchg.
	operationKindAtomicRMW16Cmpxchg

	// operationKindEnd is always placed at the bottom of this iota definition to be used in the test.
	operationKindEnd
)

// NewOperationBuiltinFunctionCheckExitCode is a constructor for unionOperation with Kind operationKindBuiltinFunctionCheckExitCode.
//
// OperationBuiltinFunctionCheckExitCode corresponds to the instruction to check the api.Module is already closed due to
// context.DeadlineExceeded, context.Canceled, or the explicit call of CloseWithExitCode on api.Module.
func newOperationBuiltinFunctionCheckExitCode() unionOperation {
	return unionOperation{Kind: operationKindBuiltinFunctionCheckExitCode}
}

// label is the unique identifier for each block in a single function in interpreterir
// where "block" consists of multiple operations, and must End with branching operations
// (e.g. operationKindBr or operationKindBrIf).
type label uint64

// Kind returns the labelKind encoded in this label.
func (l label) Kind() labelKind {
	return labelKind(uint32(l))
}

// FrameID returns the frame id encoded in this label.
func (l label) FrameID() int {
	return int(uint32(l >> 32))
}

// NewLabel is a constructor for a label.
func newLabel(kind labelKind, frameID uint32) label {
	return label(kind) | label(frameID)<<32
}

// String implements fmt.Stringer.
func (l label) String() (ret string) {
	frameID := l.FrameID()
	switch l.Kind() {
	case labelKindHeader:
		ret = fmt.Sprintf(".L%d", frameID)
	case labelKindElse:
		ret = fmt.Sprintf(".L%d_else", frameID)
	case labelKindContinuation:
		ret = fmt.Sprintf(".L%d_cont", frameID)
	case labelKindReturn:
		return ".return"
	}
	return
}

func (l label) IsReturnTarget() bool {
	return l.Kind() == labelKindReturn
}

// labelKind is the Kind of the label.
type labelKind = byte

const (
	// labelKindHeader is the header for various blocks. For example, the "then" block of
	// wasm.OpcodeIfName in Wasm has the label of this Kind.
	labelKindHeader labelKind = iota
	// labelKindElse is the Kind of label for "else" block of wasm.OpcodeIfName in Wasm.
	labelKindElse
	// labelKindContinuation is the Kind of label which is the continuation of blocks.
	// For example, for wasm text like
	// (func
	//   ....
	//   (if (local.get 0) (then (nop)) (else (nop)))
	//   return
	// )
	// we have the continuation block (of if-block) corresponding to "return" opcode.
	labelKindContinuation
	labelKindReturn
	labelKindNum
)

// unionOperation implements Operation and is the compilation (engine.lowerIR) result of a interpreterir.Operation.
//
// Not all operations result in a unionOperation, e.g. interpreterir.OperationI32ReinterpretFromF32, and some operations are
// more complex than others, e.g. interpreterir.NewOperationBrTable.
//
// Note: This is a form of union type as it can store fields needed for any operation. Hence, most fields are opaque and
// only relevant when in context of its kind.
type unionOperation struct {
	// Kind determines how to interpret the other fields in this struct.
	Kind   operationKind
	B1, B2 byte
	B3     bool
	U1, U2 uint64
	U3     uint64
	Us     []uint64
}

// String implements fmt.Stringer.
func (o unionOperation) String() string {
	switch o.Kind {
	case operationKindUnreachable,
		operationKindSelect,
		operationKindMemorySize,
		operationKindMemoryGrow,
		operationKindI32WrapFromI64,
		operationKindF32DemoteFromF64,
		operationKindF64PromoteFromF32,
		operationKindI32ReinterpretFromF32,
		operationKindI64ReinterpretFromF64,
		operationKindF32ReinterpretFromI32,
		operationKindF64ReinterpretFromI64,
		operationKindSignExtend32From8,
		operationKindSignExtend32From16,
		operationKindSignExtend64From8,
		operationKindSignExtend64From16,
		operationKindSignExtend64From32,
		operationKindMemoryInit,
		operationKindDataDrop,
		operationKindMemoryCopy,
		operationKindMemoryFill,
		operationKindTableInit,
		operationKindElemDrop,
		operationKindTableCopy,
		operationKindRefFunc,
		operationKindTableGet,
		operationKindTableSet,
		operationKindTableSize,
		operationKindTableGrow,
		operationKindTableFill,
		operationKindBuiltinFunctionCheckExitCode:
		return o.Kind.String()

	case operationKindCall,
		operationKindGlobalGet,
		operationKindGlobalSet:
		return fmt.Sprintf("%s %d", o.Kind, o.B1)

	case operationKindLabel:
		return label(o.U1).String()

	case operationKindBr:
		return fmt.Sprintf("%s %s", o.Kind, label(o.U1).String())

	case operationKindBrIf:
		thenTarget := label(o.U1)
		elseTarget := label(o.U2)
		return fmt.Sprintf("%s %s, %s", o.Kind, thenTarget, elseTarget)

	case operationKindBrTable:
		var targets []string
		var defaultLabel label
		if len(o.Us) > 0 {
			targets = make([]string, len(o.Us)-1)
			for i, t := range o.Us[1:] {
				targets[i] = label(t).String()
			}
			defaultLabel = label(o.Us[0])
		}
		return fmt.Sprintf("%s [%s] %s", o.Kind, strings.Join(targets, ","), defaultLabel)

	case operationKindCallIndirect:
		return fmt.Sprintf("%s: type=%d, table=%d", o.Kind, o.U1, o.U2)

	case operationKindDrop:
		start := int64(o.U1)
		end := int64(o.U2)
		return fmt.Sprintf("%s %d..%d", o.Kind, start, end)

	case operationKindPick, operationKindSet:
		return fmt.Sprintf("%s %d (is_vector=%v)", o.Kind, o.U1, o.B3)

	case operationKindLoad, operationKindStore:
		return fmt.Sprintf("%s.%s (align=%d, offset=%d)", unsignedType(o.B1), o.Kind, o.U1, o.U2)

	case operationKindLoad8,
		operationKindLoad16:
		return fmt.Sprintf("%s.%s (align=%d, offset=%d)", signedType(o.B1), o.Kind, o.U1, o.U2)

	case operationKindStore8,
		operationKindStore16,
		operationKindStore32:
		return fmt.Sprintf("%s (align=%d, offset=%d)", o.Kind, o.U1, o.U2)

	case operationKindLoad32:
		var t string
		if o.B1 == 1 {
			t = "i64"
		} else {
			t = "u64"
		}
		return fmt.Sprintf("%s.%s (align=%d, offset=%d)", t, o.Kind, o.U1, o.U2)

	case operationKindEq,
		operationKindNe,
		operationKindAdd,
		operationKindSub,
		operationKindMul:
		return fmt.Sprintf("%s.%s", unsignedType(o.B1), o.Kind)

	case operationKindEqz,
		operationKindClz,
		operationKindCtz,
		operationKindPopcnt,
		operationKindAnd,
		operationKindOr,
		operationKindXor,
		operationKindShl,
		operationKindRotl,
		operationKindRotr:
		return fmt.Sprintf("%s.%s", unsignedInt(o.B1), o.Kind)

	case operationKindRem, operationKindShr:
		return fmt.Sprintf("%s.%s", signedInt(o.B1), o.Kind)

	case operationKindLt,
		operationKindGt,
		operationKindLe,
		operationKindGe,
		operationKindDiv:
		return fmt.Sprintf("%s.%s", signedType(o.B1), o.Kind)

	case operationKindAbs,
		operationKindNeg,
		operationKindCeil,
		operationKindFloor,
		operationKindTrunc,
		operationKindNearest,
		operationKindSqrt,
		operationKindMin,
		operationKindMax,
		operationKindCopysign:
		return fmt.Sprintf("%s.%s", float(o.B1), o.Kind)

	case operationKindConstI32,
		operationKindConstI64:
		return fmt.Sprintf("%s %#x", o.Kind, o.U1)

	case operationKindConstF32:
		return fmt.Sprintf("%s %f", o.Kind, math.Float32frombits(uint32(o.U1)))
	case operationKindConstF64:
		return fmt.Sprintf("%s %f", o.Kind, math.Float64frombits(o.U1))

	case operationKindITruncFromF:
		return fmt.Sprintf("%s.%s.%s (non_trapping=%v)", signedInt(o.B2), o.Kind, float(o.B1), o.B3)
	case operationKindFConvertFromI:
		return fmt.Sprintf("%s.%s.%s", float(o.B2), o.Kind, signedInt(o.B1))
	case operationKindExtend:
		var in, out string
		if o.B3 {
			in = "i32"
			out = "i64"
		} else {
			in = "u32"
			out = "u64"
		}
		return fmt.Sprintf("%s.%s.%s", out, o.Kind, in)

	case operationKindV128Const:
		return fmt.Sprintf("%s [%#x, %#x]", o.Kind, o.U1, o.U2)
	case operationKindV128Add,
		operationKindV128Sub:
		return fmt.Sprintf("%s (shape=%s)", o.Kind, shapeName(o.B1))
	case operationKindV128Load,
		operationKindV128LoadLane,
		operationKindV128Store,
		operationKindV128StoreLane,
		operationKindV128ExtractLane,
		operationKindV128ReplaceLane,
		operationKindV128Splat,
		operationKindV128Shuffle,
		operationKindV128Swizzle,
		operationKindV128AnyTrue,
		operationKindV128AllTrue,
		operationKindV128BitMask,
		operationKindV128And,
		operationKindV128Not,
		operationKindV128Or,
		operationKindV128Xor,
		operationKindV128Bitselect,
		operationKindV128AndNot,
		operationKindV128Shl,
		operationKindV128Shr,
		operationKindV128Cmp,
		operationKindV128AddSat,
		operationKindV128SubSat,
		operationKindV128Mul,
		operationKindV128Div,
		operationKindV128Neg,
		operationKindV128Sqrt,
		operationKindV128Abs,
		operationKindV128Popcnt,
		operationKindV128Min,
		operationKindV128Max,
		operationKindV128AvgrU,
		operationKindV128Pmin,
		operationKindV128Pmax,
		operationKindV128Ceil,
		operationKindV128Floor,
		operationKindV128Trunc,
		operationKindV128Nearest,
		operationKindV128Extend,
		operationKindV128ExtMul,
		operationKindV128Q15mulrSatS,
		operationKindV128ExtAddPairwise,
		operationKindV128FloatPromote,
		operationKindV128FloatDemote,
		operationKindV128FConvertFromI,
		operationKindV128Dot,
		operationKindV128Narrow:
		return o.Kind.String()

	case operationKindV128ITruncSatFromF:
		if o.B3 {
			return fmt.Sprintf("%s.%sS", o.Kind, shapeName(o.B1))
		} else {
			return fmt.Sprintf("%s.%sU", o.Kind, shapeName(o.B1))
		}

	case operationKindAtomicMemoryWait,
		operationKindAtomicMemoryNotify,
		operationKindAtomicFence,
		operationKindAtomicLoad,
		operationKindAtomicLoad8,
		operationKindAtomicLoad16,
		operationKindAtomicStore,
		operationKindAtomicStore8,
		operationKindAtomicStore16,
		operationKindAtomicRMW,
		operationKindAtomicRMW8,
		operationKindAtomicRMW16,
		operationKindAtomicRMWCmpxchg,
		operationKindAtomicRMW8Cmpxchg,
		operationKindAtomicRMW16Cmpxchg:
		return o.Kind.String()

	default:
		panic(fmt.Sprintf("TODO: %v", o.Kind))
	}
}

// NewOperationUnreachable is a constructor for unionOperation with operationKindUnreachable
//
// This corresponds to wasm.OpcodeUnreachable.
//
// The engines are expected to exit the execution with wasmruntime.ErrRuntimeUnreachable error.
func newOperationUnreachable() unionOperation {
	return unionOperation{Kind: operationKindUnreachable}
}

// NewOperationLabel is a constructor for unionOperation with operationKindLabel.
//
// This is used to inform the engines of the beginning of a label.
func newOperationLabel(label label) unionOperation {
	return unionOperation{Kind: operationKindLabel, U1: uint64(label)}
}

// NewOperationBr is a constructor for unionOperation with operationKindBr.
//
// The engines are expected to branch into U1 label.
func newOperationBr(target label) unionOperation {
	return unionOperation{Kind: operationKindBr, U1: uint64(target)}
}

// NewOperationBrIf is a constructor for unionOperation with operationKindBrIf.
//
// The engines are expected to pop a value and branch into U1 label if the value equals 1.
// Otherwise, the code branches into U2 label.
func newOperationBrIf(thenTarget, elseTarget label, thenDrop inclusiveRange) unionOperation {
	return unionOperation{
		Kind: operationKindBrIf,
		U1:   uint64(thenTarget),
		U2:   uint64(elseTarget),
		U3:   thenDrop.AsU64(),
	}
}

// NewOperationBrTable is a constructor for unionOperation with operationKindBrTable.
//
// This corresponds to wasm.OpcodeBrTableName except that the label
// here means the interpreterir level, not the ones of Wasm.
//
// The engines are expected to do the br_table operation based on the default (Us[len(Us)-1], Us[len(Us)-2]) and
// targets (Us[:len(Us)-1], Rs[:len(Us)-1]). More precisely, this pops a value from the stack (called "index")
// and decides which branch we go into next based on the value.
//
// For example, assume we have operations like {default: L_DEFAULT, targets: [L0, L1, L2]}.
// If "index" >= len(defaults), then branch into the L_DEFAULT label.
// Otherwise, we enter label of targets[index].
func newOperationBrTable(targetLabelsAndRanges []uint64) unionOperation {
	return unionOperation{
		Kind: operationKindBrTable,
		Us:   targetLabelsAndRanges,
	}
}

// NewOperationCall is a constructor for unionOperation with operationKindCall.
//
// This corresponds to wasm.OpcodeCallName, and engines are expected to
// enter into a function whose index equals OperationCall.FunctionIndex.
func newOperationCall(functionIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindCall, U1: uint64(functionIndex)}
}

// NewOperationCallIndirect implements Operation.
//
// This corresponds to wasm.OpcodeCallIndirectName, and engines are expected to
// consume the one value from the top of stack (called "offset"),
// and make a function call against the function whose function address equals
// Tables[OperationCallIndirect.TableIndex][offset].
//
// Note: This is called indirect function call in the sense that the target function is indirectly
// determined by the current state (top value) of the stack.
// Therefore, two checks are performed at runtime before entering the target function:
// 1) whether "offset" exceeds the length of table Tables[OperationCallIndirect.TableIndex].
// 2) whether the type of the function table[offset] matches the function type specified by OperationCallIndirect.TypeIndex.
func newOperationCallIndirect(typeIndex, tableIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindCallIndirect, U1: uint64(typeIndex), U2: uint64(tableIndex)}
}

// inclusiveRange is the range which spans across the value stack starting from the top to the bottom, and
// both boundary are included in the range.
type inclusiveRange struct {
	Start, End int32
}

// AsU64 is be used to convert inclusiveRange to uint64 so that it can be stored in unionOperation.
func (i inclusiveRange) AsU64() uint64 {
	return uint64(uint32(i.Start))<<32 | uint64(uint32(i.End))
}

// inclusiveRangeFromU64 retrieves inclusiveRange from the given uint64 which is stored in unionOperation.
func inclusiveRangeFromU64(v uint64) inclusiveRange {
	return inclusiveRange{
		Start: int32(uint32(v >> 32)),
		End:   int32(uint32(v)),
	}
}

// nopinclusiveRange is inclusiveRange which corresponds to no-operation.
var nopinclusiveRange = inclusiveRange{Start: -1, End: -1}

// NewOperationDrop is a constructor for unionOperation with operationKindDrop.
//
// The engines are expected to discard the values selected by NewOperationDrop.Depth which
// starts from the top of the stack to the bottom.
//
// depth spans across the uint64 value stack at runtime to be dropped by this operation.
func newOperationDrop(depth inclusiveRange) unionOperation {
	return unionOperation{Kind: operationKindDrop, U1: depth.AsU64()}
}

// NewOperationSelect is a constructor for unionOperation with operationKindSelect.
//
// This corresponds to wasm.OpcodeSelect.
//
// The engines are expected to pop three values, say [..., x2, x1, c], then if the value "c" equals zero,
// "x1" is pushed back onto the stack and, otherwise "x2" is pushed back.
//
// isTargetVector true if the selection target value's type is wasm.ValueTypeV128.
func newOperationSelect(isTargetVector bool) unionOperation {
	return unionOperation{Kind: operationKindSelect, B3: isTargetVector}
}

// NewOperationPick is a constructor for unionOperation with operationKindPick.
//
// The engines are expected to copy a value pointed by depth, and push the
// copied value onto the top of the stack.
//
// depth is the location of the pick target in the uint64 value stack at runtime.
// If isTargetVector=true, this points to the location of the lower 64-bits of the vector.
func newOperationPick(depth int, isTargetVector bool) unionOperation {
	return unionOperation{Kind: operationKindPick, U1: uint64(depth), B3: isTargetVector}
}

// NewOperationSet is a constructor for unionOperation with operationKindSet.
//
// The engines are expected to set the top value of the stack to the location specified by
// depth.
//
// depth is the location of the set target in the uint64 value stack at runtime.
// If isTargetVector=true, this points the location of the lower 64-bits of the vector.
func newOperationSet(depth int, isTargetVector bool) unionOperation {
	return unionOperation{Kind: operationKindSet, U1: uint64(depth), B3: isTargetVector}
}

// NewOperationGlobalGet is a constructor for unionOperation with operationKindGlobalGet.
//
// The engines are expected to read the global value specified by OperationGlobalGet.Index,
// and push the copy of the value onto the stack.
//
// See wasm.OpcodeGlobalGet.
func newOperationGlobalGet(index uint32) unionOperation {
	return unionOperation{Kind: operationKindGlobalGet, U1: uint64(index)}
}

// NewOperationGlobalSet is a constructor for unionOperation with operationKindGlobalSet.
//
// The engines are expected to consume the value from the top of the stack,
// and write the value into the global specified by OperationGlobalSet.Index.
//
// See wasm.OpcodeGlobalSet.
func newOperationGlobalSet(index uint32) unionOperation {
	return unionOperation{Kind: operationKindGlobalSet, U1: uint64(index)}
}

// memoryArg is the "memarg" to all memory instructions.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#memory-instructions%E2%91%A0
type memoryArg struct {
	// Alignment the expected alignment (expressed as the exponent of a power of 2). Default to the natural alignment.
	//
	// "Natural alignment" is defined here as the smallest power of two that can hold the size of the value type. Ex
	// wasm.ValueTypeI64 is encoded in 8 little-endian bytes. 2^3 = 8, so the natural alignment is three.
	Alignment uint32

	// Offset is the address offset added to the instruction's dynamic address operand, yielding a 33-bit effective
	// address that is the zero-based index at which the memory is accessed. Default to zero.
	Offset uint32
}

// NewOperationLoad is a constructor for unionOperation with operationKindLoad.
//
// This corresponds to wasm.OpcodeI32LoadName wasm.OpcodeI64LoadName wasm.OpcodeF32LoadName and wasm.OpcodeF64LoadName.
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise load the corresponding value following the semantics of the corresponding WebAssembly instruction.
func newOperationLoad(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindLoad, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationLoad8 is a constructor for unionOperation with operationKindLoad8.
//
// This corresponds to wasm.OpcodeI32Load8SName wasm.OpcodeI32Load8UName wasm.OpcodeI64Load8SName wasm.OpcodeI64Load8UName.
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise load the corresponding value following the semantics of the corresponding WebAssembly instruction.
func newOperationLoad8(signedInt signedInt, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindLoad8, B1: byte(signedInt), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationLoad16 is a constructor for unionOperation with operationKindLoad16.
//
// This corresponds to wasm.OpcodeI32Load16SName wasm.OpcodeI32Load16UName wasm.OpcodeI64Load16SName wasm.OpcodeI64Load16UName.
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise load the corresponding value following the semantics of the corresponding WebAssembly instruction.
func newOperationLoad16(signedInt signedInt, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindLoad16, B1: byte(signedInt), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationLoad32 is a constructor for unionOperation with operationKindLoad32.
//
// This corresponds to wasm.OpcodeI64Load32SName wasm.OpcodeI64Load32UName.
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise load the corresponding value following the semantics of the corresponding WebAssembly instruction.
func newOperationLoad32(signed bool, arg memoryArg) unionOperation {
	sigB := byte(0)
	if signed {
		sigB = 1
	}
	return unionOperation{Kind: operationKindLoad32, B1: sigB, U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationStore is a constructor for unionOperation with operationKindStore.
//
// # This corresponds to wasm.OpcodeI32StoreName wasm.OpcodeI64StoreName wasm.OpcodeF32StoreName wasm.OpcodeF64StoreName
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise store the corresponding value following the semantics of the corresponding WebAssembly instruction.
func newOperationStore(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindStore, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationStore8 is a constructor for unionOperation with operationKindStore8.
//
// # This corresponds to wasm.OpcodeI32Store8Name wasm.OpcodeI64Store8Name
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise store the corresponding value following the semantics of the corresponding WebAssembly instruction.
func newOperationStore8(arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindStore8, U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationStore16 is a constructor for unionOperation with operationKindStore16.
//
// # This corresponds to wasm.OpcodeI32Store16Name wasm.OpcodeI64Store16Name
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise store the corresponding value following the semantics of the corresponding WebAssembly instruction.
func newOperationStore16(arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindStore16, U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationStore32 is a constructor for unionOperation with operationKindStore32.
//
// # This corresponds to wasm.OpcodeI64Store32Name
//
// The engines are expected to check the boundary of memory length, and exit the execution if this exceeds the boundary,
// otherwise store the corresponding value following the semantics of the corresponding WebAssembly instruction.
func newOperationStore32(arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindStore32, U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationMemorySize is a constructor for unionOperation with operationKindMemorySize.
//
// This corresponds to wasm.OpcodeMemorySize.
//
// The engines are expected to push the current page size of the memory onto the stack.
func newOperationMemorySize() unionOperation {
	return unionOperation{Kind: operationKindMemorySize}
}

// NewOperationMemoryGrow is a constructor for unionOperation with operationKindMemoryGrow.
//
// This corresponds to wasm.OpcodeMemoryGrow.
//
// The engines are expected to pop one value from the top of the stack, then
// execute wasm.MemoryInstance Grow with the value, and push the previous
// page size of the memory onto the stack.
func newOperationMemoryGrow() unionOperation {
	return unionOperation{Kind: operationKindMemoryGrow}
}

// NewOperationConstI32 is a constructor for unionOperation with OperationConstI32.
//
// This corresponds to wasm.OpcodeI32Const.
func newOperationConstI32(value uint32) unionOperation {
	return unionOperation{Kind: operationKindConstI32, U1: uint64(value)}
}

// NewOperationConstI64 is a constructor for unionOperation with OperationConstI64.
//
// This corresponds to wasm.OpcodeI64Const.
func newOperationConstI64(value uint64) unionOperation {
	return unionOperation{Kind: operationKindConstI64, U1: value}
}

// NewOperationConstF32 is a constructor for unionOperation with OperationConstF32.
//
// This corresponds to wasm.OpcodeF32Const.
func newOperationConstF32(value float32) unionOperation {
	return unionOperation{Kind: operationKindConstF32, U1: uint64(math.Float32bits(value))}
}

// NewOperationConstF64 is a constructor for unionOperation with OperationConstF64.
//
// This corresponds to wasm.OpcodeF64Const.
func newOperationConstF64(value float64) unionOperation {
	return unionOperation{Kind: operationKindConstF64, U1: math.Float64bits(value)}
}

// NewOperationEq is a constructor for unionOperation with operationKindEq.
//
// This corresponds to wasm.OpcodeI32EqName wasm.OpcodeI64EqName wasm.OpcodeF32EqName wasm.OpcodeF64EqName
func newOperationEq(b unsignedType) unionOperation {
	return unionOperation{Kind: operationKindEq, B1: byte(b)}
}

// NewOperationNe is a constructor for unionOperation with operationKindNe.
//
// This corresponds to wasm.OpcodeI32NeName wasm.OpcodeI64NeName wasm.OpcodeF32NeName wasm.OpcodeF64NeName
func newOperationNe(b unsignedType) unionOperation {
	return unionOperation{Kind: operationKindNe, B1: byte(b)}
}

// NewOperationEqz is a constructor for unionOperation with operationKindEqz.
//
// This corresponds to wasm.OpcodeI32EqzName wasm.OpcodeI64EqzName
func newOperationEqz(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindEqz, B1: byte(b)}
}

// NewOperationLt is a constructor for unionOperation with operationKindLt.
//
// This corresponds to wasm.OpcodeI32LtS wasm.OpcodeI32LtU wasm.OpcodeI64LtS wasm.OpcodeI64LtU wasm.OpcodeF32Lt wasm.OpcodeF64Lt
func newOperationLt(b signedType) unionOperation {
	return unionOperation{Kind: operationKindLt, B1: byte(b)}
}

// NewOperationGt is a constructor for unionOperation with operationKindGt.
//
// This corresponds to wasm.OpcodeI32GtS wasm.OpcodeI32GtU wasm.OpcodeI64GtS wasm.OpcodeI64GtU wasm.OpcodeF32Gt wasm.OpcodeF64Gt
func newOperationGt(b signedType) unionOperation {
	return unionOperation{Kind: operationKindGt, B1: byte(b)}
}

// NewOperationLe is a constructor for unionOperation with operationKindLe.
//
// This corresponds to wasm.OpcodeI32LeS wasm.OpcodeI32LeU wasm.OpcodeI64LeS wasm.OpcodeI64LeU wasm.OpcodeF32Le wasm.OpcodeF64Le
func newOperationLe(b signedType) unionOperation {
	return unionOperation{Kind: operationKindLe, B1: byte(b)}
}

// NewOperationGe is a constructor for unionOperation with operationKindGe.
//
// This corresponds to wasm.OpcodeI32GeS wasm.OpcodeI32GeU wasm.OpcodeI64GeS wasm.OpcodeI64GeU wasm.OpcodeF32Ge wasm.OpcodeF64Ge
// NewOperationGe is the constructor for OperationGe
func newOperationGe(b signedType) unionOperation {
	return unionOperation{Kind: operationKindGe, B1: byte(b)}
}

// NewOperationAdd is a constructor for unionOperation with operationKindAdd.
//
// This corresponds to wasm.OpcodeI32AddName wasm.OpcodeI64AddName wasm.OpcodeF32AddName wasm.OpcodeF64AddName.
func newOperationAdd(b unsignedType) unionOperation {
	return unionOperation{Kind: operationKindAdd, B1: byte(b)}
}

// NewOperationSub is a constructor for unionOperation with operationKindSub.
//
// This corresponds to wasm.OpcodeI32SubName wasm.OpcodeI64SubName wasm.OpcodeF32SubName wasm.OpcodeF64SubName.
func newOperationSub(b unsignedType) unionOperation {
	return unionOperation{Kind: operationKindSub, B1: byte(b)}
}

// NewOperationMul is a constructor for unionOperation with wperationKindMul.
//
// This corresponds to wasm.OpcodeI32MulName wasm.OpcodeI64MulName wasm.OpcodeF32MulName wasm.OpcodeF64MulName.
// NewOperationMul is the constructor for OperationMul
func newOperationMul(b unsignedType) unionOperation {
	return unionOperation{Kind: operationKindMul, B1: byte(b)}
}

// NewOperationClz is a constructor for unionOperation with operationKindClz.
//
// This corresponds to wasm.OpcodeI32ClzName wasm.OpcodeI64ClzName.
//
// The engines are expected to count up the leading zeros in the
// current top of the stack, and push the count result.
// For example, stack of [..., 0x00_ff_ff_ff] results in [..., 8].
// See wasm.OpcodeI32Clz wasm.OpcodeI64Clz
func newOperationClz(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindClz, B1: byte(b)}
}

// NewOperationCtz is a constructor for unionOperation with operationKindCtz.
//
// This corresponds to wasm.OpcodeI32CtzName wasm.OpcodeI64CtzName.
//
// The engines are expected to count up the trailing zeros in the
// current top of the stack, and push the count result.
// For example, stack of [..., 0xff_ff_ff_00] results in [..., 8].
func newOperationCtz(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindCtz, B1: byte(b)}
}

// NewOperationPopcnt is a constructor for unionOperation with operationKindPopcnt.
//
// This corresponds to wasm.OpcodeI32PopcntName wasm.OpcodeI64PopcntName.
//
// The engines are expected to count up the number of set bits in the
// current top of the stack, and push the count result.
// For example, stack of [..., 0b00_00_00_11] results in [..., 2].
func newOperationPopcnt(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindPopcnt, B1: byte(b)}
}

// NewOperationDiv is a constructor for unionOperation with operationKindDiv.
//
// This corresponds to wasm.OpcodeI32DivS wasm.OpcodeI32DivU wasm.OpcodeI64DivS
//
//	wasm.OpcodeI64DivU wasm.OpcodeF32Div wasm.OpcodeF64Div.
func newOperationDiv(b signedType) unionOperation {
	return unionOperation{Kind: operationKindDiv, B1: byte(b)}
}

// NewOperationRem is a constructor for unionOperation with operationKindRem.
//
// This corresponds to wasm.OpcodeI32RemS wasm.OpcodeI32RemU wasm.OpcodeI64RemS wasm.OpcodeI64RemU.
//
// The engines are expected to perform division on the top
// two values of integer type on the stack and puts the remainder of the result
// onto the stack. For example, stack [..., 10, 3] results in [..., 1] where
// the quotient is discarded.
// NewOperationRem is the constructor for OperationRem
func newOperationRem(b signedInt) unionOperation {
	return unionOperation{Kind: operationKindRem, B1: byte(b)}
}

// NewOperationAnd is a constructor for unionOperation with operationKindAnd.
//
// # This corresponds to wasm.OpcodeI32AndName wasm.OpcodeI64AndName
//
// The engines are expected to perform "And" operation on
// top two values on the stack, and pushes the result.
func newOperationAnd(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindAnd, B1: byte(b)}
}

// NewOperationOr is a constructor for unionOperation with operationKindOr.
//
// # This corresponds to wasm.OpcodeI32OrName wasm.OpcodeI64OrName
//
// The engines are expected to perform "Or" operation on
// top two values on the stack, and pushes the result.
func newOperationOr(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindOr, B1: byte(b)}
}

// NewOperationXor is a constructor for unionOperation with operationKindXor.
//
// # This corresponds to wasm.OpcodeI32XorName wasm.OpcodeI64XorName
//
// The engines are expected to perform "Xor" operation on
// top two values on the stack, and pushes the result.
func newOperationXor(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindXor, B1: byte(b)}
}

// NewOperationShl is a constructor for unionOperation with operationKindShl.
//
// # This corresponds to wasm.OpcodeI32ShlName wasm.OpcodeI64ShlName
//
// The engines are expected to perform "Shl" operation on
// top two values on the stack, and pushes the result.
func newOperationShl(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindShl, B1: byte(b)}
}

// NewOperationShr is a constructor for unionOperation with operationKindShr.
//
// # This corresponds to wasm.OpcodeI32ShrSName wasm.OpcodeI32ShrUName wasm.OpcodeI64ShrSName wasm.OpcodeI64ShrUName
//
// If OperationShr.Type is signed integer, then, the engines are expected to perform arithmetic right shift on the two
// top values on the stack, otherwise do the logical right shift.
func newOperationShr(b signedInt) unionOperation {
	return unionOperation{Kind: operationKindShr, B1: byte(b)}
}

// NewOperationRotl is a constructor for unionOperation with operationKindRotl.
//
// # This corresponds to wasm.OpcodeI32RotlName wasm.OpcodeI64RotlName
//
// The engines are expected to perform "Rotl" operation on
// top two values on the stack, and pushes the result.
func newOperationRotl(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindRotl, B1: byte(b)}
}

// NewOperationRotr is a constructor for unionOperation with operationKindRotr.
//
// # This corresponds to wasm.OpcodeI32RotrName wasm.OpcodeI64RotrName
//
// The engines are expected to perform "Rotr" operation on
// top two values on the stack, and pushes the result.
func newOperationRotr(b unsignedInt) unionOperation {
	return unionOperation{Kind: operationKindRotr, B1: byte(b)}
}

// NewOperationAbs is a constructor for unionOperation with operationKindAbs.
//
// This corresponds to wasm.OpcodeF32Abs wasm.OpcodeF64Abs
func newOperationAbs(b float) unionOperation {
	return unionOperation{Kind: operationKindAbs, B1: byte(b)}
}

// NewOperationNeg is a constructor for unionOperation with operationKindNeg.
//
// This corresponds to wasm.OpcodeF32Neg wasm.OpcodeF64Neg
func newOperationNeg(b float) unionOperation {
	return unionOperation{Kind: operationKindNeg, B1: byte(b)}
}

// NewOperationCeil is a constructor for unionOperation with operationKindCeil.
//
// This corresponds to wasm.OpcodeF32CeilName wasm.OpcodeF64CeilName
func newOperationCeil(b float) unionOperation {
	return unionOperation{Kind: operationKindCeil, B1: byte(b)}
}

// NewOperationFloor is a constructor for unionOperation with operationKindFloor.
//
// This corresponds to wasm.OpcodeF32FloorName wasm.OpcodeF64FloorName
func newOperationFloor(b float) unionOperation {
	return unionOperation{Kind: operationKindFloor, B1: byte(b)}
}

// NewOperationTrunc is a constructor for unionOperation with operationKindTrunc.
//
// This corresponds to wasm.OpcodeF32TruncName wasm.OpcodeF64TruncName
func newOperationTrunc(b float) unionOperation {
	return unionOperation{Kind: operationKindTrunc, B1: byte(b)}
}

// NewOperationNearest is a constructor for unionOperation with operationKindNearest.
//
// # This corresponds to wasm.OpcodeF32NearestName wasm.OpcodeF64NearestName
//
// Note: this is *not* equivalent to math.Round and instead has the same
// the semantics of LLVM's rint intrinsic. See https://llvm.org/docs/LangRef.html#llvm-rint-intrinsic.
// For example, math.Round(-4.5) produces -5 while we want to produce -4.
func newOperationNearest(b float) unionOperation {
	return unionOperation{Kind: operationKindNearest, B1: byte(b)}
}

// NewOperationSqrt is a constructor for unionOperation with operationKindSqrt.
//
// This corresponds to wasm.OpcodeF32SqrtName wasm.OpcodeF64SqrtName
func newOperationSqrt(b float) unionOperation {
	return unionOperation{Kind: operationKindSqrt, B1: byte(b)}
}

// NewOperationMin is a constructor for unionOperation with operationKindMin.
//
// # This corresponds to wasm.OpcodeF32MinName wasm.OpcodeF64MinName
//
// The engines are expected to pop two values from the stack, and push back the maximum of
// these two values onto the stack. For example, stack [..., 100.1, 1.9] results in [..., 1.9].
//
// Note: WebAssembly specifies that min/max must always return NaN if one of values is NaN,
// which is a different behavior different from math.Min.
func newOperationMin(b float) unionOperation {
	return unionOperation{Kind: operationKindMin, B1: byte(b)}
}

// NewOperationMax is a constructor for unionOperation with operationKindMax.
//
// # This corresponds to wasm.OpcodeF32MaxName wasm.OpcodeF64MaxName
//
// The engines are expected to pop two values from the stack, and push back the maximum of
// these two values onto the stack. For example, stack [..., 100.1, 1.9] results in [..., 100.1].
//
// Note: WebAssembly specifies that min/max must always return NaN if one of values is NaN,
// which is a different behavior different from math.Max.
func newOperationMax(b float) unionOperation {
	return unionOperation{Kind: operationKindMax, B1: byte(b)}
}

// NewOperationCopysign is a constructor for unionOperation with operationKindCopysign.
//
// # This corresponds to wasm.OpcodeF32CopysignName wasm.OpcodeF64CopysignName
//
// The engines are expected to pop two float values from the stack, and copy the signbit of
// the first-popped value to the last one.
// For example, stack [..., 1.213, -5.0] results in [..., -1.213].
func newOperationCopysign(b float) unionOperation {
	return unionOperation{Kind: operationKindCopysign, B1: byte(b)}
}

// NewOperationI32WrapFromI64 is a constructor for unionOperation with operationKindI32WrapFromI64.
//
// This corresponds to wasm.OpcodeI32WrapI64 and equivalent to uint64(uint32(v)) in Go.
//
// The engines are expected to replace the 64-bit int on top of the stack
// with the corresponding 32-bit integer.
func newOperationI32WrapFromI64() unionOperation {
	return unionOperation{Kind: operationKindI32WrapFromI64}
}

// NewOperationITruncFromF is a constructor for unionOperation with operationKindITruncFromF.
//
// This corresponds to
//
//	wasm.OpcodeI32TruncF32SName wasm.OpcodeI32TruncF32UName wasm.OpcodeI32TruncF64SName
//	wasm.OpcodeI32TruncF64UName wasm.OpcodeI64TruncF32SName wasm.OpcodeI64TruncF32UName wasm.OpcodeI64TruncF64SName
//	wasm.OpcodeI64TruncF64UName. wasm.OpcodeI32TruncSatF32SName wasm.OpcodeI32TruncSatF32UName
//	wasm.OpcodeI32TruncSatF64SName wasm.OpcodeI32TruncSatF64UName wasm.OpcodeI64TruncSatF32SName
//	wasm.OpcodeI64TruncSatF32UName wasm.OpcodeI64TruncSatF64SName wasm.OpcodeI64TruncSatF64UName
//
// See [1] and [2] for when we encounter undefined behavior in the WebAssembly specification if NewOperationITruncFromF.NonTrapping == false.
// To summarize, if the source float value is NaN or doesn't fit in the destination range of integers (incl. +=Inf),
// then the runtime behavior is undefined. In wazero, the engines are expected to exit the execution in these undefined cases with
// wasmruntime.ErrRuntimeInvalidConversionToInteger error.
//
// [1] https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#-hrefop-trunc-umathrmtruncmathsfu_m-n-z for unsigned integers.
// [2] https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#-hrefop-trunc-smathrmtruncmathsfs_m-n-z for signed integers.
//
// nonTrapping true if this conversion is "nontrapping" in the sense of the
// https://github.com/WebAssembly/spec/blob/ce4b6c4d47eb06098cc7ab2e81f24748da822f20/proposals/nontrapping-float-to-int-conversion/Overview.md
func newOperationITruncFromF(inputType float, outputType signedInt, nonTrapping bool) unionOperation {
	return unionOperation{
		Kind: operationKindITruncFromF,
		B1:   byte(inputType),
		B2:   byte(outputType),
		B3:   nonTrapping,
	}
}

// NewOperationFConvertFromI is a constructor for unionOperation with operationKindFConvertFromI.
//
// This corresponds to
//
//	wasm.OpcodeF32ConvertI32SName wasm.OpcodeF32ConvertI32UName wasm.OpcodeF32ConvertI64SName wasm.OpcodeF32ConvertI64UName
//	wasm.OpcodeF64ConvertI32SName wasm.OpcodeF64ConvertI32UName wasm.OpcodeF64ConvertI64SName wasm.OpcodeF64ConvertI64UName
//
// and equivalent to float32(uint32(x)), float32(int32(x)), etc in Go.
func newOperationFConvertFromI(inputType signedInt, outputType float) unionOperation {
	return unionOperation{
		Kind: operationKindFConvertFromI,
		B1:   byte(inputType),
		B2:   byte(outputType),
	}
}

// NewOperationF32DemoteFromF64 is a constructor for unionOperation with operationKindF32DemoteFromF64.
//
// This corresponds to wasm.OpcodeF32DemoteF64 and is equivalent float32(float64(v)).
func newOperationF32DemoteFromF64() unionOperation {
	return unionOperation{Kind: operationKindF32DemoteFromF64}
}

// NewOperationF64PromoteFromF32 is a constructor for unionOperation with operationKindF64PromoteFromF32.
//
// This corresponds to wasm.OpcodeF64PromoteF32 and is equivalent float64(float32(v)).
func newOperationF64PromoteFromF32() unionOperation {
	return unionOperation{Kind: operationKindF64PromoteFromF32}
}

// NewOperationI32ReinterpretFromF32 is a constructor for unionOperation with operationKindI32ReinterpretFromF32.
//
// This corresponds to wasm.OpcodeI32ReinterpretF32Name.
func newOperationI32ReinterpretFromF32() unionOperation {
	return unionOperation{Kind: operationKindI32ReinterpretFromF32}
}

// NewOperationI64ReinterpretFromF64 is a constructor for unionOperation with operationKindI64ReinterpretFromF64.
//
// This corresponds to wasm.OpcodeI64ReinterpretF64Name.
func newOperationI64ReinterpretFromF64() unionOperation {
	return unionOperation{Kind: operationKindI64ReinterpretFromF64}
}

// NewOperationF32ReinterpretFromI32 is a constructor for unionOperation with operationKindF32ReinterpretFromI32.
//
// This corresponds to wasm.OpcodeF32ReinterpretI32Name.
func newOperationF32ReinterpretFromI32() unionOperation {
	return unionOperation{Kind: operationKindF32ReinterpretFromI32}
}

// NewOperationF64ReinterpretFromI64 is a constructor for unionOperation with operationKindF64ReinterpretFromI64.
//
// This corresponds to wasm.OpcodeF64ReinterpretI64Name.
func newOperationF64ReinterpretFromI64() unionOperation {
	return unionOperation{Kind: operationKindF64ReinterpretFromI64}
}

// NewOperationExtend is a constructor for unionOperation with operationKindExtend.
//
// # This corresponds to wasm.OpcodeI64ExtendI32SName wasm.OpcodeI64ExtendI32UName
//
// The engines are expected to extend the 32-bit signed or unsigned int on top of the stack
// as a 64-bit integer of corresponding signedness. For unsigned case, this is just reinterpreting the
// underlying bit pattern as 64-bit integer. For signed case, this is sign-extension which preserves the
// original integer's sign.
func newOperationExtend(signed bool) unionOperation {
	op := unionOperation{Kind: operationKindExtend}
	if signed {
		op.B1 = 1
	}
	return op
}

// NewOperationSignExtend32From8 is a constructor for unionOperation with operationKindSignExtend32From8.
//
// This corresponds to wasm.OpcodeI32Extend8SName.
//
// The engines are expected to sign-extend the first 8-bits of 32-bit in as signed 32-bit int.
func newOperationSignExtend32From8() unionOperation {
	return unionOperation{Kind: operationKindSignExtend32From8}
}

// NewOperationSignExtend32From16 is a constructor for unionOperation with operationKindSignExtend32From16.
//
// This corresponds to wasm.OpcodeI32Extend16SName.
//
// The engines are expected to sign-extend the first 16-bits of 32-bit in as signed 32-bit int.
func newOperationSignExtend32From16() unionOperation {
	return unionOperation{Kind: operationKindSignExtend32From16}
}

// NewOperationSignExtend64From8 is a constructor for unionOperation with operationKindSignExtend64From8.
//
// This corresponds to wasm.OpcodeI64Extend8SName.
//
// The engines are expected to sign-extend the first 8-bits of 64-bit in as signed 32-bit int.
func newOperationSignExtend64From8() unionOperation {
	return unionOperation{Kind: operationKindSignExtend64From8}
}

// NewOperationSignExtend64From16 is a constructor for unionOperation with operationKindSignExtend64From16.
//
// This corresponds to wasm.OpcodeI64Extend16SName.
//
// The engines are expected to sign-extend the first 16-bits of 64-bit in as signed 32-bit int.
func newOperationSignExtend64From16() unionOperation {
	return unionOperation{Kind: operationKindSignExtend64From16}
}

// NewOperationSignExtend64From32 is a constructor for unionOperation with operationKindSignExtend64From32.
//
// This corresponds to wasm.OpcodeI64Extend32SName.
//
// The engines are expected to sign-extend the first 32-bits of 64-bit in as signed 32-bit int.
func newOperationSignExtend64From32() unionOperation {
	return unionOperation{Kind: operationKindSignExtend64From32}
}

// NewOperationMemoryInit is a constructor for unionOperation with operationKindMemoryInit.
//
// This corresponds to wasm.OpcodeMemoryInitName.
//
// dataIndex is the index of the data instance in ModuleInstance.DataInstances
// by which this operation instantiates a part of the memory.
func newOperationMemoryInit(dataIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindMemoryInit, U1: uint64(dataIndex)}
}

// NewOperationDataDrop implements Operation.
//
// This corresponds to wasm.OpcodeDataDropName.
//
// dataIndex is the index of the data instance in ModuleInstance.DataInstances
// which this operation drops.
func newOperationDataDrop(dataIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindDataDrop, U1: uint64(dataIndex)}
}

// NewOperationMemoryCopy is a consuctor for unionOperation with operationKindMemoryCopy.
//
// This corresponds to wasm.OpcodeMemoryCopyName.
func newOperationMemoryCopy() unionOperation {
	return unionOperation{Kind: operationKindMemoryCopy}
}

// NewOperationMemoryFill is a consuctor for unionOperation with operationKindMemoryFill.
func newOperationMemoryFill() unionOperation {
	return unionOperation{Kind: operationKindMemoryFill}
}

// NewOperationTableInit is a constructor for unionOperation with operationKindTableInit.
//
// This corresponds to wasm.OpcodeTableInitName.
//
// elemIndex is the index of the element by which this operation initializes a part of the table.
// tableIndex is the index of the table on which this operation initialize by the target element.
func newOperationTableInit(elemIndex, tableIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindTableInit, U1: uint64(elemIndex), U2: uint64(tableIndex)}
}

// NewOperationElemDrop is a constructor for unionOperation with operationKindElemDrop.
//
// This corresponds to wasm.OpcodeElemDropName.
//
// elemIndex is the index of the element which this operation drops.
func newOperationElemDrop(elemIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindElemDrop, U1: uint64(elemIndex)}
}

// NewOperationTableCopy implements Operation.
//
// This corresponds to wasm.OpcodeTableCopyName.
func newOperationTableCopy(srcTableIndex, dstTableIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindTableCopy, U1: uint64(srcTableIndex), U2: uint64(dstTableIndex)}
}

// NewOperationRefFunc constructor for unionOperation with operationKindRefFunc.
//
// This corresponds to wasm.OpcodeRefFuncName, and engines are expected to
// push the opaque pointer value of engine specific func for the given FunctionIndex.
//
// Note: in wazero, we express any reference types (funcref or externref) as opaque pointers which is uint64.
// Therefore, the engine implementations emit instructions to push the address of *function onto the stack.
func newOperationRefFunc(functionIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindRefFunc, U1: uint64(functionIndex)}
}

// NewOperationTableGet constructor for unionOperation with operationKindTableGet.
//
// This corresponds to wasm.OpcodeTableGetName.
func newOperationTableGet(tableIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindTableGet, U1: uint64(tableIndex)}
}

// NewOperationTableSet constructor for unionOperation with operationKindTableSet.
//
// This corresponds to wasm.OpcodeTableSetName.
func newOperationTableSet(tableIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindTableSet, U1: uint64(tableIndex)}
}

// NewOperationTableSize constructor for unionOperation with operationKindTableSize.
//
// This corresponds to wasm.OpcodeTableSizeName.
func newOperationTableSize(tableIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindTableSize, U1: uint64(tableIndex)}
}

// NewOperationTableGrow constructor for unionOperation with operationKindTableGrow.
//
// This corresponds to wasm.OpcodeTableGrowName.
func newOperationTableGrow(tableIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindTableGrow, U1: uint64(tableIndex)}
}

// NewOperationTableFill constructor for unionOperation with operationKindTableFill.
//
// This corresponds to wasm.OpcodeTableFillName.
func newOperationTableFill(tableIndex uint32) unionOperation {
	return unionOperation{Kind: operationKindTableFill, U1: uint64(tableIndex)}
}

// NewOperationV128Const constructor for unionOperation with operationKindV128Const
func newOperationV128Const(lo, hi uint64) unionOperation {
	return unionOperation{Kind: operationKindV128Const, U1: lo, U2: hi}
}

// shape corresponds to a shape of v128 values.
// https://webassembly.github.io/spec/core/syntax/instructions.html#syntax-shape
type shape = byte

const (
	shapeI8x16 shape = iota
	shapeI16x8
	shapeI32x4
	shapeI64x2
	shapeF32x4
	shapeF64x2
)

func shapeName(s shape) (ret string) {
	switch s {
	case shapeI8x16:
		ret = "I8x16"
	case shapeI16x8:
		ret = "I16x8"
	case shapeI32x4:
		ret = "I32x4"
	case shapeI64x2:
		ret = "I64x2"
	case shapeF32x4:
		ret = "F32x4"
	case shapeF64x2:
		ret = "F64x2"
	}
	return
}

// NewOperationV128Add constructor for unionOperation with operationKindV128Add.
//
// This corresponds to wasm.OpcodeVecI8x16AddName wasm.OpcodeVecI16x8AddName wasm.OpcodeVecI32x4AddName
//
//	wasm.OpcodeVecI64x2AddName wasm.OpcodeVecF32x4AddName wasm.OpcodeVecF64x2AddName
func newOperationV128Add(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Add, B1: shape}
}

// NewOperationV128Sub constructor for unionOperation with operationKindV128Sub.
//
// This corresponds to wasm.OpcodeVecI8x16SubName wasm.OpcodeVecI16x8SubName wasm.OpcodeVecI32x4SubName
//
//	wasm.OpcodeVecI64x2SubName wasm.OpcodeVecF32x4SubName wasm.OpcodeVecF64x2SubName
func newOperationV128Sub(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Sub, B1: shape}
}

// v128LoadType represents a type of wasm.OpcodeVecV128Load* instructions.
type v128LoadType = byte

const (
	// v128LoadType128 corresponds to wasm.OpcodeVecV128LoadName.
	v128LoadType128 v128LoadType = iota
	// v128LoadType8x8s corresponds to wasm.OpcodeVecV128Load8x8SName.
	v128LoadType8x8s
	// v128LoadType8x8u corresponds to wasm.OpcodeVecV128Load8x8UName.
	v128LoadType8x8u
	// v128LoadType16x4s corresponds to wasm.OpcodeVecV128Load16x4SName
	v128LoadType16x4s
	// v128LoadType16x4u corresponds to wasm.OpcodeVecV128Load16x4UName
	v128LoadType16x4u
	// v128LoadType32x2s corresponds to wasm.OpcodeVecV128Load32x2SName
	v128LoadType32x2s
	// v128LoadType32x2u corresponds to wasm.OpcodeVecV128Load32x2UName
	v128LoadType32x2u
	// v128LoadType8Splat corresponds to wasm.OpcodeVecV128Load8SplatName
	v128LoadType8Splat
	// v128LoadType16Splat corresponds to wasm.OpcodeVecV128Load16SplatName
	v128LoadType16Splat
	// v128LoadType32Splat corresponds to wasm.OpcodeVecV128Load32SplatName
	v128LoadType32Splat
	// v128LoadType64Splat corresponds to wasm.OpcodeVecV128Load64SplatName
	v128LoadType64Splat
	// v128LoadType32zero corresponds to wasm.OpcodeVecV128Load32zeroName
	v128LoadType32zero
	// v128LoadType64zero corresponds to wasm.OpcodeVecV128Load64zeroName
	v128LoadType64zero
)

// NewOperationV128Load is a constructor for unionOperation with operationKindV128Load.
//
// This corresponds to
//
//	wasm.OpcodeVecV128LoadName wasm.OpcodeVecV128Load8x8SName wasm.OpcodeVecV128Load8x8UName
//	wasm.OpcodeVecV128Load16x4SName wasm.OpcodeVecV128Load16x4UName wasm.OpcodeVecV128Load32x2SName
//	wasm.OpcodeVecV128Load32x2UName wasm.OpcodeVecV128Load8SplatName wasm.OpcodeVecV128Load16SplatName
//	wasm.OpcodeVecV128Load32SplatName wasm.OpcodeVecV128Load64SplatName wasm.OpcodeVecV128Load32zeroName
//	wasm.OpcodeVecV128Load64zeroName
func newOperationV128Load(loadType v128LoadType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindV128Load, B1: loadType, U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationV128LoadLane is a constructor for unionOperation with operationKindV128LoadLane.
//
// This corresponds to wasm.OpcodeVecV128Load8LaneName wasm.OpcodeVecV128Load16LaneName
//
//	wasm.OpcodeVecV128Load32LaneName wasm.OpcodeVecV128Load64LaneName.
//
// laneIndex is >=0 && <(128/LaneSize).
// laneSize is either 8, 16, 32, or 64.
func newOperationV128LoadLane(laneIndex, laneSize byte, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindV128LoadLane, B1: laneSize, B2: laneIndex, U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationV128Store is a constructor for unionOperation with operationKindV128Store.
//
// This corresponds to wasm.OpcodeVecV128Load8LaneName wasm.OpcodeVecV128Load16LaneName
//
//	wasm.OpcodeVecV128Load32LaneName wasm.OpcodeVecV128Load64LaneName.
func newOperationV128Store(arg memoryArg) unionOperation {
	return unionOperation{
		Kind: operationKindV128Store,
		U1:   uint64(arg.Alignment),
		U2:   uint64(arg.Offset),
	}
}

// NewOperationV128StoreLane implements Operation.
//
// This corresponds to wasm.OpcodeVecV128Load8LaneName wasm.OpcodeVecV128Load16LaneName
//
//	wasm.OpcodeVecV128Load32LaneName wasm.OpcodeVecV128Load64LaneName.
//
// laneIndex is >=0 && <(128/LaneSize).
// laneSize is either 8, 16, 32, or 64.
func newOperationV128StoreLane(laneIndex byte, laneSize byte, arg memoryArg) unionOperation {
	return unionOperation{
		Kind: operationKindV128StoreLane,
		B1:   laneSize,
		B2:   laneIndex,
		U1:   uint64(arg.Alignment),
		U2:   uint64(arg.Offset),
	}
}

// NewOperationV128ExtractLane is a constructor for unionOperation with operationKindV128ExtractLane.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16ExtractLaneSName wasm.OpcodeVecI8x16ExtractLaneUName
//	wasm.OpcodeVecI16x8ExtractLaneSName wasm.OpcodeVecI16x8ExtractLaneUName
//	wasm.OpcodeVecI32x4ExtractLaneName wasm.OpcodeVecI64x2ExtractLaneName
//	wasm.OpcodeVecF32x4ExtractLaneName wasm.OpcodeVecF64x2ExtractLaneName.
//
// laneIndex is >=0 && <M where shape = NxM.
// signed is used when shape is either i8x16 or i16x2 to specify whether to sign-extend or not.
func newOperationV128ExtractLane(laneIndex byte, signed bool, shape shape) unionOperation {
	return unionOperation{
		Kind: operationKindV128ExtractLane,
		B1:   shape,
		B2:   laneIndex,
		B3:   signed,
	}
}

// NewOperationV128ReplaceLane is a constructor for unionOperation with operationKindV128ReplaceLane.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16ReplaceLaneName wasm.OpcodeVecI16x8ReplaceLaneName
//	wasm.OpcodeVecI32x4ReplaceLaneName wasm.OpcodeVecI64x2ReplaceLaneName
//	wasm.OpcodeVecF32x4ReplaceLaneName wasm.OpcodeVecF64x2ReplaceLaneName.
//
// laneIndex is >=0 && <M where shape = NxM.
func newOperationV128ReplaceLane(laneIndex byte, shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128ReplaceLane, B1: shape, B2: laneIndex}
}

// NewOperationV128Splat is a constructor for unionOperation with operationKindV128Splat.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16SplatName wasm.OpcodeVecI16x8SplatName
//	wasm.OpcodeVecI32x4SplatName wasm.OpcodeVecI64x2SplatName
//	wasm.OpcodeVecF32x4SplatName wasm.OpcodeVecF64x2SplatName.
func newOperationV128Splat(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Splat, B1: shape}
}

// NewOperationV128Shuffle is a constructor for unionOperation with operationKindV128Shuffle.
func newOperationV128Shuffle(lanes []uint64) unionOperation {
	return unionOperation{Kind: operationKindV128Shuffle, Us: lanes}
}

// NewOperationV128Swizzle is a constructor for unionOperation with operationKindV128Swizzle.
//
// This corresponds to wasm.OpcodeVecI8x16SwizzleName.
func newOperationV128Swizzle() unionOperation {
	return unionOperation{Kind: operationKindV128Swizzle}
}

// NewOperationV128AnyTrue is a constructor for unionOperation with operationKindV128AnyTrue.
//
// This corresponds to wasm.OpcodeVecV128AnyTrueName.
func newOperationV128AnyTrue() unionOperation {
	return unionOperation{Kind: operationKindV128AnyTrue}
}

// NewOperationV128AllTrue is a constructor for unionOperation with operationKindV128AllTrue.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16AllTrueName wasm.OpcodeVecI16x8AllTrueName
//	wasm.OpcodeVecI32x4AllTrueName wasm.OpcodeVecI64x2AllTrueName.
func newOperationV128AllTrue(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128AllTrue, B1: shape}
}

// NewOperationV128BitMask is a constructor for unionOperation with operationKindV128BitMask.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16BitMaskName wasm.OpcodeVecI16x8BitMaskName
//	wasm.OpcodeVecI32x4BitMaskName wasm.OpcodeVecI64x2BitMaskName.
func newOperationV128BitMask(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128BitMask, B1: shape}
}

// NewOperationV128And is a constructor for unionOperation with operationKindV128And.
//
// This corresponds to wasm.OpcodeVecV128And.
func newOperationV128And() unionOperation {
	return unionOperation{Kind: operationKindV128And}
}

// NewOperationV128Not is a constructor for unionOperation with operationKindV128Not.
//
// This corresponds to wasm.OpcodeVecV128Not.
func newOperationV128Not() unionOperation {
	return unionOperation{Kind: operationKindV128Not}
}

// NewOperationV128Or is a constructor for unionOperation with operationKindV128Or.
//
// This corresponds to wasm.OpcodeVecV128Or.
func newOperationV128Or() unionOperation {
	return unionOperation{Kind: operationKindV128Or}
}

// NewOperationV128Xor is a constructor for unionOperation with operationKindV128Xor.
//
// This corresponds to wasm.OpcodeVecV128Xor.
func newOperationV128Xor() unionOperation {
	return unionOperation{Kind: operationKindV128Xor}
}

// NewOperationV128Bitselect is a constructor for unionOperation with operationKindV128Bitselect.
//
// This corresponds to wasm.OpcodeVecV128Bitselect.
func newOperationV128Bitselect() unionOperation {
	return unionOperation{Kind: operationKindV128Bitselect}
}

// NewOperationV128AndNot is a constructor for unionOperation with operationKindV128AndNot.
//
// This corresponds to wasm.OpcodeVecV128AndNot.
func newOperationV128AndNot() unionOperation {
	return unionOperation{Kind: operationKindV128AndNot}
}

// NewOperationV128Shl is a constructor for unionOperation with operationKindV128Shl.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16ShlName wasm.OpcodeVecI16x8ShlName
//	wasm.OpcodeVecI32x4ShlName wasm.OpcodeVecI64x2ShlName
func newOperationV128Shl(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Shl, B1: shape}
}

// NewOperationV128Shr is a constructor for unionOperation with operationKindV128Shr.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16ShrSName wasm.OpcodeVecI8x16ShrUName wasm.OpcodeVecI16x8ShrSName
//	wasm.OpcodeVecI16x8ShrUName wasm.OpcodeVecI32x4ShrSName wasm.OpcodeVecI32x4ShrUName.
//	wasm.OpcodeVecI64x2ShrSName wasm.OpcodeVecI64x2ShrUName.
func newOperationV128Shr(shape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128Shr, B1: shape, B3: signed}
}

// NewOperationV128Cmp is a constructor for unionOperation with operationKindV128Cmp.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16EqName, wasm.OpcodeVecI8x16NeName, wasm.OpcodeVecI8x16LtSName, wasm.OpcodeVecI8x16LtUName, wasm.OpcodeVecI8x16GtSName,
//	wasm.OpcodeVecI8x16GtUName, wasm.OpcodeVecI8x16LeSName, wasm.OpcodeVecI8x16LeUName, wasm.OpcodeVecI8x16GeSName, wasm.OpcodeVecI8x16GeUName,
//	wasm.OpcodeVecI16x8EqName, wasm.OpcodeVecI16x8NeName, wasm.OpcodeVecI16x8LtSName, wasm.OpcodeVecI16x8LtUName, wasm.OpcodeVecI16x8GtSName,
//	wasm.OpcodeVecI16x8GtUName, wasm.OpcodeVecI16x8LeSName, wasm.OpcodeVecI16x8LeUName, wasm.OpcodeVecI16x8GeSName, wasm.OpcodeVecI16x8GeUName,
//	wasm.OpcodeVecI32x4EqName, wasm.OpcodeVecI32x4NeName, wasm.OpcodeVecI32x4LtSName, wasm.OpcodeVecI32x4LtUName, wasm.OpcodeVecI32x4GtSName,
//	wasm.OpcodeVecI32x4GtUName, wasm.OpcodeVecI32x4LeSName, wasm.OpcodeVecI32x4LeUName, wasm.OpcodeVecI32x4GeSName, wasm.OpcodeVecI32x4GeUName,
//	wasm.OpcodeVecI64x2EqName, wasm.OpcodeVecI64x2NeName, wasm.OpcodeVecI64x2LtSName, wasm.OpcodeVecI64x2GtSName, wasm.OpcodeVecI64x2LeSName,
//	wasm.OpcodeVecI64x2GeSName, wasm.OpcodeVecF32x4EqName, wasm.OpcodeVecF32x4NeName, wasm.OpcodeVecF32x4LtName, wasm.OpcodeVecF32x4GtName,
//	wasm.OpcodeVecF32x4LeName, wasm.OpcodeVecF32x4GeName, wasm.OpcodeVecF64x2EqName, wasm.OpcodeVecF64x2NeName, wasm.OpcodeVecF64x2LtName,
//	wasm.OpcodeVecF64x2GtName, wasm.OpcodeVecF64x2LeName, wasm.OpcodeVecF64x2GeName
func newOperationV128Cmp(cmpType v128CmpType) unionOperation {
	return unionOperation{Kind: operationKindV128Cmp, B1: cmpType}
}

// v128CmpType represents a type of vector comparison operation.
type v128CmpType = byte

const (
	// v128CmpTypeI8x16Eq corresponds to wasm.OpcodeVecI8x16EqName.
	v128CmpTypeI8x16Eq v128CmpType = iota
	// v128CmpTypeI8x16Ne corresponds to wasm.OpcodeVecI8x16NeName.
	v128CmpTypeI8x16Ne
	// v128CmpTypeI8x16LtS corresponds to wasm.OpcodeVecI8x16LtSName.
	v128CmpTypeI8x16LtS
	// v128CmpTypeI8x16LtU corresponds to wasm.OpcodeVecI8x16LtUName.
	v128CmpTypeI8x16LtU
	// v128CmpTypeI8x16GtS corresponds to wasm.OpcodeVecI8x16GtSName.
	v128CmpTypeI8x16GtS
	// v128CmpTypeI8x16GtU corresponds to wasm.OpcodeVecI8x16GtUName.
	v128CmpTypeI8x16GtU
	// v128CmpTypeI8x16LeS corresponds to wasm.OpcodeVecI8x16LeSName.
	v128CmpTypeI8x16LeS
	// v128CmpTypeI8x16LeU corresponds to wasm.OpcodeVecI8x16LeUName.
	v128CmpTypeI8x16LeU
	// v128CmpTypeI8x16GeS corresponds to wasm.OpcodeVecI8x16GeSName.
	v128CmpTypeI8x16GeS
	// v128CmpTypeI8x16GeU corresponds to wasm.OpcodeVecI8x16GeUName.
	v128CmpTypeI8x16GeU
	// v128CmpTypeI16x8Eq corresponds to wasm.OpcodeVecI16x8EqName.
	v128CmpTypeI16x8Eq
	// v128CmpTypeI16x8Ne corresponds to wasm.OpcodeVecI16x8NeName.
	v128CmpTypeI16x8Ne
	// v128CmpTypeI16x8LtS corresponds to wasm.OpcodeVecI16x8LtSName.
	v128CmpTypeI16x8LtS
	// v128CmpTypeI16x8LtU corresponds to wasm.OpcodeVecI16x8LtUName.
	v128CmpTypeI16x8LtU
	// v128CmpTypeI16x8GtS corresponds to wasm.OpcodeVecI16x8GtSName.
	v128CmpTypeI16x8GtS
	// v128CmpTypeI16x8GtU corresponds to wasm.OpcodeVecI16x8GtUName.
	v128CmpTypeI16x8GtU
	// v128CmpTypeI16x8LeS corresponds to wasm.OpcodeVecI16x8LeSName.
	v128CmpTypeI16x8LeS
	// v128CmpTypeI16x8LeU corresponds to wasm.OpcodeVecI16x8LeUName.
	v128CmpTypeI16x8LeU
	// v128CmpTypeI16x8GeS corresponds to wasm.OpcodeVecI16x8GeSName.
	v128CmpTypeI16x8GeS
	// v128CmpTypeI16x8GeU corresponds to wasm.OpcodeVecI16x8GeUName.
	v128CmpTypeI16x8GeU
	// v128CmpTypeI32x4Eq corresponds to wasm.OpcodeVecI32x4EqName.
	v128CmpTypeI32x4Eq
	// v128CmpTypeI32x4Ne corresponds to wasm.OpcodeVecI32x4NeName.
	v128CmpTypeI32x4Ne
	// v128CmpTypeI32x4LtS corresponds to wasm.OpcodeVecI32x4LtSName.
	v128CmpTypeI32x4LtS
	// v128CmpTypeI32x4LtU corresponds to wasm.OpcodeVecI32x4LtUName.
	v128CmpTypeI32x4LtU
	// v128CmpTypeI32x4GtS corresponds to wasm.OpcodeVecI32x4GtSName.
	v128CmpTypeI32x4GtS
	// v128CmpTypeI32x4GtU corresponds to wasm.OpcodeVecI32x4GtUName.
	v128CmpTypeI32x4GtU
	// v128CmpTypeI32x4LeS corresponds to wasm.OpcodeVecI32x4LeSName.
	v128CmpTypeI32x4LeS
	// v128CmpTypeI32x4LeU corresponds to wasm.OpcodeVecI32x4LeUName.
	v128CmpTypeI32x4LeU
	// v128CmpTypeI32x4GeS corresponds to wasm.OpcodeVecI32x4GeSName.
	v128CmpTypeI32x4GeS
	// v128CmpTypeI32x4GeU corresponds to wasm.OpcodeVecI32x4GeUName.
	v128CmpTypeI32x4GeU
	// v128CmpTypeI64x2Eq corresponds to wasm.OpcodeVecI64x2EqName.
	v128CmpTypeI64x2Eq
	// v128CmpTypeI64x2Ne corresponds to wasm.OpcodeVecI64x2NeName.
	v128CmpTypeI64x2Ne
	// v128CmpTypeI64x2LtS corresponds to wasm.OpcodeVecI64x2LtSName.
	v128CmpTypeI64x2LtS
	// v128CmpTypeI64x2GtS corresponds to wasm.OpcodeVecI64x2GtSName.
	v128CmpTypeI64x2GtS
	// v128CmpTypeI64x2LeS corresponds to wasm.OpcodeVecI64x2LeSName.
	v128CmpTypeI64x2LeS
	// v128CmpTypeI64x2GeS corresponds to wasm.OpcodeVecI64x2GeSName.
	v128CmpTypeI64x2GeS
	// v128CmpTypeF32x4Eq corresponds to wasm.OpcodeVecF32x4EqName.
	v128CmpTypeF32x4Eq
	// v128CmpTypeF32x4Ne corresponds to wasm.OpcodeVecF32x4NeName.
	v128CmpTypeF32x4Ne
	// v128CmpTypeF32x4Lt corresponds to wasm.OpcodeVecF32x4LtName.
	v128CmpTypeF32x4Lt
	// v128CmpTypeF32x4Gt corresponds to wasm.OpcodeVecF32x4GtName.
	v128CmpTypeF32x4Gt
	// v128CmpTypeF32x4Le corresponds to wasm.OpcodeVecF32x4LeName.
	v128CmpTypeF32x4Le
	// v128CmpTypeF32x4Ge corresponds to wasm.OpcodeVecF32x4GeName.
	v128CmpTypeF32x4Ge
	// v128CmpTypeF64x2Eq corresponds to wasm.OpcodeVecF64x2EqName.
	v128CmpTypeF64x2Eq
	// v128CmpTypeF64x2Ne corresponds to wasm.OpcodeVecF64x2NeName.
	v128CmpTypeF64x2Ne
	// v128CmpTypeF64x2Lt corresponds to wasm.OpcodeVecF64x2LtName.
	v128CmpTypeF64x2Lt
	// v128CmpTypeF64x2Gt corresponds to wasm.OpcodeVecF64x2GtName.
	v128CmpTypeF64x2Gt
	// v128CmpTypeF64x2Le corresponds to wasm.OpcodeVecF64x2LeName.
	v128CmpTypeF64x2Le
	// v128CmpTypeF64x2Ge corresponds to wasm.OpcodeVecF64x2GeName.
	v128CmpTypeF64x2Ge
)

// NewOperationV128AddSat is a constructor for unionOperation with operationKindV128AddSat.
//
// This corresponds to wasm.OpcodeVecI8x16AddSatUName wasm.OpcodeVecI8x16AddSatSName
//
//	wasm.OpcodeVecI16x8AddSatUName wasm.OpcodeVecI16x8AddSatSName
//
// shape is either shapeI8x16 or shapeI16x8.
func newOperationV128AddSat(shape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128AddSat, B1: shape, B3: signed}
}

// NewOperationV128SubSat is a constructor for unionOperation with operationKindV128SubSat.
//
// This corresponds to wasm.OpcodeVecI8x16SubSatUName wasm.OpcodeVecI8x16SubSatSName
//
//	wasm.OpcodeVecI16x8SubSatUName wasm.OpcodeVecI16x8SubSatSName
//
// shape is either shapeI8x16 or shapeI16x8.
func newOperationV128SubSat(shape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128SubSat, B1: shape, B3: signed}
}

// NewOperationV128Mul is a constructor for unionOperation with operationKindV128Mul
//
// This corresponds to wasm.OpcodeVecF32x4MulName wasm.OpcodeVecF64x2MulName
//
//		wasm.OpcodeVecI16x8MulName wasm.OpcodeVecI32x4MulName wasm.OpcodeVecI64x2MulName.
//	 shape is either shapeI16x8, shapeI32x4, shapeI64x2, shapeF32x4 or shapeF64x2.
func newOperationV128Mul(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Mul, B1: shape}
}

// NewOperationV128Div is a constructor for unionOperation with operationKindV128Div.
//
// This corresponds to wasm.OpcodeVecF32x4DivName wasm.OpcodeVecF64x2DivName.
// shape is either shapeF32x4 or shapeF64x2.
func newOperationV128Div(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Div, B1: shape}
}

// NewOperationV128Neg is a constructor for unionOperation with operationKindV128Neg.
//
// This corresponds to wasm.OpcodeVecI8x16NegName wasm.OpcodeVecI16x8NegName wasm.OpcodeVecI32x4NegName
//
//	wasm.OpcodeVecI64x2NegName wasm.OpcodeVecF32x4NegName wasm.OpcodeVecF64x2NegName.
func newOperationV128Neg(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Neg, B1: shape}
}

// NewOperationV128Sqrt is a constructor for unionOperation with 128operationKindV128Sqrt.
//
// shape is either shapeF32x4 or shapeF64x2.
// This corresponds to wasm.OpcodeVecF32x4SqrtName wasm.OpcodeVecF64x2SqrtName.
func newOperationV128Sqrt(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Sqrt, B1: shape}
}

// NewOperationV128Abs is a constructor for unionOperation with operationKindV128Abs.
//
// This corresponds to wasm.OpcodeVecI8x16AbsName wasm.OpcodeVecI16x8AbsName wasm.OpcodeVecI32x4AbsName
//
//	wasm.OpcodeVecI64x2AbsName wasm.OpcodeVecF32x4AbsName wasm.OpcodeVecF64x2AbsName.
func newOperationV128Abs(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Abs, B1: shape}
}

// NewOperationV128Popcnt is a constructor for unionOperation with operationKindV128Popcnt.
//
// This corresponds to wasm.OpcodeVecI8x16PopcntName.
func newOperationV128Popcnt(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Popcnt, B1: shape}
}

// NewOperationV128Min is a constructor for unionOperation with operationKindV128Min.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16MinSName wasm.OpcodeVecI8x16MinUName　wasm.OpcodeVecI16x8MinSName wasm.OpcodeVecI16x8MinUName
//	wasm.OpcodeVecI32x4MinSName wasm.OpcodeVecI32x4MinUName　wasm.OpcodeVecI16x8MinSName wasm.OpcodeVecI16x8MinUName
//	wasm.OpcodeVecF32x4MinName wasm.OpcodeVecF64x2MinName
func newOperationV128Min(shape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128Min, B1: shape, B3: signed}
}

// NewOperationV128Max is a constructor for unionOperation with operationKindV128Max.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16MaxSName wasm.OpcodeVecI8x16MaxUName　wasm.OpcodeVecI16x8MaxSName wasm.OpcodeVecI16x8MaxUName
//	wasm.OpcodeVecI32x4MaxSName wasm.OpcodeVecI32x4MaxUName　wasm.OpcodeVecI16x8MaxSName wasm.OpcodeVecI16x8MaxUName
//	wasm.OpcodeVecF32x4MaxName wasm.OpcodeVecF64x2MaxName.
func newOperationV128Max(shape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128Max, B1: shape, B3: signed}
}

// NewOperationV128AvgrU is a constructor for unionOperation with operationKindV128AvgrU.
//
// This corresponds to wasm.OpcodeVecI8x16AvgrUName.
func newOperationV128AvgrU(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128AvgrU, B1: shape}
}

// NewOperationV128Pmin is a constructor for unionOperation with operationKindV128Pmin.
//
// This corresponds to wasm.OpcodeVecF32x4PminName wasm.OpcodeVecF64x2PminName.
func newOperationV128Pmin(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Pmin, B1: shape}
}

// NewOperationV128Pmax is a constructor for unionOperation with operationKindV128Pmax.
//
// This corresponds to wasm.OpcodeVecF32x4PmaxName wasm.OpcodeVecF64x2PmaxName.
func newOperationV128Pmax(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Pmax, B1: shape}
}

// NewOperationV128Ceil is a constructor for unionOperation with operationKindV128Ceil.
//
// This corresponds to wasm.OpcodeVecF32x4CeilName wasm.OpcodeVecF64x2CeilName
func newOperationV128Ceil(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Ceil, B1: shape}
}

// NewOperationV128Floor is a constructor for unionOperation with operationKindV128Floor.
//
// This corresponds to wasm.OpcodeVecF32x4FloorName wasm.OpcodeVecF64x2FloorName
func newOperationV128Floor(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Floor, B1: shape}
}

// NewOperationV128Trunc is a constructor for unionOperation with operationKindV128Trunc.
//
// This corresponds to wasm.OpcodeVecF32x4TruncName wasm.OpcodeVecF64x2TruncName
func newOperationV128Trunc(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Trunc, B1: shape}
}

// NewOperationV128Nearest is a constructor for unionOperation with operationKindV128Nearest.
//
// This corresponds to wasm.OpcodeVecF32x4NearestName wasm.OpcodeVecF64x2NearestName
func newOperationV128Nearest(shape shape) unionOperation {
	return unionOperation{Kind: operationKindV128Nearest, B1: shape}
}

// NewOperationV128Extend is a constructor for unionOperation with operationKindV128Extend.
//
// This corresponds to
//
//	wasm.OpcodeVecI16x8ExtendLowI8x16SName wasm.OpcodeVecI16x8ExtendHighI8x16SName
//	wasm.OpcodeVecI16x8ExtendLowI8x16UName wasm.OpcodeVecI16x8ExtendHighI8x16UName
//	wasm.OpcodeVecI32x4ExtendLowI16x8SName wasm.OpcodeVecI32x4ExtendHighI16x8SName
//	wasm.OpcodeVecI32x4ExtendLowI16x8UName wasm.OpcodeVecI32x4ExtendHighI16x8UName
//	wasm.OpcodeVecI64x2ExtendLowI32x4SName wasm.OpcodeVecI64x2ExtendHighI32x4SName
//	wasm.OpcodeVecI64x2ExtendLowI32x4UName wasm.OpcodeVecI64x2ExtendHighI32x4UName
//
// originshape is the shape of the original lanes for extension which is
// either shapeI8x16, shapeI16x8, or shapeI32x4.
// useLow true if it uses the lower half of vector for extension.
func newOperationV128Extend(originshape shape, signed bool, useLow bool) unionOperation {
	op := unionOperation{Kind: operationKindV128Extend}
	op.B1 = originshape
	if signed {
		op.B2 = 1
	}
	op.B3 = useLow
	return op
}

// NewOperationV128ExtMul is a constructor for unionOperation with operationKindV128ExtMul.
//
// This corresponds to
//
//		wasm.OpcodeVecI16x8ExtMulLowI8x16SName wasm.OpcodeVecI16x8ExtMulLowI8x16UName
//		wasm.OpcodeVecI16x8ExtMulHighI8x16SName wasm.OpcodeVecI16x8ExtMulHighI8x16UName
//	 wasm.OpcodeVecI32x4ExtMulLowI16x8SName wasm.OpcodeVecI32x4ExtMulLowI16x8UName
//		wasm.OpcodeVecI32x4ExtMulHighI16x8SName wasm.OpcodeVecI32x4ExtMulHighI16x8UName
//	 wasm.OpcodeVecI64x2ExtMulLowI32x4SName wasm.OpcodeVecI64x2ExtMulLowI32x4UName
//		wasm.OpcodeVecI64x2ExtMulHighI32x4SName wasm.OpcodeVecI64x2ExtMulHighI32x4UName.
//
// originshape is the shape of the original lanes for extension which is
// either shapeI8x16, shapeI16x8, or shapeI32x4.
// useLow true if it uses the lower half of vector for extension.
func newOperationV128ExtMul(originshape shape, signed bool, useLow bool) unionOperation {
	op := unionOperation{Kind: operationKindV128ExtMul}
	op.B1 = originshape
	if signed {
		op.B2 = 1
	}
	op.B3 = useLow
	return op
}

// NewOperationV128Q15mulrSatS is a constructor for unionOperation with operationKindV128Q15mulrSatS.
//
// This corresponds to wasm.OpcodeVecI16x8Q15mulrSatSName
func newOperationV128Q15mulrSatS() unionOperation {
	return unionOperation{Kind: operationKindV128Q15mulrSatS}
}

// NewOperationV128ExtAddPairwise is a constructor for unionOperation with operationKindV128ExtAddPairwise.
//
// This corresponds to
//
//	wasm.OpcodeVecI16x8ExtaddPairwiseI8x16SName wasm.OpcodeVecI16x8ExtaddPairwiseI8x16UName
//	wasm.OpcodeVecI32x4ExtaddPairwiseI16x8SName wasm.OpcodeVecI32x4ExtaddPairwiseI16x8UName.
//
// originshape is the shape of the original lanes for extension which is
// either shapeI8x16, or shapeI16x8.
func newOperationV128ExtAddPairwise(originshape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128ExtAddPairwise, B1: originshape, B3: signed}
}

// NewOperationV128FloatPromote is a constructor for unionOperation with NewOperationV128FloatPromote.
//
// This corresponds to wasm.OpcodeVecF64x2PromoteLowF32x4ZeroName
// This discards the higher 64-bit of a vector, and promotes two
// 32-bit floats in the lower 64-bit as two 64-bit floats.
func newOperationV128FloatPromote() unionOperation {
	return unionOperation{Kind: operationKindV128FloatPromote}
}

// NewOperationV128FloatDemote is a constructor for unionOperation with NewOperationV128FloatDemote.
//
// This corresponds to wasm.OpcodeVecF32x4DemoteF64x2ZeroName.
func newOperationV128FloatDemote() unionOperation {
	return unionOperation{Kind: operationKindV128FloatDemote}
}

// NewOperationV128FConvertFromI is a constructor for unionOperation with NewOperationV128FConvertFromI.
//
// This corresponds to
//
//	wasm.OpcodeVecF32x4ConvertI32x4SName wasm.OpcodeVecF32x4ConvertI32x4UName
//	wasm.OpcodeVecF64x2ConvertLowI32x4SName wasm.OpcodeVecF64x2ConvertLowI32x4UName.
//
// destinationshape is the shape of the destination lanes for conversion which is
// either shapeF32x4, or shapeF64x2.
func newOperationV128FConvertFromI(destinationshape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128FConvertFromI, B1: destinationshape, B3: signed}
}

// NewOperationV128Dot is a constructor for unionOperation with operationKindV128Dot.
//
// This corresponds to wasm.OpcodeVecI32x4DotI16x8SName
func newOperationV128Dot() unionOperation {
	return unionOperation{Kind: operationKindV128Dot}
}

// NewOperationV128Narrow is a constructor for unionOperation with operationKindV128Narrow.
//
// This corresponds to
//
//	wasm.OpcodeVecI8x16NarrowI16x8SName wasm.OpcodeVecI8x16NarrowI16x8UName
//	wasm.OpcodeVecI16x8NarrowI32x4SName wasm.OpcodeVecI16x8NarrowI32x4UName.
//
// originshape is the shape of the original lanes for narrowing which is
// either shapeI16x8, or shapeI32x4.
func newOperationV128Narrow(originshape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128Narrow, B1: originshape, B3: signed}
}

// NewOperationV128ITruncSatFromF is a constructor for unionOperation with operationKindV128ITruncSatFromF.
//
// This corresponds to
//
//	wasm.OpcodeVecI32x4TruncSatF64x2UZeroName wasm.OpcodeVecI32x4TruncSatF64x2SZeroName
//	wasm.OpcodeVecI32x4TruncSatF32x4UName wasm.OpcodeVecI32x4TruncSatF32x4SName.
//
// originshape is the shape of the original lanes for truncation which is
// either shapeF32x4, or shapeF64x2.
func newOperationV128ITruncSatFromF(originshape shape, signed bool) unionOperation {
	return unionOperation{Kind: operationKindV128ITruncSatFromF, B1: originshape, B3: signed}
}

// atomicArithmeticOp is the type for the operation kind of atomic arithmetic operations.
type atomicArithmeticOp byte

const (
	// atomicArithmeticOpAdd is the kind for an add operation.
	atomicArithmeticOpAdd atomicArithmeticOp = iota
	// atomicArithmeticOpSub is the kind for a sub operation.
	atomicArithmeticOpSub
	// atomicArithmeticOpAnd is the kind for a bitwise and operation.
	atomicArithmeticOpAnd
	// atomicArithmeticOpOr is the kind for a bitwise or operation.
	atomicArithmeticOpOr
	// atomicArithmeticOpXor is the kind for a bitwise xor operation.
	atomicArithmeticOpXor
	// atomicArithmeticOpNop is the kind for a nop operation.
	atomicArithmeticOpNop
)

// NewOperationAtomicMemoryWait is a constructor for unionOperation with operationKindAtomicMemoryWait.
//
// This corresponds to
//
//	wasm.OpcodeAtomicWait32Name wasm.OpcodeAtomicWait64Name
func newOperationAtomicMemoryWait(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicMemoryWait, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicMemoryNotify is a constructor for unionOperation with operationKindAtomicMemoryNotify.
//
// This corresponds to
//
//	wasm.OpcodeAtomicNotifyName
func newOperationAtomicMemoryNotify(arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicMemoryNotify, U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicFence is a constructor for unionOperation with operationKindAtomicFence.
//
// This corresponds to
//
//	wasm.OpcodeAtomicFenceName
func newOperationAtomicFence() unionOperation {
	return unionOperation{Kind: operationKindAtomicFence}
}

// NewOperationAtomicLoad is a constructor for unionOperation with operationKindAtomicLoad.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32LoadName wasm.OpcodeAtomicI64LoadName
func newOperationAtomicLoad(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicLoad, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicLoad8 is a constructor for unionOperation with operationKindAtomicLoad8.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32Load8UName wasm.OpcodeAtomicI64Load8UName
func newOperationAtomicLoad8(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicLoad8, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicLoad16 is a constructor for unionOperation with operationKindAtomicLoad16.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32Load16UName wasm.OpcodeAtomicI64Load16UName
func newOperationAtomicLoad16(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicLoad16, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicStore is a constructor for unionOperation with operationKindAtomicStore.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32StoreName wasm.OpcodeAtomicI64StoreName
func newOperationAtomicStore(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicStore, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicStore8 is a constructor for unionOperation with operationKindAtomicStore8.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32Store8UName wasm.OpcodeAtomicI64Store8UName
func newOperationAtomicStore8(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicStore8, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicStore16 is a constructor for unionOperation with operationKindAtomicStore16.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32Store16UName wasm.OpcodeAtomicI64Store16UName
func newOperationAtomicStore16(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicStore16, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicRMW is a constructor for unionOperation with operationKindAtomicRMW.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32RMWAddName wasm.OpcodeAtomicI64RmwAddName
//	wasm.OpcodeAtomicI32RMWSubName wasm.OpcodeAtomicI64RmwSubName
//	wasm.OpcodeAtomicI32RMWAndName wasm.OpcodeAtomicI64RmwAndName
//	wasm.OpcodeAtomicI32RMWOrName wasm.OpcodeAtomicI64RmwOrName
//	wasm.OpcodeAtomicI32RMWXorName wasm.OpcodeAtomicI64RmwXorName
func newOperationAtomicRMW(unsignedType unsignedType, arg memoryArg, op atomicArithmeticOp) unionOperation {
	return unionOperation{Kind: operationKindAtomicRMW, B1: byte(unsignedType), B2: byte(op), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicRMW8 is a constructor for unionOperation with operationKindAtomicRMW8.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32RMW8AddUName wasm.OpcodeAtomicI64Rmw8AddUName
//	wasm.OpcodeAtomicI32RMW8SubUName wasm.OpcodeAtomicI64Rmw8SubUName
//	wasm.OpcodeAtomicI32RMW8AndUName wasm.OpcodeAtomicI64Rmw8AndUName
//	wasm.OpcodeAtomicI32RMW8OrUName wasm.OpcodeAtomicI64Rmw8OrUName
//	wasm.OpcodeAtomicI32RMW8XorUName wasm.OpcodeAtomicI64Rmw8XorUName
func newOperationAtomicRMW8(unsignedType unsignedType, arg memoryArg, op atomicArithmeticOp) unionOperation {
	return unionOperation{Kind: operationKindAtomicRMW8, B1: byte(unsignedType), B2: byte(op), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicRMW16 is a constructor for unionOperation with operationKindAtomicRMW16.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32RMW16AddUName wasm.OpcodeAtomicI64Rmw16AddUName
//	wasm.OpcodeAtomicI32RMW16SubUName wasm.OpcodeAtomicI64Rmw16SubUName
//	wasm.OpcodeAtomicI32RMW16AndUName wasm.OpcodeAtomicI64Rmw16AndUName
//	wasm.OpcodeAtomicI32RMW16OrUName wasm.OpcodeAtomicI64Rmw16OrUName
//	wasm.OpcodeAtomicI32RMW16XorUName wasm.OpcodeAtomicI64Rmw16XorUName
func newOperationAtomicRMW16(unsignedType unsignedType, arg memoryArg, op atomicArithmeticOp) unionOperation {
	return unionOperation{Kind: operationKindAtomicRMW16, B1: byte(unsignedType), B2: byte(op), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicRMWCmpxchg is a constructor for unionOperation with operationKindAtomicRMWCmpxchg.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32RMWCmpxchgName wasm.OpcodeAtomicI64RmwCmpxchgName
func newOperationAtomicRMWCmpxchg(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicRMWCmpxchg, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicRMW8Cmpxchg is a constructor for unionOperation with operationKindAtomicRMW8Cmpxchg.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32RMW8CmpxchgUName wasm.OpcodeAtomicI64Rmw8CmpxchgUName
func newOperationAtomicRMW8Cmpxchg(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicRMW8Cmpxchg, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}

// NewOperationAtomicRMW16Cmpxchg is a constructor for unionOperation with operationKindAtomicRMW16Cmpxchg.
//
// This corresponds to
//
//	wasm.OpcodeAtomicI32RMW16CmpxchgUName wasm.OpcodeAtomicI64Rmw16CmpxchgUName
func newOperationAtomicRMW16Cmpxchg(unsignedType unsignedType, arg memoryArg) unionOperation {
	return unionOperation{Kind: operationKindAtomicRMW16Cmpxchg, B1: byte(unsignedType), U1: uint64(arg.Alignment), U2: uint64(arg.Offset)}
}
