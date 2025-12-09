package interpreter

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/wasm"
)

// signature represents how a Wasm opcode
// manipulates the value stacks in terms of value types.
type signature struct {
	in, out []unsignedType
}

var (
	signature_None_None    = &signature{}
	signature_Unknown_None = &signature{
		in: []unsignedType{unsignedTypeUnknown},
	}
	signature_None_I32 = &signature{
		out: []unsignedType{unsignedTypeI32},
	}
	signature_None_I64 = &signature{
		out: []unsignedType{unsignedTypeI64},
	}
	signature_None_V128 = &signature{
		out: []unsignedType{unsignedTypeV128},
	}
	signature_None_F32 = &signature{
		out: []unsignedType{unsignedTypeF32},
	}
	signature_None_F64 = &signature{
		out: []unsignedType{unsignedTypeF64},
	}
	signature_I32_None = &signature{
		in: []unsignedType{unsignedTypeI32},
	}
	signature_I64_None = &signature{
		in: []unsignedType{unsignedTypeI64},
	}
	signature_F32_None = &signature{
		in: []unsignedType{unsignedTypeF32},
	}
	signature_F64_None = &signature{
		in: []unsignedType{unsignedTypeF64},
	}
	signature_V128_None = &signature{
		in: []unsignedType{unsignedTypeV128},
	}
	signature_I32_I32 = &signature{
		in:  []unsignedType{unsignedTypeI32},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_I32_I64 = &signature{
		in:  []unsignedType{unsignedTypeI32},
		out: []unsignedType{unsignedTypeI64},
	}
	signature_I64_I64 = &signature{
		in:  []unsignedType{unsignedTypeI64},
		out: []unsignedType{unsignedTypeI64},
	}
	signature_I32_F32 = &signature{
		in:  []unsignedType{unsignedTypeI32},
		out: []unsignedType{unsignedTypeF32},
	}
	signature_I32_F64 = &signature{
		in:  []unsignedType{unsignedTypeI32},
		out: []unsignedType{unsignedTypeF64},
	}
	signature_I64_I32 = &signature{
		in:  []unsignedType{unsignedTypeI64},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_I64_F32 = &signature{
		in:  []unsignedType{unsignedTypeI64},
		out: []unsignedType{unsignedTypeF32},
	}
	signature_I64_F64 = &signature{
		in:  []unsignedType{unsignedTypeI64},
		out: []unsignedType{unsignedTypeF64},
	}
	signature_F32_I32 = &signature{
		in:  []unsignedType{unsignedTypeF32},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_F32_I64 = &signature{
		in:  []unsignedType{unsignedTypeF32},
		out: []unsignedType{unsignedTypeI64},
	}
	signature_F32_F64 = &signature{
		in:  []unsignedType{unsignedTypeF32},
		out: []unsignedType{unsignedTypeF64},
	}
	signature_F32_F32 = &signature{
		in:  []unsignedType{unsignedTypeF32},
		out: []unsignedType{unsignedTypeF32},
	}
	signature_F64_I32 = &signature{
		in:  []unsignedType{unsignedTypeF64},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_F64_F32 = &signature{
		in:  []unsignedType{unsignedTypeF64},
		out: []unsignedType{unsignedTypeF32},
	}
	signature_F64_I64 = &signature{
		in:  []unsignedType{unsignedTypeF64},
		out: []unsignedType{unsignedTypeI64},
	}
	signature_F64_F64 = &signature{
		in:  []unsignedType{unsignedTypeF64},
		out: []unsignedType{unsignedTypeF64},
	}
	signature_I32I32_None = &signature{
		in: []unsignedType{unsignedTypeI32, unsignedTypeI32},
	}

	signature_I32I32_I32 = &signature{
		in:  []unsignedType{unsignedTypeI32, unsignedTypeI32},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_I32I64_None = &signature{
		in: []unsignedType{unsignedTypeI32, unsignedTypeI64},
	}
	signature_I32F32_None = &signature{
		in: []unsignedType{unsignedTypeI32, unsignedTypeF32},
	}
	signature_I32F64_None = &signature{
		in: []unsignedType{unsignedTypeI32, unsignedTypeF64},
	}
	signature_I64I32_I32 = &signature{
		in:  []unsignedType{unsignedTypeI64, unsignedTypeI32},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_I64I64_I32 = &signature{
		in:  []unsignedType{unsignedTypeI64, unsignedTypeI64},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_I64I64_I64 = &signature{
		in:  []unsignedType{unsignedTypeI64, unsignedTypeI64},
		out: []unsignedType{unsignedTypeI64},
	}
	signature_F32F32_I32 = &signature{
		in:  []unsignedType{unsignedTypeF32, unsignedTypeF32},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_F32F32_F32 = &signature{
		in:  []unsignedType{unsignedTypeF32, unsignedTypeF32},
		out: []unsignedType{unsignedTypeF32},
	}
	signature_F64F64_I32 = &signature{
		in:  []unsignedType{unsignedTypeF64, unsignedTypeF64},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_F64F64_F64 = &signature{
		in:  []unsignedType{unsignedTypeF64, unsignedTypeF64},
		out: []unsignedType{unsignedTypeF64},
	}
	signature_I32I32I32_None = &signature{
		in: []unsignedType{unsignedTypeI32, unsignedTypeI32, unsignedTypeI32},
	}
	signature_I32I64I32_None = &signature{
		in: []unsignedType{unsignedTypeI32, unsignedTypeI64, unsignedTypeI32},
	}
	signature_UnknownUnknownI32_Unknown = &signature{
		in:  []unsignedType{unsignedTypeUnknown, unsignedTypeUnknown, unsignedTypeI32},
		out: []unsignedType{unsignedTypeUnknown},
	}
	signature_V128V128_V128 = &signature{
		in:  []unsignedType{unsignedTypeV128, unsignedTypeV128},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_V128V128V128_V32 = &signature{
		in:  []unsignedType{unsignedTypeV128, unsignedTypeV128, unsignedTypeV128},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_I32_V128 = &signature{
		in:  []unsignedType{unsignedTypeI32},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_I32V128_None = &signature{
		in: []unsignedType{unsignedTypeI32, unsignedTypeV128},
	}
	signature_I32V128_V128 = &signature{
		in:  []unsignedType{unsignedTypeI32, unsignedTypeV128},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_V128I32_V128 = &signature{
		in:  []unsignedType{unsignedTypeV128, unsignedTypeI32},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_V128I64_V128 = &signature{
		in:  []unsignedType{unsignedTypeV128, unsignedTypeI64},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_V128F32_V128 = &signature{
		in:  []unsignedType{unsignedTypeV128, unsignedTypeF32},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_V128F64_V128 = &signature{
		in:  []unsignedType{unsignedTypeV128, unsignedTypeF64},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_V128_I32 = &signature{
		in:  []unsignedType{unsignedTypeV128},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_V128_I64 = &signature{
		in:  []unsignedType{unsignedTypeV128},
		out: []unsignedType{unsignedTypeI64},
	}
	signature_V128_F32 = &signature{
		in:  []unsignedType{unsignedTypeV128},
		out: []unsignedType{unsignedTypeF32},
	}
	signature_V128_F64 = &signature{
		in:  []unsignedType{unsignedTypeV128},
		out: []unsignedType{unsignedTypeF64},
	}
	signature_V128_V128 = &signature{
		in:  []unsignedType{unsignedTypeV128},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_I64_V128 = &signature{
		in:  []unsignedType{unsignedTypeI64},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_F32_V128 = &signature{
		in:  []unsignedType{unsignedTypeF32},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_F64_V128 = &signature{
		in:  []unsignedType{unsignedTypeF64},
		out: []unsignedType{unsignedTypeV128},
	}
	signature_I32I64_I64 = &signature{
		in:  []unsignedType{unsignedTypeI32, unsignedTypeI64},
		out: []unsignedType{unsignedTypeI64},
	}
	signature_I32I32I64_I32 = &signature{
		in:  []unsignedType{unsignedTypeI32, unsignedTypeI32, unsignedTypeI64},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_I32I64I64_I32 = &signature{
		in:  []unsignedType{unsignedTypeI32, unsignedTypeI64, unsignedTypeI64},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_I32I32I32_I32 = &signature{
		in:  []unsignedType{unsignedTypeI32, unsignedTypeI32, unsignedTypeI32},
		out: []unsignedType{unsignedTypeI32},
	}
	signature_I32I64I64_I64 = &signature{
		in:  []unsignedType{unsignedTypeI32, unsignedTypeI64, unsignedTypeI64},
		out: []unsignedType{unsignedTypeI64},
	}
)

// wasmOpcodeSignature returns the signature of given Wasm opcode.
// Note that some of opcodes' signature vary depending on
// the function instance (for example, local types).
// "index" parameter is not used by most of opcodes.
// The returned signature is used for stack validation when lowering Wasm's opcodes to interpreterir.
func (c *compiler) wasmOpcodeSignature(op wasm.Opcode, index uint32) (*signature, error) {
	switch op {
	case wasm.OpcodeUnreachable, wasm.OpcodeNop, wasm.OpcodeBlock, wasm.OpcodeLoop:
		return signature_None_None, nil
	case wasm.OpcodeIf:
		return signature_I32_None, nil
	case wasm.OpcodeElse, wasm.OpcodeEnd, wasm.OpcodeBr:
		return signature_None_None, nil
	case wasm.OpcodeBrIf, wasm.OpcodeBrTable:
		return signature_I32_None, nil
	case wasm.OpcodeReturn:
		return signature_None_None, nil
	case wasm.OpcodeCall:
		return c.funcTypeToSigs.get(c.funcs[index], false /* direct */), nil
	case wasm.OpcodeCallIndirect:
		return c.funcTypeToSigs.get(index, true /* call_indirect */), nil
	case wasm.OpcodeDrop:
		return signature_Unknown_None, nil
	case wasm.OpcodeSelect, wasm.OpcodeTypedSelect:
		return signature_UnknownUnknownI32_Unknown, nil
	case wasm.OpcodeLocalGet:
		inputLen := uint32(len(c.sig.Params))
		if l := uint32(len(c.localTypes)) + inputLen; index >= l {
			return nil, fmt.Errorf("invalid local index for local.get %d >= %d", index, l)
		}
		var t wasm.ValueType
		if index < inputLen {
			t = c.sig.Params[index]
		} else {
			t = c.localTypes[index-inputLen]
		}
		return wasmValueTypeToUnsignedOutSignature(t), nil
	case wasm.OpcodeLocalSet:
		inputLen := uint32(len(c.sig.Params))
		if l := uint32(len(c.localTypes)) + inputLen; index >= l {
			return nil, fmt.Errorf("invalid local index for local.get %d >= %d", index, l)
		}
		var t wasm.ValueType
		if index < inputLen {
			t = c.sig.Params[index]
		} else {
			t = c.localTypes[index-inputLen]
		}
		return wasmValueTypeToUnsignedInSignature(t), nil
	case wasm.OpcodeLocalTee:
		inputLen := uint32(len(c.sig.Params))
		if l := uint32(len(c.localTypes)) + inputLen; index >= l {
			return nil, fmt.Errorf("invalid local index for local.get %d >= %d", index, l)
		}
		var t wasm.ValueType
		if index < inputLen {
			t = c.sig.Params[index]
		} else {
			t = c.localTypes[index-inputLen]
		}
		return wasmValueTypeToUnsignedInOutSignature(t), nil
	case wasm.OpcodeGlobalGet:
		if len(c.globals) <= int(index) {
			return nil, fmt.Errorf("invalid global index for global.get %d >= %d", index, len(c.globals))
		}
		return wasmValueTypeToUnsignedOutSignature(c.globals[index].ValType), nil
	case wasm.OpcodeGlobalSet:
		if len(c.globals) <= int(index) {
			return nil, fmt.Errorf("invalid global index for global.get %d >= %d", index, len(c.globals))
		}
		return wasmValueTypeToUnsignedInSignature(c.globals[index].ValType), nil
	case wasm.OpcodeI32Load:
		return signature_I32_I32, nil
	case wasm.OpcodeI64Load:
		return signature_I32_I64, nil
	case wasm.OpcodeF32Load:
		return signature_I32_F32, nil
	case wasm.OpcodeF64Load:
		return signature_I32_F64, nil
	case wasm.OpcodeI32Load8S, wasm.OpcodeI32Load8U, wasm.OpcodeI32Load16S, wasm.OpcodeI32Load16U:
		return signature_I32_I32, nil
	case wasm.OpcodeI64Load8S, wasm.OpcodeI64Load8U, wasm.OpcodeI64Load16S, wasm.OpcodeI64Load16U,
		wasm.OpcodeI64Load32S, wasm.OpcodeI64Load32U:
		return signature_I32_I64, nil
	case wasm.OpcodeI32Store:
		return signature_I32I32_None, nil
	case wasm.OpcodeI64Store:
		return signature_I32I64_None, nil
	case wasm.OpcodeF32Store:
		return signature_I32F32_None, nil
	case wasm.OpcodeF64Store:
		return signature_I32F64_None, nil
	case wasm.OpcodeI32Store8:
		return signature_I32I32_None, nil
	case wasm.OpcodeI32Store16:
		return signature_I32I32_None, nil
	case wasm.OpcodeI64Store8:
		return signature_I32I64_None, nil
	case wasm.OpcodeI64Store16:
		return signature_I32I64_None, nil
	case wasm.OpcodeI64Store32:
		return signature_I32I64_None, nil
	case wasm.OpcodeMemorySize:
		return signature_None_I32, nil
	case wasm.OpcodeMemoryGrow:
		return signature_I32_I32, nil
	case wasm.OpcodeI32Const:
		return signature_None_I32, nil
	case wasm.OpcodeI64Const:
		return signature_None_I64, nil
	case wasm.OpcodeF32Const:
		return signature_None_F32, nil
	case wasm.OpcodeF64Const:
		return signature_None_F64, nil
	case wasm.OpcodeI32Eqz:
		return signature_I32_I32, nil
	case wasm.OpcodeI32Eq, wasm.OpcodeI32Ne, wasm.OpcodeI32LtS,
		wasm.OpcodeI32LtU, wasm.OpcodeI32GtS, wasm.OpcodeI32GtU,
		wasm.OpcodeI32LeS, wasm.OpcodeI32LeU, wasm.OpcodeI32GeS,
		wasm.OpcodeI32GeU:
		return signature_I32I32_I32, nil
	case wasm.OpcodeI64Eqz:
		return signature_I64_I32, nil
	case wasm.OpcodeI64Eq, wasm.OpcodeI64Ne, wasm.OpcodeI64LtS,
		wasm.OpcodeI64LtU, wasm.OpcodeI64GtS, wasm.OpcodeI64GtU,
		wasm.OpcodeI64LeS, wasm.OpcodeI64LeU, wasm.OpcodeI64GeS,
		wasm.OpcodeI64GeU:
		return signature_I64I64_I32, nil
	case wasm.OpcodeF32Eq, wasm.OpcodeF32Ne, wasm.OpcodeF32Lt,
		wasm.OpcodeF32Gt, wasm.OpcodeF32Le, wasm.OpcodeF32Ge:
		return signature_F32F32_I32, nil
	case wasm.OpcodeF64Eq, wasm.OpcodeF64Ne, wasm.OpcodeF64Lt,
		wasm.OpcodeF64Gt, wasm.OpcodeF64Le, wasm.OpcodeF64Ge:
		return signature_F64F64_I32, nil
	case wasm.OpcodeI32Clz, wasm.OpcodeI32Ctz, wasm.OpcodeI32Popcnt:
		return signature_I32_I32, nil
	case wasm.OpcodeI32Add, wasm.OpcodeI32Sub, wasm.OpcodeI32Mul,
		wasm.OpcodeI32DivS, wasm.OpcodeI32DivU, wasm.OpcodeI32RemS,
		wasm.OpcodeI32RemU, wasm.OpcodeI32And, wasm.OpcodeI32Or,
		wasm.OpcodeI32Xor, wasm.OpcodeI32Shl, wasm.OpcodeI32ShrS,
		wasm.OpcodeI32ShrU, wasm.OpcodeI32Rotl, wasm.OpcodeI32Rotr:
		return signature_I32I32_I32, nil
	case wasm.OpcodeI64Clz, wasm.OpcodeI64Ctz, wasm.OpcodeI64Popcnt:
		return signature_I64_I64, nil
	case wasm.OpcodeI64Add, wasm.OpcodeI64Sub, wasm.OpcodeI64Mul,
		wasm.OpcodeI64DivS, wasm.OpcodeI64DivU, wasm.OpcodeI64RemS,
		wasm.OpcodeI64RemU, wasm.OpcodeI64And, wasm.OpcodeI64Or,
		wasm.OpcodeI64Xor, wasm.OpcodeI64Shl, wasm.OpcodeI64ShrS,
		wasm.OpcodeI64ShrU, wasm.OpcodeI64Rotl, wasm.OpcodeI64Rotr:
		return signature_I64I64_I64, nil
	case wasm.OpcodeF32Abs, wasm.OpcodeF32Neg, wasm.OpcodeF32Ceil,
		wasm.OpcodeF32Floor, wasm.OpcodeF32Trunc, wasm.OpcodeF32Nearest,
		wasm.OpcodeF32Sqrt:
		return signature_F32_F32, nil
	case wasm.OpcodeF32Add, wasm.OpcodeF32Sub, wasm.OpcodeF32Mul,
		wasm.OpcodeF32Div, wasm.OpcodeF32Min, wasm.OpcodeF32Max,
		wasm.OpcodeF32Copysign:
		return signature_F32F32_F32, nil
	case wasm.OpcodeF64Abs, wasm.OpcodeF64Neg, wasm.OpcodeF64Ceil,
		wasm.OpcodeF64Floor, wasm.OpcodeF64Trunc, wasm.OpcodeF64Nearest,
		wasm.OpcodeF64Sqrt:
		return signature_F64_F64, nil
	case wasm.OpcodeF64Add, wasm.OpcodeF64Sub, wasm.OpcodeF64Mul,
		wasm.OpcodeF64Div, wasm.OpcodeF64Min, wasm.OpcodeF64Max,
		wasm.OpcodeF64Copysign:
		return signature_F64F64_F64, nil
	case wasm.OpcodeI32WrapI64:
		return signature_I64_I32, nil
	case wasm.OpcodeI32TruncF32S, wasm.OpcodeI32TruncF32U:
		return signature_F32_I32, nil
	case wasm.OpcodeI32TruncF64S, wasm.OpcodeI32TruncF64U:
		return signature_F64_I32, nil
	case wasm.OpcodeI64ExtendI32S, wasm.OpcodeI64ExtendI32U:
		return signature_I32_I64, nil
	case wasm.OpcodeI64TruncF32S, wasm.OpcodeI64TruncF32U:
		return signature_F32_I64, nil
	case wasm.OpcodeI64TruncF64S, wasm.OpcodeI64TruncF64U:
		return signature_F64_I64, nil
	case wasm.OpcodeF32ConvertI32S, wasm.OpcodeF32ConvertI32U:
		return signature_I32_F32, nil
	case wasm.OpcodeF32ConvertI64S, wasm.OpcodeF32ConvertI64U:
		return signature_I64_F32, nil
	case wasm.OpcodeF32DemoteF64:
		return signature_F64_F32, nil
	case wasm.OpcodeF64ConvertI32S, wasm.OpcodeF64ConvertI32U:
		return signature_I32_F64, nil
	case wasm.OpcodeF64ConvertI64S, wasm.OpcodeF64ConvertI64U:
		return signature_I64_F64, nil
	case wasm.OpcodeF64PromoteF32:
		return signature_F32_F64, nil
	case wasm.OpcodeI32ReinterpretF32:
		return signature_F32_I32, nil
	case wasm.OpcodeI64ReinterpretF64:
		return signature_F64_I64, nil
	case wasm.OpcodeF32ReinterpretI32:
		return signature_I32_F32, nil
	case wasm.OpcodeF64ReinterpretI64:
		return signature_I64_F64, nil
	case wasm.OpcodeI32Extend8S, wasm.OpcodeI32Extend16S:
		return signature_I32_I32, nil
	case wasm.OpcodeI64Extend8S, wasm.OpcodeI64Extend16S, wasm.OpcodeI64Extend32S:
		return signature_I64_I64, nil
	case wasm.OpcodeTableGet:
		// table.get takes table's offset and pushes the ref type value of opaque pointer as i64 value onto the stack.
		return signature_I32_I64, nil
	case wasm.OpcodeTableSet:
		// table.set takes table's offset and the ref type value of opaque pointer as i64 value.
		return signature_I32I64_None, nil
	case wasm.OpcodeRefFunc:
		// ref.func is translated as pushing the compiled function's opaque pointer (uint64) at interpreterir layer.
		return signature_None_I64, nil
	case wasm.OpcodeRefIsNull:
		// ref.is_null is translated as checking if the uint64 on the top of the stack (opaque pointer) is zero or not.
		return signature_I64_I32, nil
	case wasm.OpcodeRefNull:
		// ref.null is translated as i64.const 0.
		return signature_None_I64, nil
	case wasm.OpcodeMiscPrefix:
		switch miscOp := c.body[c.pc+1]; miscOp {
		case wasm.OpcodeMiscI32TruncSatF32S, wasm.OpcodeMiscI32TruncSatF32U:
			return signature_F32_I32, nil
		case wasm.OpcodeMiscI32TruncSatF64S, wasm.OpcodeMiscI32TruncSatF64U:
			return signature_F64_I32, nil
		case wasm.OpcodeMiscI64TruncSatF32S, wasm.OpcodeMiscI64TruncSatF32U:
			return signature_F32_I64, nil
		case wasm.OpcodeMiscI64TruncSatF64S, wasm.OpcodeMiscI64TruncSatF64U:
			return signature_F64_I64, nil
		case wasm.OpcodeMiscMemoryInit, wasm.OpcodeMiscMemoryCopy, wasm.OpcodeMiscMemoryFill,
			wasm.OpcodeMiscTableInit, wasm.OpcodeMiscTableCopy:
			return signature_I32I32I32_None, nil
		case wasm.OpcodeMiscDataDrop, wasm.OpcodeMiscElemDrop:
			return signature_None_None, nil
		case wasm.OpcodeMiscTableGrow:
			return signature_I64I32_I32, nil
		case wasm.OpcodeMiscTableSize:
			return signature_None_I32, nil
		case wasm.OpcodeMiscTableFill:
			return signature_I32I64I32_None, nil
		default:
			return nil, fmt.Errorf("unsupported misc instruction in interpreterir: 0x%x", op)
		}
	case wasm.OpcodeVecPrefix:
		switch vecOp := c.body[c.pc+1]; vecOp {
		case wasm.OpcodeVecV128Const:
			return signature_None_V128, nil
		case wasm.OpcodeVecV128Load, wasm.OpcodeVecV128Load8x8s, wasm.OpcodeVecV128Load8x8u,
			wasm.OpcodeVecV128Load16x4s, wasm.OpcodeVecV128Load16x4u, wasm.OpcodeVecV128Load32x2s,
			wasm.OpcodeVecV128Load32x2u, wasm.OpcodeVecV128Load8Splat, wasm.OpcodeVecV128Load16Splat,
			wasm.OpcodeVecV128Load32Splat, wasm.OpcodeVecV128Load64Splat, wasm.OpcodeVecV128Load32zero,
			wasm.OpcodeVecV128Load64zero:
			return signature_I32_V128, nil
		case wasm.OpcodeVecV128Load8Lane, wasm.OpcodeVecV128Load16Lane,
			wasm.OpcodeVecV128Load32Lane, wasm.OpcodeVecV128Load64Lane:
			return signature_I32V128_V128, nil
		case wasm.OpcodeVecV128Store,
			wasm.OpcodeVecV128Store8Lane,
			wasm.OpcodeVecV128Store16Lane,
			wasm.OpcodeVecV128Store32Lane,
			wasm.OpcodeVecV128Store64Lane:
			return signature_I32V128_None, nil
		case wasm.OpcodeVecI8x16ExtractLaneS,
			wasm.OpcodeVecI8x16ExtractLaneU,
			wasm.OpcodeVecI16x8ExtractLaneS,
			wasm.OpcodeVecI16x8ExtractLaneU,
			wasm.OpcodeVecI32x4ExtractLane:
			return signature_V128_I32, nil
		case wasm.OpcodeVecI64x2ExtractLane:
			return signature_V128_I64, nil
		case wasm.OpcodeVecF32x4ExtractLane:
			return signature_V128_F32, nil
		case wasm.OpcodeVecF64x2ExtractLane:
			return signature_V128_F64, nil
		case wasm.OpcodeVecI8x16ReplaceLane, wasm.OpcodeVecI16x8ReplaceLane, wasm.OpcodeVecI32x4ReplaceLane,
			wasm.OpcodeVecI8x16Shl, wasm.OpcodeVecI8x16ShrS, wasm.OpcodeVecI8x16ShrU,
			wasm.OpcodeVecI16x8Shl, wasm.OpcodeVecI16x8ShrS, wasm.OpcodeVecI16x8ShrU,
			wasm.OpcodeVecI32x4Shl, wasm.OpcodeVecI32x4ShrS, wasm.OpcodeVecI32x4ShrU,
			wasm.OpcodeVecI64x2Shl, wasm.OpcodeVecI64x2ShrS, wasm.OpcodeVecI64x2ShrU:
			return signature_V128I32_V128, nil
		case wasm.OpcodeVecI64x2ReplaceLane:
			return signature_V128I64_V128, nil
		case wasm.OpcodeVecF32x4ReplaceLane:
			return signature_V128F32_V128, nil
		case wasm.OpcodeVecF64x2ReplaceLane:
			return signature_V128F64_V128, nil
		case wasm.OpcodeVecI8x16Splat,
			wasm.OpcodeVecI16x8Splat,
			wasm.OpcodeVecI32x4Splat:
			return signature_I32_V128, nil
		case wasm.OpcodeVecI64x2Splat:
			return signature_I64_V128, nil
		case wasm.OpcodeVecF32x4Splat:
			return signature_F32_V128, nil
		case wasm.OpcodeVecF64x2Splat:
			return signature_F64_V128, nil
		case wasm.OpcodeVecV128i8x16Shuffle, wasm.OpcodeVecI8x16Swizzle, wasm.OpcodeVecV128And, wasm.OpcodeVecV128Or, wasm.OpcodeVecV128Xor, wasm.OpcodeVecV128AndNot:
			return signature_V128V128_V128, nil
		case wasm.OpcodeVecI8x16AllTrue, wasm.OpcodeVecI16x8AllTrue, wasm.OpcodeVecI32x4AllTrue, wasm.OpcodeVecI64x2AllTrue,
			wasm.OpcodeVecV128AnyTrue,
			wasm.OpcodeVecI8x16BitMask, wasm.OpcodeVecI16x8BitMask, wasm.OpcodeVecI32x4BitMask, wasm.OpcodeVecI64x2BitMask:
			return signature_V128_I32, nil
		case wasm.OpcodeVecV128Not, wasm.OpcodeVecI8x16Neg, wasm.OpcodeVecI16x8Neg, wasm.OpcodeVecI32x4Neg, wasm.OpcodeVecI64x2Neg,
			wasm.OpcodeVecF32x4Neg, wasm.OpcodeVecF64x2Neg, wasm.OpcodeVecF32x4Sqrt, wasm.OpcodeVecF64x2Sqrt,
			wasm.OpcodeVecI8x16Abs, wasm.OpcodeVecI8x16Popcnt, wasm.OpcodeVecI16x8Abs, wasm.OpcodeVecI32x4Abs, wasm.OpcodeVecI64x2Abs,
			wasm.OpcodeVecF32x4Abs, wasm.OpcodeVecF64x2Abs,
			wasm.OpcodeVecF32x4Ceil, wasm.OpcodeVecF32x4Floor, wasm.OpcodeVecF32x4Trunc, wasm.OpcodeVecF32x4Nearest,
			wasm.OpcodeVecF64x2Ceil, wasm.OpcodeVecF64x2Floor, wasm.OpcodeVecF64x2Trunc, wasm.OpcodeVecF64x2Nearest,
			wasm.OpcodeVecI16x8ExtendLowI8x16S, wasm.OpcodeVecI16x8ExtendHighI8x16S, wasm.OpcodeVecI16x8ExtendLowI8x16U, wasm.OpcodeVecI16x8ExtendHighI8x16U,
			wasm.OpcodeVecI32x4ExtendLowI16x8S, wasm.OpcodeVecI32x4ExtendHighI16x8S, wasm.OpcodeVecI32x4ExtendLowI16x8U, wasm.OpcodeVecI32x4ExtendHighI16x8U,
			wasm.OpcodeVecI64x2ExtendLowI32x4S, wasm.OpcodeVecI64x2ExtendHighI32x4S, wasm.OpcodeVecI64x2ExtendLowI32x4U, wasm.OpcodeVecI64x2ExtendHighI32x4U,
			wasm.OpcodeVecI16x8ExtaddPairwiseI8x16S, wasm.OpcodeVecI16x8ExtaddPairwiseI8x16U, wasm.OpcodeVecI32x4ExtaddPairwiseI16x8S, wasm.OpcodeVecI32x4ExtaddPairwiseI16x8U,
			wasm.OpcodeVecF64x2PromoteLowF32x4Zero, wasm.OpcodeVecF32x4DemoteF64x2Zero,
			wasm.OpcodeVecF32x4ConvertI32x4S, wasm.OpcodeVecF32x4ConvertI32x4U,
			wasm.OpcodeVecF64x2ConvertLowI32x4S, wasm.OpcodeVecF64x2ConvertLowI32x4U,
			wasm.OpcodeVecI32x4TruncSatF32x4S, wasm.OpcodeVecI32x4TruncSatF32x4U,
			wasm.OpcodeVecI32x4TruncSatF64x2SZero, wasm.OpcodeVecI32x4TruncSatF64x2UZero:
			return signature_V128_V128, nil
		case wasm.OpcodeVecV128Bitselect:
			return signature_V128V128V128_V32, nil
		case wasm.OpcodeVecI8x16Eq, wasm.OpcodeVecI8x16Ne, wasm.OpcodeVecI8x16LtS, wasm.OpcodeVecI8x16LtU, wasm.OpcodeVecI8x16GtS,
			wasm.OpcodeVecI8x16GtU, wasm.OpcodeVecI8x16LeS, wasm.OpcodeVecI8x16LeU, wasm.OpcodeVecI8x16GeS, wasm.OpcodeVecI8x16GeU,
			wasm.OpcodeVecI16x8Eq, wasm.OpcodeVecI16x8Ne, wasm.OpcodeVecI16x8LtS, wasm.OpcodeVecI16x8LtU, wasm.OpcodeVecI16x8GtS,
			wasm.OpcodeVecI16x8GtU, wasm.OpcodeVecI16x8LeS, wasm.OpcodeVecI16x8LeU, wasm.OpcodeVecI16x8GeS, wasm.OpcodeVecI16x8GeU,
			wasm.OpcodeVecI32x4Eq, wasm.OpcodeVecI32x4Ne, wasm.OpcodeVecI32x4LtS, wasm.OpcodeVecI32x4LtU, wasm.OpcodeVecI32x4GtS,
			wasm.OpcodeVecI32x4GtU, wasm.OpcodeVecI32x4LeS, wasm.OpcodeVecI32x4LeU, wasm.OpcodeVecI32x4GeS, wasm.OpcodeVecI32x4GeU,
			wasm.OpcodeVecI64x2Eq, wasm.OpcodeVecI64x2Ne, wasm.OpcodeVecI64x2LtS, wasm.OpcodeVecI64x2GtS, wasm.OpcodeVecI64x2LeS,
			wasm.OpcodeVecI64x2GeS, wasm.OpcodeVecF32x4Eq, wasm.OpcodeVecF32x4Ne, wasm.OpcodeVecF32x4Lt, wasm.OpcodeVecF32x4Gt,
			wasm.OpcodeVecF32x4Le, wasm.OpcodeVecF32x4Ge, wasm.OpcodeVecF64x2Eq, wasm.OpcodeVecF64x2Ne, wasm.OpcodeVecF64x2Lt,
			wasm.OpcodeVecF64x2Gt, wasm.OpcodeVecF64x2Le, wasm.OpcodeVecF64x2Ge,
			wasm.OpcodeVecI8x16Add, wasm.OpcodeVecI8x16AddSatS, wasm.OpcodeVecI8x16AddSatU, wasm.OpcodeVecI8x16Sub,
			wasm.OpcodeVecI8x16SubSatS, wasm.OpcodeVecI8x16SubSatU,
			wasm.OpcodeVecI16x8Add, wasm.OpcodeVecI16x8AddSatS, wasm.OpcodeVecI16x8AddSatU, wasm.OpcodeVecI16x8Sub,
			wasm.OpcodeVecI16x8SubSatS, wasm.OpcodeVecI16x8SubSatU, wasm.OpcodeVecI16x8Mul,
			wasm.OpcodeVecI32x4Add, wasm.OpcodeVecI32x4Sub, wasm.OpcodeVecI32x4Mul,
			wasm.OpcodeVecI64x2Add, wasm.OpcodeVecI64x2Sub, wasm.OpcodeVecI64x2Mul,
			wasm.OpcodeVecF32x4Add, wasm.OpcodeVecF32x4Sub, wasm.OpcodeVecF32x4Mul, wasm.OpcodeVecF32x4Div,
			wasm.OpcodeVecF64x2Add, wasm.OpcodeVecF64x2Sub, wasm.OpcodeVecF64x2Mul, wasm.OpcodeVecF64x2Div,
			wasm.OpcodeVecI8x16MinS, wasm.OpcodeVecI8x16MinU, wasm.OpcodeVecI8x16MaxS, wasm.OpcodeVecI8x16MaxU, wasm.OpcodeVecI8x16AvgrU,
			wasm.OpcodeVecI16x8MinS, wasm.OpcodeVecI16x8MinU, wasm.OpcodeVecI16x8MaxS, wasm.OpcodeVecI16x8MaxU, wasm.OpcodeVecI16x8AvgrU,
			wasm.OpcodeVecI32x4MinS, wasm.OpcodeVecI32x4MinU, wasm.OpcodeVecI32x4MaxS, wasm.OpcodeVecI32x4MaxU,
			wasm.OpcodeVecF32x4Min, wasm.OpcodeVecF32x4Max, wasm.OpcodeVecF64x2Min, wasm.OpcodeVecF64x2Max,
			wasm.OpcodeVecF32x4Pmin, wasm.OpcodeVecF32x4Pmax, wasm.OpcodeVecF64x2Pmin, wasm.OpcodeVecF64x2Pmax,
			wasm.OpcodeVecI16x8Q15mulrSatS,
			wasm.OpcodeVecI16x8ExtMulLowI8x16S, wasm.OpcodeVecI16x8ExtMulHighI8x16S, wasm.OpcodeVecI16x8ExtMulLowI8x16U, wasm.OpcodeVecI16x8ExtMulHighI8x16U,
			wasm.OpcodeVecI32x4ExtMulLowI16x8S, wasm.OpcodeVecI32x4ExtMulHighI16x8S, wasm.OpcodeVecI32x4ExtMulLowI16x8U, wasm.OpcodeVecI32x4ExtMulHighI16x8U,
			wasm.OpcodeVecI64x2ExtMulLowI32x4S, wasm.OpcodeVecI64x2ExtMulHighI32x4S, wasm.OpcodeVecI64x2ExtMulLowI32x4U, wasm.OpcodeVecI64x2ExtMulHighI32x4U,
			wasm.OpcodeVecI32x4DotI16x8S,
			wasm.OpcodeVecI8x16NarrowI16x8S, wasm.OpcodeVecI8x16NarrowI16x8U, wasm.OpcodeVecI16x8NarrowI32x4S, wasm.OpcodeVecI16x8NarrowI32x4U:
			return signature_V128V128_V128, nil
		default:
			return nil, fmt.Errorf("unsupported vector instruction in interpreterir: %s", wasm.VectorInstructionName(vecOp))
		}
	case wasm.OpcodeAtomicPrefix:
		switch atomicOp := c.body[c.pc+1]; atomicOp {
		case wasm.OpcodeAtomicMemoryNotify:
			return signature_I32I32_I32, nil
		case wasm.OpcodeAtomicMemoryWait32:
			return signature_I32I32I64_I32, nil
		case wasm.OpcodeAtomicMemoryWait64:
			return signature_I32I64I64_I32, nil
		case wasm.OpcodeAtomicFence:
			return signature_None_None, nil
		case wasm.OpcodeAtomicI32Load, wasm.OpcodeAtomicI32Load8U, wasm.OpcodeAtomicI32Load16U:
			return signature_I32_I32, nil
		case wasm.OpcodeAtomicI64Load, wasm.OpcodeAtomicI64Load8U, wasm.OpcodeAtomicI64Load16U, wasm.OpcodeAtomicI64Load32U:
			return signature_I32_I64, nil
		case wasm.OpcodeAtomicI32Store, wasm.OpcodeAtomicI32Store8, wasm.OpcodeAtomicI32Store16:
			return signature_I32I32_None, nil
		case wasm.OpcodeAtomicI64Store, wasm.OpcodeAtomicI64Store8, wasm.OpcodeAtomicI64Store16, wasm.OpcodeAtomicI64Store32:
			return signature_I32I64_None, nil
		case wasm.OpcodeAtomicI32RmwAdd, wasm.OpcodeAtomicI32RmwSub, wasm.OpcodeAtomicI32RmwAnd, wasm.OpcodeAtomicI32RmwOr, wasm.OpcodeAtomicI32RmwXor, wasm.OpcodeAtomicI32RmwXchg,
			wasm.OpcodeAtomicI32Rmw8AddU, wasm.OpcodeAtomicI32Rmw8SubU, wasm.OpcodeAtomicI32Rmw8AndU, wasm.OpcodeAtomicI32Rmw8OrU, wasm.OpcodeAtomicI32Rmw8XorU, wasm.OpcodeAtomicI32Rmw8XchgU,
			wasm.OpcodeAtomicI32Rmw16AddU, wasm.OpcodeAtomicI32Rmw16SubU, wasm.OpcodeAtomicI32Rmw16AndU, wasm.OpcodeAtomicI32Rmw16OrU, wasm.OpcodeAtomicI32Rmw16XorU, wasm.OpcodeAtomicI32Rmw16XchgU:
			return signature_I32I32_I32, nil
		case wasm.OpcodeAtomicI64RmwAdd, wasm.OpcodeAtomicI64RmwSub, wasm.OpcodeAtomicI64RmwAnd, wasm.OpcodeAtomicI64RmwOr, wasm.OpcodeAtomicI64RmwXor, wasm.OpcodeAtomicI64RmwXchg,
			wasm.OpcodeAtomicI64Rmw8AddU, wasm.OpcodeAtomicI64Rmw8SubU, wasm.OpcodeAtomicI64Rmw8AndU, wasm.OpcodeAtomicI64Rmw8OrU, wasm.OpcodeAtomicI64Rmw8XorU, wasm.OpcodeAtomicI64Rmw8XchgU,
			wasm.OpcodeAtomicI64Rmw16AddU, wasm.OpcodeAtomicI64Rmw16SubU, wasm.OpcodeAtomicI64Rmw16AndU, wasm.OpcodeAtomicI64Rmw16OrU, wasm.OpcodeAtomicI64Rmw16XorU, wasm.OpcodeAtomicI64Rmw16XchgU,
			wasm.OpcodeAtomicI64Rmw32AddU, wasm.OpcodeAtomicI64Rmw32SubU, wasm.OpcodeAtomicI64Rmw32AndU, wasm.OpcodeAtomicI64Rmw32OrU, wasm.OpcodeAtomicI64Rmw32XorU, wasm.OpcodeAtomicI64Rmw32XchgU:
			return signature_I32I64_I64, nil
		case wasm.OpcodeAtomicI32RmwCmpxchg, wasm.OpcodeAtomicI32Rmw8CmpxchgU, wasm.OpcodeAtomicI32Rmw16CmpxchgU:
			return signature_I32I32I32_I32, nil
		case wasm.OpcodeAtomicI64RmwCmpxchg, wasm.OpcodeAtomicI64Rmw8CmpxchgU, wasm.OpcodeAtomicI64Rmw16CmpxchgU, wasm.OpcodeAtomicI64Rmw32CmpxchgU:
			return signature_I32I64I64_I64, nil
		default:
			return nil, fmt.Errorf("unsupported atomic instruction in interpreterir: %s", wasm.AtomicInstructionName(atomicOp))
		}
	default:
		return nil, fmt.Errorf("unsupported instruction in interpreterir: 0x%x", op)
	}
}

// funcTypeToIRSignatures is the central cache for a module to get the *signature
// for function calls.
type funcTypeToIRSignatures struct {
	directCalls   []*signature
	indirectCalls []*signature
	wasmTypes     []wasm.FunctionType
}

// get returns the *signature for the direct or indirect function call against functions whose type is at `typeIndex`.
func (f *funcTypeToIRSignatures) get(typeIndex wasm.Index, indirect bool) *signature {
	var sig *signature
	if indirect {
		sig = f.indirectCalls[typeIndex]
	} else {
		sig = f.directCalls[typeIndex]
	}
	if sig != nil {
		return sig
	}

	tp := &f.wasmTypes[typeIndex]
	if indirect {
		sig = &signature{
			in:  make([]unsignedType, 0, len(tp.Params)+1), // +1 to reserve space for call indirect index.
			out: make([]unsignedType, 0, len(tp.Results)),
		}
	} else {
		sig = &signature{
			in:  make([]unsignedType, 0, len(tp.Params)),
			out: make([]unsignedType, 0, len(tp.Results)),
		}
	}

	for _, vt := range tp.Params {
		sig.in = append(sig.in, wasmValueTypeTounsignedType(vt))
	}
	for _, vt := range tp.Results {
		sig.out = append(sig.out, wasmValueTypeTounsignedType(vt))
	}

	if indirect {
		sig.in = append(sig.in, unsignedTypeI32)
		f.indirectCalls[typeIndex] = sig
	} else {
		f.directCalls[typeIndex] = sig
	}
	return sig
}

func wasmValueTypeTounsignedType(vt wasm.ValueType) unsignedType {
	switch vt {
	case wasm.ValueTypeI32:
		return unsignedTypeI32
	case wasm.ValueTypeI64,
		// From interpreterir layer, ref type values are opaque 64-bit pointers.
		wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
		return unsignedTypeI64
	case wasm.ValueTypeF32:
		return unsignedTypeF32
	case wasm.ValueTypeF64:
		return unsignedTypeF64
	case wasm.ValueTypeV128:
		return unsignedTypeV128
	}
	panic("unreachable")
}

func wasmValueTypeToUnsignedOutSignature(vt wasm.ValueType) *signature {
	switch vt {
	case wasm.ValueTypeI32:
		return signature_None_I32
	case wasm.ValueTypeI64,
		// From interpreterir layer, ref type values are opaque 64-bit pointers.
		wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
		return signature_None_I64
	case wasm.ValueTypeF32:
		return signature_None_F32
	case wasm.ValueTypeF64:
		return signature_None_F64
	case wasm.ValueTypeV128:
		return signature_None_V128
	}
	panic("unreachable")
}

func wasmValueTypeToUnsignedInSignature(vt wasm.ValueType) *signature {
	switch vt {
	case wasm.ValueTypeI32:
		return signature_I32_None
	case wasm.ValueTypeI64,
		// From interpreterir layer, ref type values are opaque 64-bit pointers.
		wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
		return signature_I64_None
	case wasm.ValueTypeF32:
		return signature_F32_None
	case wasm.ValueTypeF64:
		return signature_F64_None
	case wasm.ValueTypeV128:
		return signature_V128_None
	}
	panic("unreachable")
}

func wasmValueTypeToUnsignedInOutSignature(vt wasm.ValueType) *signature {
	switch vt {
	case wasm.ValueTypeI32:
		return signature_I32_I32
	case wasm.ValueTypeI64,
		// At interpreterir layer, ref type values are opaque 64-bit pointers.
		wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
		return signature_I64_I64
	case wasm.ValueTypeF32:
		return signature_F32_F32
	case wasm.ValueTypeF64:
		return signature_F64_F64
	case wasm.ValueTypeV128:
		return signature_V128_V128
	}
	panic("unreachable")
}
