package wasm

// Opcode is the binary Opcode of an instruction. See also InstructionName
type Opcode = byte

const (
	// OpcodeUnreachable causes an unconditional trap.
	OpcodeUnreachable Opcode = 0x00
	// OpcodeNop does nothing
	OpcodeNop Opcode = 0x01
	// OpcodeBlock brackets a sequence of instructions. A branch instruction on an if label breaks out to after its
	// OpcodeEnd.
	OpcodeBlock Opcode = 0x02
	// OpcodeLoop brackets a sequence of instructions. A branch instruction on a loop label will jump back to the
	// beginning of its block.
	OpcodeLoop Opcode = 0x03
	// OpcodeIf brackets a sequence of instructions. When the top of the stack evaluates to 1, the block is executed.
	// Zero jumps to the optional OpcodeElse. A branch instruction on an if label breaks out to after its OpcodeEnd.
	OpcodeIf Opcode = 0x04
	// OpcodeElse brackets a sequence of instructions enclosed by an OpcodeIf. A branch instruction on a then label
	// breaks out to after the OpcodeEnd on the enclosing OpcodeIf.
	OpcodeElse Opcode = 0x05
	// OpcodeEnd terminates a control instruction OpcodeBlock, OpcodeLoop or OpcodeIf.
	OpcodeEnd Opcode = 0x0b

	// OpcodeBr is a stack-polymorphic opcode that performs an unconditional branch. How the stack is modified depends
	// on whether the "br" is enclosed by a loop, and if CoreFeatureMultiValue is enabled.
	//
	// Here are the rules in pseudocode about how the stack is modified based on the "br" operand L (label):
	//	if L is loop: append(L.originalStackWithoutInputs, N-values popped from the stack) where N == L.inputs
	//	else: append(L.originalStackWithoutInputs, N-values popped from the stack) where N == L.results
	//
	// In WebAssembly 1.0 (20191205), N can be zero or one. When CoreFeatureMultiValue is enabled, N can be more than one,
	// depending on the type use of the label L.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#-hrefsyntax-instr-controlmathsfbrl
	OpcodeBr Opcode = 0x0c

	OpcodeBrIf         Opcode = 0x0d
	OpcodeBrTable      Opcode = 0x0e
	OpcodeReturn       Opcode = 0x0f
	OpcodeCall         Opcode = 0x10
	OpcodeCallIndirect Opcode = 0x11

	// parametric instructions

	OpcodeDrop        Opcode = 0x1a
	OpcodeSelect      Opcode = 0x1b
	OpcodeTypedSelect Opcode = 0x1c

	// variable instructions

	OpcodeLocalGet  Opcode = 0x20
	OpcodeLocalSet  Opcode = 0x21
	OpcodeLocalTee  Opcode = 0x22
	OpcodeGlobalGet Opcode = 0x23
	OpcodeGlobalSet Opcode = 0x24

	// Below are toggled with CoreFeatureReferenceTypes

	OpcodeTableGet Opcode = 0x25
	OpcodeTableSet Opcode = 0x26

	// memory instructions

	OpcodeI32Load    Opcode = 0x28
	OpcodeI64Load    Opcode = 0x29
	OpcodeF32Load    Opcode = 0x2a
	OpcodeF64Load    Opcode = 0x2b
	OpcodeI32Load8S  Opcode = 0x2c
	OpcodeI32Load8U  Opcode = 0x2d
	OpcodeI32Load16S Opcode = 0x2e
	OpcodeI32Load16U Opcode = 0x2f
	OpcodeI64Load8S  Opcode = 0x30
	OpcodeI64Load8U  Opcode = 0x31
	OpcodeI64Load16S Opcode = 0x32
	OpcodeI64Load16U Opcode = 0x33
	OpcodeI64Load32S Opcode = 0x34
	OpcodeI64Load32U Opcode = 0x35
	OpcodeI32Store   Opcode = 0x36
	OpcodeI64Store   Opcode = 0x37
	OpcodeF32Store   Opcode = 0x38
	OpcodeF64Store   Opcode = 0x39
	OpcodeI32Store8  Opcode = 0x3a
	OpcodeI32Store16 Opcode = 0x3b
	OpcodeI64Store8  Opcode = 0x3c
	OpcodeI64Store16 Opcode = 0x3d
	OpcodeI64Store32 Opcode = 0x3e
	OpcodeMemorySize Opcode = 0x3f
	OpcodeMemoryGrow Opcode = 0x40

	// const instructions

	OpcodeI32Const Opcode = 0x41
	OpcodeI64Const Opcode = 0x42
	OpcodeF32Const Opcode = 0x43
	OpcodeF64Const Opcode = 0x44

	// numeric instructions

	OpcodeI32Eqz Opcode = 0x45
	OpcodeI32Eq  Opcode = 0x46
	OpcodeI32Ne  Opcode = 0x47
	OpcodeI32LtS Opcode = 0x48
	OpcodeI32LtU Opcode = 0x49
	OpcodeI32GtS Opcode = 0x4a
	OpcodeI32GtU Opcode = 0x4b
	OpcodeI32LeS Opcode = 0x4c
	OpcodeI32LeU Opcode = 0x4d
	OpcodeI32GeS Opcode = 0x4e
	OpcodeI32GeU Opcode = 0x4f

	OpcodeI64Eqz Opcode = 0x50
	OpcodeI64Eq  Opcode = 0x51
	OpcodeI64Ne  Opcode = 0x52
	OpcodeI64LtS Opcode = 0x53
	OpcodeI64LtU Opcode = 0x54
	OpcodeI64GtS Opcode = 0x55
	OpcodeI64GtU Opcode = 0x56
	OpcodeI64LeS Opcode = 0x57
	OpcodeI64LeU Opcode = 0x58
	OpcodeI64GeS Opcode = 0x59
	OpcodeI64GeU Opcode = 0x5a

	OpcodeF32Eq Opcode = 0x5b
	OpcodeF32Ne Opcode = 0x5c
	OpcodeF32Lt Opcode = 0x5d
	OpcodeF32Gt Opcode = 0x5e
	OpcodeF32Le Opcode = 0x5f
	OpcodeF32Ge Opcode = 0x60

	OpcodeF64Eq Opcode = 0x61
	OpcodeF64Ne Opcode = 0x62
	OpcodeF64Lt Opcode = 0x63
	OpcodeF64Gt Opcode = 0x64
	OpcodeF64Le Opcode = 0x65
	OpcodeF64Ge Opcode = 0x66

	OpcodeI32Clz    Opcode = 0x67
	OpcodeI32Ctz    Opcode = 0x68
	OpcodeI32Popcnt Opcode = 0x69
	OpcodeI32Add    Opcode = 0x6a
	OpcodeI32Sub    Opcode = 0x6b
	OpcodeI32Mul    Opcode = 0x6c
	OpcodeI32DivS   Opcode = 0x6d
	OpcodeI32DivU   Opcode = 0x6e
	OpcodeI32RemS   Opcode = 0x6f
	OpcodeI32RemU   Opcode = 0x70
	OpcodeI32And    Opcode = 0x71
	OpcodeI32Or     Opcode = 0x72
	OpcodeI32Xor    Opcode = 0x73
	OpcodeI32Shl    Opcode = 0x74
	OpcodeI32ShrS   Opcode = 0x75
	OpcodeI32ShrU   Opcode = 0x76
	OpcodeI32Rotl   Opcode = 0x77
	OpcodeI32Rotr   Opcode = 0x78

	OpcodeI64Clz    Opcode = 0x79
	OpcodeI64Ctz    Opcode = 0x7a
	OpcodeI64Popcnt Opcode = 0x7b
	OpcodeI64Add    Opcode = 0x7c
	OpcodeI64Sub    Opcode = 0x7d
	OpcodeI64Mul    Opcode = 0x7e
	OpcodeI64DivS   Opcode = 0x7f
	OpcodeI64DivU   Opcode = 0x80
	OpcodeI64RemS   Opcode = 0x81
	OpcodeI64RemU   Opcode = 0x82
	OpcodeI64And    Opcode = 0x83
	OpcodeI64Or     Opcode = 0x84
	OpcodeI64Xor    Opcode = 0x85
	OpcodeI64Shl    Opcode = 0x86
	OpcodeI64ShrS   Opcode = 0x87
	OpcodeI64ShrU   Opcode = 0x88
	OpcodeI64Rotl   Opcode = 0x89
	OpcodeI64Rotr   Opcode = 0x8a

	OpcodeF32Abs      Opcode = 0x8b
	OpcodeF32Neg      Opcode = 0x8c
	OpcodeF32Ceil     Opcode = 0x8d
	OpcodeF32Floor    Opcode = 0x8e
	OpcodeF32Trunc    Opcode = 0x8f
	OpcodeF32Nearest  Opcode = 0x90
	OpcodeF32Sqrt     Opcode = 0x91
	OpcodeF32Add      Opcode = 0x92
	OpcodeF32Sub      Opcode = 0x93
	OpcodeF32Mul      Opcode = 0x94
	OpcodeF32Div      Opcode = 0x95
	OpcodeF32Min      Opcode = 0x96
	OpcodeF32Max      Opcode = 0x97
	OpcodeF32Copysign Opcode = 0x98

	OpcodeF64Abs      Opcode = 0x99
	OpcodeF64Neg      Opcode = 0x9a
	OpcodeF64Ceil     Opcode = 0x9b
	OpcodeF64Floor    Opcode = 0x9c
	OpcodeF64Trunc    Opcode = 0x9d
	OpcodeF64Nearest  Opcode = 0x9e
	OpcodeF64Sqrt     Opcode = 0x9f
	OpcodeF64Add      Opcode = 0xa0
	OpcodeF64Sub      Opcode = 0xa1
	OpcodeF64Mul      Opcode = 0xa2
	OpcodeF64Div      Opcode = 0xa3
	OpcodeF64Min      Opcode = 0xa4
	OpcodeF64Max      Opcode = 0xa5
	OpcodeF64Copysign Opcode = 0xa6

	OpcodeI32WrapI64   Opcode = 0xa7
	OpcodeI32TruncF32S Opcode = 0xa8
	OpcodeI32TruncF32U Opcode = 0xa9
	OpcodeI32TruncF64S Opcode = 0xaa
	OpcodeI32TruncF64U Opcode = 0xab

	OpcodeI64ExtendI32S Opcode = 0xac
	OpcodeI64ExtendI32U Opcode = 0xad
	OpcodeI64TruncF32S  Opcode = 0xae
	OpcodeI64TruncF32U  Opcode = 0xaf
	OpcodeI64TruncF64S  Opcode = 0xb0
	OpcodeI64TruncF64U  Opcode = 0xb1

	OpcodeF32ConvertI32S Opcode = 0xb2
	OpcodeF32ConvertI32U Opcode = 0xb3
	OpcodeF32ConvertI64S Opcode = 0xb4
	OpcodeF32ConvertI64U Opcode = 0xb5
	OpcodeF32DemoteF64   Opcode = 0xb6

	OpcodeF64ConvertI32S Opcode = 0xb7
	OpcodeF64ConvertI32U Opcode = 0xb8
	OpcodeF64ConvertI64S Opcode = 0xb9
	OpcodeF64ConvertI64U Opcode = 0xba
	OpcodeF64PromoteF32  Opcode = 0xbb

	OpcodeI32ReinterpretF32 Opcode = 0xbc
	OpcodeI64ReinterpretF64 Opcode = 0xbd
	OpcodeF32ReinterpretI32 Opcode = 0xbe
	OpcodeF64ReinterpretI64 Opcode = 0xbf

	// OpcodeRefNull pushes a null reference value whose type is specified by immediate to this opcode.
	// This is defined in the reference-types proposal, but necessary for CoreFeatureBulkMemoryOperations as well.
	//
	// Currently only supported in the constant expression in element segments.
	OpcodeRefNull = 0xd0
	// OpcodeRefIsNull pops a reference value, and pushes 1 if it is null, 0 otherwise.
	// This is defined in the reference-types proposal, but necessary for CoreFeatureBulkMemoryOperations as well.
	//
	// Currently not supported.
	OpcodeRefIsNull = 0xd1
	// OpcodeRefFunc pushes a funcref value whose index equals the immediate to this opcode.
	// This is defined in the reference-types proposal, but necessary for CoreFeatureBulkMemoryOperations as well.
	//
	// Currently, this is only supported in the constant expression in element segments.
	OpcodeRefFunc = 0xd2

	// Below are toggled with CoreFeatureSignExtensionOps

	// OpcodeI32Extend8S extends a signed 8-bit integer to a 32-bit integer.
	// Note: This is dependent on the flag CoreFeatureSignExtensionOps
	OpcodeI32Extend8S Opcode = 0xc0

	// OpcodeI32Extend16S extends a signed 16-bit integer to a 32-bit integer.
	// Note: This is dependent on the flag CoreFeatureSignExtensionOps
	OpcodeI32Extend16S Opcode = 0xc1

	// OpcodeI64Extend8S extends a signed 8-bit integer to a 64-bit integer.
	// Note: This is dependent on the flag CoreFeatureSignExtensionOps
	OpcodeI64Extend8S Opcode = 0xc2

	// OpcodeI64Extend16S extends a signed 16-bit integer to a 64-bit integer.
	// Note: This is dependent on the flag CoreFeatureSignExtensionOps
	OpcodeI64Extend16S Opcode = 0xc3

	// OpcodeI64Extend32S extends a signed 32-bit integer to a 64-bit integer.
	// Note: This is dependent on the flag CoreFeatureSignExtensionOps
	OpcodeI64Extend32S Opcode = 0xc4

	// OpcodeMiscPrefix is the prefix of various multi-byte opcodes.
	// Introduced in CoreFeatureNonTrappingFloatToIntConversion, but used in other
	// features, such as CoreFeatureBulkMemoryOperations.
	OpcodeMiscPrefix Opcode = 0xfc

	// OpcodeVecPrefix is the prefix of all vector isntructions introduced in
	// CoreFeatureSIMD.
	OpcodeVecPrefix Opcode = 0xfd

	// OpcodeAtomicPrefix is the prefix of all atomic instructions introduced in
	// CoreFeatureThreads.
	OpcodeAtomicPrefix Opcode = 0xfe
)

// OpcodeMisc represents opcodes of the miscellaneous operations.
// Such an operations has multi-byte encoding which is prefixed by OpcodeMiscPrefix.
type OpcodeMisc = byte

const (
	// Below are toggled with CoreFeatureNonTrappingFloatToIntConversion.
	// https://github.com/WebAssembly/spec/blob/ce4b6c4d47eb06098cc7ab2e81f24748da822f20/proposals/nontrapping-float-to-int-conversion/Overview.md

	OpcodeMiscI32TruncSatF32S OpcodeMisc = 0x00
	OpcodeMiscI32TruncSatF32U OpcodeMisc = 0x01
	OpcodeMiscI32TruncSatF64S OpcodeMisc = 0x02
	OpcodeMiscI32TruncSatF64U OpcodeMisc = 0x03
	OpcodeMiscI64TruncSatF32S OpcodeMisc = 0x04
	OpcodeMiscI64TruncSatF32U OpcodeMisc = 0x05
	OpcodeMiscI64TruncSatF64S OpcodeMisc = 0x06
	OpcodeMiscI64TruncSatF64U OpcodeMisc = 0x07

	// Below are toggled with CoreFeatureBulkMemoryOperations.
	// Opcodes are those new in document/core/appendix/index-instructions.rst (the commit that merged the feature).
	// See https://github.com/WebAssembly/spec/commit/7fa2f20a6df4cf1c114582c8cb60f5bfcdbf1be1
	// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/appendix/changes.html#bulk-memory-and-table-instructions

	OpcodeMiscMemoryInit OpcodeMisc = 0x08
	OpcodeMiscDataDrop   OpcodeMisc = 0x09
	OpcodeMiscMemoryCopy OpcodeMisc = 0x0a
	OpcodeMiscMemoryFill OpcodeMisc = 0x0b
	OpcodeMiscTableInit  OpcodeMisc = 0x0c
	OpcodeMiscElemDrop   OpcodeMisc = 0x0d
	OpcodeMiscTableCopy  OpcodeMisc = 0x0e

	// Below are toggled with CoreFeatureReferenceTypes

	OpcodeMiscTableGrow OpcodeMisc = 0x0f
	OpcodeMiscTableSize OpcodeMisc = 0x10
	OpcodeMiscTableFill OpcodeMisc = 0x11
)

