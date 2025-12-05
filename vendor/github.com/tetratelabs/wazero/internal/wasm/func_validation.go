package wasm

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// The wazero specific limitation described at RATIONALE.md.
const maximumValuesOnStack = 1 << 27

// validateFunction validates the instruction sequence of a function.
// following the specification https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#instructions%E2%91%A2.
//
// * idx is the index in the FunctionSection
// * functions are the function index, which is prefixed by imports. The value is the TypeSection index.
// * globals are the global index, which is prefixed by imports.
// * memory is the potentially imported memory and can be nil.
// * table is the potentially imported table and can be nil.
// * declaredFunctionIndexes is the set of function indexes declared by declarative element segments which can be acceed by OpcodeRefFunc instruction.
//
// Returns an error if the instruction sequence is not valid,
// or potentially it can exceed the maximum number of values on the stack.
func (m *Module) validateFunction(sts *stacks, enabledFeatures api.CoreFeatures, idx Index, functions []Index,
	globals []GlobalType, memory *Memory, tables []Table, declaredFunctionIndexes map[Index]struct{}, br *bytes.Reader,
) error {
	return m.validateFunctionWithMaxStackValues(sts, enabledFeatures, idx, functions, globals, memory, tables, maximumValuesOnStack, declaredFunctionIndexes, br)
}

func readMemArg(pc uint64, body []byte) (align, offset uint32, read uint64, err error) {
	align, num, err := leb128.LoadUint32(body[pc:])
	if err != nil {
		err = fmt.Errorf("read memory align: %v", err)
		return
	}
	read += num

	offset, num, err = leb128.LoadUint32(body[pc+num:])
	if err != nil {
		err = fmt.Errorf("read memory offset: %v", err)
		return
	}

	read += num
	return align, offset, read, nil
}