// OpcodeVec represents an opcode of a vector instructions which has
// multi-byte encoding and is prefixed by OpcodeMiscPrefix.
//
// These opcodes are toggled with CoreFeatureSIMD.
type OpcodeVec = byte

const (
	// Loads and stores.

	OpcodeVecV128Load        OpcodeVec = 0x00
	OpcodeVecV128Load8x8s    OpcodeVec = 0x01
	OpcodeVecV128Load8x8u    OpcodeVec = 0x02
	OpcodeVecV128Load16x4s   OpcodeVec = 0x03
	OpcodeVecV128Load16x4u   OpcodeVec = 0x04
	OpcodeVecV128Load32x2s   OpcodeVec = 0x05
	OpcodeVecV128Load32x2u   OpcodeVec = 0x06
	OpcodeVecV128Load8Splat  OpcodeVec = 0x07
	OpcodeVecV128Load16Splat OpcodeVec = 0x08
	OpcodeVecV128Load32Splat OpcodeVec = 0x09
	OpcodeVecV128Load64Splat OpcodeVec = 0x0a

	OpcodeVecV128Load32zero OpcodeVec = 0x5c
	OpcodeVecV128Load64zero OpcodeVec = 0x5d

	OpcodeVecV128Store       OpcodeVec = 0x0b
	OpcodeVecV128Load8Lane   OpcodeVec = 0x54
	OpcodeVecV128Load16Lane  OpcodeVec = 0x55
	OpcodeVecV128Load32Lane  OpcodeVec = 0x56
	OpcodeVecV128Load64Lane  OpcodeVec = 0x57
	OpcodeVecV128Store8Lane  OpcodeVec = 0x58
	OpcodeVecV128Store16Lane OpcodeVec = 0x59
	OpcodeVecV128Store32Lane OpcodeVec = 0x5a
	OpcodeVecV128Store64Lane OpcodeVec = 0x5b

	// OpcodeVecV128Const is the vector const instruction.
	OpcodeVecV128Const OpcodeVec = 0x0c

	// OpcodeVecV128i8x16Shuffle is the vector shuffle instruction.
	OpcodeVecV128i8x16Shuffle OpcodeVec = 0x0d

	// Extrac and replaces.

	OpcodeVecI8x16ExtractLaneS OpcodeVec = 0x15
	OpcodeVecI8x16ExtractLaneU OpcodeVec = 0x16
	OpcodeVecI8x16ReplaceLane  OpcodeVec = 0x17
	OpcodeVecI16x8ExtractLaneS OpcodeVec = 0x18
	OpcodeVecI16x8ExtractLaneU OpcodeVec = 0x19
	OpcodeVecI16x8ReplaceLane  OpcodeVec = 0x1a
	OpcodeVecI32x4ExtractLane  OpcodeVec = 0x1b
	OpcodeVecI32x4ReplaceLane  OpcodeVec = 0x1c
	OpcodeVecI64x2ExtractLane  OpcodeVec = 0x1d
	OpcodeVecI64x2ReplaceLane  OpcodeVec = 0x1e
	OpcodeVecF32x4ExtractLane  OpcodeVec = 0x1f
	OpcodeVecF32x4ReplaceLane  OpcodeVec = 0x20
	OpcodeVecF64x2ExtractLane  OpcodeVec = 0x21
	OpcodeVecF64x2ReplaceLane  OpcodeVec = 0x22

	// Splat and swizzle.

	OpcodeVecI8x16Swizzle OpcodeVec = 0x0e
	OpcodeVecI8x16Splat   OpcodeVec = 0x0f
	OpcodeVecI16x8Splat   OpcodeVec = 0x10
	OpcodeVecI32x4Splat   OpcodeVec = 0x11
	OpcodeVecI64x2Splat   OpcodeVec = 0x12
	OpcodeVecF32x4Splat   OpcodeVec = 0x13
	OpcodeVecF64x2Splat   OpcodeVec = 0x14

	// i8 comparisons.

	OpcodeVecI8x16Eq  OpcodeVec = 0x23
	OpcodeVecI8x16Ne  OpcodeVec = 0x24
	OpcodeVecI8x16LtS OpcodeVec = 0x25
	OpcodeVecI8x16LtU OpcodeVec = 0x26
	OpcodeVecI8x16GtS OpcodeVec = 0x27
	OpcodeVecI8x16GtU OpcodeVec = 0x28
	OpcodeVecI8x16LeS OpcodeVec = 0x29
	OpcodeVecI8x16LeU OpcodeVec = 0x2a
	OpcodeVecI8x16GeS OpcodeVec = 0x2b
	OpcodeVecI8x16GeU OpcodeVec = 0x2c

	// i16 comparisons.

	OpcodeVecI16x8Eq  OpcodeVec = 0x2d
	OpcodeVecI16x8Ne  OpcodeVec = 0x2e
	OpcodeVecI16x8LtS OpcodeVec = 0x2f
	OpcodeVecI16x8LtU OpcodeVec = 0x30
	OpcodeVecI16x8GtS OpcodeVec = 0x31
	OpcodeVecI16x8GtU OpcodeVec = 0x32
	OpcodeVecI16x8LeS OpcodeVec = 0x33
	OpcodeVecI16x8LeU OpcodeVec = 0x34
	OpcodeVecI16x8GeS OpcodeVec = 0x35
	OpcodeVecI16x8GeU OpcodeVec = 0x36

	// i32 comparisons.

	OpcodeVecI32x4Eq  OpcodeVec = 0x37
	OpcodeVecI32x4Ne  OpcodeVec = 0x38
	OpcodeVecI32x4LtS OpcodeVec = 0x39
	OpcodeVecI32x4LtU OpcodeVec = 0x3a
	OpcodeVecI32x4GtS OpcodeVec = 0x3b
	OpcodeVecI32x4GtU OpcodeVec = 0x3c
	OpcodeVecI32x4LeS OpcodeVec = 0x3d
	OpcodeVecI32x4LeU OpcodeVec = 0x3e
	OpcodeVecI32x4GeS OpcodeVec = 0x3f
	OpcodeVecI32x4GeU OpcodeVec = 0x40

	// i64 comparisons.

	OpcodeVecI64x2Eq  OpcodeVec = 0xd6
	OpcodeVecI64x2Ne  OpcodeVec = 0xd7
	OpcodeVecI64x2LtS OpcodeVec = 0xd8
	OpcodeVecI64x2GtS OpcodeVec = 0xd9
	OpcodeVecI64x2LeS OpcodeVec = 0xda
	OpcodeVecI64x2GeS OpcodeVec = 0xdb

	// f32 comparisons.

	OpcodeVecF32x4Eq OpcodeVec = 0x41
	OpcodeVecF32x4Ne OpcodeVec = 0x42
	OpcodeVecF32x4Lt OpcodeVec = 0x43
	OpcodeVecF32x4Gt OpcodeVec = 0x44
	OpcodeVecF32x4Le OpcodeVec = 0x45
	OpcodeVecF32x4Ge OpcodeVec = 0x46

	// f64 comparisons.

	OpcodeVecF64x2Eq OpcodeVec = 0x47
	OpcodeVecF64x2Ne OpcodeVec = 0x48
	OpcodeVecF64x2Lt OpcodeVec = 0x49
	OpcodeVecF64x2Gt OpcodeVec = 0x4a
	OpcodeVecF64x2Le OpcodeVec = 0x4b
	OpcodeVecF64x2Ge OpcodeVec = 0x4c

	// v128 logical instructions.

	OpcodeVecV128Not       OpcodeVec = 0x4d
	OpcodeVecV128And       OpcodeVec = 0x4e
	OpcodeVecV128AndNot    OpcodeVec = 0x4f
	OpcodeVecV128Or        OpcodeVec = 0x50
	OpcodeVecV128Xor       OpcodeVec = 0x51
	OpcodeVecV128Bitselect OpcodeVec = 0x52
	OpcodeVecV128AnyTrue   OpcodeVec = 0x53

	// i8 misc.

	OpcodeVecI8x16Abs          OpcodeVec = 0x60
	OpcodeVecI8x16Neg          OpcodeVec = 0x61
	OpcodeVecI8x16Popcnt       OpcodeVec = 0x62
	OpcodeVecI8x16AllTrue      OpcodeVec = 0x63
	OpcodeVecI8x16BitMask      OpcodeVec = 0x64
	OpcodeVecI8x16NarrowI16x8S OpcodeVec = 0x65
	OpcodeVecI8x16NarrowI16x8U OpcodeVec = 0x66

	OpcodeVecI8x16Shl     OpcodeVec = 0x6b
	OpcodeVecI8x16ShrS    OpcodeVec = 0x6c
	OpcodeVecI8x16ShrU    OpcodeVec = 0x6d
	OpcodeVecI8x16Add     OpcodeVec = 0x6e
	OpcodeVecI8x16AddSatS OpcodeVec = 0x6f

	OpcodeVecI8x16AddSatU OpcodeVec = 0x70
	OpcodeVecI8x16Sub     OpcodeVec = 0x71
	OpcodeVecI8x16SubSatS OpcodeVec = 0x72
	OpcodeVecI8x16SubSatU OpcodeVec = 0x73
	OpcodeVecI8x16MinS    OpcodeVec = 0x76
	OpcodeVecI8x16MinU    OpcodeVec = 0x77
	OpcodeVecI8x16MaxS    OpcodeVec = 0x78
	OpcodeVecI8x16MaxU    OpcodeVec = 0x79
	OpcodeVecI8x16AvgrU   OpcodeVec = 0x7b

	// i16 misc.

	OpcodeVecI16x8ExtaddPairwiseI8x16S OpcodeVec = 0x7c
	OpcodeVecI16x8ExtaddPairwiseI8x16U OpcodeVec = 0x7d
	OpcodeVecI16x8Abs                  OpcodeVec = 0x80
	OpcodeVecI16x8Neg                  OpcodeVec = 0x81
	OpcodeVecI16x8Q15mulrSatS          OpcodeVec = 0x82
	OpcodeVecI16x8AllTrue              OpcodeVec = 0x83
	OpcodeVecI16x8BitMask              OpcodeVec = 0x84
	OpcodeVecI16x8NarrowI32x4S         OpcodeVec = 0x85
	OpcodeVecI16x8NarrowI32x4U         OpcodeVec = 0x86
	OpcodeVecI16x8ExtendLowI8x16S      OpcodeVec = 0x87
	OpcodeVecI16x8ExtendHighI8x16S     OpcodeVec = 0x88
	OpcodeVecI16x8ExtendLowI8x16U      OpcodeVec = 0x89
	OpcodeVecI16x8ExtendHighI8x16U     OpcodeVec = 0x8a
	OpcodeVecI16x8Shl                  OpcodeVec = 0x8b
	OpcodeVecI16x8ShrS                 OpcodeVec = 0x8c
	OpcodeVecI16x8ShrU                 OpcodeVec = 0x8d
	OpcodeVecI16x8Add                  OpcodeVec = 0x8e
	OpcodeVecI16x8AddSatS              OpcodeVec = 0x8f
	OpcodeVecI16x8AddSatU              OpcodeVec = 0x90
	OpcodeVecI16x8Sub                  OpcodeVec = 0x91
	OpcodeVecI16x8SubSatS              OpcodeVec = 0x92
	OpcodeVecI16x8SubSatU              OpcodeVec = 0x93
	OpcodeVecI16x8Mul                  OpcodeVec = 0x95
	OpcodeVecI16x8MinS                 OpcodeVec = 0x96
	OpcodeVecI16x8MinU                 OpcodeVec = 0x97
	OpcodeVecI16x8MaxS                 OpcodeVec = 0x98
	OpcodeVecI16x8MaxU                 OpcodeVec = 0x99
	OpcodeVecI16x8AvgrU                OpcodeVec = 0x9b
	OpcodeVecI16x8ExtMulLowI8x16S      OpcodeVec = 0x9c
	OpcodeVecI16x8ExtMulHighI8x16S     OpcodeVec = 0x9d
	OpcodeVecI16x8ExtMulLowI8x16U      OpcodeVec = 0x9e
	OpcodeVecI16x8ExtMulHighI8x16U     OpcodeVec = 0x9f

	// i32 misc.

	OpcodeVecI32x4ExtaddPairwiseI16x8S OpcodeVec = 0x7e
	OpcodeVecI32x4ExtaddPairwiseI16x8U OpcodeVec = 0x7f
	OpcodeVecI32x4Abs                  OpcodeVec = 0xa0
	OpcodeVecI32x4Neg                  OpcodeVec = 0xa1
	OpcodeVecI32x4AllTrue              OpcodeVec = 0xa3
	OpcodeVecI32x4BitMask              OpcodeVec = 0xa4
	OpcodeVecI32x4ExtendLowI16x8S      OpcodeVec = 0xa7
	OpcodeVecI32x4ExtendHighI16x8S     OpcodeVec = 0xa8
	OpcodeVecI32x4ExtendLowI16x8U      OpcodeVec = 0xa9
	OpcodeVecI32x4ExtendHighI16x8U     OpcodeVec = 0xaa
	OpcodeVecI32x4Shl                  OpcodeVec = 0xab
	OpcodeVecI32x4ShrS                 OpcodeVec = 0xac
	OpcodeVecI32x4ShrU                 OpcodeVec = 0xad
	OpcodeVecI32x4Add                  OpcodeVec = 0xae
	OpcodeVecI32x4Sub                  OpcodeVec = 0xb1
	OpcodeVecI32x4Mul                  OpcodeVec = 0xb5
	OpcodeVecI32x4MinS                 OpcodeVec = 0xb6
	OpcodeVecI32x4MinU                 OpcodeVec = 0xb7
	OpcodeVecI32x4MaxS                 OpcodeVec = 0xb8
	OpcodeVecI32x4MaxU                 OpcodeVec = 0xb9
	OpcodeVecI32x4DotI16x8S            OpcodeVec = 0xba
	OpcodeVecI32x4ExtMulLowI16x8S      OpcodeVec = 0xbc
	OpcodeVecI32x4ExtMulHighI16x8S     OpcodeVec = 0xbd
	OpcodeVecI32x4ExtMulLowI16x8U      OpcodeVec = 0xbe
	OpcodeVecI32x4ExtMulHighI16x8U     OpcodeVec = 0xbf

	// i64 misc.

	OpcodeVecI64x2Abs              OpcodeVec = 0xc0
	OpcodeVecI64x2Neg              OpcodeVec = 0xc1
	OpcodeVecI64x2AllTrue          OpcodeVec = 0xc3
	OpcodeVecI64x2BitMask          OpcodeVec = 0xc4
	OpcodeVecI64x2ExtendLowI32x4S  OpcodeVec = 0xc7
	OpcodeVecI64x2ExtendHighI32x4S OpcodeVec = 0xc8
	OpcodeVecI64x2ExtendLowI32x4U  OpcodeVec = 0xc9
	OpcodeVecI64x2ExtendHighI32x4U OpcodeVec = 0xca
	OpcodeVecI64x2Shl              OpcodeVec = 0xcb
	OpcodeVecI64x2ShrS             OpcodeVec = 0xcc
	OpcodeVecI64x2ShrU             OpcodeVec = 0xcd
	OpcodeVecI64x2Add              OpcodeVec = 0xce
	OpcodeVecI64x2Sub              OpcodeVec = 0xd1
	OpcodeVecI64x2Mul              OpcodeVec = 0xd5
	OpcodeVecI64x2ExtMulLowI32x4S  OpcodeVec = 0xdc
	OpcodeVecI64x2ExtMulHighI32x4S OpcodeVec = 0xdd
	OpcodeVecI64x2ExtMulLowI32x4U  OpcodeVec = 0xde
	OpcodeVecI64x2ExtMulHighI32x4U OpcodeVec = 0xdf

	// f32 misc.

	OpcodeVecF32x4Ceil    OpcodeVec = 0x67
	OpcodeVecF32x4Floor   OpcodeVec = 0x68
	OpcodeVecF32x4Trunc   OpcodeVec = 0x69
	OpcodeVecF32x4Nearest OpcodeVec = 0x6a
	OpcodeVecF32x4Abs     OpcodeVec = 0xe0
	OpcodeVecF32x4Neg     OpcodeVec = 0xe1
	OpcodeVecF32x4Sqrt    OpcodeVec = 0xe3
	OpcodeVecF32x4Add     OpcodeVec = 0xe4
	OpcodeVecF32x4Sub     OpcodeVec = 0xe5
	OpcodeVecF32x4Mul     OpcodeVec = 0xe6
	OpcodeVecF32x4Div     OpcodeVec = 0xe7
	OpcodeVecF32x4Min     OpcodeVec = 0xe8
	OpcodeVecF32x4Max     OpcodeVec = 0xe9
	OpcodeVecF32x4Pmin    OpcodeVec = 0xea
	OpcodeVecF32x4Pmax    OpcodeVec = 0xeb

	// f64 misc.

	OpcodeVecF64x2Ceil    OpcodeVec = 0x74
	OpcodeVecF64x2Floor   OpcodeVec = 0x75
	OpcodeVecF64x2Trunc   OpcodeVec = 0x7a
	OpcodeVecF64x2Nearest OpcodeVec = 0x94
	OpcodeVecF64x2Abs     OpcodeVec = 0xec
	OpcodeVecF64x2Neg     OpcodeVec = 0xed
	OpcodeVecF64x2Sqrt    OpcodeVec = 0xef
	OpcodeVecF64x2Add     OpcodeVec = 0xf0
	OpcodeVecF64x2Sub     OpcodeVec = 0xf1
	OpcodeVecF64x2Mul     OpcodeVec = 0xf2
	OpcodeVecF64x2Div     OpcodeVec = 0xf3
	OpcodeVecF64x2Min     OpcodeVec = 0xf4
	OpcodeVecF64x2Max     OpcodeVec = 0xf5
	OpcodeVecF64x2Pmin    OpcodeVec = 0xf6
	OpcodeVecF64x2Pmax    OpcodeVec = 0xf7

	// conversions.

	OpcodeVecI32x4TruncSatF32x4S      OpcodeVec = 0xf8
	OpcodeVecI32x4TruncSatF32x4U      OpcodeVec = 0xf9
	OpcodeVecF32x4ConvertI32x4S       OpcodeVec = 0xfa
	OpcodeVecF32x4ConvertI32x4U       OpcodeVec = 0xfb
	OpcodeVecI32x4TruncSatF64x2SZero  OpcodeVec = 0xfc
	OpcodeVecI32x4TruncSatF64x2UZero  OpcodeVec = 0xfd
	OpcodeVecF64x2ConvertLowI32x4S    OpcodeVec = 0xfe
	OpcodeVecF64x2ConvertLowI32x4U    OpcodeVec = 0xff
	OpcodeVecF32x4DemoteF64x2Zero     OpcodeVec = 0x5e
	OpcodeVecF64x2PromoteLowF32x4Zero OpcodeVec = 0x5f
)

// OpcodeAtomic represents an opcode of atomic instructions which has
// multi-byte encoding and is prefixed by OpcodeAtomicPrefix.
//
// These opcodes are toggled with CoreFeaturesThreads.
type OpcodeAtomic = byte

const (
	// OpcodeAtomicMemoryNotify represents the instruction memory.atomic.notify.
	OpcodeAtomicMemoryNotify OpcodeAtomic = 0x00
	// OpcodeAtomicMemoryWait32 represents the instruction memory.atomic.wait32.
	OpcodeAtomicMemoryWait32 OpcodeAtomic = 0x01
	// OpcodeAtomicMemoryWait64 represents the instruction memory.atomic.wait64.
	OpcodeAtomicMemoryWait64 OpcodeAtomic = 0x02
	// OpcodeAtomicFence represents the instruction atomic.fence.
	OpcodeAtomicFence OpcodeAtomic = 0x03

	// OpcodeAtomicI32Load represents the instruction i32.atomic.load.
	OpcodeAtomicI32Load OpcodeAtomic = 0x10
	// OpcodeAtomicI64Load represents the instruction i64.atomic.load.
	OpcodeAtomicI64Load OpcodeAtomic = 0x11
	// OpcodeAtomicI32Load8U represents the instruction i32.atomic.load8_u.
	OpcodeAtomicI32Load8U OpcodeAtomic = 0x12
	// OpcodeAtomicI32Load16U represents the instruction i32.atomic.load16_u.
	OpcodeAtomicI32Load16U OpcodeAtomic = 0x13
	// OpcodeAtomicI64Load8U represents the instruction i64.atomic.load8_u.
	OpcodeAtomicI64Load8U OpcodeAtomic = 0x14
	// OpcodeAtomicI64Load16U represents the instruction i64.atomic.load16_u.
	OpcodeAtomicI64Load16U OpcodeAtomic = 0x15
	// OpcodeAtomicI64Load32U represents the instruction i64.atomic.load32_u.
	OpcodeAtomicI64Load32U OpcodeAtomic = 0x16
	// OpcodeAtomicI32Store represents the instruction i32.atomic.store.
	OpcodeAtomicI32Store OpcodeAtomic = 0x17
	// OpcodeAtomicI64Store represents the instruction i64.atomic.store.
	OpcodeAtomicI64Store OpcodeAtomic = 0x18
	// OpcodeAtomicI32Store8 represents the instruction i32.atomic.store8.
	OpcodeAtomicI32Store8 OpcodeAtomic = 0x19
	// OpcodeAtomicI32Store16 represents the instruction i32.atomic.store16.
	OpcodeAtomicI32Store16 OpcodeAtomic = 0x1a
	// OpcodeAtomicI64Store8 represents the instruction i64.atomic.store8.
	OpcodeAtomicI64Store8 OpcodeAtomic = 0x1b
	// OpcodeAtomicI64Store16 represents the instruction i64.atomic.store16.
	OpcodeAtomicI64Store16 OpcodeAtomic = 0x1c
	// OpcodeAtomicI64Store32 represents the instruction i64.atomic.store32.
	OpcodeAtomicI64Store32 OpcodeAtomic = 0x1d

	// OpcodeAtomicI32RmwAdd represents the instruction i32.atomic.rmw.add.
	OpcodeAtomicI32RmwAdd OpcodeAtomic = 0x1e
	// OpcodeAtomicI64RmwAdd represents the instruction i64.atomic.rmw.add.
	OpcodeAtomicI64RmwAdd OpcodeAtomic = 0x1f
	// OpcodeAtomicI32Rmw8AddU represents the instruction i32.atomic.rmw8.add_u.
	OpcodeAtomicI32Rmw8AddU OpcodeAtomic = 0x20
	// OpcodeAtomicI32Rmw16AddU represents the instruction i32.atomic.rmw16.add_u.
	OpcodeAtomicI32Rmw16AddU OpcodeAtomic = 0x21
	// OpcodeAtomicI64Rmw8AddU represents the instruction i64.atomic.rmw8.add_u.
	OpcodeAtomicI64Rmw8AddU OpcodeAtomic = 0x22
	// OpcodeAtomicI64Rmw16AddU represents the instruction i64.atomic.rmw16.add_u.
	OpcodeAtomicI64Rmw16AddU OpcodeAtomic = 0x23
	// OpcodeAtomicI64Rmw32AddU represents the instruction i64.atomic.rmw32.add_u.
	OpcodeAtomicI64Rmw32AddU OpcodeAtomic = 0x24

	// OpcodeAtomicI32RmwSub represents the instruction i32.atomic.rmw.sub.
	OpcodeAtomicI32RmwSub OpcodeAtomic = 0x25
	// OpcodeAtomicI64RmwSub represents the instruction i64.atomic.rmw.sub.
	OpcodeAtomicI64RmwSub OpcodeAtomic = 0x26
	// OpcodeAtomicI32Rmw8SubU represents the instruction i32.atomic.rmw8.sub_u.
	OpcodeAtomicI32Rmw8SubU OpcodeAtomic = 0x27
	// OpcodeAtomicI32Rmw16SubU represents the instruction i32.atomic.rmw16.sub_u.
	OpcodeAtomicI32Rmw16SubU OpcodeAtomic = 0x28
	// OpcodeAtomicI64Rmw8SubU represents the instruction i64.atomic.rmw8.sub_u.
	OpcodeAtomicI64Rmw8SubU OpcodeAtomic = 0x29
	// OpcodeAtomicI64Rmw16SubU represents the instruction i64.atomic.rmw16.sub_u.
	OpcodeAtomicI64Rmw16SubU OpcodeAtomic = 0x2a
	// OpcodeAtomicI64Rmw32SubU represents the instruction i64.atomic.rmw32.sub_u.
	OpcodeAtomicI64Rmw32SubU OpcodeAtomic = 0x2b

	// OpcodeAtomicI32RmwAnd represents the instruction i32.atomic.rmw.and.
	OpcodeAtomicI32RmwAnd OpcodeAtomic = 0x2c
	// OpcodeAtomicI64RmwAnd represents the instruction i64.atomic.rmw.and.
	OpcodeAtomicI64RmwAnd OpcodeAtomic = 0x2d
	// OpcodeAtomicI32Rmw8AndU represents the instruction i32.atomic.rmw8.and_u.
	OpcodeAtomicI32Rmw8AndU OpcodeAtomic = 0x2e
	// OpcodeAtomicI32Rmw16AndU represents the instruction i32.atomic.rmw16.and_u.
	OpcodeAtomicI32Rmw16AndU OpcodeAtomic = 0x2f
	// OpcodeAtomicI64Rmw8AndU represents the instruction i64.atomic.rmw8.and_u.
	OpcodeAtomicI64Rmw8AndU OpcodeAtomic = 0x30
	// OpcodeAtomicI64Rmw16AndU represents the instruction i64.atomic.rmw16.and_u.
	OpcodeAtomicI64Rmw16AndU OpcodeAtomic = 0x31
	// OpcodeAtomicI64Rmw32AndU represents the instruction i64.atomic.rmw32.and_u.
	OpcodeAtomicI64Rmw32AndU OpcodeAtomic = 0x32

	// OpcodeAtomicI32RmwOr represents the instruction i32.atomic.rmw.or.
	OpcodeAtomicI32RmwOr OpcodeAtomic = 0x33
	// OpcodeAtomicI64RmwOr represents the instruction i64.atomic.rmw.or.
	OpcodeAtomicI64RmwOr OpcodeAtomic = 0x34
	// OpcodeAtomicI32Rmw8OrU represents the instruction i32.atomic.rmw8.or_u.
	OpcodeAtomicI32Rmw8OrU OpcodeAtomic = 0x35
	// OpcodeAtomicI32Rmw16OrU represents the instruction i32.atomic.rmw16.or_u.
	OpcodeAtomicI32Rmw16OrU OpcodeAtomic = 0x36
	// OpcodeAtomicI64Rmw8OrU represents the instruction i64.atomic.rmw8.or_u.
	OpcodeAtomicI64Rmw8OrU OpcodeAtomic = 0x37
	// OpcodeAtomicI64Rmw16OrU represents the instruction i64.atomic.rmw16.or_u.
	OpcodeAtomicI64Rmw16OrU OpcodeAtomic = 0x38
	// OpcodeAtomicI64Rmw32OrU represents the instruction i64.atomic.rmw32.or_u.
	OpcodeAtomicI64Rmw32OrU OpcodeAtomic = 0x39

	// OpcodeAtomicI32RmwXor represents the instruction i32.atomic.rmw.xor.
	OpcodeAtomicI32RmwXor OpcodeAtomic = 0x3a
	// OpcodeAtomicI64RmwXor represents the instruction i64.atomic.rmw.xor.
	OpcodeAtomicI64RmwXor OpcodeAtomic = 0x3b
	// OpcodeAtomicI32Rmw8XorU represents the instruction i32.atomic.rmw8.xor_u.
	OpcodeAtomicI32Rmw8XorU OpcodeAtomic = 0x3c
	// OpcodeAtomicI32Rmw16XorU represents the instruction i32.atomic.rmw16.xor_u.
	OpcodeAtomicI32Rmw16XorU OpcodeAtomic = 0x3d
	// OpcodeAtomicI64Rmw8XorU represents the instruction i64.atomic.rmw8.xor_u.
	OpcodeAtomicI64Rmw8XorU OpcodeAtomic = 0x3e
	// OpcodeAtomicI64Rmw16XorU represents the instruction i64.atomic.rmw16.xor_u.
	OpcodeAtomicI64Rmw16XorU OpcodeAtomic = 0x3f
	// OpcodeAtomicI64Rmw32XorU represents the instruction i64.atomic.rmw32.xor_u.
	OpcodeAtomicI64Rmw32XorU OpcodeAtomic = 0x40

	// OpcodeAtomicI32RmwXchg represents the instruction i32.atomic.rmw.xchg.
	OpcodeAtomicI32RmwXchg OpcodeAtomic = 0x41
	// OpcodeAtomicI64RmwXchg represents the instruction i64.atomic.rmw.xchg.
	OpcodeAtomicI64RmwXchg OpcodeAtomic = 0x42
	// OpcodeAtomicI32Rmw8XchgU represents the instruction i32.atomic.rmw8.xchg_u.
	OpcodeAtomicI32Rmw8XchgU OpcodeAtomic = 0x43
	// OpcodeAtomicI32Rmw16XchgU represents the instruction i32.atomic.rmw16.xchg_u.
	OpcodeAtomicI32Rmw16XchgU OpcodeAtomic = 0x44
	// OpcodeAtomicI64Rmw8XchgU represents the instruction i64.atomic.rmw8.xchg_u.
	OpcodeAtomicI64Rmw8XchgU OpcodeAtomic = 0x45
	// OpcodeAtomicI64Rmw16XchgU represents the instruction i64.atomic.rmw16.xchg_u.
	OpcodeAtomicI64Rmw16XchgU OpcodeAtomic = 0x46
	// OpcodeAtomicI64Rmw32XchgU represents the instruction i64.atomic.rmw32.xchg_u.
	OpcodeAtomicI64Rmw32XchgU OpcodeAtomic = 0x47

	// OpcodeAtomicI32RmwCmpxchg represents the instruction i32.atomic.rmw.cmpxchg.
	OpcodeAtomicI32RmwCmpxchg OpcodeAtomic = 0x48
	// OpcodeAtomicI64RmwCmpxchg represents the instruction i64.atomic.rmw.cmpxchg.
	OpcodeAtomicI64RmwCmpxchg OpcodeAtomic = 0x49
	// OpcodeAtomicI32Rmw8CmpxchgU represents the instruction i32.atomic.rmw8.cmpxchg_u.
	OpcodeAtomicI32Rmw8CmpxchgU OpcodeAtomic = 0x4a
	// OpcodeAtomicI32Rmw16CmpxchgU represents the instruction i32.atomic.rmw16.cmpxchg_u.
	OpcodeAtomicI32Rmw16CmpxchgU OpcodeAtomic = 0x4b
	// OpcodeAtomicI64Rmw8CmpxchgU represents the instruction i64.atomic.rmw8.cmpxchg_u.
	OpcodeAtomicI64Rmw8CmpxchgU OpcodeAtomic = 0x4c
	// OpcodeAtomicI64Rmw16CmpxchgU represents the instruction i64.atomic.rmw16.cmpxchg_u.
	OpcodeAtomicI64Rmw16CmpxchgU OpcodeAtomic = 0x4d
	// OpcodeAtomicI64Rmw32CmpxchgU represents the instruction i64.atomic.rmw32.cmpxchg_u.
	OpcodeAtomicI64Rmw32CmpxchgU OpcodeAtomic = 0x4e
)