// validateFunctionWithMaxStackValues is like validateFunction, but allows overriding maxStackValues for testing.
//
// * stacks is to track the state of Wasm value and control frame stacks at anypoint of execution, and reused to reduce allocation.
// * maxStackValues is the maximum height of values stack which the target is allowed to reach.
func (m *Module) validateFunctionWithMaxStackValues(
	sts *stacks,
	enabledFeatures api.CoreFeatures,
	idx Index,
	functions []Index,
	globals []GlobalType,
	memory *Memory,
	tables []Table,
	maxStackValues int,
	declaredFunctionIndexes map[Index]struct{},
	br *bytes.Reader,
) error {
	functionType := &m.TypeSection[m.FunctionSection[idx]]
	code := &m.CodeSection[idx]
	body := code.Body
	localTypes := code.LocalTypes

	sts.reset(functionType)
	valueTypeStack := &sts.vs
	// We start with the outermost control block which is for function return if the code branches into it.
	controlBlockStack := &sts.cs

	// Now start walking through all the instructions in the body while tracking
	// control blocks and value types to check the validity of all instructions.
	for pc := uint64(0); pc < uint64(len(body)); pc++ {
		op := body[pc]
		if false {
			var instName string
			if op == OpcodeMiscPrefix {
				instName = MiscInstructionName(body[pc+1])
			} else if op == OpcodeVecPrefix {
				instName = VectorInstructionName(body[pc+1])
			} else if op == OpcodeAtomicPrefix {
				instName = AtomicInstructionName(body[pc+1])
			} else {
				instName = InstructionName(op)
			}
			fmt.Printf("handling %s, stack=%s, blocks: %v\n", instName, valueTypeStack.stack, controlBlockStack)
		}

		if len(controlBlockStack.stack) == 0 {
			return fmt.Errorf("unexpected end of function at pc=%#x", pc)
		}

		if OpcodeI32Load <= op && op <= OpcodeI64Store32 {
			if memory == nil {
				return fmt.Errorf("memory must exist for %s", InstructionName(op))
			}
			pc++
			align, _, read, err := readMemArg(pc, body)
			if err != nil {
				return err
			}
			pc += read - 1
			switch op {
			case OpcodeI32Load:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeF32Load:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeF32)
			case OpcodeI32Store:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeF32Store:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeI64Load:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeF64Load:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeF64)
			case OpcodeI64Store:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeF64Store:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeI32Load8S:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI32Load8U:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64Load8S, OpcodeI64Load8U:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeI32Store8:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeI64Store8:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeI32Load16S, OpcodeI32Load16U:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64Load16S, OpcodeI64Load16U:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeI32Store16:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeI64Store16:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeI64Load32S, OpcodeI64Load32U:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeI64Store32:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			}
		} else if OpcodeMemorySize <= op && op <= OpcodeMemoryGrow {
			if memory == nil {
				return fmt.Errorf("memory must exist for %s", InstructionName(op))
			}
			pc++
			val, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("read immediate: %v", err)
			}
			if val != 0 || num != 1 {
				return fmt.Errorf("memory instruction reserved bytes not zero with 1 byte")
			}
			switch Opcode(op) {
			case OpcodeMemoryGrow:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeMemorySize:
				valueTypeStack.push(ValueTypeI32)
			}
			pc += num - 1
		} else if OpcodeI32Const <= op && op <= OpcodeF64Const {
			pc++
			switch Opcode(op) {
			case OpcodeI32Const:
				_, num, err := leb128.LoadInt32(body[pc:])
				if err != nil {
					return fmt.Errorf("read i32 immediate: %s", err)
				}
				pc += num - 1
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64Const:
				_, num, err := leb128.LoadInt64(body[pc:])
				if err != nil {
					return fmt.Errorf("read i64 immediate: %v", err)
				}
				valueTypeStack.push(ValueTypeI64)
				pc += num - 1
			case OpcodeF32Const:
				valueTypeStack.push(ValueTypeF32)
				pc += 3
			case OpcodeF64Const:
				valueTypeStack.push(ValueTypeF64)
				pc += 7
			}
		} else if OpcodeLocalGet <= op && op <= OpcodeGlobalSet {
			pc++
			index, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("read immediate: %v", err)
			}
			pc += num - 1
			switch op {
			case OpcodeLocalGet:
				inputLen := uint32(len(functionType.Params))
				if l := uint32(len(localTypes)) + inputLen; index >= l {
					return fmt.Errorf("invalid local index for %s %d >= %d(=len(locals)+len(parameters))",
						OpcodeLocalGetName, index, l)
				}
				if index < inputLen {
					valueTypeStack.push(functionType.Params[index])
				} else {
					valueTypeStack.push(localTypes[index-inputLen])
				}
			case OpcodeLocalSet:
				inputLen := uint32(len(functionType.Params))
				if l := uint32(len(localTypes)) + inputLen; index >= l {
					return fmt.Errorf("invalid local index for %s %d >= %d(=len(locals)+len(parameters))",
						OpcodeLocalSetName, index, l)
				}
				var expType ValueType
				if index < inputLen {
					expType = functionType.Params[index]
				} else {
					expType = localTypes[index-inputLen]
				}
				if err := valueTypeStack.popAndVerifyType(expType); err != nil {
					return err
				}
			case OpcodeLocalTee:
				inputLen := uint32(len(functionType.Params))
				if l := uint32(len(localTypes)) + inputLen; index >= l {
					return fmt.Errorf("invalid local index for %s %d >= %d(=len(locals)+len(parameters))",
						OpcodeLocalTeeName, index, l)
				}
				var expType ValueType
				if index < inputLen {
					expType = functionType.Params[index]
				} else {
					expType = localTypes[index-inputLen]
				}
				if err := valueTypeStack.popAndVerifyType(expType); err != nil {
					return err
				}
				valueTypeStack.push(expType)
			case OpcodeGlobalGet:
				if index >= uint32(len(globals)) {
					return fmt.Errorf("invalid index for %s", OpcodeGlobalGetName)
				}
				valueTypeStack.push(globals[index].ValType)
			case OpcodeGlobalSet:
				if index >= uint32(len(globals)) {
					return fmt.Errorf("invalid global index")
				} else if !globals[index].Mutable {
					return fmt.Errorf("%s when not mutable", OpcodeGlobalSetName)
				} else if err := valueTypeStack.popAndVerifyType(
					globals[index].ValType); err != nil {
					return err
				}
			}
		} else if op == OpcodeBr {
			pc++
			index, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("read immediate: %v", err)
			} else if int(index) >= len(controlBlockStack.stack) {
				return fmt.Errorf("invalid %s operation: index out of range", OpcodeBrName)
			}
			pc += num - 1
			// Check type soundness.
			target := &controlBlockStack.stack[len(controlBlockStack.stack)-int(index)-1]
			var targetResultType []ValueType
			if target.op == OpcodeLoop {
				targetResultType = target.blockType.Params
			} else {
				targetResultType = target.blockType.Results
			}
			if err = valueTypeStack.popResults(op, targetResultType, false); err != nil {
				return err
			}
			// br instruction is stack-polymorphic.
			valueTypeStack.unreachable()
		} else if op == OpcodeBrIf {
			pc++
			index, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("read immediate: %v", err)
			} else if int(index) >= len(controlBlockStack.stack) {
				return fmt.Errorf(
					"invalid ln param given for %s: index=%d with %d for the current label stack length",
					OpcodeBrIfName, index, len(controlBlockStack.stack))
			}
			pc += num - 1
			if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
				return fmt.Errorf("cannot pop the required operand for %s", OpcodeBrIfName)
			}
			// Check type soundness.
			target := &controlBlockStack.stack[len(controlBlockStack.stack)-int(index)-1]
			var targetResultType []ValueType
			if target.op == OpcodeLoop {
				targetResultType = target.blockType.Params
			} else {
				targetResultType = target.blockType.Results
			}
			if err := valueTypeStack.popResults(op, targetResultType, false); err != nil {
				return err
			}
			// Push back the result
			for _, t := range targetResultType {
				valueTypeStack.push(t)
			}
		} else if op == OpcodeBrTable {
			pc++
			br.Reset(body[pc:])
			nl, num, err := leb128.DecodeUint32(br)
			if err != nil {
				return fmt.Errorf("read immediate: %w", err)
			}

			sts.ls = sts.ls[:0]
			for i := uint32(0); i < nl; i++ {
				l, n, err := leb128.DecodeUint32(br)
				if err != nil {
					return fmt.Errorf("read immediate: %w", err)
				}
				num += n
				sts.ls = append(sts.ls, l)
			}
			ln, n, err := leb128.DecodeUint32(br)
			if err != nil {
				return fmt.Errorf("read immediate: %w", err)
			} else if int(ln) >= len(controlBlockStack.stack) {
				return fmt.Errorf(
					"invalid ln param given for %s: ln=%d with %d for the current label stack length",
					OpcodeBrTableName, ln, len(controlBlockStack.stack))
			}
			pc += n + num - 1
			// Check type soundness.
			if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
				return fmt.Errorf("cannot pop the required operand for %s", OpcodeBrTableName)
			}
			lnLabel := &controlBlockStack.stack[len(controlBlockStack.stack)-1-int(ln)]
			var defaultLabelType []ValueType
			// Below, we might modify the slice in case of unreachable. Therefore,
			// we have to copy the content of block result types, otherwise the original
			// function type might result in invalid value types if the block is the outermost label
			// which equals the function's type.
			if lnLabel.op != OpcodeLoop { // Loop operation doesn't require results since the continuation is the beginning of the loop.
				defaultLabelType = make([]ValueType, len(lnLabel.blockType.Results))
				copy(defaultLabelType, lnLabel.blockType.Results)
			} else {
				defaultLabelType = make([]ValueType, len(lnLabel.blockType.Params))
				copy(defaultLabelType, lnLabel.blockType.Params)
			}

			if enabledFeatures.IsEnabled(api.CoreFeatureReferenceTypes) {
				// As of reference-types proposal, br_table on unreachable state
				// can choose unknown types for expected parameter types for each label.
				// https://github.com/WebAssembly/reference-types/pull/116
				for i := range defaultLabelType {
					index := len(defaultLabelType) - 1 - i
					exp := defaultLabelType[index]
					actual, err := valueTypeStack.pop()
					if err != nil {
						return err
					}
					if actual == valueTypeUnknown {
						// Re-assign the expected type to unknown.
						defaultLabelType[index] = valueTypeUnknown
					} else if actual != exp {
						return typeMismatchError(true, OpcodeBrTableName, actual, exp, i)
					}
				}
			} else {
				if err = valueTypeStack.popResults(op, defaultLabelType, false); err != nil {
					return err
				}
			}

			for _, l := range sts.ls {
				if int(l) >= len(controlBlockStack.stack) {
					return fmt.Errorf("invalid l param given for %s", OpcodeBrTableName)
				}
				label := &controlBlockStack.stack[len(controlBlockStack.stack)-1-int(l)]
				var tableLabelType []ValueType
				if label.op != OpcodeLoop {
					tableLabelType = label.blockType.Results
				} else {
					tableLabelType = label.blockType.Params
				}
				if len(defaultLabelType) != len(tableLabelType) {
					return fmt.Errorf("inconsistent block type length for %s at %d; %v (ln=%d) != %v (l=%d)", OpcodeBrTableName, l, defaultLabelType, ln, tableLabelType, l)
				}
				for i := range defaultLabelType {
					if defaultLabelType[i] != valueTypeUnknown && defaultLabelType[i] != tableLabelType[i] {
						return fmt.Errorf("incosistent block type for %s at %d", OpcodeBrTableName, l)
					}
				}
			}

			// br_table instruction is stack-polymorphic.
			valueTypeStack.unreachable()
		} else if op == OpcodeCall {
			pc++
			index, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("read immediate: %v", err)
			}
			pc += num - 1
			if int(index) >= len(functions) {
				return fmt.Errorf("invalid function index")
			}
			funcType := &m.TypeSection[functions[index]]
			for i := 0; i < len(funcType.Params); i++ {
				if err := valueTypeStack.popAndVerifyType(funcType.Params[len(funcType.Params)-1-i]); err != nil {
					return fmt.Errorf("type mismatch on %s operation param type: %v", OpcodeCallName, err)
				}
			}
			for _, exp := range funcType.Results {
				valueTypeStack.push(exp)
			}
		} else if op == OpcodeCallIndirect {
			pc++
			typeIndex, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("read immediate: %v", err)
			}
			pc += num

			if int(typeIndex) >= len(m.TypeSection) {
				return fmt.Errorf("invalid type index at %s: %d", OpcodeCallIndirectName, typeIndex)
			}

			tableIndex, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("read table index: %v", err)
			}
			pc += num - 1
			if tableIndex != 0 {
				if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
					return fmt.Errorf("table index must be zero but was %d: %w", tableIndex, err)
				}
			}

			if tableIndex >= uint32(len(tables)) {
				return fmt.Errorf("unknown table index: %d", tableIndex)
			}

			table := tables[tableIndex]
			if table.Type != RefTypeFuncref {
				return fmt.Errorf("table is not funcref type but was %s for %s", RefTypeName(table.Type), OpcodeCallIndirectName)
			}

			if err = valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
				return fmt.Errorf("cannot pop the offset in table for %s", OpcodeCallIndirectName)
			}
			funcType := &m.TypeSection[typeIndex]
			for i := 0; i < len(funcType.Params); i++ {
				if err = valueTypeStack.popAndVerifyType(funcType.Params[len(funcType.Params)-1-i]); err != nil {
					return fmt.Errorf("type mismatch on %s operation input type", OpcodeCallIndirectName)
				}
			}
			for _, exp := range funcType.Results {
				valueTypeStack.push(exp)
			}
		} else if OpcodeI32Eqz <= op && op <= OpcodeI64Extend32S {
			switch op {
			case OpcodeI32Eqz:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeI32EqzName, err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI32Eq, OpcodeI32Ne, OpcodeI32LtS,
				OpcodeI32LtU, OpcodeI32GtS, OpcodeI32GtU, OpcodeI32LeS,
				OpcodeI32LeU, OpcodeI32GeS, OpcodeI32GeU:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the 1st i32 operand for %s: %v", InstructionName(op), err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the 2nd i32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64Eqz:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeI64EqzName, err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64Eq, OpcodeI64Ne, OpcodeI64LtS,
				OpcodeI64LtU, OpcodeI64GtS, OpcodeI64GtU,
				OpcodeI64LeS, OpcodeI64LeU, OpcodeI64GeS, OpcodeI64GeU:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the 1st i64 operand for %s: %v", InstructionName(op), err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the 2nd i64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeF32Eq, OpcodeF32Ne, OpcodeF32Lt, OpcodeF32Gt, OpcodeF32Le, OpcodeF32Ge:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the 1st f32 operand for %s: %v", InstructionName(op), err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the 2nd f32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeF64Eq, OpcodeF64Ne, OpcodeF64Lt, OpcodeF64Gt, OpcodeF64Le, OpcodeF64Ge:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the 1st f64 operand for %s: %v", InstructionName(op), err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the 2nd f64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI32Clz, OpcodeI32Ctz, OpcodeI32Popcnt:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the i32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI32Add, OpcodeI32Sub, OpcodeI32Mul, OpcodeI32DivS,
				OpcodeI32DivU, OpcodeI32RemS, OpcodeI32RemU, OpcodeI32And,
				OpcodeI32Or, OpcodeI32Xor, OpcodeI32Shl, OpcodeI32ShrS,
				OpcodeI32ShrU, OpcodeI32Rotl, OpcodeI32Rotr:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the 1st operand for %s: %v", InstructionName(op), err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the 2nd operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64Clz, OpcodeI64Ctz, OpcodeI64Popcnt:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the i64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeI64Add, OpcodeI64Sub, OpcodeI64Mul, OpcodeI64DivS,
				OpcodeI64DivU, OpcodeI64RemS, OpcodeI64RemU, OpcodeI64And,
				OpcodeI64Or, OpcodeI64Xor, OpcodeI64Shl, OpcodeI64ShrS,
				OpcodeI64ShrU, OpcodeI64Rotl, OpcodeI64Rotr:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the 1st i64 operand for %s: %v", InstructionName(op), err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the 2nd i64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeF32Abs, OpcodeF32Neg, OpcodeF32Ceil,
				OpcodeF32Floor, OpcodeF32Trunc, OpcodeF32Nearest,
				OpcodeF32Sqrt:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the 1st f32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeF32)
			case OpcodeF32Add, OpcodeF32Sub, OpcodeF32Mul,
				OpcodeF32Div, OpcodeF32Min, OpcodeF32Max,
				OpcodeF32Copysign:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the 1st f32 operand for %s: %v", InstructionName(op), err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the 2nd f32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeF32)
			case OpcodeF64Abs, OpcodeF64Neg, OpcodeF64Ceil,
				OpcodeF64Floor, OpcodeF64Trunc, OpcodeF64Nearest,
				OpcodeF64Sqrt:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the 1st f64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeF64)
			case OpcodeF64Add, OpcodeF64Sub, OpcodeF64Mul,
				OpcodeF64Div, OpcodeF64Min, OpcodeF64Max,
				OpcodeF64Copysign:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the 1st f64 operand for %s: %v", InstructionName(op), err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the 2nd f64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeF64)
			case OpcodeI32WrapI64:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeI32WrapI64Name, err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI32TruncF32S, OpcodeI32TruncF32U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the f32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI32TruncF64S, OpcodeI32TruncF64U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the f64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64ExtendI32S, OpcodeI64ExtendI32U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the i32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeI64TruncF32S, OpcodeI64TruncF32U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the f32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeI64TruncF64S, OpcodeI64TruncF64U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the f64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeF32ConvertI32S, OpcodeF32ConvertI32U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the i32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeF32)
			case OpcodeF32ConvertI64S, OpcodeF32ConvertI64U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the i64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeF32)
			case OpcodeF32DemoteF64:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeF32DemoteF64Name, err)
				}
				valueTypeStack.push(ValueTypeF32)
			case OpcodeF64ConvertI32S, OpcodeF64ConvertI32U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the i32 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeF64)
			case OpcodeF64ConvertI64S, OpcodeF64ConvertI64U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the i64 operand for %s: %v", InstructionName(op), err)
				}
				valueTypeStack.push(ValueTypeF64)
			case OpcodeF64PromoteF32:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeF64PromoteF32Name, err)
				}
				valueTypeStack.push(ValueTypeF64)
			case OpcodeI32ReinterpretF32:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeI32ReinterpretF32Name, err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64ReinterpretF64:
				if err := valueTypeStack.popAndVerifyType(ValueTypeF64); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeI64ReinterpretF64Name, err)
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeF32ReinterpretI32:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeF32ReinterpretI32Name, err)
				}
				valueTypeStack.push(ValueTypeF32)
			case OpcodeF64ReinterpretI64:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeF64ReinterpretI64Name, err)
				}
				valueTypeStack.push(ValueTypeF64)
			case OpcodeI32Extend8S, OpcodeI32Extend16S:
				if err := enabledFeatures.RequireEnabled(api.CoreFeatureSignExtensionOps); err != nil {
					return fmt.Errorf("%s invalid as %v", instructionNames[op], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", instructionNames[op], err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeI64Extend8S, OpcodeI64Extend16S, OpcodeI64Extend32S:
				if err := enabledFeatures.RequireEnabled(api.CoreFeatureSignExtensionOps); err != nil {
					return fmt.Errorf("%s invalid as %v", instructionNames[op], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", instructionNames[op], err)
				}
				valueTypeStack.push(ValueTypeI64)
			default:
				return fmt.Errorf("invalid numeric instruction 0x%x", op)
			}
		} else if op >= OpcodeRefNull && op <= OpcodeRefFunc {
			if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
				return fmt.Errorf("%s invalid as %v", instructionNames[op], err)
			}
			switch op {
			case OpcodeRefNull:
				pc++
				switch reftype := body[pc]; reftype {
				case ValueTypeExternref:
					valueTypeStack.push(ValueTypeExternref)
				case ValueTypeFuncref:
					valueTypeStack.push(ValueTypeFuncref)
				default:
					return fmt.Errorf("unknown type for ref.null: 0x%x", reftype)
				}
			case OpcodeRefIsNull:
				tp, err := valueTypeStack.pop()
				if err != nil {
					return fmt.Errorf("cannot pop the operand for ref.is_null: %v", err)
				} else if !isReferenceValueType(tp) && tp != valueTypeUnknown {
					return fmt.Errorf("type mismatch: expected reference type but was %s", ValueTypeName(tp))
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeRefFunc:
				pc++
				index, num, err := leb128.LoadUint32(body[pc:])
				if err != nil {
					return fmt.Errorf("failed to read function index for ref.func: %v", err)
				}
				if _, ok := declaredFunctionIndexes[index]; !ok {
					return fmt.Errorf("undeclared function index %d for ref.func", index)
				}
				pc += num - 1
				valueTypeStack.push(ValueTypeFuncref)
			}
		} else if op == OpcodeTableGet || op == OpcodeTableSet {
			if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
				return fmt.Errorf("%s is invalid as %v", InstructionName(op), err)
			}
			pc++
			tableIndex, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("read immediate: %v", err)
			}
			if tableIndex >= uint32(len(tables)) {
				return fmt.Errorf("table of index %d not found", tableIndex)
			}

			refType := tables[tableIndex].Type
			if op == OpcodeTableGet {
				if err := valueTypeStack.popAndVerifyType(api.ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for table.get: %v", err)
				}
				valueTypeStack.push(refType)
			} else {
				if err := valueTypeStack.popAndVerifyType(refType); err != nil {
					return fmt.Errorf("cannot pop the operand for table.set: %v", err)
				}
				if err := valueTypeStack.popAndVerifyType(api.ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for table.set: %v", err)
				}
			}
			pc += num - 1
		} else if op == OpcodeMiscPrefix {
			pc++
			// A misc opcode is encoded as an unsigned variable 32-bit integer.
			miscOp32, num, err := leb128.LoadUint32(body[pc:])
			if err != nil {
				return fmt.Errorf("failed to read misc opcode: %v", err)
			}
			pc += num - 1
			miscOpcode := byte(miscOp32)
			// If the misc opcode is beyond byte range, it is highly likely this is an invalid binary, or
			// it is due to the new opcode from a new proposal. In the latter case, we have to
			// change the alias type of OpcodeMisc (which is currently byte) to uint32.
			if uint32(byte(miscOp32)) != miscOp32 {
				return fmt.Errorf("invalid misc opcode: %#x", miscOp32)
			}
			if miscOpcode >= OpcodeMiscI32TruncSatF32S && miscOpcode <= OpcodeMiscI64TruncSatF64U {
				if err := enabledFeatures.RequireEnabled(api.CoreFeatureNonTrappingFloatToIntConversion); err != nil {
					return fmt.Errorf("%s invalid as %v", miscInstructionNames[miscOpcode], err)
				}
				var inType, outType ValueType
				switch miscOpcode {
				case OpcodeMiscI32TruncSatF32S, OpcodeMiscI32TruncSatF32U:
					inType, outType = ValueTypeF32, ValueTypeI32
				case OpcodeMiscI32TruncSatF64S, OpcodeMiscI32TruncSatF64U:
					inType, outType = ValueTypeF64, ValueTypeI32
				case OpcodeMiscI64TruncSatF32S, OpcodeMiscI64TruncSatF32U:
					inType, outType = ValueTypeF32, ValueTypeI64
				case OpcodeMiscI64TruncSatF64S, OpcodeMiscI64TruncSatF64U:
					inType, outType = ValueTypeF64, ValueTypeI64
				}
				if err := valueTypeStack.popAndVerifyType(inType); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", miscInstructionNames[miscOpcode], err)
				}
				valueTypeStack.push(outType)
			} else if miscOpcode >= OpcodeMiscMemoryInit && miscOpcode <= OpcodeMiscTableCopy {
				if err := enabledFeatures.RequireEnabled(api.CoreFeatureBulkMemoryOperations); err != nil {
					return fmt.Errorf("%s invalid as %v", miscInstructionNames[miscOpcode], err)
				}
				var params []ValueType
				// Handle opcodes added in bulk-memory-operations/WebAssembly 2.0.
				switch miscOpcode {
				case OpcodeMiscDataDrop:
					if m.DataCountSection == nil {
						return fmt.Errorf("%s requires data count section", MiscInstructionName(miscOpcode))
					}

					// We need to read the index to the data section.
					pc++
					index, num, err := leb128.LoadUint32(body[pc:])
					if err != nil {
						return fmt.Errorf("failed to read data segment index for %s: %v", MiscInstructionName(miscOpcode), err)
					}
					if int(index) >= len(m.DataSection) {
						return fmt.Errorf("index %d out of range of data section(len=%d)", index, len(m.DataSection))
					}
					pc += num - 1
				case OpcodeMiscMemoryInit, OpcodeMiscMemoryCopy, OpcodeMiscMemoryFill:
					if memory == nil {
						return fmt.Errorf("memory must exist for %s", MiscInstructionName(miscOpcode))
					}
					params = []ValueType{ValueTypeI32, ValueTypeI32, ValueTypeI32}

					if miscOpcode == OpcodeMiscMemoryInit {
						if m.DataCountSection == nil {
							return fmt.Errorf("%s requires data count section", MiscInstructionName(miscOpcode))
						}

						// We need to read the index to the data section.
						pc++
						index, num, err := leb128.LoadUint32(body[pc:])
						if err != nil {
							return fmt.Errorf("failed to read data segment index for %s: %v", MiscInstructionName(miscOpcode), err)
						}
						if int(index) >= len(m.DataSection) {
							return fmt.Errorf("index %d out of range of data section(len=%d)", index, len(m.DataSection))
						}
						pc += num - 1
					}

					pc++
					val, num, err := leb128.LoadUint32(body[pc:])
					if err != nil {
						return fmt.Errorf("failed to read memory index for %s: %v", MiscInstructionName(miscOpcode), err)
					}
					if val != 0 || num != 1 {
						return fmt.Errorf("%s reserved byte must be zero encoded with 1 byte", MiscInstructionName(miscOpcode))
					}
					if miscOpcode == OpcodeMiscMemoryCopy {
						pc++
						// memory.copy needs two memory index which are reserved as zero.
						val, num, err := leb128.LoadUint32(body[pc:])
						if err != nil {
							return fmt.Errorf("failed to read memory index for %s: %v", MiscInstructionName(miscOpcode), err)
						}
						if val != 0 || num != 1 {
							return fmt.Errorf("%s reserved byte must be zero encoded with 1 byte", MiscInstructionName(miscOpcode))
						}
					}

				case OpcodeMiscTableInit:
					params = []ValueType{ValueTypeI32, ValueTypeI32, ValueTypeI32}
					pc++
					elementIndex, num, err := leb128.LoadUint32(body[pc:])
					if err != nil {
						return fmt.Errorf("failed to read element segment index for %s: %v", MiscInstructionName(miscOpcode), err)
					}
					if int(elementIndex) >= len(m.ElementSection) {
						return fmt.Errorf("index %d out of range of element section(len=%d)", elementIndex, len(m.ElementSection))
					}
					pc += num

					tableIndex, num, err := leb128.LoadUint32(body[pc:])
					if err != nil {
						return fmt.Errorf("failed to read source table index for %s: %v", MiscInstructionName(miscOpcode), err)
					}
					if tableIndex != 0 {
						if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
							return fmt.Errorf("source table index must be zero for %s as %v", MiscInstructionName(miscOpcode), err)
						}
					}
					if tableIndex >= uint32(len(tables)) {
						return fmt.Errorf("table of index %d not found", tableIndex)
					}

					if m.ElementSection[elementIndex].Type != tables[tableIndex].Type {
						return fmt.Errorf("type mismatch for table.init: element type %s does not match table type %s",
							RefTypeName(m.ElementSection[elementIndex].Type),
							RefTypeName(tables[tableIndex].Type),
						)
					}
					pc += num - 1
				case OpcodeMiscElemDrop:
					pc++
					elementIndex, num, err := leb128.LoadUint32(body[pc:])
					if err != nil {
						return fmt.Errorf("failed to read element segment index for %s: %v", MiscInstructionName(miscOpcode), err)
					} else if int(elementIndex) >= len(m.ElementSection) {
						return fmt.Errorf("index %d out of range of element section(len=%d)", elementIndex, len(m.ElementSection))
					}
					pc += num - 1
				case OpcodeMiscTableCopy:
					params = []ValueType{ValueTypeI32, ValueTypeI32, ValueTypeI32}
					pc++

					dstTableIndex, num, err := leb128.LoadUint32(body[pc:])
					if err != nil {
						return fmt.Errorf("failed to read destination table index for %s: %v", MiscInstructionName(miscOpcode), err)
					}
					if dstTableIndex != 0 {
						if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
							return fmt.Errorf("destination table index must be zero for %s as %v", MiscInstructionName(miscOpcode), err)
						}
					}
					if dstTableIndex >= uint32(len(tables)) {
						return fmt.Errorf("table of index %d not found", dstTableIndex)
					}
					pc += num

					srcTableIndex, num, err := leb128.LoadUint32(body[pc:])
					if err != nil {
						return fmt.Errorf("failed to read source table index for %s: %v", MiscInstructionName(miscOpcode), err)
					}
					if srcTableIndex != 0 {
						if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
							return fmt.Errorf("source table index must be zero for %s as %v", MiscInstructionName(miscOpcode), err)
						}
					}
					if srcTableIndex >= uint32(len(tables)) {
						return fmt.Errorf("table of index %d not found", srcTableIndex)
					}

					if tables[srcTableIndex].Type != tables[dstTableIndex].Type {
						return fmt.Errorf("table type mismatch for table.copy: %s (src) != %s (dst)",
							RefTypeName(tables[srcTableIndex].Type), RefTypeName(tables[dstTableIndex].Type))
					}

					pc += num - 1
				}
				for _, p := range params {
					if err := valueTypeStack.popAndVerifyType(p); err != nil {
						return fmt.Errorf("cannot pop the operand for %s: %v", miscInstructionNames[miscOpcode], err)
					}
				}
			} else if miscOpcode >= OpcodeMiscTableGrow && miscOpcode <= OpcodeMiscTableFill {
				if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
					return fmt.Errorf("%s invalid as %v", miscInstructionNames[miscOpcode], err)
				}

				pc++
				tableIndex, num, err := leb128.LoadUint32(body[pc:])
				if err != nil {
					return fmt.Errorf("failed to read table index for %s: %v", MiscInstructionName(miscOpcode), err)
				}
				if tableIndex >= uint32(len(tables)) {
					return fmt.Errorf("table of index %d not found", tableIndex)
				}
				pc += num - 1

				var params, results []ValueType
				reftype := tables[tableIndex].Type
				if miscOpcode == OpcodeMiscTableGrow {
					params = []ValueType{ValueTypeI32, reftype}
					results = []ValueType{ValueTypeI32}
				} else if miscOpcode == OpcodeMiscTableSize {
					results = []ValueType{ValueTypeI32}
				} else if miscOpcode == OpcodeMiscTableFill {
					params = []ValueType{ValueTypeI32, reftype, ValueTypeI32}
				}

				for _, p := range params {
					if err := valueTypeStack.popAndVerifyType(p); err != nil {
						return fmt.Errorf("cannot pop the operand for %s: %v", miscInstructionNames[miscOpcode], err)
					}
				}
				for _, r := range results {
					valueTypeStack.push(r)
				}
			} else {
				return fmt.Errorf("unknown misc opcode %#x", miscOpcode)
			}
		} else if op == OpcodeVecPrefix {
			pc++
			// Vector instructions come with two bytes where the first byte is always OpcodeVecPrefix,
			// and the second byte determines the actual instruction.
			vecOpcode := body[pc]
			if err := enabledFeatures.RequireEnabled(api.CoreFeatureSIMD); err != nil {
				return fmt.Errorf("%s invalid as %v", vectorInstructionName[vecOpcode], err)
			}

			switch vecOpcode {
			case OpcodeVecV128Const:
				// Read 128-bit = 16 bytes constants
				if int(pc+16) >= len(body) {
					return fmt.Errorf("cannot read constant vector value for %s", vectorInstructionName[vecOpcode])
				}
				pc += 16
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecV128AnyTrue, OpcodeVecI8x16AllTrue, OpcodeVecI16x8AllTrue, OpcodeVecI32x4AllTrue, OpcodeVecI64x2AllTrue,
				OpcodeVecI8x16BitMask, OpcodeVecI16x8BitMask, OpcodeVecI32x4BitMask, OpcodeVecI64x2BitMask:
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeVecV128Load, OpcodeVecV128Load8x8s, OpcodeVecV128Load8x8u, OpcodeVecV128Load16x4s, OpcodeVecV128Load16x4u,
				OpcodeVecV128Load32x2s, OpcodeVecV128Load32x2u, OpcodeVecV128Load8Splat, OpcodeVecV128Load16Splat,
				OpcodeVecV128Load32Splat, OpcodeVecV128Load64Splat,
				OpcodeVecV128Load32zero, OpcodeVecV128Load64zero:
				if memory == nil {
					return fmt.Errorf("memory must exist for %s", VectorInstructionName(vecOpcode))
				}
				pc++
				align, _, read, err := readMemArg(pc, body)
				if err != nil {
					return err
				}
				pc += read - 1
				var maxAlign uint32
				switch vecOpcode {
				case OpcodeVecV128Load:
					maxAlign = 128 / 8
				case OpcodeVecV128Load8x8s, OpcodeVecV128Load8x8u, OpcodeVecV128Load16x4s, OpcodeVecV128Load16x4u,
					OpcodeVecV128Load32x2s, OpcodeVecV128Load32x2u:
					maxAlign = 64 / 8
				case OpcodeVecV128Load8Splat:
					maxAlign = 1
				case OpcodeVecV128Load16Splat:
					maxAlign = 16 / 8
				case OpcodeVecV128Load32Splat:
					maxAlign = 32 / 8
				case OpcodeVecV128Load64Splat:
					maxAlign = 64 / 8
				case OpcodeVecV128Load32zero:
					maxAlign = 32 / 8
				case OpcodeVecV128Load64zero:
					maxAlign = 64 / 8
				}

				if 1<<align > maxAlign {
					return fmt.Errorf("invalid memory alignment %d for %s", align, VectorInstructionName(vecOpcode))
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", VectorInstructionName(vecOpcode), err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecV128Store:
				if memory == nil {
					return fmt.Errorf("memory must exist for %s", VectorInstructionName(vecOpcode))
				}
				pc++
				align, _, read, err := readMemArg(pc, body)
				if err != nil {
					return err
				}
				pc += read - 1
				if 1<<align > 128/8 {
					return fmt.Errorf("invalid memory alignment %d for %s", align, OpcodeVecV128StoreName)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeVecV128StoreName, err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", OpcodeVecV128StoreName, err)
				}
			case OpcodeVecV128Load8Lane, OpcodeVecV128Load16Lane, OpcodeVecV128Load32Lane, OpcodeVecV128Load64Lane:
				if memory == nil {
					return fmt.Errorf("memory must exist for %s", VectorInstructionName(vecOpcode))
				}
				attr := vecLoadLanes[vecOpcode]
				pc++
				align, _, read, err := readMemArg(pc, body)
				if err != nil {
					return err
				}
				if 1<<align > attr.alignMax {
					return fmt.Errorf("invalid memory alignment %d for %s", align, vectorInstructionName[vecOpcode])
				}
				pc += read
				if pc >= uint64(len(body)) {
					return fmt.Errorf("lane for %s not found", OpcodeVecV128Load64LaneName)
				}
				lane := body[pc]
				if lane >= attr.laneCeil {
					return fmt.Errorf("invalid lane index %d >= %d for %s", lane, attr.laneCeil, vectorInstructionName[vecOpcode])
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecV128Store8Lane, OpcodeVecV128Store16Lane, OpcodeVecV128Store32Lane, OpcodeVecV128Store64Lane:
				if memory == nil {
					return fmt.Errorf("memory must exist for %s", VectorInstructionName(vecOpcode))
				}
				attr := vecStoreLanes[vecOpcode]
				pc++
				align, _, read, err := readMemArg(pc, body)
				if err != nil {
					return err
				}
				if 1<<align > attr.alignMax {
					return fmt.Errorf("invalid memory alignment %d for %s", align, vectorInstructionName[vecOpcode])
				}
				pc += read
				if pc >= uint64(len(body)) {
					return fmt.Errorf("lane for %s not found", vectorInstructionName[vecOpcode])
				}
				lane := body[pc]
				if lane >= attr.laneCeil {
					return fmt.Errorf("invalid lane index %d >= %d for %s", lane, attr.laneCeil, vectorInstructionName[vecOpcode])
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
			case OpcodeVecI8x16ExtractLaneS,
				OpcodeVecI8x16ExtractLaneU,
				OpcodeVecI16x8ExtractLaneS,
				OpcodeVecI16x8ExtractLaneU,
				OpcodeVecI32x4ExtractLane,
				OpcodeVecI64x2ExtractLane,
				OpcodeVecF32x4ExtractLane,
				OpcodeVecF64x2ExtractLane:
				pc++
				if pc >= uint64(len(body)) {
					return fmt.Errorf("lane for %s not found", vectorInstructionName[vecOpcode])
				}
				attr := vecExtractLanes[vecOpcode]
				lane := body[pc]
				if lane >= attr.laneCeil {
					return fmt.Errorf("invalid lane index %d >= %d for %s", lane, attr.laneCeil, vectorInstructionName[vecOpcode])
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(attr.resultType)
			case OpcodeVecI8x16ReplaceLane, OpcodeVecI16x8ReplaceLane, OpcodeVecI32x4ReplaceLane,
				OpcodeVecI64x2ReplaceLane, OpcodeVecF32x4ReplaceLane, OpcodeVecF64x2ReplaceLane:
				pc++
				if pc >= uint64(len(body)) {
					return fmt.Errorf("lane for %s not found", vectorInstructionName[vecOpcode])
				}
				attr := vecReplaceLanes[vecOpcode]
				lane := body[pc]
				if lane >= attr.laneCeil {
					return fmt.Errorf("invalid lane index %d >= %d for %s", lane, attr.laneCeil, vectorInstructionName[vecOpcode])
				}
				if err := valueTypeStack.popAndVerifyType(attr.paramType); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecI8x16Splat, OpcodeVecI16x8Splat, OpcodeVecI32x4Splat,
				OpcodeVecI64x2Splat, OpcodeVecF32x4Splat, OpcodeVecF64x2Splat:
				tp := vecSplatValueTypes[vecOpcode]
				if err := valueTypeStack.popAndVerifyType(tp); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecI8x16Swizzle, OpcodeVecV128And, OpcodeVecV128Or, OpcodeVecV128Xor, OpcodeVecV128AndNot:
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecV128Bitselect:
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecV128Not:
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecV128i8x16Shuffle:
				pc++
				if pc+15 >= uint64(len(body)) {
					return fmt.Errorf("16 lane indexes for %s not found", vectorInstructionName[vecOpcode])
				}
				lanes := body[pc : pc+16]
				for i, l := range lanes {
					if l >= 32 {
						return fmt.Errorf("invalid lane index[%d] %d >= %d for %s", i, l, 32, vectorInstructionName[vecOpcode])
					}
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
				pc += 15
			case OpcodeVecI8x16Shl, OpcodeVecI8x16ShrS, OpcodeVecI8x16ShrU,
				OpcodeVecI16x8Shl, OpcodeVecI16x8ShrS, OpcodeVecI16x8ShrU,
				OpcodeVecI32x4Shl, OpcodeVecI32x4ShrS, OpcodeVecI32x4ShrU,
				OpcodeVecI64x2Shl, OpcodeVecI64x2ShrS, OpcodeVecI64x2ShrU:
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecI8x16Eq, OpcodeVecI8x16Ne, OpcodeVecI8x16LtS, OpcodeVecI8x16LtU, OpcodeVecI8x16GtS,
				OpcodeVecI8x16GtU, OpcodeVecI8x16LeS, OpcodeVecI8x16LeU, OpcodeVecI8x16GeS, OpcodeVecI8x16GeU,
				OpcodeVecI16x8Eq, OpcodeVecI16x8Ne, OpcodeVecI16x8LtS, OpcodeVecI16x8LtU, OpcodeVecI16x8GtS,
				OpcodeVecI16x8GtU, OpcodeVecI16x8LeS, OpcodeVecI16x8LeU, OpcodeVecI16x8GeS, OpcodeVecI16x8GeU,
				OpcodeVecI32x4Eq, OpcodeVecI32x4Ne, OpcodeVecI32x4LtS, OpcodeVecI32x4LtU, OpcodeVecI32x4GtS,
				OpcodeVecI32x4GtU, OpcodeVecI32x4LeS, OpcodeVecI32x4LeU, OpcodeVecI32x4GeS, OpcodeVecI32x4GeU,
				OpcodeVecI64x2Eq, OpcodeVecI64x2Ne, OpcodeVecI64x2LtS, OpcodeVecI64x2GtS, OpcodeVecI64x2LeS,
				OpcodeVecI64x2GeS, OpcodeVecF32x4Eq, OpcodeVecF32x4Ne, OpcodeVecF32x4Lt, OpcodeVecF32x4Gt,
				OpcodeVecF32x4Le, OpcodeVecF32x4Ge, OpcodeVecF64x2Eq, OpcodeVecF64x2Ne, OpcodeVecF64x2Lt,
				OpcodeVecF64x2Gt, OpcodeVecF64x2Le, OpcodeVecF64x2Ge,
				OpcodeVecI32x4DotI16x8S,
				OpcodeVecI8x16NarrowI16x8S, OpcodeVecI8x16NarrowI16x8U, OpcodeVecI16x8NarrowI32x4S, OpcodeVecI16x8NarrowI32x4U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			case OpcodeVecI8x16Neg, OpcodeVecI16x8Neg, OpcodeVecI32x4Neg, OpcodeVecI64x2Neg, OpcodeVecF32x4Neg, OpcodeVecF64x2Neg,
				OpcodeVecF32x4Sqrt, OpcodeVecF64x2Sqrt,
				OpcodeVecI8x16Abs, OpcodeVecI8x16Popcnt, OpcodeVecI16x8Abs, OpcodeVecI32x4Abs, OpcodeVecI64x2Abs,
				OpcodeVecF32x4Abs, OpcodeVecF64x2Abs,
				OpcodeVecF32x4Ceil, OpcodeVecF32x4Floor, OpcodeVecF32x4Trunc, OpcodeVecF32x4Nearest,
				OpcodeVecF64x2Ceil, OpcodeVecF64x2Floor, OpcodeVecF64x2Trunc, OpcodeVecF64x2Nearest,
				OpcodeVecI16x8ExtendLowI8x16S, OpcodeVecI16x8ExtendHighI8x16S, OpcodeVecI16x8ExtendLowI8x16U, OpcodeVecI16x8ExtendHighI8x16U,
				OpcodeVecI32x4ExtendLowI16x8S, OpcodeVecI32x4ExtendHighI16x8S, OpcodeVecI32x4ExtendLowI16x8U, OpcodeVecI32x4ExtendHighI16x8U,
				OpcodeVecI64x2ExtendLowI32x4S, OpcodeVecI64x2ExtendHighI32x4S, OpcodeVecI64x2ExtendLowI32x4U, OpcodeVecI64x2ExtendHighI32x4U,
				OpcodeVecI16x8ExtaddPairwiseI8x16S, OpcodeVecI16x8ExtaddPairwiseI8x16U,
				OpcodeVecI32x4ExtaddPairwiseI16x8S, OpcodeVecI32x4ExtaddPairwiseI16x8U,
				OpcodeVecF64x2PromoteLowF32x4Zero, OpcodeVecF32x4DemoteF64x2Zero,
				OpcodeVecF32x4ConvertI32x4S, OpcodeVecF32x4ConvertI32x4U,
				OpcodeVecF64x2ConvertLowI32x4S, OpcodeVecF64x2ConvertLowI32x4U,
				OpcodeVecI32x4TruncSatF32x4S, OpcodeVecI32x4TruncSatF32x4U, OpcodeVecI32x4TruncSatF64x2SZero, OpcodeVecI32x4TruncSatF64x2UZero:
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)

			case OpcodeVecI8x16Add, OpcodeVecI8x16AddSatS, OpcodeVecI8x16AddSatU, OpcodeVecI8x16Sub, OpcodeVecI8x16SubSatS, OpcodeVecI8x16SubSatU,
				OpcodeVecI16x8Add, OpcodeVecI16x8AddSatS, OpcodeVecI16x8AddSatU, OpcodeVecI16x8Sub, OpcodeVecI16x8SubSatS, OpcodeVecI16x8SubSatU, OpcodeVecI16x8Mul,
				OpcodeVecI32x4Add, OpcodeVecI32x4Sub, OpcodeVecI32x4Mul,
				OpcodeVecI64x2Add, OpcodeVecI64x2Sub, OpcodeVecI64x2Mul,
				OpcodeVecF32x4Add, OpcodeVecF32x4Sub, OpcodeVecF32x4Mul, OpcodeVecF32x4Div,
				OpcodeVecF64x2Add, OpcodeVecF64x2Sub, OpcodeVecF64x2Mul, OpcodeVecF64x2Div,
				OpcodeVecI8x16MinS, OpcodeVecI8x16MinU, OpcodeVecI8x16MaxS, OpcodeVecI8x16MaxU,
				OpcodeVecI8x16AvgrU,
				OpcodeVecI16x8MinS, OpcodeVecI16x8MinU, OpcodeVecI16x8MaxS, OpcodeVecI16x8MaxU,
				OpcodeVecI16x8AvgrU,
				OpcodeVecI32x4MinS, OpcodeVecI32x4MinU, OpcodeVecI32x4MaxS, OpcodeVecI32x4MaxU,
				OpcodeVecF32x4Min, OpcodeVecF32x4Max, OpcodeVecF64x2Min, OpcodeVecF64x2Max,
				OpcodeVecF32x4Pmin, OpcodeVecF32x4Pmax, OpcodeVecF64x2Pmin, OpcodeVecF64x2Pmax,
				OpcodeVecI16x8Q15mulrSatS,
				OpcodeVecI16x8ExtMulLowI8x16S, OpcodeVecI16x8ExtMulHighI8x16S, OpcodeVecI16x8ExtMulLowI8x16U, OpcodeVecI16x8ExtMulHighI8x16U,
				OpcodeVecI32x4ExtMulLowI16x8S, OpcodeVecI32x4ExtMulHighI16x8S, OpcodeVecI32x4ExtMulLowI16x8U, OpcodeVecI32x4ExtMulHighI16x8U,
				OpcodeVecI64x2ExtMulLowI32x4S, OpcodeVecI64x2ExtMulHighI32x4S, OpcodeVecI64x2ExtMulLowI32x4U, OpcodeVecI64x2ExtMulHighI32x4U:
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeV128); err != nil {
					return fmt.Errorf("cannot pop the operand for %s: %v", vectorInstructionName[vecOpcode], err)
				}
				valueTypeStack.push(ValueTypeV128)
			default:
				return fmt.Errorf("unknown SIMD instruction %s", vectorInstructionName[vecOpcode])
			}
		} else if op == OpcodeBlock {
			br.Reset(body[pc+1:])
			bt, num, err := DecodeBlockType(m.TypeSection, br, enabledFeatures)
			if err != nil {
				return fmt.Errorf("read block: %w", err)
			}
			controlBlockStack.push(pc, 0, 0, bt, num, 0)
			if err = valueTypeStack.popParams(op, bt.Params, false); err != nil {
				return err
			}
			// Plus we have to push any block params again.
			for _, p := range bt.Params {
				valueTypeStack.push(p)
			}
			valueTypeStack.pushStackLimit(len(bt.Params))
			pc += num
		} else if op == OpcodeAtomicPrefix {
			pc++
			// Atomic instructions come with two bytes where the first byte is always OpcodeAtomicPrefix,
			// and the second byte determines the actual instruction.
			atomicOpcode := body[pc]
			if err := enabledFeatures.RequireEnabled(experimental.CoreFeaturesThreads); err != nil {
				return fmt.Errorf("%s invalid as %v", atomicInstructionName[atomicOpcode], err)
			}
			pc++

			if atomicOpcode == OpcodeAtomicFence {
				// No memory requirement and no arguments or return, however the immediate byte value must be 0.
				imm := body[pc]
				if imm != 0x0 {
					return fmt.Errorf("invalid immediate value for %s", AtomicInstructionName(atomicOpcode))
				}
				continue
			}

			// All atomic operations except fence (checked above) require memory
			if memory == nil {
				return fmt.Errorf("memory must exist for %s", AtomicInstructionName(atomicOpcode))
			}
			align, _, read, err := readMemArg(pc, body)
			if err != nil {
				return err
			}
			pc += read - 1
			switch atomicOpcode {
			case OpcodeAtomicMemoryNotify:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicMemoryWait32:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicMemoryWait64:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI32Load:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI64Load:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI32Load8U:
				if 1<<align != 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI32Load16U:
				if 1<<align != 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI64Load8U:
				if 1<<align != 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI64Load16U:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI64Load32U:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI32Store:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeAtomicI64Store:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeAtomicI32Store8:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeAtomicI32Store16:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeAtomicI64Store8:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeAtomicI64Store16:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeAtomicI64Store32:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
			case OpcodeAtomicI32RmwAdd, OpcodeAtomicI32RmwSub, OpcodeAtomicI32RmwAnd, OpcodeAtomicI32RmwOr, OpcodeAtomicI32RmwXor, OpcodeAtomicI32RmwXchg:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI32Rmw8AddU, OpcodeAtomicI32Rmw8SubU, OpcodeAtomicI32Rmw8AndU, OpcodeAtomicI32Rmw8OrU, OpcodeAtomicI32Rmw8XorU, OpcodeAtomicI32Rmw8XchgU:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI32Rmw16AddU, OpcodeAtomicI32Rmw16SubU, OpcodeAtomicI32Rmw16AndU, OpcodeAtomicI32Rmw16OrU, OpcodeAtomicI32Rmw16XorU, OpcodeAtomicI32Rmw16XchgU:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI64RmwAdd, OpcodeAtomicI64RmwSub, OpcodeAtomicI64RmwAnd, OpcodeAtomicI64RmwOr, OpcodeAtomicI64RmwXor, OpcodeAtomicI64RmwXchg:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI64Rmw8AddU, OpcodeAtomicI64Rmw8SubU, OpcodeAtomicI64Rmw8AndU, OpcodeAtomicI64Rmw8OrU, OpcodeAtomicI64Rmw8XorU, OpcodeAtomicI64Rmw8XchgU:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI64Rmw16AddU, OpcodeAtomicI64Rmw16SubU, OpcodeAtomicI64Rmw16AndU, OpcodeAtomicI64Rmw16OrU, OpcodeAtomicI64Rmw16XorU, OpcodeAtomicI64Rmw16XchgU:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI64Rmw32AddU, OpcodeAtomicI64Rmw32SubU, OpcodeAtomicI64Rmw32AndU, OpcodeAtomicI64Rmw32OrU, OpcodeAtomicI64Rmw32XorU, OpcodeAtomicI64Rmw32XchgU:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI32RmwCmpxchg:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI32Rmw8CmpxchgU:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI32Rmw16CmpxchgU:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI32)
			case OpcodeAtomicI64RmwCmpxchg:
				if 1<<align > 64/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI64Rmw8CmpxchgU:
				if 1<<align > 1 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI64Rmw16CmpxchgU:
				if 1<<align > 16/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			case OpcodeAtomicI64Rmw32CmpxchgU:
				if 1<<align > 32/8 {
					return fmt.Errorf("invalid memory alignment")
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI64); err != nil {
					return err
				}
				if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
					return err
				}
				valueTypeStack.push(ValueTypeI64)
			default:
				return fmt.Errorf("invalid atomic opcode: 0x%x", atomicOpcode)
			}
		} else if op == OpcodeLoop {
			br.Reset(body[pc+1:])
			bt, num, err := DecodeBlockType(m.TypeSection, br, enabledFeatures)
			if err != nil {
				return fmt.Errorf("read block: %w", err)
			}
			controlBlockStack.push(pc, 0, 0, bt, num, op)
			if err = valueTypeStack.popParams(op, bt.Params, false); err != nil {
				return err
			}
			// Plus we have to push any block params again.
			for _, p := range bt.Params {
				valueTypeStack.push(p)
			}
			valueTypeStack.pushStackLimit(len(bt.Params))
			pc += num
		} else if op == OpcodeIf {
			br.Reset(body[pc+1:])
			bt, num, err := DecodeBlockType(m.TypeSection, br, enabledFeatures)
			if err != nil {
				return fmt.Errorf("read block: %w", err)
			}
			controlBlockStack.push(pc, 0, 0, bt, num, op)
			if err = valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
				return fmt.Errorf("cannot pop the operand for 'if': %v", err)
			}
			if err = valueTypeStack.popParams(op, bt.Params, false); err != nil {
				return err
			}
			// Plus we have to push any block params again.
			for _, p := range bt.Params {
				valueTypeStack.push(p)
			}
			valueTypeStack.pushStackLimit(len(bt.Params))
			pc += num
		} else if op == OpcodeElse {
			bl := &controlBlockStack.stack[len(controlBlockStack.stack)-1]
			if bl.op != OpcodeIf {
				return fmt.Errorf("else instruction must be used in if block: %#x", pc)
			}
			bl.op = OpcodeElse
			bl.elseAt = pc
			// Check the type soundness of the instructions *before* entering this else Op.
			if err := valueTypeStack.popResults(OpcodeIf, bl.blockType.Results, true); err != nil {
				return err
			}
			// Before entering instructions inside else, we pop all the values pushed by then block.
			valueTypeStack.resetAtStackLimit()
			// Plus we have to push any block params again.
			for _, p := range bl.blockType.Params {
				valueTypeStack.push(p)
			}
		} else if op == OpcodeEnd {
			bl := controlBlockStack.pop()
			bl.endAt = pc

			// OpcodeEnd can end a block or the function itself. Check to see what it is:

			ifMissingElse := bl.op == OpcodeIf && bl.elseAt <= bl.startAt
			if ifMissingElse {
				// If this is the end of block without else, the number of block's results and params must be same.
				// Otherwise, the value stack would result in the inconsistent state at runtime.
				if !bytes.Equal(bl.blockType.Results, bl.blockType.Params) {
					return typeCountError(false, OpcodeElseName, bl.blockType.Params, bl.blockType.Results)
				}
				// -1 skips else, to handle if block without else properly.
				bl.elseAt = bl.endAt - 1
			}

			// Determine the block context
			ctx := "" // the outer-most block: the function return
			if bl.op == OpcodeIf && !ifMissingElse && bl.elseAt > 0 {
				ctx = OpcodeElseName
			} else if bl.op != 0 {
				ctx = InstructionName(bl.op)
			}

			// Check return types match
			if err := valueTypeStack.requireStackValues(false, ctx, bl.blockType.Results, true); err != nil {
				return err
			}

			// Put the result types at the end after resetting at the stack limit
			// since we might have Any type between the limit and the current top.
			valueTypeStack.resetAtStackLimit()
			for _, exp := range bl.blockType.Results {
				valueTypeStack.push(exp)
			}
			// We exit if/loop/block, so reset the constraints on the stack manipulation
			// on values previously pushed by outer blocks.
			valueTypeStack.popStackLimit()
		} else if op == OpcodeReturn {
			// Same formatting as OpcodeEnd on the outer-most block
			if err := valueTypeStack.requireStackValues(false, "", functionType.Results, false); err != nil {
				return err
			}
			// return instruction is stack-polymorphic.
			valueTypeStack.unreachable()
		} else if op == OpcodeDrop {
			_, err := valueTypeStack.pop()
			if err != nil {
				return fmt.Errorf("invalid drop: %v", err)
			}
		} else if op == OpcodeSelect || op == OpcodeTypedSelect {
			if err := valueTypeStack.popAndVerifyType(ValueTypeI32); err != nil {
				return fmt.Errorf("type mismatch on 3rd select operand: %v", err)
			}
			v1, err := valueTypeStack.pop()
			if err != nil {
				return fmt.Errorf("invalid select: %v", err)
			}
			v2, err := valueTypeStack.pop()
			if err != nil {
				return fmt.Errorf("invalid select: %v", err)
			}

			if op == OpcodeTypedSelect {
				if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
					return fmt.Errorf("%s is invalid as %w", InstructionName(op), err)
				}
				pc++
				if numTypeImmeidates := body[pc]; numTypeImmeidates != 1 {
					return fmt.Errorf("too many type immediates for %s", InstructionName(op))
				}
				pc++
				tp := body[pc]
				if tp != ValueTypeI32 && tp != ValueTypeI64 && tp != ValueTypeF32 && tp != ValueTypeF64 &&
					tp != api.ValueTypeExternref && tp != ValueTypeFuncref && tp != ValueTypeV128 {
					return fmt.Errorf("invalid type %s for %s", ValueTypeName(tp), OpcodeTypedSelectName)
				}
			} else if isReferenceValueType(v1) || isReferenceValueType(v2) {
				return fmt.Errorf("reference types cannot be used for non typed select instruction")
			}

			if v1 != v2 && v1 != valueTypeUnknown && v2 != valueTypeUnknown {
				return fmt.Errorf("type mismatch on 1st and 2nd select operands")
			}
			if v1 == valueTypeUnknown {
				valueTypeStack.push(v2)
			} else {
				valueTypeStack.push(v1)
			}
		} else if op == OpcodeUnreachable {
			// unreachable instruction is stack-polymorphic.
			valueTypeStack.unreachable()
		} else if op == OpcodeNop {
		} else {
			return fmt.Errorf("invalid instruction 0x%x", op)
		}
	}

	if len(controlBlockStack.stack) > 0 {
		return fmt.Errorf("ill-nested block exists")
	}
	if valueTypeStack.maximumStackPointer > maxStackValues {
		return fmt.Errorf("function may have %d stack values, which exceeds limit %d", valueTypeStack.maximumStackPointer, maxStackValues)
	}
	return nil
}