const (
	OpcodeUnreachableName       = "unreachable"
	OpcodeNopName               = "nop"
	OpcodeBlockName             = "block"
	OpcodeLoopName              = "loop"
	OpcodeIfName                = "if"
	OpcodeElseName              = "else"
	OpcodeEndName               = "end"
	OpcodeBrName                = "br"
	OpcodeBrIfName              = "br_if"
	OpcodeBrTableName           = "br_table"
	OpcodeReturnName            = "return"
	OpcodeCallName              = "call"
	OpcodeCallIndirectName      = "call_indirect"
	OpcodeDropName              = "drop"
	OpcodeSelectName            = "select"
	OpcodeTypedSelectName       = "typed_select"
	OpcodeLocalGetName          = "local.get"
	OpcodeLocalSetName          = "local.set"
	OpcodeLocalTeeName          = "local.tee"
	OpcodeGlobalGetName         = "global.get"
	OpcodeGlobalSetName         = "global.set"
	OpcodeI32LoadName           = "i32.load"
	OpcodeI64LoadName           = "i64.load"
	OpcodeF32LoadName           = "f32.load"
	OpcodeF64LoadName           = "f64.load"
	OpcodeI32Load8SName         = "i32.load8_s"
	OpcodeI32Load8UName         = "i32.load8_u"
	OpcodeI32Load16SName        = "i32.load16_s"
	OpcodeI32Load16UName        = "i32.load16_u"
	OpcodeI64Load8SName         = "i64.load8_s"
	OpcodeI64Load8UName         = "i64.load8_u"
	OpcodeI64Load16SName        = "i64.load16_s"
	OpcodeI64Load16UName        = "i64.load16_u"
	OpcodeI64Load32SName        = "i64.load32_s"
	OpcodeI64Load32UName        = "i64.load32_u"
	OpcodeI32StoreName          = "i32.store"
	OpcodeI64StoreName          = "i64.store"
	OpcodeF32StoreName          = "f32.store"
	OpcodeF64StoreName          = "f64.store"
	OpcodeI32Store8Name         = "i32.store8"
	OpcodeI32Store16Name        = "i32.store16"
	OpcodeI64Store8Name         = "i64.store8"
	OpcodeI64Store16Name        = "i64.store16"
	OpcodeI64Store32Name        = "i64.store32"
	OpcodeMemorySizeName        = "memory.size"
	OpcodeMemoryGrowName        = "memory.grow"
	OpcodeI32ConstName          = "i32.const"
	OpcodeI64ConstName          = "i64.const"
	OpcodeF32ConstName          = "f32.const"
	OpcodeF64ConstName          = "f64.const"
	OpcodeI32EqzName            = "i32.eqz"
	OpcodeI32EqName             = "i32.eq"
	OpcodeI32NeName             = "i32.ne"
	OpcodeI32LtSName            = "i32.lt_s"
	OpcodeI32LtUName            = "i32.lt_u"
	OpcodeI32GtSName            = "i32.gt_s"
	OpcodeI32GtUName            = "i32.gt_u"
	OpcodeI32LeSName            = "i32.le_s"
	OpcodeI32LeUName            = "i32.le_u"
	OpcodeI32GeSName            = "i32.ge_s"
	OpcodeI32GeUName            = "i32.ge_u"
	OpcodeI64EqzName            = "i64.eqz"
	OpcodeI64EqName             = "i64.eq"
	OpcodeI64NeName             = "i64.ne"
	OpcodeI64LtSName            = "i64.lt_s"
	OpcodeI64LtUName            = "i64.lt_u"
	OpcodeI64GtSName            = "i64.gt_s"
	OpcodeI64GtUName            = "i64.gt_u"
	OpcodeI64LeSName            = "i64.le_s"
	OpcodeI64LeUName            = "i64.le_u"
	OpcodeI64GeSName            = "i64.ge_s"
	OpcodeI64GeUName            = "i64.ge_u"
	OpcodeF32EqName             = "f32.eq"
	OpcodeF32NeName             = "f32.ne"
	OpcodeF32LtName             = "f32.lt"
	OpcodeF32GtName             = "f32.gt"
	OpcodeF32LeName             = "f32.le"
	OpcodeF32GeName             = "f32.ge"
	OpcodeF64EqName             = "f64.eq"
	OpcodeF64NeName             = "f64.ne"
	OpcodeF64LtName             = "f64.lt"
	OpcodeF64GtName             = "f64.gt"
	OpcodeF64LeName             = "f64.le"
	OpcodeF64GeName             = "f64.ge"
	OpcodeI32ClzName            = "i32.clz"
	OpcodeI32CtzName            = "i32.ctz"
	OpcodeI32PopcntName         = "i32.popcnt"
	OpcodeI32AddName            = "i32.add"
	OpcodeI32SubName            = "i32.sub"
	OpcodeI32MulName            = "i32.mul"
	OpcodeI32DivSName           = "i32.div_s"
	OpcodeI32DivUName           = "i32.div_u"
	OpcodeI32RemSName           = "i32.rem_s"
	OpcodeI32RemUName           = "i32.rem_u"
	OpcodeI32AndName            = "i32.and"
	OpcodeI32OrName             = "i32.or"
	OpcodeI32XorName            = "i32.xor"
	OpcodeI32ShlName            = "i32.shl"
	OpcodeI32ShrSName           = "i32.shr_s"
	OpcodeI32ShrUName           = "i32.shr_u"
	OpcodeI32RotlName           = "i32.rotl"
	OpcodeI32RotrName           = "i32.rotr"
	OpcodeI64ClzName            = "i64.clz"
	OpcodeI64CtzName            = "i64.ctz"
	OpcodeI64PopcntName         = "i64.popcnt"
	OpcodeI64AddName            = "i64.add"
	OpcodeI64SubName            = "i64.sub"
	OpcodeI64MulName            = "i64.mul"
	OpcodeI64DivSName           = "i64.div_s"
	OpcodeI64DivUName           = "i64.div_u"
	OpcodeI64RemSName           = "i64.rem_s"
	OpcodeI64RemUName           = "i64.rem_u"
	OpcodeI64AndName            = "i64.and"
	OpcodeI64OrName             = "i64.or"
	OpcodeI64XorName            = "i64.xor"
	OpcodeI64ShlName            = "i64.shl"
	OpcodeI64ShrSName           = "i64.shr_s"
	OpcodeI64ShrUName           = "i64.shr_u"
	OpcodeI64RotlName           = "i64.rotl"
	OpcodeI64RotrName           = "i64.rotr"
	OpcodeF32AbsName            = "f32.abs"
	OpcodeF32NegName            = "f32.neg"
	OpcodeF32CeilName           = "f32.ceil"
	OpcodeF32FloorName          = "f32.floor"
	OpcodeF32TruncName          = "f32.trunc"
	OpcodeF32NearestName        = "f32.nearest"
	OpcodeF32SqrtName           = "f32.sqrt"
	OpcodeF32AddName            = "f32.add"
	OpcodeF32SubName            = "f32.sub"
	OpcodeF32MulName            = "f32.mul"
	OpcodeF32DivName            = "f32.div"
	OpcodeF32MinName            = "f32.min"
	OpcodeF32MaxName            = "f32.max"
	OpcodeF32CopysignName       = "f32.copysign"
	OpcodeF64AbsName            = "f64.abs"
	OpcodeF64NegName            = "f64.neg"
	OpcodeF64CeilName           = "f64.ceil"
	OpcodeF64FloorName          = "f64.floor"
	OpcodeF64TruncName          = "f64.trunc"
	OpcodeF64NearestName        = "f64.nearest"
	OpcodeF64SqrtName           = "f64.sqrt"
	OpcodeF64AddName            = "f64.add"
	OpcodeF64SubName            = "f64.sub"
	OpcodeF64MulName            = "f64.mul"
	OpcodeF64DivName            = "f64.div"
	OpcodeF64MinName            = "f64.min"
	OpcodeF64MaxName            = "f64.max"
	OpcodeF64CopysignName       = "f64.copysign"
	OpcodeI32WrapI64Name        = "i32.wrap_i64"
	OpcodeI32TruncF32SName      = "i32.trunc_f32_s"
	OpcodeI32TruncF32UName      = "i32.trunc_f32_u"
	OpcodeI32TruncF64SName      = "i32.trunc_f64_s"
	OpcodeI32TruncF64UName      = "i32.trunc_f64_u"
	OpcodeI64ExtendI32SName     = "i64.extend_i32_s"
	OpcodeI64ExtendI32UName     = "i64.extend_i32_u"
	OpcodeI64TruncF32SName      = "i64.trunc_f32_s"
	OpcodeI64TruncF32UName      = "i64.trunc_f32_u"
	OpcodeI64TruncF64SName      = "i64.trunc_f64_s"
	OpcodeI64TruncF64UName      = "i64.trunc_f64_u"
	OpcodeF32ConvertI32SName    = "f32.convert_i32_s"
	OpcodeF32ConvertI32UName    = "f32.convert_i32_u"
	OpcodeF32ConvertI64SName    = "f32.convert_i64_s"
	OpcodeF32ConvertI64UName    = "f32.convert_i64u"
	OpcodeF32DemoteF64Name      = "f32.demote_f64"
	OpcodeF64ConvertI32SName    = "f64.convert_i32_s"
	OpcodeF64ConvertI32UName    = "f64.convert_i32_u"
	OpcodeF64ConvertI64SName    = "f64.convert_i64_s"
	OpcodeF64ConvertI64UName    = "f64.convert_i64_u"
	OpcodeF64PromoteF32Name     = "f64.promote_f32"
	OpcodeI32ReinterpretF32Name = "i32.reinterpret_f32"
	OpcodeI64ReinterpretF64Name = "i64.reinterpret_f64"
	OpcodeF32ReinterpretI32Name = "f32.reinterpret_i32"
	OpcodeF64ReinterpretI64Name = "f64.reinterpret_i64"

	OpcodeRefNullName   = "ref.null"
	OpcodeRefIsNullName = "ref.is_null"
	OpcodeRefFuncName   = "ref.func"

	OpcodeTableGetName = "table.get"
	OpcodeTableSetName = "table.set"

	// Below are toggled with CoreFeatureSignExtensionOps

	OpcodeI32Extend8SName  = "i32.extend8_s"
	OpcodeI32Extend16SName = "i32.extend16_s"
	OpcodeI64Extend8SName  = "i64.extend8_s"
	OpcodeI64Extend16SName = "i64.extend16_s"
	OpcodeI64Extend32SName = "i64.extend32_s"

	OpcodeMiscPrefixName   = "misc_prefix"
	OpcodeVecPrefixName    = "vector_prefix"
	OpcodeAtomicPrefixName = "atomic_prefix"
)

var instructionNames = [256]string{
	OpcodeUnreachable:       OpcodeUnreachableName,
	OpcodeNop:               OpcodeNopName,
	OpcodeBlock:             OpcodeBlockName,
	OpcodeLoop:              OpcodeLoopName,
	OpcodeIf:                OpcodeIfName,
	OpcodeElse:              OpcodeElseName,
	OpcodeEnd:               OpcodeEndName,
	OpcodeBr:                OpcodeBrName,
	OpcodeBrIf:              OpcodeBrIfName,
	OpcodeBrTable:           OpcodeBrTableName,
	OpcodeReturn:            OpcodeReturnName,
	OpcodeCall:              OpcodeCallName,
	OpcodeCallIndirect:      OpcodeCallIndirectName,
	OpcodeDrop:              OpcodeDropName,
	OpcodeSelect:            OpcodeSelectName,
	OpcodeTypedSelect:       OpcodeTypedSelectName,
	OpcodeLocalGet:          OpcodeLocalGetName,
	OpcodeLocalSet:          OpcodeLocalSetName,
	OpcodeLocalTee:          OpcodeLocalTeeName,
	OpcodeGlobalGet:         OpcodeGlobalGetName,
	OpcodeGlobalSet:         OpcodeGlobalSetName,
	OpcodeI32Load:           OpcodeI32LoadName,
	OpcodeI64Load:           OpcodeI64LoadName,
	OpcodeF32Load:           OpcodeF32LoadName,
	OpcodeF64Load:           OpcodeF64LoadName,
	OpcodeI32Load8S:         OpcodeI32Load8SName,
	OpcodeI32Load8U:         OpcodeI32Load8UName,
	OpcodeI32Load16S:        OpcodeI32Load16SName,
	OpcodeI32Load16U:        OpcodeI32Load16UName,
	OpcodeI64Load8S:         OpcodeI64Load8SName,
	OpcodeI64Load8U:         OpcodeI64Load8UName,
	OpcodeI64Load16S:        OpcodeI64Load16SName,
	OpcodeI64Load16U:        OpcodeI64Load16UName,
	OpcodeI64Load32S:        OpcodeI64Load32SName,
	OpcodeI64Load32U:        OpcodeI64Load32UName,
	OpcodeI32Store:          OpcodeI32StoreName,
	OpcodeI64Store:          OpcodeI64StoreName,
	OpcodeF32Store:          OpcodeF32StoreName,
	OpcodeF64Store:          OpcodeF64StoreName,
	OpcodeI32Store8:         OpcodeI32Store8Name,
	OpcodeI32Store16:        OpcodeI32Store16Name,
	OpcodeI64Store8:         OpcodeI64Store8Name,
	OpcodeI64Store16:        OpcodeI64Store16Name,
	OpcodeI64Store32:        OpcodeI64Store32Name,
	OpcodeMemorySize:        OpcodeMemorySizeName,
	OpcodeMemoryGrow:        OpcodeMemoryGrowName,
	OpcodeI32Const:          OpcodeI32ConstName,
	OpcodeI64Const:          OpcodeI64ConstName,
	OpcodeF32Const:          OpcodeF32ConstName,
	OpcodeF64Const:          OpcodeF64ConstName,
	OpcodeI32Eqz:            OpcodeI32EqzName,
	OpcodeI32Eq:             OpcodeI32EqName,
	OpcodeI32Ne:             OpcodeI32NeName,
	OpcodeI32LtS:            OpcodeI32LtSName,
	OpcodeI32LtU:            OpcodeI32LtUName,
	OpcodeI32GtS:            OpcodeI32GtSName,
	OpcodeI32GtU:            OpcodeI32GtUName,
	OpcodeI32LeS:            OpcodeI32LeSName,
	OpcodeI32LeU:            OpcodeI32LeUName,
	OpcodeI32GeS:            OpcodeI32GeSName,
	OpcodeI32GeU:            OpcodeI32GeUName,
	OpcodeI64Eqz:            OpcodeI64EqzName,
	OpcodeI64Eq:             OpcodeI64EqName,
	OpcodeI64Ne:             OpcodeI64NeName,
	OpcodeI64LtS:            OpcodeI64LtSName,
	OpcodeI64LtU:            OpcodeI64LtUName,
	OpcodeI64GtS:            OpcodeI64GtSName,
	OpcodeI64GtU:            OpcodeI64GtUName,
	OpcodeI64LeS:            OpcodeI64LeSName,
	OpcodeI64LeU:            OpcodeI64LeUName,
	OpcodeI64GeS:            OpcodeI64GeSName,
	OpcodeI64GeU:            OpcodeI64GeUName,
	OpcodeF32Eq:             OpcodeF32EqName,
	OpcodeF32Ne:             OpcodeF32NeName,
	OpcodeF32Lt:             OpcodeF32LtName,
	OpcodeF32Gt:             OpcodeF32GtName,
	OpcodeF32Le:             OpcodeF32LeName,
	OpcodeF32Ge:             OpcodeF32GeName,
	OpcodeF64Eq:             OpcodeF64EqName,
	OpcodeF64Ne:             OpcodeF64NeName,
	OpcodeF64Lt:             OpcodeF64LtName,
	OpcodeF64Gt:             OpcodeF64GtName,
	OpcodeF64Le:             OpcodeF64LeName,
	OpcodeF64Ge:             OpcodeF64GeName,
	OpcodeI32Clz:            OpcodeI32ClzName,
	OpcodeI32Ctz:            OpcodeI32CtzName,
	OpcodeI32Popcnt:         OpcodeI32PopcntName,
	OpcodeI32Add:            OpcodeI32AddName,
	OpcodeI32Sub:            OpcodeI32SubName,
	OpcodeI32Mul:            OpcodeI32MulName,
	OpcodeI32DivS:           OpcodeI32DivSName,
	OpcodeI32DivU:           OpcodeI32DivUName,
	OpcodeI32RemS:           OpcodeI32RemSName,
	OpcodeI32RemU:           OpcodeI32RemUName,
	OpcodeI32And:            OpcodeI32AndName,
	OpcodeI32Or:             OpcodeI32OrName,
	OpcodeI32Xor:            OpcodeI32XorName,
	OpcodeI32Shl:            OpcodeI32ShlName,
	OpcodeI32ShrS:           OpcodeI32ShrSName,
	OpcodeI32ShrU:           OpcodeI32ShrUName,
	OpcodeI32Rotl:           OpcodeI32RotlName,
	OpcodeI32Rotr:           OpcodeI32RotrName,
	OpcodeI64Clz:            OpcodeI64ClzName,
	OpcodeI64Ctz:            OpcodeI64CtzName,
	OpcodeI64Popcnt:         OpcodeI64PopcntName,
	OpcodeI64Add:            OpcodeI64AddName,
	OpcodeI64Sub:            OpcodeI64SubName,
	OpcodeI64Mul:            OpcodeI64MulName,
	OpcodeI64DivS:           OpcodeI64DivSName,
	OpcodeI64DivU:           OpcodeI64DivUName,
	OpcodeI64RemS:           OpcodeI64RemSName,
	OpcodeI64RemU:           OpcodeI64RemUName,
	OpcodeI64And:            OpcodeI64AndName,
	OpcodeI64Or:             OpcodeI64OrName,
	OpcodeI64Xor:            OpcodeI64XorName,
	OpcodeI64Shl:            OpcodeI64ShlName,
	OpcodeI64ShrS:           OpcodeI64ShrSName,
	OpcodeI64ShrU:           OpcodeI64ShrUName,
	OpcodeI64Rotl:           OpcodeI64RotlName,
	OpcodeI64Rotr:           OpcodeI64RotrName,
	OpcodeF32Abs:            OpcodeF32AbsName,
	OpcodeF32Neg:            OpcodeF32NegName,
	OpcodeF32Ceil:           OpcodeF32CeilName,
	OpcodeF32Floor:          OpcodeF32FloorName,
	OpcodeF32Trunc:          OpcodeF32TruncName,
	OpcodeF32Nearest:        OpcodeF32NearestName,
	OpcodeF32Sqrt:           OpcodeF32SqrtName,
	OpcodeF32Add:            OpcodeF32AddName,
	OpcodeF32Sub:            OpcodeF32SubName,
	OpcodeF32Mul:            OpcodeF32MulName,
	OpcodeF32Div:            OpcodeF32DivName,
	OpcodeF32Min:            OpcodeF32MinName,
	OpcodeF32Max:            OpcodeF32MaxName,
	OpcodeF32Copysign:       OpcodeF32CopysignName,
	OpcodeF64Abs:            OpcodeF64AbsName,
	OpcodeF64Neg:            OpcodeF64NegName,
	OpcodeF64Ceil:           OpcodeF64CeilName,
	OpcodeF64Floor:          OpcodeF64FloorName,
	OpcodeF64Trunc:          OpcodeF64TruncName,
	OpcodeF64Nearest:        OpcodeF64NearestName,
	OpcodeF64Sqrt:           OpcodeF64SqrtName,
	OpcodeF64Add:            OpcodeF64AddName,
	OpcodeF64Sub:            OpcodeF64SubName,
	OpcodeF64Mul:            OpcodeF64MulName,
	OpcodeF64Div:            OpcodeF64DivName,
	OpcodeF64Min:            OpcodeF64MinName,
	OpcodeF64Max:            OpcodeF64MaxName,
	OpcodeF64Copysign:       OpcodeF64CopysignName,
	OpcodeI32WrapI64:        OpcodeI32WrapI64Name,
	OpcodeI32TruncF32S:      OpcodeI32TruncF32SName,
	OpcodeI32TruncF32U:      OpcodeI32TruncF32UName,
	OpcodeI32TruncF64S:      OpcodeI32TruncF64SName,
	OpcodeI32TruncF64U:      OpcodeI32TruncF64UName,
	OpcodeI64ExtendI32S:     OpcodeI64ExtendI32SName,
	OpcodeI64ExtendI32U:     OpcodeI64ExtendI32UName,
	OpcodeI64TruncF32S:      OpcodeI64TruncF32SName,
	OpcodeI64TruncF32U:      OpcodeI64TruncF32UName,
	OpcodeI64TruncF64S:      OpcodeI64TruncF64SName,
	OpcodeI64TruncF64U:      OpcodeI64TruncF64UName,
	OpcodeF32ConvertI32S:    OpcodeF32ConvertI32SName,
	OpcodeF32ConvertI32U:    OpcodeF32ConvertI32UName,
	OpcodeF32ConvertI64S:    OpcodeF32ConvertI64SName,
	OpcodeF32ConvertI64U:    OpcodeF32ConvertI64UName,
	OpcodeF32DemoteF64:      OpcodeF32DemoteF64Name,
	OpcodeF64ConvertI32S:    OpcodeF64ConvertI32SName,
	OpcodeF64ConvertI32U:    OpcodeF64ConvertI32UName,
	OpcodeF64ConvertI64S:    OpcodeF64ConvertI64SName,
	OpcodeF64ConvertI64U:    OpcodeF64ConvertI64UName,
	OpcodeF64PromoteF32:     OpcodeF64PromoteF32Name,
	OpcodeI32ReinterpretF32: OpcodeI32ReinterpretF32Name,
	OpcodeI64ReinterpretF64: OpcodeI64ReinterpretF64Name,
	OpcodeF32ReinterpretI32: OpcodeF32ReinterpretI32Name,
	OpcodeF64ReinterpretI64: OpcodeF64ReinterpretI64Name,

	OpcodeRefNull:   OpcodeRefNullName,
	OpcodeRefIsNull: OpcodeRefIsNullName,
	OpcodeRefFunc:   OpcodeRefFuncName,

	OpcodeTableGet: OpcodeTableGetName,
	OpcodeTableSet: OpcodeTableSetName,

	// Below are toggled with CoreFeatureSignExtensionOps

	OpcodeI32Extend8S:  OpcodeI32Extend8SName,
	OpcodeI32Extend16S: OpcodeI32Extend16SName,
	OpcodeI64Extend8S:  OpcodeI64Extend8SName,
	OpcodeI64Extend16S: OpcodeI64Extend16SName,
	OpcodeI64Extend32S: OpcodeI64Extend32SName,

	OpcodeMiscPrefix: OpcodeMiscPrefixName,
	OpcodeVecPrefix:  OpcodeVecPrefixName,
}

// InstructionName returns the instruction corresponding to this binary Opcode.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#a7-index-of-instructions
func InstructionName(oc Opcode) string {
	return instructionNames[oc]
}

const (
	OpcodeI32TruncSatF32SName = "i32.trunc_sat_f32_s"
	OpcodeI32TruncSatF32UName = "i32.trunc_sat_f32_u"
	OpcodeI32TruncSatF64SName = "i32.trunc_sat_f64_s"
	OpcodeI32TruncSatF64UName = "i32.trunc_sat_f64_u"
	OpcodeI64TruncSatF32SName = "i64.trunc_sat_f32_s"
	OpcodeI64TruncSatF32UName = "i64.trunc_sat_f32_u"
	OpcodeI64TruncSatF64SName = "i64.trunc_sat_f64_s"
	OpcodeI64TruncSatF64UName = "i64.trunc_sat_f64_u"

	OpcodeMemoryInitName = "memory.init"
	OpcodeDataDropName   = "data.drop"
	OpcodeMemoryCopyName = "memory.copy"
	OpcodeMemoryFillName = "memory.fill"
	OpcodeTableInitName  = "table.init"
	OpcodeElemDropName   = "elem.drop"
	OpcodeTableCopyName  = "table.copy"
	OpcodeTableGrowName  = "table.grow"
	OpcodeTableSizeName  = "table.size"
	OpcodeTableFillName  = "table.fill"
)

var miscInstructionNames = [256]string{
	OpcodeMiscI32TruncSatF32S: OpcodeI32TruncSatF32SName,
	OpcodeMiscI32TruncSatF32U: OpcodeI32TruncSatF32UName,
	OpcodeMiscI32TruncSatF64S: OpcodeI32TruncSatF64SName,
	OpcodeMiscI32TruncSatF64U: OpcodeI32TruncSatF64UName,
	OpcodeMiscI64TruncSatF32S: OpcodeI64TruncSatF32SName,
	OpcodeMiscI64TruncSatF32U: OpcodeI64TruncSatF32UName,
	OpcodeMiscI64TruncSatF64S: OpcodeI64TruncSatF64SName,
	OpcodeMiscI64TruncSatF64U: OpcodeI64TruncSatF64UName,

	OpcodeMiscMemoryInit: OpcodeMemoryInitName,
	OpcodeMiscDataDrop:   OpcodeDataDropName,
	OpcodeMiscMemoryCopy: OpcodeMemoryCopyName,
	OpcodeMiscMemoryFill: OpcodeMemoryFillName,
	OpcodeMiscTableInit:  OpcodeTableInitName,
	OpcodeMiscElemDrop:   OpcodeElemDropName,
	OpcodeMiscTableCopy:  OpcodeTableCopyName,
	OpcodeMiscTableGrow:  OpcodeTableGrowName,
	OpcodeMiscTableSize:  OpcodeTableSizeName,
	OpcodeMiscTableFill:  OpcodeTableFillName,
}

// MiscInstructionName returns the instruction corresponding to this miscellaneous Opcode.
func MiscInstructionName(oc OpcodeMisc) string {
	return miscInstructionNames[oc]
}