var vecExtractLanes = [...]struct {
	laneCeil   byte
	resultType ValueType
}{
	OpcodeVecI8x16ExtractLaneS: {laneCeil: 16, resultType: ValueTypeI32},
	OpcodeVecI8x16ExtractLaneU: {laneCeil: 16, resultType: ValueTypeI32},
	OpcodeVecI16x8ExtractLaneS: {laneCeil: 8, resultType: ValueTypeI32},
	OpcodeVecI16x8ExtractLaneU: {laneCeil: 8, resultType: ValueTypeI32},
	OpcodeVecI32x4ExtractLane:  {laneCeil: 4, resultType: ValueTypeI32},
	OpcodeVecI64x2ExtractLane:  {laneCeil: 2, resultType: ValueTypeI64},
	OpcodeVecF32x4ExtractLane:  {laneCeil: 4, resultType: ValueTypeF32},
	OpcodeVecF64x2ExtractLane:  {laneCeil: 2, resultType: ValueTypeF64},
}

var vecReplaceLanes = [...]struct {
	laneCeil  byte
	paramType ValueType
}{
	OpcodeVecI8x16ReplaceLane: {laneCeil: 16, paramType: ValueTypeI32},
	OpcodeVecI16x8ReplaceLane: {laneCeil: 8, paramType: ValueTypeI32},
	OpcodeVecI32x4ReplaceLane: {laneCeil: 4, paramType: ValueTypeI32},
	OpcodeVecI64x2ReplaceLane: {laneCeil: 2, paramType: ValueTypeI64},
	OpcodeVecF32x4ReplaceLane: {laneCeil: 4, paramType: ValueTypeF32},
	OpcodeVecF64x2ReplaceLane: {laneCeil: 2, paramType: ValueTypeF64},
}

var vecStoreLanes = [...]struct {
	alignMax uint32
	laneCeil byte
}{
	OpcodeVecV128Store64Lane: {alignMax: 64 / 8, laneCeil: 128 / 64},
	OpcodeVecV128Store32Lane: {alignMax: 32 / 8, laneCeil: 128 / 32},
	OpcodeVecV128Store16Lane: {alignMax: 16 / 8, laneCeil: 128 / 16},
	OpcodeVecV128Store8Lane:  {alignMax: 1, laneCeil: 128 / 8},
}

var vecLoadLanes = [...]struct {
	alignMax uint32
	laneCeil byte
}{
	OpcodeVecV128Load64Lane: {alignMax: 64 / 8, laneCeil: 128 / 64},
	OpcodeVecV128Load32Lane: {alignMax: 32 / 8, laneCeil: 128 / 32},
	OpcodeVecV128Load16Lane: {alignMax: 16 / 8, laneCeil: 128 / 16},
	OpcodeVecV128Load8Lane:  {alignMax: 1, laneCeil: 128 / 8},
}

var vecSplatValueTypes = [...]ValueType{
	OpcodeVecI8x16Splat: ValueTypeI32,
	OpcodeVecI16x8Splat: ValueTypeI32,
	OpcodeVecI32x4Splat: ValueTypeI32,
	OpcodeVecI64x2Splat: ValueTypeI64,
	OpcodeVecF32x4Splat: ValueTypeF32,
	OpcodeVecF64x2Splat: ValueTypeF64,
}