const (
	OpcodeVecV128LoadName                  = "v128.load"
	OpcodeVecV128Load8x8SName              = "v128.load8x8_s"
	OpcodeVecV128Load8x8UName              = "v128.load8x8_u"
	OpcodeVecV128Load16x4SName             = "v128.load16x4_s"
	OpcodeVecV128Load16x4UName             = "v128.load16x4_u"
	OpcodeVecV128Load32x2SName             = "v128.load32x2_s"
	OpcodeVecV128Load32x2UName             = "v128.load32x2_u"
	OpcodeVecV128Load8SplatName            = "v128.load8_splat"
	OpcodeVecV128Load16SplatName           = "v128.load16_splat"
	OpcodeVecV128Load32SplatName           = "v128.load32_splat"
	OpcodeVecV128Load64SplatName           = "v128.load64_splat"
	OpcodeVecV128Load32zeroName            = "v128.load32_zero"
	OpcodeVecV128Load64zeroName            = "v128.load64_zero"
	OpcodeVecV128StoreName                 = "v128.store"
	OpcodeVecV128Load8LaneName             = "v128.load8_lane"
	OpcodeVecV128Load16LaneName            = "v128.load16_lane"
	OpcodeVecV128Load32LaneName            = "v128.load32_lane"
	OpcodeVecV128Load64LaneName            = "v128.load64_lane"
	OpcodeVecV128Store8LaneName            = "v128.store8_lane"
	OpcodeVecV128Store16LaneName           = "v128.store16_lane"
	OpcodeVecV128Store32LaneName           = "v128.store32_lane"
	OpcodeVecV128Store64LaneName           = "v128.store64_lane"
	OpcodeVecV128ConstName                 = "v128.const"
	OpcodeVecV128i8x16ShuffleName          = "v128.shuffle"
	OpcodeVecI8x16ExtractLaneSName         = "i8x16.extract_lane_s"
	OpcodeVecI8x16ExtractLaneUName         = "i8x16.extract_lane_u"
	OpcodeVecI8x16ReplaceLaneName          = "i8x16.replace_lane"
	OpcodeVecI16x8ExtractLaneSName         = "i16x8.extract_lane_s"
	OpcodeVecI16x8ExtractLaneUName         = "i16x8.extract_lane_u"
	OpcodeVecI16x8ReplaceLaneName          = "i16x8.replace_lane"
	OpcodeVecI32x4ExtractLaneName          = "i32x4.extract_lane"
	OpcodeVecI32x4ReplaceLaneName          = "i32x4.replace_lane"
	OpcodeVecI64x2ExtractLaneName          = "i64x2.extract_lane"
	OpcodeVecI64x2ReplaceLaneName          = "i64x2.replace_lane"
	OpcodeVecF32x4ExtractLaneName          = "f32x4.extract_lane"
	OpcodeVecF32x4ReplaceLaneName          = "f32x4.replace_lane"
	OpcodeVecF64x2ExtractLaneName          = "f64x2.extract_lane"
	OpcodeVecF64x2ReplaceLaneName          = "f64x2.replace_lane"
	OpcodeVecI8x16SwizzleName              = "i8x16.swizzle"
	OpcodeVecI8x16SplatName                = "i8x16.splat"
	OpcodeVecI16x8SplatName                = "i16x8.splat"
	OpcodeVecI32x4SplatName                = "i32x4.splat"
	OpcodeVecI64x2SplatName                = "i64x2.splat"
	OpcodeVecF32x4SplatName                = "f32x4.splat"
	OpcodeVecF64x2SplatName                = "f64x2.splat"
	OpcodeVecI8x16EqName                   = "i8x16.eq"
	OpcodeVecI8x16NeName                   = "i8x16.ne"
	OpcodeVecI8x16LtSName                  = "i8x16.lt_s"
	OpcodeVecI8x16LtUName                  = "i8x16.lt_u"
	OpcodeVecI8x16GtSName                  = "i8x16.gt_s"
	OpcodeVecI8x16GtUName                  = "i8x16.gt_u"
	OpcodeVecI8x16LeSName                  = "i8x16.le_s"
	OpcodeVecI8x16LeUName                  = "i8x16.le_u"
	OpcodeVecI8x16GeSName                  = "i8x16.ge_s"
	OpcodeVecI8x16GeUName                  = "i8x16.ge_u"
	OpcodeVecI16x8EqName                   = "i16x8.eq"
	OpcodeVecI16x8NeName                   = "i16x8.ne"
	OpcodeVecI16x8LtSName                  = "i16x8.lt_s"
	OpcodeVecI16x8LtUName                  = "i16x8.lt_u"
	OpcodeVecI16x8GtSName                  = "i16x8.gt_s"
	OpcodeVecI16x8GtUName                  = "i16x8.gt_u"
	OpcodeVecI16x8LeSName                  = "i16x8.le_s"
	OpcodeVecI16x8LeUName                  = "i16x8.le_u"
	OpcodeVecI16x8GeSName                  = "i16x8.ge_s"
	OpcodeVecI16x8GeUName                  = "i16x8.ge_u"
	OpcodeVecI32x4EqName                   = "i32x4.eq"
	OpcodeVecI32x4NeName                   = "i32x4.ne"
	OpcodeVecI32x4LtSName                  = "i32x4.lt_s"
	OpcodeVecI32x4LtUName                  = "i32x4.lt_u"
	OpcodeVecI32x4GtSName                  = "i32x4.gt_s"
	OpcodeVecI32x4GtUName                  = "i32x4.gt_u"
	OpcodeVecI32x4LeSName                  = "i32x4.le_s"
	OpcodeVecI32x4LeUName                  = "i32x4.le_u"
	OpcodeVecI32x4GeSName                  = "i32x4.ge_s"
	OpcodeVecI32x4GeUName                  = "i32x4.ge_u"
	OpcodeVecI64x2EqName                   = "i64x2.eq"
	OpcodeVecI64x2NeName                   = "i64x2.ne"
	OpcodeVecI64x2LtSName                  = "i64x2.lt"
	OpcodeVecI64x2GtSName                  = "i64x2.gt"
	OpcodeVecI64x2LeSName                  = "i64x2.le"
	OpcodeVecI64x2GeSName                  = "i64x2.ge"
	OpcodeVecF32x4EqName                   = "f32x4.eq"
	OpcodeVecF32x4NeName                   = "f32x4.ne"
	OpcodeVecF32x4LtName                   = "f32x4.lt"
	OpcodeVecF32x4GtName                   = "f32x4.gt"
	OpcodeVecF32x4LeName                   = "f32x4.le"
	OpcodeVecF32x4GeName                   = "f32x4.ge"
	OpcodeVecF64x2EqName                   = "f64x2.eq"
	OpcodeVecF64x2NeName                   = "f64x2.ne"
	OpcodeVecF64x2LtName                   = "f64x2.lt"
	OpcodeVecF64x2GtName                   = "f64x2.gt"
	OpcodeVecF64x2LeName                   = "f64x2.le"
	OpcodeVecF64x2GeName                   = "f64x2.ge"
	OpcodeVecV128NotName                   = "v128.not"
	OpcodeVecV128AndName                   = "v128.and"
	OpcodeVecV128AndNotName                = "v128.andnot"
	OpcodeVecV128OrName                    = "v128.or"
	OpcodeVecV128XorName                   = "v128.xor"
	OpcodeVecV128BitselectName             = "v128.bitselect"
	OpcodeVecV128AnyTrueName               = "v128.any_true"
	OpcodeVecI8x16AbsName                  = "i8x16.abs"
	OpcodeVecI8x16NegName                  = "i8x16.neg"
	OpcodeVecI8x16PopcntName               = "i8x16.popcnt"
	OpcodeVecI8x16AllTrueName              = "i8x16.all_true"
	OpcodeVecI8x16BitMaskName              = "i8x16.bitmask"
	OpcodeVecI8x16NarrowI16x8SName         = "i8x16.narrow_i16x8_s"
	OpcodeVecI8x16NarrowI16x8UName         = "i8x16.narrow_i16x8_u"
	OpcodeVecI8x16ShlName                  = "i8x16.shl"
	OpcodeVecI8x16ShrSName                 = "i8x16.shr_s"
	OpcodeVecI8x16ShrUName                 = "i8x16.shr_u"
	OpcodeVecI8x16AddName                  = "i8x16.add"
	OpcodeVecI8x16AddSatSName              = "i8x16.add_sat_s"
	OpcodeVecI8x16AddSatUName              = "i8x16.add_sat_u"
	OpcodeVecI8x16SubName                  = "i8x16.sub"
	OpcodeVecI8x16SubSatSName              = "i8x16.sub_s"
	OpcodeVecI8x16SubSatUName              = "i8x16.sub_u"
	OpcodeVecI8x16MinSName                 = "i8x16.min_s"
	OpcodeVecI8x16MinUName                 = "i8x16.min_u"
	OpcodeVecI8x16MaxSName                 = "i8x16.max_s"
	OpcodeVecI8x16MaxUName                 = "i8x16.max_u"
	OpcodeVecI8x16AvgrUName                = "i8x16.avgr_u"
	OpcodeVecI16x8ExtaddPairwiseI8x16SName = "i16x8.extadd_pairwise_i8x16_s"
	OpcodeVecI16x8ExtaddPairwiseI8x16UName = "i16x8.extadd_pairwise_i8x16_u"
	OpcodeVecI16x8AbsName                  = "i16x8.abs"
	OpcodeVecI16x8NegName                  = "i16x8.neg"
	OpcodeVecI16x8Q15mulrSatSName          = "i16x8.q15mulr_sat_s"
	OpcodeVecI16x8AllTrueName              = "i16x8.all_true"
	OpcodeVecI16x8BitMaskName              = "i16x8.bitmask"
	OpcodeVecI16x8NarrowI32x4SName         = "i16x8.narrow_i32x4_s"
	OpcodeVecI16x8NarrowI32x4UName         = "i16x8.narrow_i32x4_u"
	OpcodeVecI16x8ExtendLowI8x16SName      = "i16x8.extend_low_i8x16_s"
	OpcodeVecI16x8ExtendHighI8x16SName     = "i16x8.extend_high_i8x16_s"
	OpcodeVecI16x8ExtendLowI8x16UName      = "i16x8.extend_low_i8x16_u"
	OpcodeVecI16x8ExtendHighI8x16UName     = "i16x8.extend_high_i8x16_u"
	OpcodeVecI16x8ShlName                  = "i16x8.shl"
	OpcodeVecI16x8ShrSName                 = "i16x8.shr_s"
	OpcodeVecI16x8ShrUName                 = "i16x8.shr_u"
	OpcodeVecI16x8AddName                  = "i16x8.add"
	OpcodeVecI16x8AddSatSName              = "i16x8.add_sat_s"
	OpcodeVecI16x8AddSatUName              = "i16x8.add_sat_u"
	OpcodeVecI16x8SubName                  = "i16x8.sub"
	OpcodeVecI16x8SubSatSName              = "i16x8.sub_sat_s"
	OpcodeVecI16x8SubSatUName              = "i16x8.sub_sat_u"
	OpcodeVecI16x8MulName                  = "i16x8.mul"
	OpcodeVecI16x8MinSName                 = "i16x8.min_s"
	OpcodeVecI16x8MinUName                 = "i16x8.min_u"
	OpcodeVecI16x8MaxSName                 = "i16x8.max_s"
	OpcodeVecI16x8MaxUName                 = "i16x8.max_u"
	OpcodeVecI16x8AvgrUName                = "i16x8.avgr_u"
	OpcodeVecI16x8ExtMulLowI8x16SName      = "i16x8.extmul_low_i8x16_s"
	OpcodeVecI16x8ExtMulHighI8x16SName     = "i16x8.extmul_high_i8x16_s"
	OpcodeVecI16x8ExtMulLowI8x16UName      = "i16x8.extmul_low_i8x16_u"
	OpcodeVecI16x8ExtMulHighI8x16UName     = "i16x8.extmul_high_i8x16_u"
	OpcodeVecI32x4ExtaddPairwiseI16x8SName = "i32x4.extadd_pairwise_i16x8_s"
	OpcodeVecI32x4ExtaddPairwiseI16x8UName = "i32x4.extadd_pairwise_i16x8_u"
	OpcodeVecI32x4AbsName                  = "i32x4.abs"
	OpcodeVecI32x4NegName                  = "i32x4.neg"
	OpcodeVecI32x4AllTrueName              = "i32x4.all_true"
	OpcodeVecI32x4BitMaskName              = "i32x4.bitmask"
	OpcodeVecI32x4ExtendLowI16x8SName      = "i32x4.extend_low_i16x8_s"
	OpcodeVecI32x4ExtendHighI16x8SName     = "i32x4.extend_high_i16x8_s"
	OpcodeVecI32x4ExtendLowI16x8UName      = "i32x4.extend_low_i16x8_u"
	OpcodeVecI32x4ExtendHighI16x8UName     = "i32x4.extend_high_i16x8_u"
	OpcodeVecI32x4ShlName                  = "i32x4.shl"
	OpcodeVecI32x4ShrSName                 = "i32x4.shr_s"
	OpcodeVecI32x4ShrUName                 = "i32x4.shr_u"
	OpcodeVecI32x4AddName                  = "i32x4.add"
	OpcodeVecI32x4SubName                  = "i32x4.sub"
	OpcodeVecI32x4MulName                  = "i32x4.mul"
	OpcodeVecI32x4MinSName                 = "i32x4.min_s"
	OpcodeVecI32x4MinUName                 = "i32x4.min_u"
	OpcodeVecI32x4MaxSName                 = "i32x4.max_s"
	OpcodeVecI32x4MaxUName                 = "i32x4.max_u"
	OpcodeVecI32x4DotI16x8SName            = "i32x4.dot_i16x8_s"
	OpcodeVecI32x4ExtMulLowI16x8SName      = "i32x4.extmul_low_i16x8_s"
	OpcodeVecI32x4ExtMulHighI16x8SName     = "i32x4.extmul_high_i16x8_s"
	OpcodeVecI32x4ExtMulLowI16x8UName      = "i32x4.extmul_low_i16x8_u"
	OpcodeVecI32x4ExtMulHighI16x8UName     = "i32x4.extmul_high_i16x8_u"
	OpcodeVecI64x2AbsName                  = "i64x2.abs"
	OpcodeVecI64x2NegName                  = "i64x2.neg"
	OpcodeVecI64x2AllTrueName              = "i64x2.all_true"
	OpcodeVecI64x2BitMaskName              = "i64x2.bitmask"
	OpcodeVecI64x2ExtendLowI32x4SName      = "i64x2.extend_low_i32x4_s"
	OpcodeVecI64x2ExtendHighI32x4SName     = "i64x2.extend_high_i32x4_s"
	OpcodeVecI64x2ExtendLowI32x4UName      = "i64x2.extend_low_i32x4_u"
	OpcodeVecI64x2ExtendHighI32x4UName     = "i64x2.extend_high_i32x4_u"
	OpcodeVecI64x2ShlName                  = "i64x2.shl"
	OpcodeVecI64x2ShrSName                 = "i64x2.shr_s"
	OpcodeVecI64x2ShrUName                 = "i64x2.shr_u"
	OpcodeVecI64x2AddName                  = "i64x2.add"
	OpcodeVecI64x2SubName                  = "i64x2.sub"
	OpcodeVecI64x2MulName                  = "i64x2.mul"
	OpcodeVecI64x2ExtMulLowI32x4SName      = "i64x2.extmul_low_i32x4_s"
	OpcodeVecI64x2ExtMulHighI32x4SName     = "i64x2.extmul_high_i32x4_s"
	OpcodeVecI64x2ExtMulLowI32x4UName      = "i64x2.extmul_low_i32x4_u"
	OpcodeVecI64x2ExtMulHighI32x4UName     = "i64x2.extmul_high_i32x4_u"
	OpcodeVecF32x4CeilName                 = "f32x4.ceil"
	OpcodeVecF32x4FloorName                = "f32x4.floor"
	OpcodeVecF32x4TruncName                = "f32x4.trunc"
	OpcodeVecF32x4NearestName              = "f32x4.nearest"
	OpcodeVecF32x4AbsName                  = "f32x4.abs"
	OpcodeVecF32x4NegName                  = "f32x4.neg"
	OpcodeVecF32x4SqrtName                 = "f32x4.sqrt"
	OpcodeVecF32x4AddName                  = "f32x4.add"
	OpcodeVecF32x4SubName                  = "f32x4.sub"
	OpcodeVecF32x4MulName                  = "f32x4.mul"
	OpcodeVecF32x4DivName                  = "f32x4.div"
	OpcodeVecF32x4MinName                  = "f32x4.min"
	OpcodeVecF32x4MaxName                  = "f32x4.max"
	OpcodeVecF32x4PminName                 = "f32x4.pmin"
	OpcodeVecF32x4PmaxName                 = "f32x4.pmax"
	OpcodeVecF64x2CeilName                 = "f64x2.ceil"
	OpcodeVecF64x2FloorName                = "f64x2.floor"
	OpcodeVecF64x2TruncName                = "f64x2.trunc"
	OpcodeVecF64x2NearestName              = "f64x2.nearest"
	OpcodeVecF64x2AbsName                  = "f64x2.abs"
	OpcodeVecF64x2NegName                  = "f64x2.neg"
	OpcodeVecF64x2SqrtName                 = "f64x2.sqrt"
	OpcodeVecF64x2AddName                  = "f64x2.add"
	OpcodeVecF64x2SubName                  = "f64x2.sub"
	OpcodeVecF64x2MulName                  = "f64x2.mul"
	OpcodeVecF64x2DivName                  = "f64x2.div"
	OpcodeVecF64x2MinName                  = "f64x2.min"
	OpcodeVecF64x2MaxName                  = "f64x2.max"
	OpcodeVecF64x2PminName                 = "f64x2.pmin"
	OpcodeVecF64x2PmaxName                 = "f64x2.pmax"
	OpcodeVecI32x4TruncSatF32x4SName       = "i32x4.trunc_sat_f32x4_s"
	OpcodeVecI32x4TruncSatF32x4UName       = "i32x4.trunc_sat_f32x4_u"
	OpcodeVecF32x4ConvertI32x4SName        = "f32x4.convert_i32x4_s"
	OpcodeVecF32x4ConvertI32x4UName        = "f32x4.convert_i32x4_u"
	OpcodeVecI32x4TruncSatF64x2SZeroName   = "i32x4.trunc_sat_f64x2_s_zero"
	OpcodeVecI32x4TruncSatF64x2UZeroName   = "i32x4.trunc_sat_f64x2_u_zero"
	OpcodeVecF64x2ConvertLowI32x4SName     = "f64x2.convert_low_i32x4_s"
	OpcodeVecF64x2ConvertLowI32x4UName     = "f64x2.convert_low_i32x4_u"
	OpcodeVecF32x4DemoteF64x2ZeroName      = "f32x4.demote_f64x2_zero"
	OpcodeVecF64x2PromoteLowF32x4ZeroName  = "f64x2.promote_low_f32x4"
)