type stacks struct {
	vs valueTypeStack
	cs controlBlockStack
	// ls is the label slice that is reused for each br_table instruction.
	ls []uint32
}

func (sts *stacks) reset(functionType *FunctionType) {
	// Reset valueStack for reuse.
	sts.vs.stack = sts.vs.stack[:0]
	sts.vs.stackLimits = sts.vs.stackLimits[:0]
	sts.vs.maximumStackPointer = 0
	sts.cs.stack = sts.cs.stack[:0]
	sts.cs.stack = append(sts.cs.stack, controlBlock{blockType: functionType})
	sts.ls = sts.ls[:0]
}

type controlBlockStack struct {
	stack []controlBlock
}

func (s *controlBlockStack) pop() *controlBlock {
	tail := len(s.stack) - 1
	ret := &s.stack[tail]
	s.stack = s.stack[:tail]
	return ret
}

func (s *controlBlockStack) push(startAt, elseAt, endAt uint64, blockType *FunctionType, blockTypeBytes uint64, op Opcode) {
	s.stack = append(s.stack, controlBlock{
		startAt:        startAt,
		elseAt:         elseAt,
		endAt:          endAt,
		blockType:      blockType,
		blockTypeBytes: blockTypeBytes,
		op:             op,
	})
}

type valueTypeStack struct {
	stack               []ValueType
	stackLimits         []int
	maximumStackPointer int
	// requireStackValuesTmp is used in requireStackValues function to reduce the allocation.
	requireStackValuesTmp []ValueType
}