var vectorInstructionName = map[OpcodeVec]string{
	OpcodeVecV128Load:                  OpcodeVecV128LoadName,
	OpcodeVecV128Load8x8s:              OpcodeVecV128Load8x8SName,
	OpcodeVecV128Load8x8u:              OpcodeVecV128Load8x8UName,
	OpcodeVecV128Load16x4s:             OpcodeVecV128Load16x4SName,
	OpcodeVecV128Load16x4u:             OpcodeVecV128Load16x4UName,
	OpcodeVecV128Load32x2s:             OpcodeVecV128Load32x2SName,
	OpcodeVecV128Load32x2u:             OpcodeVecV128Load32x2UName,
	OpcodeVecV128Load8Splat:            OpcodeVecV128Load8SplatName,
	OpcodeVecV128Load16Splat:           OpcodeVecV128Load16SplatName,
	OpcodeVecV128Load32Splat:           OpcodeVecV128Load32SplatName,
	OpcodeVecV128Load64Splat:           OpcodeVecV128Load64SplatName,
	OpcodeVecV128Load32zero:            OpcodeVecV128Load32zeroName,
	OpcodeVecV128Load64zero:            OpcodeVecV128Load64zeroName,
	OpcodeVecV128Store:                 OpcodeVecV128StoreName,
	OpcodeVecV128Load8Lane:             OpcodeVecV128Load8LaneName,
	OpcodeVecV128Load16Lane:            OpcodeVecV128Load16LaneName,
	OpcodeVecV128Load32Lane:            OpcodeVecV128Load32LaneName,
	OpcodeVecV128Load64Lane:            OpcodeVecV128Load64LaneName,
	OpcodeVecV128Store8Lane:            OpcodeVecV128Store8LaneName,
	OpcodeVecV128Store16Lane:           OpcodeVecV128Store16LaneName,
	OpcodeVecV128Store32Lane:           OpcodeVecV128Store32LaneName,
	OpcodeVecV128Store64Lane:           OpcodeVecV128Store64LaneName,
	OpcodeVecV128Const:                 OpcodeVecV128ConstName,
	OpcodeVecV128i8x16Shuffle:          OpcodeVecV128i8x16ShuffleName,
	OpcodeVecI8x16ExtractLaneS:         OpcodeVecI8x16ExtractLaneSName,
	OpcodeVecI8x16ExtractLaneU:         OpcodeVecI8x16ExtractLaneUName,
	OpcodeVecI8x16ReplaceLane:          OpcodeVecI8x16ReplaceLaneName,
	OpcodeVecI16x8ExtractLaneS:         OpcodeVecI16x8ExtractLaneSName,
	OpcodeVecI16x8ExtractLaneU:         OpcodeVecI16x8ExtractLaneUName,
	OpcodeVecI16x8ReplaceLane:          OpcodeVecI16x8ReplaceLaneName,
	OpcodeVecI32x4ExtractLane:          OpcodeVecI32x4ExtractLaneName,
	OpcodeVecI32x4ReplaceLane:          OpcodeVecI32x4ReplaceLaneName,
	OpcodeVecI64x2ExtractLane:          OpcodeVecI64x2ExtractLaneName,
	OpcodeVecI64x2ReplaceLane:          OpcodeVecI64x2ReplaceLaneName,
	OpcodeVecF32x4ExtractLane:          OpcodeVecF32x4ExtractLaneName,
	OpcodeVecF32x4ReplaceLane:          OpcodeVecF32x4ReplaceLaneName,
	OpcodeVecF64x2ExtractLane:          OpcodeVecF64x2ExtractLaneName,
	OpcodeVecF64x2ReplaceLane:          OpcodeVecF64x2ReplaceLaneName,
	OpcodeVecI8x16Swizzle:              OpcodeVecI8x16SwizzleName,
	OpcodeVecI8x16Splat:                OpcodeVecI8x16SplatName,
	OpcodeVecI16x8Splat:                OpcodeVecI16x8SplatName,
	OpcodeVecI32x4Splat:                OpcodeVecI32x4SplatName,
	OpcodeVecI64x2Splat:                OpcodeVecI64x2SplatName,
	OpcodeVecF32x4Splat:                OpcodeVecF32x4SplatName,
	OpcodeVecF64x2Splat:                OpcodeVecF64x2SplatName,
	OpcodeVecI8x16Eq:                   OpcodeVecI8x16EqName,
	OpcodeVecI8x16Ne:                   OpcodeVecI8x16NeName,
	OpcodeVecI8x16LtS:                  OpcodeVecI8x16LtSName,
	OpcodeVecI8x16LtU:                  OpcodeVecI8x16LtUName,
	OpcodeVecI8x16GtS:                  OpcodeVecI8x16GtSName,
	OpcodeVecI8x16GtU:                  OpcodeVecI8x16GtUName,
	OpcodeVecI8x16LeS:                  OpcodeVecI8x16LeSName,
	OpcodeVecI8x16LeU:                  OpcodeVecI8x16LeUName,
	OpcodeVecI8x16GeS:                  OpcodeVecI8x16GeSName,
	OpcodeVecI8x16GeU:                  OpcodeVecI8x16GeUName,
	OpcodeVecI16x8Eq:                   OpcodeVecI16x8EqName,
	OpcodeVecI16x8Ne:                   OpcodeVecI16x8NeName,
	OpcodeVecI16x8LtS:                  OpcodeVecI16x8LtSName,
	OpcodeVecI16x8LtU:                  OpcodeVecI16x8LtUName,
	OpcodeVecI16x8GtS:                  OpcodeVecI16x8GtSName,
	OpcodeVecI16x8GtU:                  OpcodeVecI16x8GtUName,
	OpcodeVecI16x8LeS:                  OpcodeVecI16x8LeSName,
	OpcodeVecI16x8LeU:                  OpcodeVecI16x8LeUName,
	OpcodeVecI16x8GeS:                  OpcodeVecI16x8GeSName,
	OpcodeVecI16x8GeU:                  OpcodeVecI16x8GeUName,
	OpcodeVecI32x4Eq:                   OpcodeVecI32x4EqName,
	OpcodeVecI32x4Ne:                   OpcodeVecI32x4NeName,
	OpcodeVecI32x4LtS:                  OpcodeVecI32x4LtSName,
	OpcodeVecI32x4LtU:                  OpcodeVecI32x4LtUName,
	OpcodeVecI32x4GtS:                  OpcodeVecI32x4GtSName,
	OpcodeVecI32x4GtU:                  OpcodeVecI32x4GtUName,
	OpcodeVecI32x4LeS:                  OpcodeVecI32x4LeSName,
	OpcodeVecI32x4LeU:                  OpcodeVecI32x4LeUName,
	OpcodeVecI32x4GeS:                  OpcodeVecI32x4GeSName,
	OpcodeVecI32x4GeU:                  OpcodeVecI32x4GeUName,
	OpcodeVecI64x2Eq:                   OpcodeVecI64x2EqName,
	OpcodeVecI64x2Ne:                   OpcodeVecI64x2NeName,
	OpcodeVecI64x2LtS:                  OpcodeVecI64x2LtSName,
	OpcodeVecI64x2GtS:                  OpcodeVecI64x2GtSName,
	OpcodeVecI64x2LeS:                  OpcodeVecI64x2LeSName,
	OpcodeVecI64x2GeS:                  OpcodeVecI64x2GeSName,
	OpcodeVecF32x4Eq:                   OpcodeVecF32x4EqName,
	OpcodeVecF32x4Ne:                   OpcodeVecF32x4NeName,
	OpcodeVecF32x4Lt:                   OpcodeVecF32x4LtName,
	OpcodeVecF32x4Gt:                   OpcodeVecF32x4GtName,
	OpcodeVecF32x4Le:                   OpcodeVecF32x4LeName,
	OpcodeVecF32x4Ge:                   OpcodeVecF32x4GeName,
	OpcodeVecF64x2Eq:                   OpcodeVecF64x2EqName,
	OpcodeVecF64x2Ne:                   OpcodeVecF64x2NeName,
	OpcodeVecF64x2Lt:                   OpcodeVecF64x2LtName,
	OpcodeVecF64x2Gt:                   OpcodeVecF64x2GtName,
	OpcodeVecF64x2Le:                   OpcodeVecF64x2LeName,
	OpcodeVecF64x2Ge:                   OpcodeVecF64x2GeName,
	OpcodeVecV128Not:                   OpcodeVecV128NotName,
	OpcodeVecV128And:                   OpcodeVecV128AndName,
	OpcodeVecV128AndNot:                OpcodeVecV128AndNotName,
	OpcodeVecV128Or:                    OpcodeVecV128OrName,
	OpcodeVecV128Xor:                   OpcodeVecV128XorName,
	OpcodeVecV128Bitselect:             OpcodeVecV128BitselectName,
	OpcodeVecV128AnyTrue:               OpcodeVecV128AnyTrueName,
	OpcodeVecI8x16Abs:                  OpcodeVecI8x16AbsName,
	OpcodeVecI8x16Neg:                  OpcodeVecI8x16NegName,
	OpcodeVecI8x16Popcnt:               OpcodeVecI8x16PopcntName,
	OpcodeVecI8x16AllTrue:              OpcodeVecI8x16AllTrueName,
	OpcodeVecI8x16BitMask:              OpcodeVecI8x16BitMaskName,
	OpcodeVecI8x16NarrowI16x8S:         OpcodeVecI8x16NarrowI16x8SName,
	OpcodeVecI8x16NarrowI16x8U:         OpcodeVecI8x16NarrowI16x8UName,
	OpcodeVecI8x16Shl:                  OpcodeVecI8x16ShlName,
	OpcodeVecI8x16ShrS:                 OpcodeVecI8x16ShrSName,
	OpcodeVecI8x16ShrU:                 OpcodeVecI8x16ShrUName,
	OpcodeVecI8x16Add:                  OpcodeVecI8x16AddName,
	OpcodeVecI8x16AddSatS:              OpcodeVecI8x16AddSatSName,
	OpcodeVecI8x16AddSatU:              OpcodeVecI8x16AddSatUName,
	OpcodeVecI8x16Sub:                  OpcodeVecI8x16SubName,
	OpcodeVecI8x16SubSatS:              OpcodeVecI8x16SubSatSName,
	OpcodeVecI8x16SubSatU:              OpcodeVecI8x16SubSatUName,
	OpcodeVecI8x16MinS:                 OpcodeVecI8x16MinSName,
	OpcodeVecI8x16MinU:                 OpcodeVecI8x16MinUName,
	OpcodeVecI8x16MaxS:                 OpcodeVecI8x16MaxSName,
	OpcodeVecI8x16MaxU:                 OpcodeVecI8x16MaxUName,
	OpcodeVecI8x16AvgrU:                OpcodeVecI8x16AvgrUName,
	OpcodeVecI16x8ExtaddPairwiseI8x16S: OpcodeVecI16x8ExtaddPairwiseI8x16SName,
	OpcodeVecI16x8ExtaddPairwiseI8x16U: OpcodeVecI16x8ExtaddPairwiseI8x16UName,
	OpcodeVecI16x8Abs:                  OpcodeVecI16x8AbsName,
	OpcodeVecI16x8Neg:                  OpcodeVecI16x8NegName,
	OpcodeVecI16x8Q15mulrSatS:          OpcodeVecI16x8Q15mulrSatSName,
	OpcodeVecI16x8AllTrue:              OpcodeVecI16x8AllTrueName,
	OpcodeVecI16x8BitMask:              OpcodeVecI16x8BitMaskName,
	OpcodeVecI16x8NarrowI32x4S:         OpcodeVecI16x8NarrowI32x4SName,
	OpcodeVecI16x8NarrowI32x4U:         OpcodeVecI16x8NarrowI32x4UName,
	OpcodeVecI16x8ExtendLowI8x16S:      OpcodeVecI16x8ExtendLowI8x16SName,
	OpcodeVecI16x8ExtendHighI8x16S:     OpcodeVecI16x8ExtendHighI8x16SName,
	OpcodeVecI16x8ExtendLowI8x16U:      OpcodeVecI16x8ExtendLowI8x16UName,
	OpcodeVecI16x8ExtendHighI8x16U:     OpcodeVecI16x8ExtendHighI8x16UName,
	OpcodeVecI16x8Shl:                  OpcodeVecI16x8ShlName,
	OpcodeVecI16x8ShrS:                 OpcodeVecI16x8ShrSName,
	OpcodeVecI16x8ShrU:                 OpcodeVecI16x8ShrUName,
	OpcodeVecI16x8Add:                  OpcodeVecI16x8AddName,
	OpcodeVecI16x8AddSatS:              OpcodeVecI16x8AddSatSName,
	OpcodeVecI16x8AddSatU:              OpcodeVecI16x8AddSatUName,
	OpcodeVecI16x8Sub:                  OpcodeVecI16x8SubName,
	OpcodeVecI16x8SubSatS:              OpcodeVecI16x8SubSatSName,
	OpcodeVecI16x8SubSatU:              OpcodeVecI16x8SubSatUName,
	OpcodeVecI16x8Mul:                  OpcodeVecI16x8MulName,
	OpcodeVecI16x8MinS:                 OpcodeVecI16x8MinSName,
	OpcodeVecI16x8MinU:                 OpcodeVecI16x8MinUName,
	OpcodeVecI16x8MaxS:                 OpcodeVecI16x8MaxSName,
	OpcodeVecI16x8MaxU:                 OpcodeVecI16x8MaxUName,
	OpcodeVecI16x8AvgrU:                OpcodeVecI16x8AvgrUName,
	OpcodeVecI16x8ExtMulLowI8x16S:      OpcodeVecI16x8ExtMulLowI8x16SName,
	OpcodeVecI16x8ExtMulHighI8x16S:     OpcodeVecI16x8ExtMulHighI8x16SName,
	OpcodeVecI16x8ExtMulLowI8x16U:      OpcodeVecI16x8ExtMulLowI8x16UName,
	OpcodeVecI16x8ExtMulHighI8x16U:     OpcodeVecI16x8ExtMulHighI8x16UName,
	OpcodeVecI32x4ExtaddPairwiseI16x8S: OpcodeVecI32x4ExtaddPairwiseI16x8SName,
	OpcodeVecI32x4ExtaddPairwiseI16x8U: OpcodeVecI32x4ExtaddPairwiseI16x8UName,
	OpcodeVecI32x4Abs:                  OpcodeVecI32x4AbsName,
	OpcodeVecI32x4Neg:                  OpcodeVecI32x4NegName,
	OpcodeVecI32x4AllTrue:              OpcodeVecI32x4AllTrueName,
	OpcodeVecI32x4BitMask:              OpcodeVecI32x4BitMaskName,
	OpcodeVecI32x4ExtendLowI16x8S:      OpcodeVecI32x4ExtendLowI16x8SName,
	OpcodeVecI32x4ExtendHighI16x8S:     OpcodeVecI32x4ExtendHighI16x8SName,
	OpcodeVecI32x4ExtendLowI16x8U:      OpcodeVecI32x4ExtendLowI16x8UName,
	OpcodeVecI32x4ExtendHighI16x8U:     OpcodeVecI32x4ExtendHighI16x8UName,
	OpcodeVecI32x4Shl:                  OpcodeVecI32x4ShlName,
	OpcodeVecI32x4ShrS:                 OpcodeVecI32x4ShrSName,
	OpcodeVecI32x4ShrU:                 OpcodeVecI32x4ShrUName,
	OpcodeVecI32x4Add:                  OpcodeVecI32x4AddName,
	OpcodeVecI32x4Sub:                  OpcodeVecI32x4SubName,
	OpcodeVecI32x4Mul:                  OpcodeVecI32x4MulName,
	OpcodeVecI32x4MinS:                 OpcodeVecI32x4MinSName,
	OpcodeVecI32x4MinU:                 OpcodeVecI32x4MinUName,
	OpcodeVecI32x4MaxS:                 OpcodeVecI32x4MaxSName,
	OpcodeVecI32x4MaxU:                 OpcodeVecI32x4MaxUName,
	OpcodeVecI32x4DotI16x8S:            OpcodeVecI32x4DotI16x8SName,
	OpcodeVecI32x4ExtMulLowI16x8S:      OpcodeVecI32x4ExtMulLowI16x8SName,
	OpcodeVecI32x4ExtMulHighI16x8S:     OpcodeVecI32x4ExtMulHighI16x8SName,
	OpcodeVecI32x4ExtMulLowI16x8U:      OpcodeVecI32x4ExtMulLowI16x8UName,
	OpcodeVecI32x4ExtMulHighI16x8U:     OpcodeVecI32x4ExtMulHighI16x8UName,
	OpcodeVecI64x2Abs:                  OpcodeVecI64x2AbsName,
	OpcodeVecI64x2Neg:                  OpcodeVecI64x2NegName,
	OpcodeVecI64x2AllTrue:              OpcodeVecI64x2AllTrueName,
	OpcodeVecI64x2BitMask:              OpcodeVecI64x2BitMaskName,
	OpcodeVecI64x2ExtendLowI32x4S:      OpcodeVecI64x2ExtendLowI32x4SName,
	OpcodeVecI64x2ExtendHighI32x4S:     OpcodeVecI64x2ExtendHighI32x4SName,
	OpcodeVecI64x2ExtendLowI32x4U:      OpcodeVecI64x2ExtendLowI32x4UName,
	OpcodeVecI64x2ExtendHighI32x4U:     OpcodeVecI64x2ExtendHighI32x4UName,
	OpcodeVecI64x2Shl:                  OpcodeVecI64x2ShlName,
	OpcodeVecI64x2ShrS:                 OpcodeVecI64x2ShrSName,
	OpcodeVecI64x2ShrU:                 OpcodeVecI64x2ShrUName,
	OpcodeVecI64x2Add:                  OpcodeVecI64x2AddName,
	OpcodeVecI64x2Sub:                  OpcodeVecI64x2SubName,
	OpcodeVecI64x2Mul:                  OpcodeVecI64x2MulName,
	OpcodeVecI64x2ExtMulLowI32x4S:      OpcodeVecI64x2ExtMulLowI32x4SName,
	OpcodeVecI64x2ExtMulHighI32x4S:     OpcodeVecI64x2ExtMulHighI32x4SName,
	OpcodeVecI64x2ExtMulLowI32x4U:      OpcodeVecI64x2ExtMulLowI32x4UName,
	OpcodeVecI64x2ExtMulHighI32x4U:     OpcodeVecI64x2ExtMulHighI32x4UName,
	OpcodeVecF32x4Ceil:                 OpcodeVecF32x4CeilName,
	OpcodeVecF32x4Floor:                OpcodeVecF32x4FloorName,
	OpcodeVecF32x4Trunc:                OpcodeVecF32x4TruncName,
	OpcodeVecF32x4Nearest:              OpcodeVecF32x4NearestName,
	OpcodeVecF32x4Abs:                  OpcodeVecF32x4AbsName,
	OpcodeVecF32x4Neg:                  OpcodeVecF32x4NegName,
	OpcodeVecF32x4Sqrt:                 OpcodeVecF32x4SqrtName,
	OpcodeVecF32x4Add:                  OpcodeVecF32x4AddName,
	OpcodeVecF32x4Sub:                  OpcodeVecF32x4SubName,
	OpcodeVecF32x4Mul:                  OpcodeVecF32x4MulName,
	OpcodeVecF32x4Div:                  OpcodeVecF32x4DivName,
	OpcodeVecF32x4Min:                  OpcodeVecF32x4MinName,
	OpcodeVecF32x4Max:                  OpcodeVecF32x4MaxName,
	OpcodeVecF32x4Pmin:                 OpcodeVecF32x4PminName,
	OpcodeVecF32x4Pmax:                 OpcodeVecF32x4PmaxName,
	OpcodeVecF64x2Ceil:                 OpcodeVecF64x2CeilName,
	OpcodeVecF64x2Floor:                OpcodeVecF64x2FloorName,
	OpcodeVecF64x2Trunc:                OpcodeVecF64x2TruncName,
	OpcodeVecF64x2Nearest:              OpcodeVecF64x2NearestName,
	OpcodeVecF64x2Abs:                  OpcodeVecF64x2AbsName,
	OpcodeVecF64x2Neg:                  OpcodeVecF64x2NegName,
	OpcodeVecF64x2Sqrt:                 OpcodeVecF64x2SqrtName,
	OpcodeVecF64x2Add:                  OpcodeVecF64x2AddName,
	OpcodeVecF64x2Sub:                  OpcodeVecF64x2SubName,
	OpcodeVecF64x2Mul:                  OpcodeVecF64x2MulName,
	OpcodeVecF64x2Div:                  OpcodeVecF64x2DivName,
	OpcodeVecF64x2Min:                  OpcodeVecF64x2MinName,
	OpcodeVecF64x2Max:                  OpcodeVecF64x2MaxName,
	OpcodeVecF64x2Pmin:                 OpcodeVecF64x2PminName,
	OpcodeVecF64x2Pmax:                 OpcodeVecF64x2PmaxName,
	OpcodeVecI32x4TruncSatF32x4S:       OpcodeVecI32x4TruncSatF32x4SName,
	OpcodeVecI32x4TruncSatF32x4U:       OpcodeVecI32x4TruncSatF32x4UName,
	OpcodeVecF32x4ConvertI32x4S:        OpcodeVecF32x4ConvertI32x4SName,
	OpcodeVecF32x4ConvertI32x4U:        OpcodeVecF32x4ConvertI32x4UName,
	OpcodeVecI32x4TruncSatF64x2SZero:   OpcodeVecI32x4TruncSatF64x2SZeroName,
	OpcodeVecI32x4TruncSatF64x2UZero:   OpcodeVecI32x4TruncSatF64x2UZeroName,
	OpcodeVecF64x2ConvertLowI32x4S:     OpcodeVecF64x2ConvertLowI32x4SName,
	OpcodeVecF64x2ConvertLowI32x4U:     OpcodeVecF64x2ConvertLowI32x4UName,
	OpcodeVecF32x4DemoteF64x2Zero:      OpcodeVecF32x4DemoteF64x2ZeroName,
	OpcodeVecF64x2PromoteLowF32x4Zero:  OpcodeVecF64x2PromoteLowF32x4ZeroName,
}