// Only used in the analyzeFunction below.
const valueTypeUnknown = ValueType(0xFF)

func (s *valueTypeStack) tryPop() (vt ValueType, limit int, ok bool) {
	if len(s.stackLimits) > 0 {
		limit = s.stackLimits[len(s.stackLimits)-1]
	}
	stackLen := len(s.stack)
	if stackLen <= limit {
		return
	} else if stackLen == limit+1 && s.stack[limit] == valueTypeUnknown {
		vt = valueTypeUnknown
		ok = true
		return
	} else {
		vt = s.stack[stackLen-1]
		s.stack = s.stack[:stackLen-1]
		ok = true
		return
	}
}

func (s *valueTypeStack) pop() (ValueType, error) {
	if vt, limit, ok := s.tryPop(); ok {
		return vt, nil
	} else {
		return 0, fmt.Errorf("invalid operation: trying to pop at %d with limit %d", len(s.stack), limit)
	}
}

// popAndVerifyType returns an error if the stack value is unexpected.
func (s *valueTypeStack) popAndVerifyType(expected ValueType) error {
	have, _, ok := s.tryPop()
	if !ok {
		return fmt.Errorf("%s missing", ValueTypeName(expected))
	}
	if have != expected && have != valueTypeUnknown && expected != valueTypeUnknown {
		return fmt.Errorf("type mismatch: expected %s, but was %s", ValueTypeName(expected), ValueTypeName(have))
	}
	return nil
}