// VectorInstructionName returns the instruction name corresponding to the vector Opcode.
func VectorInstructionName(oc OpcodeVec) (ret string) {
	return vectorInstructionName[oc]
}

const (
	OpcodeAtomicMemoryNotifyName = "memory.atomic.notify"
	OpcodeAtomicMemoryWait32Name = "memory.atomic.wait32"
	OpcodeAtomicMemoryWait64Name = "memory.atomic.wait64"
	OpcodeAtomicFenceName        = "atomic.fence"

	OpcodeAtomicI32LoadName    = "i32.atomic.load"
	OpcodeAtomicI64LoadName    = "i64.atomic.load"
	OpcodeAtomicI32Load8UName  = "i32.atomic.load8_u"
	OpcodeAtomicI32Load16UName = "i32.atomic.load16_u"
	OpcodeAtomicI64Load8UName  = "i64.atomic.load8_u"
	OpcodeAtomicI64Load16UName = "i64.atomic.load16_u"
	OpcodeAtomicI64Load32UName = "i64.atomic.load32_u"
	OpcodeAtomicI32StoreName   = "i32.atomic.store"
	OpcodeAtomicI64StoreName   = "i64.atomic.store"
	OpcodeAtomicI32Store8Name  = "i32.atomic.store8"
	OpcodeAtomicI32Store16Name = "i32.atomic.store16"
	OpcodeAtomicI64Store8Name  = "i64.atomic.store8"
	OpcodeAtomicI64Store16Name = "i64.atomic.store16"
	OpcodeAtomicI64Store32Name = "i64.atomic.store32"

	OpcodeAtomicI32RmwAddName    = "i32.atomic.rmw.add"
	OpcodeAtomicI64RmwAddName    = "i64.atomic.rmw.add"
	OpcodeAtomicI32Rmw8AddUName  = "i32.atomic.rmw8.add_u"
	OpcodeAtomicI32Rmw16AddUName = "i32.atomic.rmw16.add_u"
	OpcodeAtomicI64Rmw8AddUName  = "i64.atomic.rmw8.add_u"
	OpcodeAtomicI64Rmw16AddUName = "i64.atomic.rmw16.add_u"
	OpcodeAtomicI64Rmw32AddUName = "i64.atomic.rmw32.add_u"

	OpcodeAtomicI32RmwSubName    = "i32.atomic.rmw.sub"
	OpcodeAtomicI64RmwSubName    = "i64.atomic.rmw.sub"
	OpcodeAtomicI32Rmw8SubUName  = "i32.atomic.rmw8.sub_u"
	OpcodeAtomicI32Rmw16SubUName = "i32.atomic.rmw16.sub_u"
	OpcodeAtomicI64Rmw8SubUName  = "i64.atomic.rmw8.sub_u"
	OpcodeAtomicI64Rmw16SubUName = "i64.atomic.rmw16.sub_u"
	OpcodeAtomicI64Rmw32SubUName = "i64.atomic.rmw32.sub_u"

	OpcodeAtomicI32RmwAndName    = "i32.atomic.rmw.and"
	OpcodeAtomicI64RmwAndName    = "i64.atomic.rmw.and"
	OpcodeAtomicI32Rmw8AndUName  = "i32.atomic.rmw8.and_u"
	OpcodeAtomicI32Rmw16AndUName = "i32.atomic.rmw16.and_u"
	OpcodeAtomicI64Rmw8AndUName  = "i64.atomic.rmw8.and_u"
	OpcodeAtomicI64Rmw16AndUName = "i64.atomic.rmw16.and_u"
	OpcodeAtomicI64Rmw32AndUName = "i64.atomic.rmw32.and_u"

	OpcodeAtomicI32RmwOrName    = "i32.atomic.rmw.or"
	OpcodeAtomicI64RmwOrName    = "i64.atomic.rmw.or"
	OpcodeAtomicI32Rmw8OrUName  = "i32.atomic.rmw8.or_u"
	OpcodeAtomicI32Rmw16OrUName = "i32.atomic.rmw16.or_u"
	OpcodeAtomicI64Rmw8OrUName  = "i64.atomic.rmw8.or_u"
	OpcodeAtomicI64Rmw16OrUName = "i64.atomic.rmw16.or_u"
	OpcodeAtomicI64Rmw32OrUName = "i64.atomic.rmw32.or_u"

	OpcodeAtomicI32RmwXorName    = "i32.atomic.rmw.xor"
	OpcodeAtomicI64RmwXorName    = "i64.atomic.rmw.xor"
	OpcodeAtomicI32Rmw8XorUName  = "i32.atomic.rmw8.xor_u"
	OpcodeAtomicI32Rmw16XorUName = "i32.atomic.rmw16.xor_u"
	OpcodeAtomicI64Rmw8XorUName  = "i64.atomic.rmw8.xor_u"
	OpcodeAtomicI64Rmw16XorUName = "i64.atomic.rmw16.xor_u"
	OpcodeAtomicI64Rmw32XorUName = "i64.atomic.rmw32.xor_u"

	OpcodeAtomicI32RmwXchgName    = "i32.atomic.rmw.xchg"
	OpcodeAtomicI64RmwXchgName    = "i64.atomic.rmw.xchg"
	OpcodeAtomicI32Rmw8XchgUName  = "i32.atomic.rmw8.xchg_u"
	OpcodeAtomicI32Rmw16XchgUName = "i32.atomic.rmw16.xchg_u"
	OpcodeAtomicI64Rmw8XchgUName  = "i64.atomic.rmw8.xchg_u"
	OpcodeAtomicI64Rmw16XchgUName = "i64.atomic.rmw16.xchg_u"
	OpcodeAtomicI64Rmw32XchgUName = "i64.atomic.rmw32.xchg_u"

	OpcodeAtomicI32RmwCmpxchgName    = "i32.atomic.rmw.cmpxchg"
	OpcodeAtomicI64RmwCmpxchgName    = "i64.atomic.rmw.cmpxchg"
	OpcodeAtomicI32Rmw8CmpxchgUName  = "i32.atomic.rmw8.cmpxchg_u"
	OpcodeAtomicI32Rmw16CmpxchgUName = "i32.atomic.rmw16.cmpxchg_u"
	OpcodeAtomicI64Rmw8CmpxchgUName  = "i64.atomic.rmw8.cmpxchg_u"
	OpcodeAtomicI64Rmw16CmpxchgUName = "i64.atomic.rmw16.cmpxchg_u"
	OpcodeAtomicI64Rmw32CmpxchgUName = "i64.atomic.rmw32.cmpxchg_u"
)

var atomicInstructionName = map[OpcodeAtomic]string{
	OpcodeAtomicMemoryNotify: OpcodeAtomicMemoryNotifyName,
	OpcodeAtomicMemoryWait32: OpcodeAtomicMemoryWait32Name,
	OpcodeAtomicMemoryWait64: OpcodeAtomicMemoryWait64Name,
	OpcodeAtomicFence:        OpcodeAtomicFenceName,

	OpcodeAtomicI32Load:    OpcodeAtomicI32LoadName,
	OpcodeAtomicI64Load:    OpcodeAtomicI64LoadName,
	OpcodeAtomicI32Load8U:  OpcodeAtomicI32Load8UName,
	OpcodeAtomicI32Load16U: OpcodeAtomicI32Load16UName,
	OpcodeAtomicI64Load8U:  OpcodeAtomicI64Load8UName,
	OpcodeAtomicI64Load16U: OpcodeAtomicI64Load16UName,
	OpcodeAtomicI64Load32U: OpcodeAtomicI64Load32UName,
	OpcodeAtomicI32Store:   OpcodeAtomicI32StoreName,
	OpcodeAtomicI64Store:   OpcodeAtomicI64StoreName,
	OpcodeAtomicI32Store8:  OpcodeAtomicI32Store8Name,
	OpcodeAtomicI32Store16: OpcodeAtomicI32Store16Name,
	OpcodeAtomicI64Store8:  OpcodeAtomicI64Store8Name,
	OpcodeAtomicI64Store16: OpcodeAtomicI64Store16Name,
	OpcodeAtomicI64Store32: OpcodeAtomicI64Store32Name,

	OpcodeAtomicI32RmwAdd:    OpcodeAtomicI32RmwAddName,
	OpcodeAtomicI64RmwAdd:    OpcodeAtomicI64RmwAddName,
	OpcodeAtomicI32Rmw8AddU:  OpcodeAtomicI32Rmw8AddUName,
	OpcodeAtomicI32Rmw16AddU: OpcodeAtomicI32Rmw16AddUName,
	OpcodeAtomicI64Rmw8AddU:  OpcodeAtomicI64Rmw8AddUName,
	OpcodeAtomicI64Rmw16AddU: OpcodeAtomicI64Rmw16AddUName,
	OpcodeAtomicI64Rmw32AddU: OpcodeAtomicI64Rmw32AddUName,

	OpcodeAtomicI32RmwSub:    OpcodeAtomicI32RmwSubName,
	OpcodeAtomicI64RmwSub:    OpcodeAtomicI64RmwSubName,
	OpcodeAtomicI32Rmw8SubU:  OpcodeAtomicI32Rmw8SubUName,
	OpcodeAtomicI32Rmw16SubU: OpcodeAtomicI32Rmw16SubUName,
	OpcodeAtomicI64Rmw8SubU:  OpcodeAtomicI64Rmw8SubUName,
	OpcodeAtomicI64Rmw16SubU: OpcodeAtomicI64Rmw16SubUName,
	OpcodeAtomicI64Rmw32SubU: OpcodeAtomicI64Rmw32SubUName,

	OpcodeAtomicI32RmwAnd:    OpcodeAtomicI32RmwAndName,
	OpcodeAtomicI64RmwAnd:    OpcodeAtomicI64RmwAndName,
	OpcodeAtomicI32Rmw8AndU:  OpcodeAtomicI32Rmw8AndUName,
	OpcodeAtomicI32Rmw16AndU: OpcodeAtomicI32Rmw16AndUName,
	OpcodeAtomicI64Rmw8AndU:  OpcodeAtomicI64Rmw8AndUName,
	OpcodeAtomicI64Rmw16AndU: OpcodeAtomicI64Rmw16AndUName,
	OpcodeAtomicI64Rmw32AndU: OpcodeAtomicI64Rmw32AndUName,

	OpcodeAtomicI32RmwOr:    OpcodeAtomicI32RmwOrName,
	OpcodeAtomicI64RmwOr:    OpcodeAtomicI64RmwOrName,
	OpcodeAtomicI32Rmw8OrU:  OpcodeAtomicI32Rmw8OrUName,
	OpcodeAtomicI32Rmw16OrU: OpcodeAtomicI32Rmw16OrUName,
	OpcodeAtomicI64Rmw8OrU:  OpcodeAtomicI64Rmw8OrUName,
	OpcodeAtomicI64Rmw16OrU: OpcodeAtomicI64Rmw16OrUName,
	OpcodeAtomicI64Rmw32OrU: OpcodeAtomicI64Rmw32OrUName,

	OpcodeAtomicI32RmwXor:    OpcodeAtomicI32RmwXorName,
	OpcodeAtomicI64RmwXor:    OpcodeAtomicI64RmwXorName,
	OpcodeAtomicI32Rmw8XorU:  OpcodeAtomicI32Rmw8XorUName,
	OpcodeAtomicI32Rmw16XorU: OpcodeAtomicI32Rmw16XorUName,
	OpcodeAtomicI64Rmw8XorU:  OpcodeAtomicI64Rmw8XorUName,
	OpcodeAtomicI64Rmw16XorU: OpcodeAtomicI64Rmw16XorUName,
	OpcodeAtomicI64Rmw32XorU: OpcodeAtomicI64Rmw32XorUName,

	OpcodeAtomicI32RmwXchg:    OpcodeAtomicI32RmwXchgName,
	OpcodeAtomicI64RmwXchg:    OpcodeAtomicI64RmwXchgName,
	OpcodeAtomicI32Rmw8XchgU:  OpcodeAtomicI32Rmw8XchgUName,
	OpcodeAtomicI32Rmw16XchgU: OpcodeAtomicI32Rmw16XchgUName,
	OpcodeAtomicI64Rmw8XchgU:  OpcodeAtomicI64Rmw8XchgUName,
	OpcodeAtomicI64Rmw16XchgU: OpcodeAtomicI64Rmw16XchgUName,
	OpcodeAtomicI64Rmw32XchgU: OpcodeAtomicI64Rmw32XchgUName,

	OpcodeAtomicI32RmwCmpxchg:    OpcodeAtomicI32RmwCmpxchgName,
	OpcodeAtomicI64RmwCmpxchg:    OpcodeAtomicI64RmwCmpxchgName,
	OpcodeAtomicI32Rmw8CmpxchgU:  OpcodeAtomicI32Rmw8CmpxchgUName,
	OpcodeAtomicI32Rmw16CmpxchgU: OpcodeAtomicI32Rmw16CmpxchgUName,
	OpcodeAtomicI64Rmw8CmpxchgU:  OpcodeAtomicI64Rmw8CmpxchgUName,
	OpcodeAtomicI64Rmw16CmpxchgU: OpcodeAtomicI64Rmw16CmpxchgUName,
	OpcodeAtomicI64Rmw32CmpxchgU: OpcodeAtomicI64Rmw32CmpxchgUName,
}

// AtomicInstructionName returns the instruction name corresponding to the atomic Opcode.
func AtomicInstructionName(oc OpcodeAtomic) (ret string) {
	return atomicInstructionName[oc]
}