func (s *valueTypeStack) push(v ValueType) {
	s.stack = append(s.stack, v)
	if sp := len(s.stack); sp > s.maximumStackPointer {
		s.maximumStackPointer = sp
	}
}

func (s *valueTypeStack) unreachable() {
	s.resetAtStackLimit()
	s.stack = append(s.stack, valueTypeUnknown)
}

func (s *valueTypeStack) resetAtStackLimit() {
	if len(s.stackLimits) != 0 {
		s.stack = s.stack[:s.stackLimits[len(s.stackLimits)-1]]
	} else {
		s.stack = s.stack[:0]
	}
}

func (s *valueTypeStack) popStackLimit() {
	if len(s.stackLimits) != 0 {
		s.stackLimits = s.stackLimits[:len(s.stackLimits)-1]
	}
}

// pushStackLimit pushes the control frame's bottom of the stack.
func (s *valueTypeStack) pushStackLimit(params int) {
	limit := len(s.stack) - params
	s.stackLimits = append(s.stackLimits, limit)
}

func (s *valueTypeStack) popParams(oc Opcode, want []ValueType, checkAboveLimit bool) error {
	return s.requireStackValues(true, InstructionName(oc), want, checkAboveLimit)
}

func (s *valueTypeStack) popResults(oc Opcode, want []ValueType, checkAboveLimit bool) error {
	return s.requireStackValues(false, InstructionName(oc), want, checkAboveLimit)
}

func (s *valueTypeStack) requireStackValues(
	isParam bool,
	context string,
	want []ValueType,
	checkAboveLimit bool,
) error {
	limit := 0
	if len(s.stackLimits) > 0 {
		limit = s.stackLimits[len(s.stackLimits)-1]
	}
	// Iterate backwards as we are comparing the desired slice against stack value types.
	countWanted := len(want)

	// First, check if there are enough values on the stack.
	s.requireStackValuesTmp = s.requireStackValuesTmp[:0]
	for i := countWanted - 1; i >= 0; i-- {
		popped, _, ok := s.tryPop()
		if !ok {
			if len(s.requireStackValuesTmp) > len(want) {
				return typeCountError(isParam, context, s.requireStackValuesTmp, want)
			}
			return typeCountError(isParam, context, s.requireStackValuesTmp, want)
		}
		s.requireStackValuesTmp = append(s.requireStackValuesTmp, popped)
	}

	// Now, check if there are too many values.
	if checkAboveLimit {
		if !(limit == len(s.stack) || (limit+1 == len(s.stack) && s.stack[limit] == valueTypeUnknown)) {
			return typeCountError(isParam, context, append(s.stack, want...), want)
		}
	}

	// Finally, check the types of the values:
	for i, v := range s.requireStackValuesTmp {
		nextWant := want[countWanted-i-1] // have is in reverse order (stack)
		if v != nextWant && v != valueTypeUnknown && nextWant != valueTypeUnknown {
			return typeMismatchError(isParam, context, v, nextWant, i)
		}
	}
	return nil
}

// typeMismatchError returns an error similar to go compiler's error on type mismatch.
func typeMismatchError(isParam bool, context string, have ValueType, want ValueType, i int) error {
	var ret strings.Builder
	ret.WriteString("cannot use ")
	ret.WriteString(ValueTypeName(have))
	if context != "" {
		ret.WriteString(" in ")
		ret.WriteString(context)
		ret.WriteString(" block")
	}
	if isParam {
		ret.WriteString(" as param")
	} else {
		ret.WriteString(" as result")
	}
	ret.WriteString("[")
	ret.WriteString(strconv.Itoa(i))
	ret.WriteString("] type ")
	ret.WriteString(ValueTypeName(want))
	return errors.New(ret.String())
}

// typeCountError returns an error similar to go compiler's error on type count mismatch.
func typeCountError(isParam bool, context string, have []ValueType, want []ValueType) error {
	var ret strings.Builder
	if len(have) > len(want) {
		ret.WriteString("too many ")
	} else {
		ret.WriteString("not enough ")
	}
	if isParam {
		ret.WriteString("params")
	} else {
		ret.WriteString("results")
	}
	if context != "" {
		if isParam {
			ret.WriteString(" for ")
		} else {
			ret.WriteString(" in ")
		}
		ret.WriteString(context)
		ret.WriteString(" block")
	}
	ret.WriteString("\n\thave (")
	writeValueTypes(have, &ret)
	ret.WriteString(")\n\twant (")
	writeValueTypes(want, &ret)
	ret.WriteByte(')')
	return errors.New(ret.String())
}

func writeValueTypes(vts []ValueType, ret *strings.Builder) {
	switch len(vts) {
	case 0:
	case 1:
		ret.WriteString(ValueTypeName(vts[0]))
	default:
		ret.WriteString(ValueTypeName(vts[0]))
		for _, vt := range vts[1:] {
			ret.WriteString(", ")
			ret.WriteString(ValueTypeName(vt))
		}
	}
}

func (s *valueTypeStack) String() string {
	var typeStrs, limits []string
	for _, v := range s.stack {
		var str string
		if v == valueTypeUnknown {
			str = "unknown"
		} else {
			str = ValueTypeName(v)
		}
		typeStrs = append(typeStrs, str)
	}
	for _, d := range s.stackLimits {
		limits = append(limits, fmt.Sprintf("%d", d))
	}
	return fmt.Sprintf("{stack: [%s], limits: [%s]}",
		strings.Join(typeStrs, ", "), strings.Join(limits, ","))
}

type controlBlock struct {
	startAt, elseAt, endAt uint64
	blockType              *FunctionType
	blockTypeBytes         uint64
	// op is zero when the outermost block
	op Opcode
}

// DecodeBlockType decodes the type index from a positive 33-bit signed integer. Negative numbers indicate up to one
// WebAssembly 1.0 (20191205) compatible result type. Positive numbers are decoded when `enabledFeatures` include
// CoreFeatureMultiValue and include an index in the Module.TypeSection.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-blocktype
// See https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/multi-value/Overview.md
func DecodeBlockType(types []FunctionType, r *bytes.Reader, enabledFeatures api.CoreFeatures) (*FunctionType, uint64, error) {
	raw, num, err := leb128.DecodeInt33AsInt64(r)
	if err != nil {
		return nil, 0, fmt.Errorf("decode int33: %w", err)
	}

	var ret *FunctionType
	switch raw {
	case -64: // 0x40 in original byte = nil
		ret = blockType_v_v
	case -1: // 0x7f in original byte = i32
		ret = blockType_v_i32
	case -2: // 0x7e in original byte = i64
		ret = blockType_v_i64
	case -3: // 0x7d in original byte = f32
		ret = blockType_v_f32
	case -4: // 0x7c in original byte = f64
		ret = blockType_v_f64
	case -5: // 0x7b in original byte = v128
		ret = blockType_v_v128
	case -16: // 0x70 in original byte = funcref
		ret = blockType_v_funcref
	case -17: // 0x6f in original byte = externref
		ret = blockType_v_externref
	default:
		if err = enabledFeatures.RequireEnabled(api.CoreFeatureMultiValue); err != nil {
			return nil, num, fmt.Errorf("block with function type return invalid as %v", err)
		}
		if raw < 0 || (raw >= int64(len(types))) {
			return nil, 0, fmt.Errorf("type index out of range: %d", raw)
		}
		ret = &types[raw]
	}
	return ret, num, err
}

// These block types are defined as globals in order to avoid allocations in DecodeBlockType.
var (
	blockType_v_v         = &FunctionType{}
	blockType_v_i32       = &FunctionType{Results: []ValueType{ValueTypeI32}, ResultNumInUint64: 1}
	blockType_v_i64       = &FunctionType{Results: []ValueType{ValueTypeI64}, ResultNumInUint64: 1}
	blockType_v_f32       = &FunctionType{Results: []ValueType{ValueTypeF32}, ResultNumInUint64: 1}
	blockType_v_f64       = &FunctionType{Results: []ValueType{ValueTypeF64}, ResultNumInUint64: 1}
	blockType_v_v128      = &FunctionType{Results: []ValueType{ValueTypeV128}, ResultNumInUint64: 2}
	blockType_v_funcref   = &FunctionType{Results: []ValueType{ValueTypeFuncref}, ResultNumInUint64: 1}
	blockType_v_externref = &FunctionType{Results: []ValueType{ValueTypeExternref}, ResultNumInUint64: 1}
)

// SplitCallStack returns the input stack resliced to the count of params and
// results, or errors if it isn't long enough for either.
func SplitCallStack(ft *FunctionType, stack []uint64) (params []uint64, results []uint64, err error) {
	stackLen := len(stack)
	if n := ft.ParamNumInUint64; n > stackLen {
		return nil, nil, fmt.Errorf("need %d params, but stack size is %d", n, stackLen)
	} else if n > 0 {
		params = stack[:n]
	}
	if n := ft.ResultNumInUint64; n > stackLen {
		return nil, nil, fmt.Errorf("need %d results, but stack size is %d", n, stackLen)
	} else if n > 0 {
		results = stack[:n]
	}
	return
}
