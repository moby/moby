package interpreter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

type controlFrameKind byte

const (
	controlFrameKindBlockWithContinuationLabel controlFrameKind = iota
	controlFrameKindBlockWithoutContinuationLabel
	controlFrameKindFunction
	controlFrameKindLoop
	controlFrameKindIfWithElse
	controlFrameKindIfWithoutElse
)

type (
	controlFrame struct {
		frameID uint32
		// originalStackLenWithoutParam holds the number of values on the stack
		// when Start executing this control frame minus params for the block.
		originalStackLenWithoutParam int
		// originalStackLenWithoutParamUint64 is almost the same as originalStackLenWithoutParam
		// except that it holds the number of values on the stack in uint64.
		originalStackLenWithoutParamUint64 int
		blockType                          *wasm.FunctionType
		kind                               controlFrameKind
	}
	controlFrames struct{ frames []controlFrame }
)

func (c *controlFrame) ensureContinuation() {
	// Make sure that if the frame is block and doesn't have continuation,
	// change the Kind so we can emit the continuation block
	// later when we reach the End instruction of this frame.
	if c.kind == controlFrameKindBlockWithoutContinuationLabel {
		c.kind = controlFrameKindBlockWithContinuationLabel
	}
}

func (c *controlFrame) asLabel() label {
	switch c.kind {
	case controlFrameKindBlockWithContinuationLabel,
		controlFrameKindBlockWithoutContinuationLabel:
		return newLabel(labelKindContinuation, c.frameID)
	case controlFrameKindLoop:
		return newLabel(labelKindHeader, c.frameID)
	case controlFrameKindFunction:
		return newLabel(labelKindReturn, 0)
	case controlFrameKindIfWithElse,
		controlFrameKindIfWithoutElse:
		return newLabel(labelKindContinuation, c.frameID)
	}
	panic(fmt.Sprintf("unreachable: a bug in interpreterir implementation: %v", c.kind))
}

func (c *controlFrames) functionFrame() *controlFrame {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	return &c.frames[0]
}

func (c *controlFrames) get(n int) *controlFrame {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	return &c.frames[len(c.frames)-n-1]
}

func (c *controlFrames) top() *controlFrame {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	return &c.frames[len(c.frames)-1]
}

func (c *controlFrames) empty() bool {
	return len(c.frames) == 0
}

func (c *controlFrames) pop() (frame *controlFrame) {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	frame = c.top()
	c.frames = c.frames[:len(c.frames)-1]
	return
}

func (c *controlFrames) push(frame controlFrame) {
	c.frames = append(c.frames, frame)
}

func (c *compiler) initializeStack() {
	// Reuse the existing slice.
	c.localIndexToStackHeightInUint64 = c.localIndexToStackHeightInUint64[:0]
	var current int
	for _, lt := range c.sig.Params {
		c.localIndexToStackHeightInUint64 = append(c.localIndexToStackHeightInUint64, current)
		if lt == wasm.ValueTypeV128 {
			current++
		}
		current++
	}

	if c.callFrameStackSizeInUint64 > 0 {
		// We reserve the stack slots for result values below the return call frame slots.
		if diff := c.sig.ResultNumInUint64 - c.sig.ParamNumInUint64; diff > 0 {
			current += diff
		}
	}

	// Non-func param locals Start after the return call frame.
	current += c.callFrameStackSizeInUint64

	for _, lt := range c.localTypes {
		c.localIndexToStackHeightInUint64 = append(c.localIndexToStackHeightInUint64, current)
		if lt == wasm.ValueTypeV128 {
			current++
		}
		current++
	}

	// Push function arguments.
	for _, t := range c.sig.Params {
		c.stackPush(wasmValueTypeTounsignedType(t))
	}

	if c.callFrameStackSizeInUint64 > 0 {
		// Reserve the stack slots for results.
		for i := 0; i < c.sig.ResultNumInUint64-c.sig.ParamNumInUint64; i++ {
			c.stackPush(unsignedTypeI64)
		}

		// Reserve the stack slots for call frame.
		for i := 0; i < c.callFrameStackSizeInUint64; i++ {
			c.stackPush(unsignedTypeI64)
		}
	}
}

// compiler is in charge of lowering raw Wasm function body to get compilationResult.
// This is created per *wasm.Module and reused for all functions in it to reduce memory allocations.
type compiler struct {
	module                     *wasm.Module
	enabledFeatures            api.CoreFeatures
	callFrameStackSizeInUint64 int
	stack                      []unsignedType
	// stackLenInUint64 is the length of the stack in uint64.
	stackLenInUint64 int
	currentFrameID   uint32
	controlFrames    controlFrames
	unreachableState struct {
		on    bool
		depth int
	}
	pc, currentOpPC uint64
	result          compilationResult

	// body holds the code for the function's body where Wasm instructions are stored.
	body []byte
	// sig is the function type of the target function.
	sig *wasm.FunctionType
	// localTypes holds the target function locals' value types except function params.
	localTypes []wasm.ValueType
	// localIndexToStackHeightInUint64 maps the local index (starting with function params) to the stack height
	// where the local is places. This is the necessary mapping for functions who contain vector type locals.
	localIndexToStackHeightInUint64 []int

	// types hold all the function types in the module where the targe function exists.
	types []wasm.FunctionType
	// funcs holds the type indexes for all declared functions in the module where the target function exists.
	funcs []uint32
	// globals holds the global types for all declared globals in the module where the target function exists.
	globals []wasm.GlobalType

	// needSourceOffset is true if this module requires DWARF based stack trace.
	needSourceOffset bool
	// bodyOffsetInCodeSection is the offset of the body of this function in the original Wasm binary's code section.
	bodyOffsetInCodeSection uint64

	ensureTermination bool
	// Pre-allocated bytes.Reader to be used in various places.
	br             *bytes.Reader
	funcTypeToSigs funcTypeToIRSignatures

	next int
}

//lint:ignore U1000 for debugging only.
func (c *compiler) stackDump() string {
	strs := make([]string, 0, len(c.stack))
	for _, s := range c.stack {
		strs = append(strs, s.String())
	}
	return "[" + strings.Join(strs, ", ") + "]"
}

func (c *compiler) markUnreachable() {
	c.unreachableState.on = true
}

func (c *compiler) resetUnreachable() {
	c.unreachableState.on = false
}

// memoryType is the type of memory in a compiled module.
type memoryType byte

const (
	// memoryTypeNone indicates there is no memory.
	memoryTypeNone memoryType = iota
	// memoryTypeStandard indicates there is a non-shared memory.
	memoryTypeStandard
	// memoryTypeShared indicates there is a shared memory.
	memoryTypeShared
)

type compilationResult struct {
	// Operations holds interpreterir operations compiled from Wasm instructions in a Wasm function.
	Operations []unionOperation

	// IROperationSourceOffsetsInWasmBinary is index-correlated with Operation and maps each operation to the corresponding source instruction's
	// offset in the original WebAssembly binary.
	// Non nil only when the given Wasm module has the DWARF section.
	IROperationSourceOffsetsInWasmBinary []uint64

	// LabelCallers maps label to the number of callers to that label.
	// Here "callers" means that the call-sites which jumps to the label with br, br_if or br_table
	// instructions.
	//
	// Note: zero possible and allowed in wasm. e.g.
	//
	//	(block
	//	  (br 0)
	//	  (block i32.const 1111)
	//	)
	//
	// This example the label corresponding to `(block i32.const 1111)` is never be reached at runtime because `br 0` exits the function before we reach there
	LabelCallers map[label]uint32
	// UsesMemory is true if this function might use memory.
	UsesMemory bool

	// The following fields are per-module values, not per-function.

	// Globals holds all the declarations of globals in the module from which this function is compiled.
	Globals []wasm.GlobalType
	// Functions holds all the declarations of function in the module from which this function is compiled, including itself.
	Functions []wasm.Index
	// Types holds all the types in the module from which this function is compiled.
	Types []wasm.FunctionType
	// Memory indicates the type of memory of the module.
	Memory memoryType
	// HasTable is true if the module from which this function is compiled has table declaration.
	HasTable bool
	// HasDataInstances is true if the module has data instances which might be used by memory.init or data.drop instructions.
	HasDataInstances bool
	// HasDataInstances is true if the module has element instances which might be used by table.init or elem.drop instructions.
	HasElementInstances bool
}

// newCompiler returns the new *compiler for the given parameters.
// Use compiler.Next function to get compilation result per function.
func newCompiler(enabledFeatures api.CoreFeatures, callFrameStackSizeInUint64 int, module *wasm.Module, ensureTermination bool) (*compiler, error) {
	functions, globals, mem, tables, err := module.AllDeclarations()
	if err != nil {
		return nil, err
	}

	hasTable, hasDataInstances, hasElementInstances := len(tables) > 0,
		len(module.DataSection) > 0, len(module.ElementSection) > 0

	var mt memoryType
	switch {
	case mem == nil:
		mt = memoryTypeNone
	case mem.IsShared:
		mt = memoryTypeShared
	default:
		mt = memoryTypeStandard
	}

	types := module.TypeSection

	c := &compiler{
		module:                     module,
		enabledFeatures:            enabledFeatures,
		controlFrames:              controlFrames{},
		callFrameStackSizeInUint64: callFrameStackSizeInUint64,
		result: compilationResult{
			Globals:             globals,
			Functions:           functions,
			Types:               types,
			Memory:              mt,
			HasTable:            hasTable,
			HasDataInstances:    hasDataInstances,
			HasElementInstances: hasElementInstances,
			LabelCallers:        map[label]uint32{},
		},
		globals:           globals,
		funcs:             functions,
		types:             types,
		ensureTermination: ensureTermination,
		br:                bytes.NewReader(nil),
		funcTypeToSigs: funcTypeToIRSignatures{
			indirectCalls: make([]*signature, len(types)),
			directCalls:   make([]*signature, len(types)),
			wasmTypes:     types,
		},
		needSourceOffset: module.DWARFLines != nil,
	}
	return c, nil
}

// Next returns the next compilationResult for this compiler.
func (c *compiler) Next() (*compilationResult, error) {
	funcIndex := c.next
	code := &c.module.CodeSection[funcIndex]
	sig := &c.types[c.module.FunctionSection[funcIndex]]

	// Reset the previous result.
	c.result.Operations = c.result.Operations[:0]
	c.result.IROperationSourceOffsetsInWasmBinary = c.result.IROperationSourceOffsetsInWasmBinary[:0]
	c.result.UsesMemory = false
	// Clears the existing entries in LabelCallers.
	for frameID := uint32(0); frameID <= c.currentFrameID; frameID++ {
		for k := labelKind(0); k < labelKindNum; k++ {
			delete(c.result.LabelCallers, newLabel(k, frameID))
		}
	}
	// Reset the previous states.
	c.pc = 0
	c.currentOpPC = 0
	c.currentFrameID = 0
	c.stackLenInUint64 = 0
	c.unreachableState.on, c.unreachableState.depth = false, 0

	if err := c.compile(sig, code.Body, code.LocalTypes, code.BodyOffsetInCodeSection); err != nil {
		return nil, err
	}
	c.next++
	return &c.result, nil
}

// Compile lowers given function instance into interpreterir operations
// so that the resulting operations can be consumed by the interpreter
// or the compiler compilation engine.
func (c *compiler) compile(sig *wasm.FunctionType, body []byte, localTypes []wasm.ValueType, bodyOffsetInCodeSection uint64) error {
	// Set function specific fields.
	c.body = body
	c.localTypes = localTypes
	c.sig = sig
	c.bodyOffsetInCodeSection = bodyOffsetInCodeSection

	// Reuses the underlying slices.
	c.stack = c.stack[:0]
	c.controlFrames.frames = c.controlFrames.frames[:0]

	c.initializeStack()

	// Emit const expressions for locals.
	// Note that here we don't take function arguments
	// into account, meaning that callers must push
	// arguments before entering into the function body.
	for _, t := range c.localTypes {
		c.emitDefaultValue(t)
	}

	// Insert the function control frame.
	c.controlFrames.push(controlFrame{
		frameID:   c.nextFrameID(),
		blockType: c.sig,
		kind:      controlFrameKindFunction,
	})

	// Now, enter the function body.
	for !c.controlFrames.empty() && c.pc < uint64(len(c.body)) {
		if err := c.handleInstruction(); err != nil {
			return fmt.Errorf("handling instruction: %w", err)
		}
	}
	return nil
}

// Translate the current Wasm instruction to interpreterir's operations,
// and emit the results into c.results.
func (c *compiler) handleInstruction() error {
	op := c.body[c.pc]
	c.currentOpPC = c.pc
	if false {
		var instName string
		if op == wasm.OpcodeVecPrefix {
			instName = wasm.VectorInstructionName(c.body[c.pc+1])
		} else if op == wasm.OpcodeAtomicPrefix {
			instName = wasm.AtomicInstructionName(c.body[c.pc+1])
		} else if op == wasm.OpcodeMiscPrefix {
			instName = wasm.MiscInstructionName(c.body[c.pc+1])
		} else {
			instName = wasm.InstructionName(op)
		}
		fmt.Printf("handling %s, unreachable_state(on=%v,depth=%d), stack=%v\n",
			instName, c.unreachableState.on, c.unreachableState.depth, c.stack,
		)
	}

	var peekValueType unsignedType
	if len(c.stack) > 0 {
		peekValueType = c.stackPeek()
	}

	// Modify the stack according the current instruction.
	// Note that some instructions will read "index" in
	// applyToStack and advance c.pc inside the function.
	index, err := c.applyToStack(op)
	if err != nil {
		return fmt.Errorf("apply stack failed for %s: %w", wasm.InstructionName(op), err)
	}
	// Now we handle each instruction, and
	// emit the corresponding interpreterir operations to the results.
operatorSwitch:
	switch op {
	case wasm.OpcodeUnreachable:
		c.emit(newOperationUnreachable())
		c.markUnreachable()
	case wasm.OpcodeNop:
		// Nop is noop!
	case wasm.OpcodeBlock:
		c.br.Reset(c.body[c.pc+1:])
		bt, num, err := wasm.DecodeBlockType(c.types, c.br, c.enabledFeatures)
		if err != nil {
			return fmt.Errorf("reading block type for block instruction: %w", err)
		}
		c.pc += num

		if c.unreachableState.on {
			// If it is currently in unreachable,
			// just remove the entire block.
			c.unreachableState.depth++
			break operatorSwitch
		}

		// Create a new frame -- entering this block.
		frame := controlFrame{
			frameID:                            c.nextFrameID(),
			originalStackLenWithoutParam:       len(c.stack) - len(bt.Params),
			originalStackLenWithoutParamUint64: c.stackLenInUint64 - bt.ParamNumInUint64,
			kind:                               controlFrameKindBlockWithoutContinuationLabel,
			blockType:                          bt,
		}
		c.controlFrames.push(frame)

	case wasm.OpcodeLoop:
		c.br.Reset(c.body[c.pc+1:])
		bt, num, err := wasm.DecodeBlockType(c.types, c.br, c.enabledFeatures)
		if err != nil {
			return fmt.Errorf("reading block type for loop instruction: %w", err)
		}
		c.pc += num

		if c.unreachableState.on {
			// If it is currently in unreachable,
			// just remove the entire block.
			c.unreachableState.depth++
			break operatorSwitch
		}

		// Create a new frame -- entering loop.
		frame := controlFrame{
			frameID:                            c.nextFrameID(),
			originalStackLenWithoutParam:       len(c.stack) - len(bt.Params),
			originalStackLenWithoutParamUint64: c.stackLenInUint64 - bt.ParamNumInUint64,
			kind:                               controlFrameKindLoop,
			blockType:                          bt,
		}
		c.controlFrames.push(frame)

		// Prep labels for inside and the continuation of this loop.
		loopLabel := newLabel(labelKindHeader, frame.frameID)
		c.result.LabelCallers[loopLabel]++

		// Emit the branch operation to enter inside the loop.
		c.emit(newOperationBr(loopLabel))
		c.emit(newOperationLabel(loopLabel))

		// Insert the exit code check on the loop header, which is the only necessary point in the function body
		// to prevent infinite loop.
		//
		// Note that this is a little aggressive: this checks the exit code regardless the loop header is actually
		// the loop. In other words, this checks even when no br/br_if/br_table instructions jumping to this loop
		// exist. However, in reality, that shouldn't be an issue since such "noop" loop header will highly likely be
		// optimized out by almost all guest language compilers which have the control flow optimization passes.
		if c.ensureTermination {
			c.emit(newOperationBuiltinFunctionCheckExitCode())
		}
	case wasm.OpcodeIf:
		c.br.Reset(c.body[c.pc+1:])
		bt, num, err := wasm.DecodeBlockType(c.types, c.br, c.enabledFeatures)
		if err != nil {
			return fmt.Errorf("reading block type for if instruction: %w", err)
		}
		c.pc += num

		if c.unreachableState.on {
			// If it is currently in unreachable,
			// just remove the entire block.
			c.unreachableState.depth++
			break operatorSwitch
		}

		// Create a new frame -- entering if.
		frame := controlFrame{
			frameID:                            c.nextFrameID(),
			originalStackLenWithoutParam:       len(c.stack) - len(bt.Params),
			originalStackLenWithoutParamUint64: c.stackLenInUint64 - bt.ParamNumInUint64,
			// Note this will be set to controlFrameKindIfWithElse
			// when else opcode found later.
			kind:      controlFrameKindIfWithoutElse,
			blockType: bt,
		}
		c.controlFrames.push(frame)

		// Prep labels for if and else of this if.
		thenLabel := newLabel(labelKindHeader, frame.frameID)
		elseLabel := newLabel(labelKindElse, frame.frameID)
		c.result.LabelCallers[thenLabel]++
		c.result.LabelCallers[elseLabel]++

		// Emit the branch operation to enter the then block.
		c.emit(newOperationBrIf(thenLabel, elseLabel, nopinclusiveRange))
		c.emit(newOperationLabel(thenLabel))
	case wasm.OpcodeElse:
		frame := c.controlFrames.top()
		if c.unreachableState.on && c.unreachableState.depth > 0 {
			// If it is currently in unreachable, and the nested if,
			// just remove the entire else block.
			break operatorSwitch
		} else if c.unreachableState.on {
			// If it is currently in unreachable, and the non-nested if,
			// reset the stack so we can correctly handle the else block.
			top := c.controlFrames.top()
			c.stackSwitchAt(top)
			top.kind = controlFrameKindIfWithElse

			// Re-push the parameters to the if block so that else block can use them.
			for _, t := range frame.blockType.Params {
				c.stackPush(wasmValueTypeTounsignedType(t))
			}

			// We are no longer unreachable in else frame,
			// so emit the correct label, and reset the unreachable state.
			elseLabel := newLabel(labelKindElse, frame.frameID)
			c.resetUnreachable()
			c.emit(
				newOperationLabel(elseLabel),
			)
			break operatorSwitch
		}

		// Change the Kind of this If block, indicating that
		// the if has else block.
		frame.kind = controlFrameKindIfWithElse

		// We need to reset the stack so that
		// the values pushed inside the then block
		// do not affect the else block.
		dropOp := newOperationDrop(c.getFrameDropRange(frame, false))

		// Reset the stack manipulated by the then block, and re-push the block param types to the stack.

		c.stackSwitchAt(frame)
		for _, t := range frame.blockType.Params {
			c.stackPush(wasmValueTypeTounsignedType(t))
		}

		// Prep labels for else and the continuation of this if block.
		elseLabel := newLabel(labelKindElse, frame.frameID)
		continuationLabel := newLabel(labelKindContinuation, frame.frameID)
		c.result.LabelCallers[continuationLabel]++

		// Emit the instructions for exiting the if loop,
		// and then the initiation of else block.
		c.emit(dropOp)
		// Jump to the continuation of this block.
		c.emit(newOperationBr(continuationLabel))
		// Initiate the else block.
		c.emit(newOperationLabel(elseLabel))
	case wasm.OpcodeEnd:
		if c.unreachableState.on && c.unreachableState.depth > 0 {
			c.unreachableState.depth--
			break operatorSwitch
		} else if c.unreachableState.on {
			c.resetUnreachable()

			frame := c.controlFrames.pop()
			if c.controlFrames.empty() {
				return nil
			}

			c.stackSwitchAt(frame)
			for _, t := range frame.blockType.Results {
				c.stackPush(wasmValueTypeTounsignedType(t))
			}

			continuationLabel := newLabel(labelKindContinuation, frame.frameID)
			if frame.kind == controlFrameKindIfWithoutElse {
				// Emit the else label.
				elseLabel := newLabel(labelKindElse, frame.frameID)
				c.result.LabelCallers[continuationLabel]++
				c.emit(newOperationLabel(elseLabel))
				c.emit(newOperationBr(continuationLabel))
				c.emit(newOperationLabel(continuationLabel))
			} else {
				c.emit(
					newOperationLabel(continuationLabel),
				)
			}

			break operatorSwitch
		}

		frame := c.controlFrames.pop()

		// We need to reset the stack so that
		// the values pushed inside the block.
		dropOp := newOperationDrop(c.getFrameDropRange(frame, true))
		c.stackSwitchAt(frame)

		// Push the result types onto the stack.
		for _, t := range frame.blockType.Results {
			c.stackPush(wasmValueTypeTounsignedType(t))
		}

		// Emit the instructions according to the Kind of the current control frame.
		switch frame.kind {
		case controlFrameKindFunction:
			if !c.controlFrames.empty() {
				// Should never happen. If so, there's a bug in the translation.
				panic("bug: found more function control frames")
			}
			// Return from function.
			c.emit(dropOp)
			c.emit(newOperationBr(newLabel(labelKindReturn, 0)))
		case controlFrameKindIfWithoutElse:
			// This case we have to emit "empty" else label.
			elseLabel := newLabel(labelKindElse, frame.frameID)
			continuationLabel := newLabel(labelKindContinuation, frame.frameID)
			c.result.LabelCallers[continuationLabel] += 2
			c.emit(dropOp)
			c.emit(newOperationBr(continuationLabel))
			// Emit the else which soon branches into the continuation.
			c.emit(newOperationLabel(elseLabel))
			c.emit(newOperationBr(continuationLabel))
			// Initiate the continuation.
			c.emit(newOperationLabel(continuationLabel))
		case controlFrameKindBlockWithContinuationLabel,
			controlFrameKindIfWithElse:
			continuationLabel := newLabel(labelKindContinuation, frame.frameID)
			c.result.LabelCallers[continuationLabel]++
			c.emit(dropOp)
			c.emit(newOperationBr(continuationLabel))
			c.emit(newOperationLabel(continuationLabel))
		case controlFrameKindLoop, controlFrameKindBlockWithoutContinuationLabel:
			c.emit(
				dropOp,
			)
		default:
			// Should never happen. If so, there's a bug in the translation.
			panic(fmt.Errorf("bug: invalid control frame Kind: 0x%x", frame.kind))
		}

	case wasm.OpcodeBr:
		targetIndex, n, err := leb128.LoadUint32(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("read the target for br_if: %w", err)
		}
		c.pc += n

		if c.unreachableState.on {
			// If it is currently in unreachable, br is no-op.
			break operatorSwitch
		}

		targetFrame := c.controlFrames.get(int(targetIndex))
		targetFrame.ensureContinuation()
		dropOp := newOperationDrop(c.getFrameDropRange(targetFrame, false))
		targetID := targetFrame.asLabel()
		c.result.LabelCallers[targetID]++
		c.emit(dropOp)
		c.emit(newOperationBr(targetID))
		// Br operation is stack-polymorphic, and mark the state as unreachable.
		// That means subsequent instructions in the current control frame are "unreachable"
		// and can be safely removed.
		c.markUnreachable()
	case wasm.OpcodeBrIf:
		targetIndex, n, err := leb128.LoadUint32(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("read the target for br_if: %w", err)
		}
		c.pc += n

		if c.unreachableState.on {
			// If it is currently in unreachable, br-if is no-op.
			break operatorSwitch
		}

		targetFrame := c.controlFrames.get(int(targetIndex))
		targetFrame.ensureContinuation()
		drop := c.getFrameDropRange(targetFrame, false)
		target := targetFrame.asLabel()
		c.result.LabelCallers[target]++

		continuationLabel := newLabel(labelKindHeader, c.nextFrameID())
		c.result.LabelCallers[continuationLabel]++
		c.emit(newOperationBrIf(target, continuationLabel, drop))
		// Start emitting else block operations.
		c.emit(newOperationLabel(continuationLabel))
	case wasm.OpcodeBrTable:
		c.br.Reset(c.body[c.pc+1:])
		r := c.br
		numTargets, n, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("error reading number of targets in br_table: %w", err)
		}
		c.pc += n

		if c.unreachableState.on {
			// If it is currently in unreachable, br_table is no-op.
			// But before proceeding to the next instruction, we must advance the pc
			// according to the number of br_table targets.
			for i := uint32(0); i <= numTargets; i++ { // inclusive as we also need to read the index of default target.
				_, n, err := leb128.DecodeUint32(r)
				if err != nil {
					return fmt.Errorf("error reading target %d in br_table: %w", i, err)
				}
				c.pc += n
			}
			break operatorSwitch
		}

		// Read the branch targets.
		s := numTargets * 2
		targetLabels := make([]uint64, 2+s) // (label, inclusiveRange) * (default+numTargets)
		for i := uint32(0); i < s; i += 2 {
			l, n, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("error reading target %d in br_table: %w", i, err)
			}
			c.pc += n
			targetFrame := c.controlFrames.get(int(l))
			targetFrame.ensureContinuation()
			drop := c.getFrameDropRange(targetFrame, false)
			targetLabel := targetFrame.asLabel()
			targetLabels[i] = uint64(targetLabel)
			targetLabels[i+1] = drop.AsU64()
			c.result.LabelCallers[targetLabel]++
		}

		// Prep default target control frame.
		l, n, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("error reading default target of br_table: %w", err)
		}
		c.pc += n
		defaultTargetFrame := c.controlFrames.get(int(l))
		defaultTargetFrame.ensureContinuation()
		defaultTargetDrop := c.getFrameDropRange(defaultTargetFrame, false)
		defaultLabel := defaultTargetFrame.asLabel()
		c.result.LabelCallers[defaultLabel]++
		targetLabels[s] = uint64(defaultLabel)
		targetLabels[s+1] = defaultTargetDrop.AsU64()
		c.emit(newOperationBrTable(targetLabels))

		// br_table operation is stack-polymorphic, and mark the state as unreachable.
		// That means subsequent instructions in the current control frame are "unreachable"
		// and can be safely removed.
		c.markUnreachable()
	case wasm.OpcodeReturn:
		functionFrame := c.controlFrames.functionFrame()
		dropOp := newOperationDrop(c.getFrameDropRange(functionFrame, false))

		// Cleanup the stack and then jmp to function frame's continuation (meaning return).
		c.emit(dropOp)
		c.emit(newOperationBr(functionFrame.asLabel()))

		// Return operation is stack-polymorphic, and mark the state as unreachable.
		// That means subsequent instructions in the current control frame are "unreachable"
		// and can be safely removed.
		c.markUnreachable()
	case wasm.OpcodeCall:
		c.emit(
			newOperationCall(index),
		)
	case wasm.OpcodeCallIndirect:
		typeIndex := index
		tableIndex, n, err := leb128.LoadUint32(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("read target for br_table: %w", err)
		}
		c.pc += n
		c.emit(
			newOperationCallIndirect(typeIndex, tableIndex),
		)
	case wasm.OpcodeDrop:
		r := inclusiveRange{Start: 0, End: 0}
		if peekValueType == unsignedTypeV128 {
			// inclusiveRange is the range in uint64 representation, so dropping a vector value on top
			// should be translated as drop [0..1] inclusively.
			r.End++
		}
		c.emit(newOperationDrop(r))
	case wasm.OpcodeSelect:
		// If it is on the unreachable state, ignore the instruction.
		if c.unreachableState.on {
			break operatorSwitch
		}
		isTargetVector := c.stackPeek() == unsignedTypeV128
		c.emit(
			newOperationSelect(isTargetVector),
		)
	case wasm.OpcodeTypedSelect:
		// Skips two bytes: vector size fixed to 1, and the value type for select.
		c.pc += 2
		// If it is on the unreachable state, ignore the instruction.
		if c.unreachableState.on {
			break operatorSwitch
		}
		// Typed select is semantically equivalent to select at runtime.
		isTargetVector := c.stackPeek() == unsignedTypeV128
		c.emit(
			newOperationSelect(isTargetVector),
		)
	case wasm.OpcodeLocalGet:
		depth := c.localDepth(index)
		if isVector := c.localType(index) == wasm.ValueTypeV128; !isVector {
			c.emit(
				// -1 because we already manipulated the stack before
				// called localDepth ^^.
				newOperationPick(depth-1, isVector),
			)
		} else {
			c.emit(
				// -2 because we already manipulated the stack before
				// called localDepth ^^.
				newOperationPick(depth-2, isVector),
			)
		}
	case wasm.OpcodeLocalSet:
		depth := c.localDepth(index)

		isVector := c.localType(index) == wasm.ValueTypeV128
		if isVector {
			c.emit(
				// +2 because we already popped the operands for this operation from the c.stack before
				// called localDepth ^^,
				newOperationSet(depth+2, isVector),
			)
		} else {
			c.emit(
				// +1 because we already popped the operands for this operation from the c.stack before
				// called localDepth ^^,
				newOperationSet(depth+1, isVector),
			)
		}
	case wasm.OpcodeLocalTee:
		depth := c.localDepth(index)
		isVector := c.localType(index) == wasm.ValueTypeV128
		if isVector {
			c.emit(newOperationPick(1, isVector))
			c.emit(newOperationSet(depth+2, isVector))
		} else {
			c.emit(
				newOperationPick(0, isVector))
			c.emit(newOperationSet(depth+1, isVector))
		}
	case wasm.OpcodeGlobalGet:
		c.emit(
			newOperationGlobalGet(index),
		)
	case wasm.OpcodeGlobalSet:
		c.emit(
			newOperationGlobalSet(index),
		)
	case wasm.OpcodeI32Load:
		imm, err := c.readMemoryArg(wasm.OpcodeI32LoadName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad(unsignedTypeI32, imm))
	case wasm.OpcodeI64Load:
		imm, err := c.readMemoryArg(wasm.OpcodeI64LoadName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad(unsignedTypeI64, imm))
	case wasm.OpcodeF32Load:
		imm, err := c.readMemoryArg(wasm.OpcodeF32LoadName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad(unsignedTypeF32, imm))
	case wasm.OpcodeF64Load:
		imm, err := c.readMemoryArg(wasm.OpcodeF64LoadName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad(unsignedTypeF64, imm))
	case wasm.OpcodeI32Load8S:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Load8SName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad8(signedInt32, imm))
	case wasm.OpcodeI32Load8U:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Load8UName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad8(signedUint32, imm))
	case wasm.OpcodeI32Load16S:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Load16SName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad16(signedInt32, imm))
	case wasm.OpcodeI32Load16U:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Load16UName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad16(signedUint32, imm))
	case wasm.OpcodeI64Load8S:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load8SName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad8(signedInt64, imm))
	case wasm.OpcodeI64Load8U:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load8UName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad8(signedUint64, imm))
	case wasm.OpcodeI64Load16S:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load16SName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad16(signedInt64, imm))
	case wasm.OpcodeI64Load16U:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load16UName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad16(signedUint64, imm))
	case wasm.OpcodeI64Load32S:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load32SName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad32(true, imm))
	case wasm.OpcodeI64Load32U:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Load32UName)
		if err != nil {
			return err
		}
		c.emit(newOperationLoad32(false, imm))
	case wasm.OpcodeI32Store:
		imm, err := c.readMemoryArg(wasm.OpcodeI32StoreName)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore(unsignedTypeI32, imm),
		)
	case wasm.OpcodeI64Store:
		imm, err := c.readMemoryArg(wasm.OpcodeI64StoreName)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore(unsignedTypeI64, imm),
		)
	case wasm.OpcodeF32Store:
		imm, err := c.readMemoryArg(wasm.OpcodeF32StoreName)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore(unsignedTypeF32, imm),
		)
	case wasm.OpcodeF64Store:
		imm, err := c.readMemoryArg(wasm.OpcodeF64StoreName)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore(unsignedTypeF64, imm),
		)
	case wasm.OpcodeI32Store8:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Store8Name)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore8(imm),
		)
	case wasm.OpcodeI32Store16:
		imm, err := c.readMemoryArg(wasm.OpcodeI32Store16Name)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore16(imm),
		)
	case wasm.OpcodeI64Store8:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Store8Name)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore8(imm),
		)
	case wasm.OpcodeI64Store16:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Store16Name)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore16(imm),
		)
	case wasm.OpcodeI64Store32:
		imm, err := c.readMemoryArg(wasm.OpcodeI64Store32Name)
		if err != nil {
			return err
		}
		c.emit(
			newOperationStore32(imm),
		)
	case wasm.OpcodeMemorySize:
		c.result.UsesMemory = true
		c.pc++ // Skip the reserved one byte.
		c.emit(
			newOperationMemorySize(),
		)
	case wasm.OpcodeMemoryGrow:
		c.result.UsesMemory = true
		c.pc++ // Skip the reserved one byte.
		c.emit(
			newOperationMemoryGrow(),
		)
	case wasm.OpcodeI32Const:
		val, num, err := leb128.LoadInt32(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("reading i32.const value: %v", err)
		}
		c.pc += num
		c.emit(
			newOperationConstI32(uint32(val)),
		)
	case wasm.OpcodeI64Const:
		val, num, err := leb128.LoadInt64(c.body[c.pc+1:])
		if err != nil {
			return fmt.Errorf("reading i64.const value: %v", err)
		}
		c.pc += num
		c.emit(
			newOperationConstI64(uint64(val)),
		)
	case wasm.OpcodeF32Const:
		v := math.Float32frombits(binary.LittleEndian.Uint32(c.body[c.pc+1:]))
		c.pc += 4
		c.emit(
			newOperationConstF32(v),
		)
	case wasm.OpcodeF64Const:
		v := math.Float64frombits(binary.LittleEndian.Uint64(c.body[c.pc+1:]))
		c.pc += 8
		c.emit(
			newOperationConstF64(v),
		)
	case wasm.OpcodeI32Eqz:
		c.emit(
			newOperationEqz(unsignedInt32),
		)
	case wasm.OpcodeI32Eq:
		c.emit(
			newOperationEq(unsignedTypeI32),
		)
	case wasm.OpcodeI32Ne:
		c.emit(
			newOperationNe(unsignedTypeI32),
		)
	case wasm.OpcodeI32LtS:
		c.emit(
			newOperationLt(signedTypeInt32),
		)
	case wasm.OpcodeI32LtU:
		c.emit(
			newOperationLt(signedTypeUint32),
		)
	case wasm.OpcodeI32GtS:
		c.emit(
			newOperationGt(signedTypeInt32),
		)
	case wasm.OpcodeI32GtU:
		c.emit(
			newOperationGt(signedTypeUint32),
		)
	case wasm.OpcodeI32LeS:
		c.emit(
			newOperationLe(signedTypeInt32),
		)
	case wasm.OpcodeI32LeU:
		c.emit(
			newOperationLe(signedTypeUint32),
		)
	case wasm.OpcodeI32GeS:
		c.emit(
			newOperationGe(signedTypeInt32),
		)
	case wasm.OpcodeI32GeU:
		c.emit(
			newOperationGe(signedTypeUint32),
		)
	case wasm.OpcodeI64Eqz:
		c.emit(
			newOperationEqz(unsignedInt64),
		)
	case wasm.OpcodeI64Eq:
		c.emit(
			newOperationEq(unsignedTypeI64),
		)
	case wasm.OpcodeI64Ne:
		c.emit(
			newOperationNe(unsignedTypeI64),
		)
	case wasm.OpcodeI64LtS:
		c.emit(
			newOperationLt(signedTypeInt64),
		)
	case wasm.OpcodeI64LtU:
		c.emit(
			newOperationLt(signedTypeUint64),
		)
	case wasm.OpcodeI64GtS:
		c.emit(
			newOperationGt(signedTypeInt64),
		)
	case wasm.OpcodeI64GtU:
		c.emit(
			newOperationGt(signedTypeUint64),
		)
	case wasm.OpcodeI64LeS:
		c.emit(
			newOperationLe(signedTypeInt64),
		)
	case wasm.OpcodeI64LeU:
		c.emit(
			newOperationLe(signedTypeUint64),
		)
	case wasm.OpcodeI64GeS:
		c.emit(
			newOperationGe(signedTypeInt64),
		)
	case wasm.OpcodeI64GeU:
		c.emit(
			newOperationGe(signedTypeUint64),
		)
	case wasm.OpcodeF32Eq:
		c.emit(
			newOperationEq(unsignedTypeF32),
		)
	case wasm.OpcodeF32Ne:
		c.emit(
			newOperationNe(unsignedTypeF32),
		)
	case wasm.OpcodeF32Lt:
		c.emit(
			newOperationLt(signedTypeFloat32),
		)
	case wasm.OpcodeF32Gt:
		c.emit(
			newOperationGt(signedTypeFloat32),
		)
	case wasm.OpcodeF32Le:
		c.emit(
			newOperationLe(signedTypeFloat32),
		)
	case wasm.OpcodeF32Ge:
		c.emit(
			newOperationGe(signedTypeFloat32),
		)
	case wasm.OpcodeF64Eq:
		c.emit(
			newOperationEq(unsignedTypeF64),
		)
	case wasm.OpcodeF64Ne:
		c.emit(
			newOperationNe(unsignedTypeF64),
		)
	case wasm.OpcodeF64Lt:
		c.emit(
			newOperationLt(signedTypeFloat64),
		)
	case wasm.OpcodeF64Gt:
		c.emit(
			newOperationGt(signedTypeFloat64),
		)
	case wasm.OpcodeF64Le:
		c.emit(
			newOperationLe(signedTypeFloat64),
		)
	case wasm.OpcodeF64Ge:
		c.emit(
			newOperationGe(signedTypeFloat64),
		)
	case wasm.OpcodeI32Clz:
		c.emit(
			newOperationClz(unsignedInt32),
		)
	case wasm.OpcodeI32Ctz:
		c.emit(
			newOperationCtz(unsignedInt32),
		)
	case wasm.OpcodeI32Popcnt:
		c.emit(
			newOperationPopcnt(unsignedInt32),
		)
	case wasm.OpcodeI32Add:
		c.emit(
			newOperationAdd(unsignedTypeI32),
		)
	case wasm.OpcodeI32Sub:
		c.emit(
			newOperationSub(unsignedTypeI32),
		)
	case wasm.OpcodeI32Mul:
		c.emit(
			newOperationMul(unsignedTypeI32),
		)
	case wasm.OpcodeI32DivS:
		c.emit(
			newOperationDiv(signedTypeInt32),
		)
	case wasm.OpcodeI32DivU:
		c.emit(
			newOperationDiv(signedTypeUint32),
		)
	case wasm.OpcodeI32RemS:
		c.emit(
			newOperationRem(signedInt32),
		)
	case wasm.OpcodeI32RemU:
		c.emit(
			newOperationRem(signedUint32),
		)
	case wasm.OpcodeI32And:
		c.emit(
			newOperationAnd(unsignedInt32),
		)
	case wasm.OpcodeI32Or:
		c.emit(
			newOperationOr(unsignedInt32),
		)
	case wasm.OpcodeI32Xor:
		c.emit(
			newOperationXor(unsignedInt64),
		)
	case wasm.OpcodeI32Shl:
		c.emit(
			newOperationShl(unsignedInt32),
		)
	case wasm.OpcodeI32ShrS:
		c.emit(
			newOperationShr(signedInt32),
		)
	case wasm.OpcodeI32ShrU:
		c.emit(
			newOperationShr(signedUint32),
		)
	case wasm.OpcodeI32Rotl:
		c.emit(
			newOperationRotl(unsignedInt32),
		)
	case wasm.OpcodeI32Rotr:
		c.emit(
			newOperationRotr(unsignedInt32),
		)
	case wasm.OpcodeI64Clz:
		c.emit(
			newOperationClz(unsignedInt64),
		)
	case wasm.OpcodeI64Ctz:
		c.emit(
			newOperationCtz(unsignedInt64),
		)
	case wasm.OpcodeI64Popcnt:
		c.emit(
			newOperationPopcnt(unsignedInt64),
		)
	case wasm.OpcodeI64Add:
		c.emit(
			newOperationAdd(unsignedTypeI64),
		)
	case wasm.OpcodeI64Sub:
		c.emit(
			newOperationSub(unsignedTypeI64),
		)
	case wasm.OpcodeI64Mul:
		c.emit(
			newOperationMul(unsignedTypeI64),
		)
	case wasm.OpcodeI64DivS:
		c.emit(
			newOperationDiv(signedTypeInt64),
		)
	case wasm.OpcodeI64DivU:
		c.emit(
			newOperationDiv(signedTypeUint64),
		)
	case wasm.OpcodeI64RemS:
		c.emit(
			newOperationRem(signedInt64),
		)
	case wasm.OpcodeI64RemU:
		c.emit(
			newOperationRem(signedUint64),
		)
	case wasm.OpcodeI64And:
		c.emit(
			newOperationAnd(unsignedInt64),
		)
	case wasm.OpcodeI64Or:
		c.emit(
			newOperationOr(unsignedInt64),
		)
	case wasm.OpcodeI64Xor:
		c.emit(
			newOperationXor(unsignedInt64),
		)
	case wasm.OpcodeI64Shl:
		c.emit(
			newOperationShl(unsignedInt64),
		)
	case wasm.OpcodeI64ShrS:
		c.emit(
			newOperationShr(signedInt64),
		)
	case wasm.OpcodeI64ShrU:
		c.emit(
			newOperationShr(signedUint64),
		)
	case wasm.OpcodeI64Rotl:
		c.emit(
			newOperationRotl(unsignedInt64),
		)
	case wasm.OpcodeI64Rotr:
		c.emit(
			newOperationRotr(unsignedInt64),
		)
	case wasm.OpcodeF32Abs:
		c.emit(
			newOperationAbs(f32),
		)
	case wasm.OpcodeF32Neg:
		c.emit(
			newOperationNeg(f32),
		)
	case wasm.OpcodeF32Ceil:
		c.emit(
			newOperationCeil(f32),
		)
	case wasm.OpcodeF32Floor:
		c.emit(
			newOperationFloor(f32),
		)
	case wasm.OpcodeF32Trunc:
		c.emit(
			newOperationTrunc(f32),
		)
	case wasm.OpcodeF32Nearest:
		c.emit(
			newOperationNearest(f32),
		)
	case wasm.OpcodeF32Sqrt:
		c.emit(
			newOperationSqrt(f32),
		)
	case wasm.OpcodeF32Add:
		c.emit(
			newOperationAdd(unsignedTypeF32),
		)
	case wasm.OpcodeF32Sub:
		c.emit(
			newOperationSub(unsignedTypeF32),
		)
	case wasm.OpcodeF32Mul:
		c.emit(
			newOperationMul(unsignedTypeF32),
		)
	case wasm.OpcodeF32Div:
		c.emit(
			newOperationDiv(signedTypeFloat32),
		)
	case wasm.OpcodeF32Min:
		c.emit(
			newOperationMin(f32),
		)
	case wasm.OpcodeF32Max:
		c.emit(
			newOperationMax(f32),
		)
	case wasm.OpcodeF32Copysign:
		c.emit(
			newOperationCopysign(f32),
		)
	case wasm.OpcodeF64Abs:
		c.emit(
			newOperationAbs(f64),
		)
	case wasm.OpcodeF64Neg:
		c.emit(
			newOperationNeg(f64),
		)
	case wasm.OpcodeF64Ceil:
		c.emit(
			newOperationCeil(f64),
		)
	case wasm.OpcodeF64Floor:
		c.emit(
			newOperationFloor(f64),
		)
	case wasm.OpcodeF64Trunc:
		c.emit(
			newOperationTrunc(f64),
		)
	case wasm.OpcodeF64Nearest:
		c.emit(
			newOperationNearest(f64),
		)
	case wasm.OpcodeF64Sqrt:
		c.emit(
			newOperationSqrt(f64),
		)
	case wasm.OpcodeF64Add:
		c.emit(
			newOperationAdd(unsignedTypeF64),
		)
	case wasm.OpcodeF64Sub:
		c.emit(
			newOperationSub(unsignedTypeF64),
		)
	case wasm.OpcodeF64Mul:
		c.emit(
			newOperationMul(unsignedTypeF64),
		)
	case wasm.OpcodeF64Div:
		c.emit(
			newOperationDiv(signedTypeFloat64),
		)
	case wasm.OpcodeF64Min:
		c.emit(
			newOperationMin(f64),
		)
	case wasm.OpcodeF64Max:
		c.emit(
			newOperationMax(f64),
		)
	case wasm.OpcodeF64Copysign:
		c.emit(
			newOperationCopysign(f64),
		)
	case wasm.OpcodeI32WrapI64:
		c.emit(
			newOperationI32WrapFromI64(),
		)
	case wasm.OpcodeI32TruncF32S:
		c.emit(
			newOperationITruncFromF(f32, signedInt32, false),
		)
	case wasm.OpcodeI32TruncF32U:
		c.emit(
			newOperationITruncFromF(f32, signedUint32, false),
		)
	case wasm.OpcodeI32TruncF64S:
		c.emit(
			newOperationITruncFromF(f64, signedInt32, false),
		)
	case wasm.OpcodeI32TruncF64U:
		c.emit(
			newOperationITruncFromF(f64, signedUint32, false),
		)
	case wasm.OpcodeI64ExtendI32S:
		c.emit(
			newOperationExtend(true),
		)
	case wasm.OpcodeI64ExtendI32U:
		c.emit(
			newOperationExtend(false),
		)
	case wasm.OpcodeI64TruncF32S:
		c.emit(
			newOperationITruncFromF(f32, signedInt64, false),
		)
	case wasm.OpcodeI64TruncF32U:
		c.emit(
			newOperationITruncFromF(f32, signedUint64, false),
		)
	case wasm.OpcodeI64TruncF64S:
		c.emit(
			newOperationITruncFromF(f64, signedInt64, false),
		)
	case wasm.OpcodeI64TruncF64U:
		c.emit(
			newOperationITruncFromF(f64, signedUint64, false),
		)
	case wasm.OpcodeF32ConvertI32S:
		c.emit(
			newOperationFConvertFromI(signedInt32, f32),
		)
	case wasm.OpcodeF32ConvertI32U:
		c.emit(
			newOperationFConvertFromI(signedUint32, f32),
		)
	case wasm.OpcodeF32ConvertI64S:
		c.emit(
			newOperationFConvertFromI(signedInt64, f32),
		)
	case wasm.OpcodeF32ConvertI64U:
		c.emit(
			newOperationFConvertFromI(signedUint64, f32),
		)
	case wasm.OpcodeF32DemoteF64:
		c.emit(
			newOperationF32DemoteFromF64(),
		)
	case wasm.OpcodeF64ConvertI32S:
		c.emit(
			newOperationFConvertFromI(signedInt32, f64),
		)
	case wasm.OpcodeF64ConvertI32U:
		c.emit(
			newOperationFConvertFromI(signedUint32, f64),
		)
	case wasm.OpcodeF64ConvertI64S:
		c.emit(
			newOperationFConvertFromI(signedInt64, f64),
		)
	case wasm.OpcodeF64ConvertI64U:
		c.emit(
			newOperationFConvertFromI(signedUint64, f64),
		)
	case wasm.OpcodeF64PromoteF32:
		c.emit(
			newOperationF64PromoteFromF32(),
		)
	case wasm.OpcodeI32ReinterpretF32:
		c.emit(
			newOperationI32ReinterpretFromF32(),
		)
	case wasm.OpcodeI64ReinterpretF64:
		c.emit(
			newOperationI64ReinterpretFromF64(),
		)
	case wasm.OpcodeF32ReinterpretI32:
		c.emit(
			newOperationF32ReinterpretFromI32(),
		)
	case wasm.OpcodeF64ReinterpretI64:
		c.emit(
			newOperationF64ReinterpretFromI64(),
		)
	case wasm.OpcodeI32Extend8S:
		c.emit(
			newOperationSignExtend32From8(),
		)
	case wasm.OpcodeI32Extend16S:
		c.emit(
			newOperationSignExtend32From16(),
		)
	case wasm.OpcodeI64Extend8S:
		c.emit(
			newOperationSignExtend64From8(),
		)
	case wasm.OpcodeI64Extend16S:
		c.emit(
			newOperationSignExtend64From16(),
		)
	case wasm.OpcodeI64Extend32S:
		c.emit(
			newOperationSignExtend64From32(),
		)
	case wasm.OpcodeRefFunc:
		c.pc++
		index, num, err := leb128.LoadUint32(c.body[c.pc:])
		if err != nil {
			return fmt.Errorf("failed to read function index for ref.func: %v", err)
		}
		c.pc += num - 1
		c.emit(
			newOperationRefFunc(index),
		)
	case wasm.OpcodeRefNull:
		c.pc++ // Skip the type of reftype as every ref value is opaque pointer.
		c.emit(
			newOperationConstI64(0),
		)
	case wasm.OpcodeRefIsNull:
		// Simply compare the opaque pointer (i64) with zero.
		c.emit(
			newOperationEqz(unsignedInt64),
		)
	case wasm.OpcodeTableGet:
		c.pc++
		tableIndex, num, err := leb128.LoadUint32(c.body[c.pc:])
		if err != nil {
			return fmt.Errorf("failed to read function index for table.get: %v", err)
		}
		c.pc += num - 1
		c.emit(
			newOperationTableGet(tableIndex),
		)
	case wasm.OpcodeTableSet:
		c.pc++
		tableIndex, num, err := leb128.LoadUint32(c.body[c.pc:])
		if err != nil {
			return fmt.Errorf("failed to read function index for table.set: %v", err)
		}
		c.pc += num - 1
		c.emit(
			newOperationTableSet(tableIndex),
		)
	case wasm.OpcodeMiscPrefix:
		c.pc++
		// A misc opcode is encoded as an unsigned variable 32-bit integer.
		miscOp, num, err := leb128.LoadUint32(c.body[c.pc:])
		if err != nil {
			return fmt.Errorf("failed to read misc opcode: %v", err)
		}
		c.pc += num - 1
		switch byte(miscOp) {
		case wasm.OpcodeMiscI32TruncSatF32S:
			c.emit(
				newOperationITruncFromF(f32, signedInt32, true),
			)
		case wasm.OpcodeMiscI32TruncSatF32U:
			c.emit(
				newOperationITruncFromF(f32, signedUint32, true),
			)
		case wasm.OpcodeMiscI32TruncSatF64S:
			c.emit(
				newOperationITruncFromF(f64, signedInt32, true),
			)
		case wasm.OpcodeMiscI32TruncSatF64U:
			c.emit(
				newOperationITruncFromF(f64, signedUint32, true),
			)
		case wasm.OpcodeMiscI64TruncSatF32S:
			c.emit(
				newOperationITruncFromF(f32, signedInt64, true),
			)
		case wasm.OpcodeMiscI64TruncSatF32U:
			c.emit(
				newOperationITruncFromF(f32, signedUint64, true),
			)
		case wasm.OpcodeMiscI64TruncSatF64S:
			c.emit(
				newOperationITruncFromF(f64, signedInt64, true),
			)
		case wasm.OpcodeMiscI64TruncSatF64U:
			c.emit(
				newOperationITruncFromF(f64, signedUint64, true),
			)
		case wasm.OpcodeMiscMemoryInit:
			c.result.UsesMemory = true
			dataIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num + 1 // +1 to skip the memory index which is fixed to zero.
			c.emit(
				newOperationMemoryInit(dataIndex),
			)
		case wasm.OpcodeMiscDataDrop:
			dataIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				newOperationDataDrop(dataIndex),
			)
		case wasm.OpcodeMiscMemoryCopy:
			c.result.UsesMemory = true
			c.pc += 2 // +2 to skip two memory indexes which are fixed to zero.
			c.emit(
				newOperationMemoryCopy(),
			)
		case wasm.OpcodeMiscMemoryFill:
			c.result.UsesMemory = true
			c.pc += 1 // +1 to skip the memory index which is fixed to zero.
			c.emit(
				newOperationMemoryFill(),
			)
		case wasm.OpcodeMiscTableInit:
			elemIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			// Read table index which is fixed to zero currently.
			tableIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				newOperationTableInit(elemIndex, tableIndex),
			)
		case wasm.OpcodeMiscElemDrop:
			elemIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				newOperationElemDrop(elemIndex),
			)
		case wasm.OpcodeMiscTableCopy:
			// Read the source table inde.g.
			dst, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			// Read the destination table inde.g.
			src, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				newOperationTableCopy(src, dst),
			)
		case wasm.OpcodeMiscTableGrow:
			// Read the source table inde.g.
			tableIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				newOperationTableGrow(tableIndex),
			)
		case wasm.OpcodeMiscTableSize:
			// Read the source table inde.g.
			tableIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				newOperationTableSize(tableIndex),
			)
		case wasm.OpcodeMiscTableFill:
			// Read the source table index.
			tableIndex, num, err := leb128.LoadUint32(c.body[c.pc+1:])
			if err != nil {
				return fmt.Errorf("reading i32.const value: %v", err)
			}
			c.pc += num
			c.emit(
				newOperationTableFill(tableIndex),
			)
		default:
			return fmt.Errorf("unsupported misc instruction in interpreterir: 0x%x", op)
		}
	case wasm.OpcodeVecPrefix:
		c.pc++
		switch vecOp := c.body[c.pc]; vecOp {
		case wasm.OpcodeVecV128Const:
			c.pc++
			lo := binary.LittleEndian.Uint64(c.body[c.pc : c.pc+8])
			c.pc += 8
			hi := binary.LittleEndian.Uint64(c.body[c.pc : c.pc+8])
			c.emit(
				newOperationV128Const(lo, hi),
			)
			c.pc += 7
		case wasm.OpcodeVecV128Load:
			arg, err := c.readMemoryArg(wasm.OpcodeI32LoadName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType128, arg),
			)
		case wasm.OpcodeVecV128Load8x8s:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load8x8SName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType8x8s, arg),
			)
		case wasm.OpcodeVecV128Load8x8u:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load8x8UName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType8x8u, arg),
			)
		case wasm.OpcodeVecV128Load16x4s:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load16x4SName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType16x4s, arg),
			)
		case wasm.OpcodeVecV128Load16x4u:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load16x4UName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType16x4u, arg),
			)
		case wasm.OpcodeVecV128Load32x2s:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32x2SName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType32x2s, arg),
			)
		case wasm.OpcodeVecV128Load32x2u:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32x2UName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType32x2u, arg),
			)
		case wasm.OpcodeVecV128Load8Splat:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load8SplatName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType8Splat, arg),
			)
		case wasm.OpcodeVecV128Load16Splat:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load16SplatName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType16Splat, arg),
			)
		case wasm.OpcodeVecV128Load32Splat:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32SplatName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType32Splat, arg),
			)
		case wasm.OpcodeVecV128Load64Splat:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load64SplatName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType64Splat, arg),
			)
		case wasm.OpcodeVecV128Load32zero:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32zeroName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType32zero, arg),
			)
		case wasm.OpcodeVecV128Load64zero:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load64zeroName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Load(v128LoadType64zero, arg),
			)
		case wasm.OpcodeVecV128Load8Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load8LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128LoadLane(laneIndex, 8, arg),
			)
		case wasm.OpcodeVecV128Load16Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load16LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128LoadLane(laneIndex, 16, arg),
			)
		case wasm.OpcodeVecV128Load32Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load32LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128LoadLane(laneIndex, 32, arg),
			)
		case wasm.OpcodeVecV128Load64Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Load64LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128LoadLane(laneIndex, 64, arg),
			)
		case wasm.OpcodeVecV128Store:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128StoreName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationV128Store(arg),
			)
		case wasm.OpcodeVecV128Store8Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Store8LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128StoreLane(laneIndex, 8, arg),
			)
		case wasm.OpcodeVecV128Store16Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Store16LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128StoreLane(laneIndex, 16, arg),
			)
		case wasm.OpcodeVecV128Store32Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Store32LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128StoreLane(laneIndex, 32, arg),
			)
		case wasm.OpcodeVecV128Store64Lane:
			arg, err := c.readMemoryArg(wasm.OpcodeVecV128Store64LaneName)
			if err != nil {
				return err
			}
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128StoreLane(laneIndex, 64, arg),
			)
		case wasm.OpcodeVecI8x16ExtractLaneS:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ExtractLane(laneIndex, true, shapeI8x16),
			)
		case wasm.OpcodeVecI8x16ExtractLaneU:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ExtractLane(laneIndex, false, shapeI8x16),
			)
		case wasm.OpcodeVecI16x8ExtractLaneS:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ExtractLane(laneIndex, true, shapeI16x8),
			)
		case wasm.OpcodeVecI16x8ExtractLaneU:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ExtractLane(laneIndex, false, shapeI16x8),
			)
		case wasm.OpcodeVecI32x4ExtractLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ExtractLane(laneIndex, false, shapeI32x4),
			)
		case wasm.OpcodeVecI64x2ExtractLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ExtractLane(laneIndex, false, shapeI64x2),
			)
		case wasm.OpcodeVecF32x4ExtractLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ExtractLane(laneIndex, false, shapeF32x4),
			)
		case wasm.OpcodeVecF64x2ExtractLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ExtractLane(laneIndex, false, shapeF64x2),
			)
		case wasm.OpcodeVecI8x16ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ReplaceLane(laneIndex, shapeI8x16),
			)
		case wasm.OpcodeVecI16x8ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ReplaceLane(laneIndex, shapeI16x8),
			)
		case wasm.OpcodeVecI32x4ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ReplaceLane(laneIndex, shapeI32x4),
			)
		case wasm.OpcodeVecI64x2ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ReplaceLane(laneIndex, shapeI64x2),
			)
		case wasm.OpcodeVecF32x4ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ReplaceLane(laneIndex, shapeF32x4),
			)
		case wasm.OpcodeVecF64x2ReplaceLane:
			c.pc++
			laneIndex := c.body[c.pc]
			c.emit(
				newOperationV128ReplaceLane(laneIndex, shapeF64x2),
			)
		case wasm.OpcodeVecI8x16Splat:
			c.emit(
				newOperationV128Splat(shapeI8x16),
			)
		case wasm.OpcodeVecI16x8Splat:
			c.emit(
				newOperationV128Splat(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4Splat:
			c.emit(
				newOperationV128Splat(shapeI32x4),
			)
		case wasm.OpcodeVecI64x2Splat:
			c.emit(
				newOperationV128Splat(shapeI64x2),
			)
		case wasm.OpcodeVecF32x4Splat:
			c.emit(
				newOperationV128Splat(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Splat:
			c.emit(
				newOperationV128Splat(shapeF64x2),
			)
		case wasm.OpcodeVecI8x16Swizzle:
			c.emit(
				newOperationV128Swizzle(),
			)
		case wasm.OpcodeVecV128i8x16Shuffle:
			c.pc++
			lanes := make([]uint64, 16)
			for i := uint64(0); i < 16; i++ {
				lanes[i] = uint64(c.body[c.pc+i])
			}
			op := newOperationV128Shuffle(lanes)
			c.emit(op)
			c.pc += 15
		case wasm.OpcodeVecV128AnyTrue:
			c.emit(
				newOperationV128AnyTrue(),
			)
		case wasm.OpcodeVecI8x16AllTrue:
			c.emit(
				newOperationV128AllTrue(shapeI8x16),
			)
		case wasm.OpcodeVecI16x8AllTrue:
			c.emit(
				newOperationV128AllTrue(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4AllTrue:
			c.emit(
				newOperationV128AllTrue(shapeI32x4),
			)
		case wasm.OpcodeVecI64x2AllTrue:
			c.emit(
				newOperationV128AllTrue(shapeI64x2),
			)
		case wasm.OpcodeVecI8x16BitMask:
			c.emit(
				newOperationV128BitMask(shapeI8x16),
			)
		case wasm.OpcodeVecI16x8BitMask:
			c.emit(
				newOperationV128BitMask(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4BitMask:
			c.emit(
				newOperationV128BitMask(shapeI32x4),
			)
		case wasm.OpcodeVecI64x2BitMask:
			c.emit(
				newOperationV128BitMask(shapeI64x2),
			)
		case wasm.OpcodeVecV128And:
			c.emit(
				newOperationV128And(),
			)
		case wasm.OpcodeVecV128Not:
			c.emit(
				newOperationV128Not(),
			)
		case wasm.OpcodeVecV128Or:
			c.emit(
				newOperationV128Or(),
			)
		case wasm.OpcodeVecV128Xor:
			c.emit(
				newOperationV128Xor(),
			)
		case wasm.OpcodeVecV128Bitselect:
			c.emit(
				newOperationV128Bitselect(),
			)
		case wasm.OpcodeVecV128AndNot:
			c.emit(
				newOperationV128AndNot(),
			)
		case wasm.OpcodeVecI8x16Shl:
			c.emit(
				newOperationV128Shl(shapeI8x16),
			)
		case wasm.OpcodeVecI8x16ShrS:
			c.emit(
				newOperationV128Shr(shapeI8x16, true),
			)
		case wasm.OpcodeVecI8x16ShrU:
			c.emit(
				newOperationV128Shr(shapeI8x16, false),
			)
		case wasm.OpcodeVecI16x8Shl:
			c.emit(
				newOperationV128Shl(shapeI16x8),
			)
		case wasm.OpcodeVecI16x8ShrS:
			c.emit(
				newOperationV128Shr(shapeI16x8, true),
			)
		case wasm.OpcodeVecI16x8ShrU:
			c.emit(
				newOperationV128Shr(shapeI16x8, false),
			)
		case wasm.OpcodeVecI32x4Shl:
			c.emit(
				newOperationV128Shl(shapeI32x4),
			)
		case wasm.OpcodeVecI32x4ShrS:
			c.emit(
				newOperationV128Shr(shapeI32x4, true),
			)
		case wasm.OpcodeVecI32x4ShrU:
			c.emit(
				newOperationV128Shr(shapeI32x4, false),
			)
		case wasm.OpcodeVecI64x2Shl:
			c.emit(
				newOperationV128Shl(shapeI64x2),
			)
		case wasm.OpcodeVecI64x2ShrS:
			c.emit(
				newOperationV128Shr(shapeI64x2, true),
			)
		case wasm.OpcodeVecI64x2ShrU:
			c.emit(
				newOperationV128Shr(shapeI64x2, false),
			)
		case wasm.OpcodeVecI8x16Eq:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16Eq),
			)
		case wasm.OpcodeVecI8x16Ne:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16Ne),
			)
		case wasm.OpcodeVecI8x16LtS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16LtS),
			)
		case wasm.OpcodeVecI8x16LtU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16LtU),
			)
		case wasm.OpcodeVecI8x16GtS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16GtS),
			)
		case wasm.OpcodeVecI8x16GtU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16GtU),
			)
		case wasm.OpcodeVecI8x16LeS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16LeS),
			)
		case wasm.OpcodeVecI8x16LeU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16LeU),
			)
		case wasm.OpcodeVecI8x16GeS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16GeS),
			)
		case wasm.OpcodeVecI8x16GeU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI8x16GeU),
			)
		case wasm.OpcodeVecI16x8Eq:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8Eq),
			)
		case wasm.OpcodeVecI16x8Ne:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8Ne),
			)
		case wasm.OpcodeVecI16x8LtS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8LtS),
			)
		case wasm.OpcodeVecI16x8LtU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8LtU),
			)
		case wasm.OpcodeVecI16x8GtS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8GtS),
			)
		case wasm.OpcodeVecI16x8GtU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8GtU),
			)
		case wasm.OpcodeVecI16x8LeS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8LeS),
			)
		case wasm.OpcodeVecI16x8LeU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8LeU),
			)
		case wasm.OpcodeVecI16x8GeS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8GeS),
			)
		case wasm.OpcodeVecI16x8GeU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI16x8GeU),
			)
		case wasm.OpcodeVecI32x4Eq:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4Eq),
			)
		case wasm.OpcodeVecI32x4Ne:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4Ne),
			)
		case wasm.OpcodeVecI32x4LtS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4LtS),
			)
		case wasm.OpcodeVecI32x4LtU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4LtU),
			)
		case wasm.OpcodeVecI32x4GtS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4GtS),
			)
		case wasm.OpcodeVecI32x4GtU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4GtU),
			)
		case wasm.OpcodeVecI32x4LeS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4LeS),
			)
		case wasm.OpcodeVecI32x4LeU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4LeU),
			)
		case wasm.OpcodeVecI32x4GeS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4GeS),
			)
		case wasm.OpcodeVecI32x4GeU:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI32x4GeU),
			)
		case wasm.OpcodeVecI64x2Eq:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI64x2Eq),
			)
		case wasm.OpcodeVecI64x2Ne:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI64x2Ne),
			)
		case wasm.OpcodeVecI64x2LtS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI64x2LtS),
			)
		case wasm.OpcodeVecI64x2GtS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI64x2GtS),
			)
		case wasm.OpcodeVecI64x2LeS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI64x2LeS),
			)
		case wasm.OpcodeVecI64x2GeS:
			c.emit(
				newOperationV128Cmp(v128CmpTypeI64x2GeS),
			)
		case wasm.OpcodeVecF32x4Eq:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF32x4Eq),
			)
		case wasm.OpcodeVecF32x4Ne:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF32x4Ne),
			)
		case wasm.OpcodeVecF32x4Lt:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF32x4Lt),
			)
		case wasm.OpcodeVecF32x4Gt:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF32x4Gt),
			)
		case wasm.OpcodeVecF32x4Le:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF32x4Le),
			)
		case wasm.OpcodeVecF32x4Ge:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF32x4Ge),
			)
		case wasm.OpcodeVecF64x2Eq:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF64x2Eq),
			)
		case wasm.OpcodeVecF64x2Ne:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF64x2Ne),
			)
		case wasm.OpcodeVecF64x2Lt:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF64x2Lt),
			)
		case wasm.OpcodeVecF64x2Gt:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF64x2Gt),
			)
		case wasm.OpcodeVecF64x2Le:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF64x2Le),
			)
		case wasm.OpcodeVecF64x2Ge:
			c.emit(
				newOperationV128Cmp(v128CmpTypeF64x2Ge),
			)
		case wasm.OpcodeVecI8x16Neg:
			c.emit(
				newOperationV128Neg(shapeI8x16),
			)
		case wasm.OpcodeVecI16x8Neg:
			c.emit(
				newOperationV128Neg(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4Neg:
			c.emit(
				newOperationV128Neg(shapeI32x4),
			)
		case wasm.OpcodeVecI64x2Neg:
			c.emit(
				newOperationV128Neg(shapeI64x2),
			)
		case wasm.OpcodeVecF32x4Neg:
			c.emit(
				newOperationV128Neg(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Neg:
			c.emit(
				newOperationV128Neg(shapeF64x2),
			)
		case wasm.OpcodeVecI8x16Add:
			c.emit(
				newOperationV128Add(shapeI8x16),
			)
		case wasm.OpcodeVecI16x8Add:
			c.emit(
				newOperationV128Add(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4Add:
			c.emit(
				newOperationV128Add(shapeI32x4),
			)
		case wasm.OpcodeVecI64x2Add:
			c.emit(
				newOperationV128Add(shapeI64x2),
			)
		case wasm.OpcodeVecF32x4Add:
			c.emit(
				newOperationV128Add(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Add:
			c.emit(
				newOperationV128Add(shapeF64x2),
			)
		case wasm.OpcodeVecI8x16Sub:
			c.emit(
				newOperationV128Sub(shapeI8x16),
			)
		case wasm.OpcodeVecI16x8Sub:
			c.emit(
				newOperationV128Sub(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4Sub:
			c.emit(
				newOperationV128Sub(shapeI32x4),
			)
		case wasm.OpcodeVecI64x2Sub:
			c.emit(
				newOperationV128Sub(shapeI64x2),
			)
		case wasm.OpcodeVecF32x4Sub:
			c.emit(
				newOperationV128Sub(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Sub:
			c.emit(
				newOperationV128Sub(shapeF64x2),
			)
		case wasm.OpcodeVecI8x16AddSatS:
			c.emit(
				newOperationV128AddSat(shapeI8x16, true),
			)
		case wasm.OpcodeVecI8x16AddSatU:
			c.emit(
				newOperationV128AddSat(shapeI8x16, false),
			)
		case wasm.OpcodeVecI16x8AddSatS:
			c.emit(
				newOperationV128AddSat(shapeI16x8, true),
			)
		case wasm.OpcodeVecI16x8AddSatU:
			c.emit(
				newOperationV128AddSat(shapeI16x8, false),
			)
		case wasm.OpcodeVecI8x16SubSatS:
			c.emit(
				newOperationV128SubSat(shapeI8x16, true),
			)
		case wasm.OpcodeVecI8x16SubSatU:
			c.emit(
				newOperationV128SubSat(shapeI8x16, false),
			)
		case wasm.OpcodeVecI16x8SubSatS:
			c.emit(
				newOperationV128SubSat(shapeI16x8, true),
			)
		case wasm.OpcodeVecI16x8SubSatU:
			c.emit(
				newOperationV128SubSat(shapeI16x8, false),
			)
		case wasm.OpcodeVecI16x8Mul:
			c.emit(
				newOperationV128Mul(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4Mul:
			c.emit(
				newOperationV128Mul(shapeI32x4),
			)
		case wasm.OpcodeVecI64x2Mul:
			c.emit(
				newOperationV128Mul(shapeI64x2),
			)
		case wasm.OpcodeVecF32x4Mul:
			c.emit(
				newOperationV128Mul(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Mul:
			c.emit(
				newOperationV128Mul(shapeF64x2),
			)
		case wasm.OpcodeVecF32x4Sqrt:
			c.emit(
				newOperationV128Sqrt(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Sqrt:
			c.emit(
				newOperationV128Sqrt(shapeF64x2),
			)
		case wasm.OpcodeVecF32x4Div:
			c.emit(
				newOperationV128Div(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Div:
			c.emit(
				newOperationV128Div(shapeF64x2),
			)
		case wasm.OpcodeVecI8x16Abs:
			c.emit(
				newOperationV128Abs(shapeI8x16),
			)
		case wasm.OpcodeVecI8x16Popcnt:
			c.emit(
				newOperationV128Popcnt(shapeI8x16),
			)
		case wasm.OpcodeVecI16x8Abs:
			c.emit(
				newOperationV128Abs(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4Abs:
			c.emit(
				newOperationV128Abs(shapeI32x4),
			)
		case wasm.OpcodeVecI64x2Abs:
			c.emit(
				newOperationV128Abs(shapeI64x2),
			)
		case wasm.OpcodeVecF32x4Abs:
			c.emit(
				newOperationV128Abs(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Abs:
			c.emit(
				newOperationV128Abs(shapeF64x2),
			)
		case wasm.OpcodeVecI8x16MinS:
			c.emit(
				newOperationV128Min(shapeI8x16, true),
			)
		case wasm.OpcodeVecI8x16MinU:
			c.emit(
				newOperationV128Min(shapeI8x16, false),
			)
		case wasm.OpcodeVecI8x16MaxS:
			c.emit(
				newOperationV128Max(shapeI8x16, true),
			)
		case wasm.OpcodeVecI8x16MaxU:
			c.emit(
				newOperationV128Max(shapeI8x16, false),
			)
		case wasm.OpcodeVecI8x16AvgrU:
			c.emit(
				newOperationV128AvgrU(shapeI8x16),
			)
		case wasm.OpcodeVecI16x8MinS:
			c.emit(
				newOperationV128Min(shapeI16x8, true),
			)
		case wasm.OpcodeVecI16x8MinU:
			c.emit(
				newOperationV128Min(shapeI16x8, false),
			)
		case wasm.OpcodeVecI16x8MaxS:
			c.emit(
				newOperationV128Max(shapeI16x8, true),
			)
		case wasm.OpcodeVecI16x8MaxU:
			c.emit(
				newOperationV128Max(shapeI16x8, false),
			)
		case wasm.OpcodeVecI16x8AvgrU:
			c.emit(
				newOperationV128AvgrU(shapeI16x8),
			)
		case wasm.OpcodeVecI32x4MinS:
			c.emit(
				newOperationV128Min(shapeI32x4, true),
			)
		case wasm.OpcodeVecI32x4MinU:
			c.emit(
				newOperationV128Min(shapeI32x4, false),
			)
		case wasm.OpcodeVecI32x4MaxS:
			c.emit(
				newOperationV128Max(shapeI32x4, true),
			)
		case wasm.OpcodeVecI32x4MaxU:
			c.emit(
				newOperationV128Max(shapeI32x4, false),
			)
		case wasm.OpcodeVecF32x4Min:
			c.emit(
				newOperationV128Min(shapeF32x4, false),
			)
		case wasm.OpcodeVecF32x4Max:
			c.emit(
				newOperationV128Max(shapeF32x4, false),
			)
		case wasm.OpcodeVecF64x2Min:
			c.emit(
				newOperationV128Min(shapeF64x2, false),
			)
		case wasm.OpcodeVecF64x2Max:
			c.emit(
				newOperationV128Max(shapeF64x2, false),
			)
		case wasm.OpcodeVecF32x4Pmin:
			c.emit(
				newOperationV128Pmin(shapeF32x4),
			)
		case wasm.OpcodeVecF32x4Pmax:
			c.emit(
				newOperationV128Pmax(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Pmin:
			c.emit(
				newOperationV128Pmin(shapeF64x2),
			)
		case wasm.OpcodeVecF64x2Pmax:
			c.emit(
				newOperationV128Pmax(shapeF64x2),
			)
		case wasm.OpcodeVecF32x4Ceil:
			c.emit(
				newOperationV128Ceil(shapeF32x4),
			)
		case wasm.OpcodeVecF32x4Floor:
			c.emit(
				newOperationV128Floor(shapeF32x4),
			)
		case wasm.OpcodeVecF32x4Trunc:
			c.emit(
				newOperationV128Trunc(shapeF32x4),
			)
		case wasm.OpcodeVecF32x4Nearest:
			c.emit(
				newOperationV128Nearest(shapeF32x4),
			)
		case wasm.OpcodeVecF64x2Ceil:
			c.emit(
				newOperationV128Ceil(shapeF64x2),
			)
		case wasm.OpcodeVecF64x2Floor:
			c.emit(
				newOperationV128Floor(shapeF64x2),
			)
		case wasm.OpcodeVecF64x2Trunc:
			c.emit(
				newOperationV128Trunc(shapeF64x2),
			)
		case wasm.OpcodeVecF64x2Nearest:
			c.emit(
				newOperationV128Nearest(shapeF64x2),
			)
		case wasm.OpcodeVecI16x8ExtendLowI8x16S:
			c.emit(
				newOperationV128Extend(shapeI8x16, true, true),
			)
		case wasm.OpcodeVecI16x8ExtendHighI8x16S:
			c.emit(
				newOperationV128Extend(shapeI8x16, true, false),
			)
		case wasm.OpcodeVecI16x8ExtendLowI8x16U:
			c.emit(
				newOperationV128Extend(shapeI8x16, false, true),
			)
		case wasm.OpcodeVecI16x8ExtendHighI8x16U:
			c.emit(
				newOperationV128Extend(shapeI8x16, false, false),
			)
		case wasm.OpcodeVecI32x4ExtendLowI16x8S:
			c.emit(
				newOperationV128Extend(shapeI16x8, true, true),
			)
		case wasm.OpcodeVecI32x4ExtendHighI16x8S:
			c.emit(
				newOperationV128Extend(shapeI16x8, true, false),
			)
		case wasm.OpcodeVecI32x4ExtendLowI16x8U:
			c.emit(
				newOperationV128Extend(shapeI16x8, false, true),
			)
		case wasm.OpcodeVecI32x4ExtendHighI16x8U:
			c.emit(
				newOperationV128Extend(shapeI16x8, false, false),
			)
		case wasm.OpcodeVecI64x2ExtendLowI32x4S:
			c.emit(
				newOperationV128Extend(shapeI32x4, true, true),
			)
		case wasm.OpcodeVecI64x2ExtendHighI32x4S:
			c.emit(
				newOperationV128Extend(shapeI32x4, true, false),
			)
		case wasm.OpcodeVecI64x2ExtendLowI32x4U:
			c.emit(
				newOperationV128Extend(shapeI32x4, false, true),
			)
		case wasm.OpcodeVecI64x2ExtendHighI32x4U:
			c.emit(
				newOperationV128Extend(shapeI32x4, false, false),
			)
		case wasm.OpcodeVecI16x8Q15mulrSatS:
			c.emit(
				newOperationV128Q15mulrSatS(),
			)
		case wasm.OpcodeVecI16x8ExtMulLowI8x16S:
			c.emit(
				newOperationV128ExtMul(shapeI8x16, true, true),
			)
		case wasm.OpcodeVecI16x8ExtMulHighI8x16S:
			c.emit(
				newOperationV128ExtMul(shapeI8x16, true, false),
			)
		case wasm.OpcodeVecI16x8ExtMulLowI8x16U:
			c.emit(
				newOperationV128ExtMul(shapeI8x16, false, true),
			)
		case wasm.OpcodeVecI16x8ExtMulHighI8x16U:
			c.emit(
				newOperationV128ExtMul(shapeI8x16, false, false),
			)
		case wasm.OpcodeVecI32x4ExtMulLowI16x8S:
			c.emit(
				newOperationV128ExtMul(shapeI16x8, true, true),
			)
		case wasm.OpcodeVecI32x4ExtMulHighI16x8S:
			c.emit(
				newOperationV128ExtMul(shapeI16x8, true, false),
			)
		case wasm.OpcodeVecI32x4ExtMulLowI16x8U:
			c.emit(
				newOperationV128ExtMul(shapeI16x8, false, true),
			)
		case wasm.OpcodeVecI32x4ExtMulHighI16x8U:
			c.emit(
				newOperationV128ExtMul(shapeI16x8, false, false),
			)
		case wasm.OpcodeVecI64x2ExtMulLowI32x4S:
			c.emit(
				newOperationV128ExtMul(shapeI32x4, true, true),
			)
		case wasm.OpcodeVecI64x2ExtMulHighI32x4S:
			c.emit(
				newOperationV128ExtMul(shapeI32x4, true, false),
			)
		case wasm.OpcodeVecI64x2ExtMulLowI32x4U:
			c.emit(
				newOperationV128ExtMul(shapeI32x4, false, true),
			)
		case wasm.OpcodeVecI64x2ExtMulHighI32x4U:
			c.emit(
				newOperationV128ExtMul(shapeI32x4, false, false),
			)
		case wasm.OpcodeVecI16x8ExtaddPairwiseI8x16S:
			c.emit(
				newOperationV128ExtAddPairwise(shapeI8x16, true),
			)
		case wasm.OpcodeVecI16x8ExtaddPairwiseI8x16U:
			c.emit(
				newOperationV128ExtAddPairwise(shapeI8x16, false),
			)
		case wasm.OpcodeVecI32x4ExtaddPairwiseI16x8S:
			c.emit(
				newOperationV128ExtAddPairwise(shapeI16x8, true),
			)
		case wasm.OpcodeVecI32x4ExtaddPairwiseI16x8U:
			c.emit(
				newOperationV128ExtAddPairwise(shapeI16x8, false),
			)
		case wasm.OpcodeVecF64x2PromoteLowF32x4Zero:
			c.emit(
				newOperationV128FloatPromote(),
			)
		case wasm.OpcodeVecF32x4DemoteF64x2Zero:
			c.emit(
				newOperationV128FloatDemote(),
			)
		case wasm.OpcodeVecF32x4ConvertI32x4S:
			c.emit(
				newOperationV128FConvertFromI(shapeF32x4, true),
			)
		case wasm.OpcodeVecF32x4ConvertI32x4U:
			c.emit(
				newOperationV128FConvertFromI(shapeF32x4, false),
			)
		case wasm.OpcodeVecF64x2ConvertLowI32x4S:
			c.emit(
				newOperationV128FConvertFromI(shapeF64x2, true),
			)
		case wasm.OpcodeVecF64x2ConvertLowI32x4U:
			c.emit(
				newOperationV128FConvertFromI(shapeF64x2, false),
			)
		case wasm.OpcodeVecI32x4DotI16x8S:
			c.emit(
				newOperationV128Dot(),
			)
		case wasm.OpcodeVecI8x16NarrowI16x8S:
			c.emit(
				newOperationV128Narrow(shapeI16x8, true),
			)
		case wasm.OpcodeVecI8x16NarrowI16x8U:
			c.emit(
				newOperationV128Narrow(shapeI16x8, false),
			)
		case wasm.OpcodeVecI16x8NarrowI32x4S:
			c.emit(
				newOperationV128Narrow(shapeI32x4, true),
			)
		case wasm.OpcodeVecI16x8NarrowI32x4U:
			c.emit(
				newOperationV128Narrow(shapeI32x4, false),
			)
		case wasm.OpcodeVecI32x4TruncSatF32x4S:
			c.emit(
				newOperationV128ITruncSatFromF(shapeF32x4, true),
			)
		case wasm.OpcodeVecI32x4TruncSatF32x4U:
			c.emit(
				newOperationV128ITruncSatFromF(shapeF32x4, false),
			)
		case wasm.OpcodeVecI32x4TruncSatF64x2SZero:
			c.emit(
				newOperationV128ITruncSatFromF(shapeF64x2, true),
			)
		case wasm.OpcodeVecI32x4TruncSatF64x2UZero:
			c.emit(
				newOperationV128ITruncSatFromF(shapeF64x2, false),
			)
		default:
			return fmt.Errorf("unsupported vector instruction in interpreterir: %s", wasm.VectorInstructionName(vecOp))
		}
	case wasm.OpcodeAtomicPrefix:
		c.pc++
		atomicOp := c.body[c.pc]
		switch atomicOp {
		case wasm.OpcodeAtomicMemoryWait32:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicMemoryWait32Name)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicMemoryWait(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicMemoryWait64:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicMemoryWait64Name)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicMemoryWait(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicMemoryNotify:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicMemoryNotifyName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicMemoryNotify(imm),
			)
		case wasm.OpcodeAtomicFence:
			// Skip immediate value
			c.pc++
			_ = c.body[c.pc]
			c.emit(
				newOperationAtomicFence(),
			)
		case wasm.OpcodeAtomicI32Load:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32LoadName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicLoad(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI64Load:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64LoadName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicLoad(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI32Load8U:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Load8UName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicLoad8(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI32Load16U:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Load16UName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicLoad16(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI64Load8U:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Load8UName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicLoad8(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI64Load16U:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Load16UName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicLoad16(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI64Load32U:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Load32UName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicLoad(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI32Store:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32StoreName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicStore(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI32Store8:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Store8Name)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicStore8(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI32Store16:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Store16Name)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicStore16(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI64Store:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64StoreName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicStore(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI64Store8:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Store8Name)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicStore8(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI64Store16:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Store16Name)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicStore16(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI64Store32:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Store32Name)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicStore(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI32RmwAdd:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32RmwAddName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpAdd),
			)
		case wasm.OpcodeAtomicI64RmwAdd:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64RmwAddName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI64, imm, atomicArithmeticOpAdd),
			)
		case wasm.OpcodeAtomicI32Rmw8AddU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw8AddUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI32, imm, atomicArithmeticOpAdd),
			)
		case wasm.OpcodeAtomicI64Rmw8AddU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw8AddUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI64, imm, atomicArithmeticOpAdd),
			)
		case wasm.OpcodeAtomicI32Rmw16AddU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw16AddUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI32, imm, atomicArithmeticOpAdd),
			)
		case wasm.OpcodeAtomicI64Rmw16AddU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw16AddUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI64, imm, atomicArithmeticOpAdd),
			)
		case wasm.OpcodeAtomicI64Rmw32AddU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw32AddUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpAdd),
			)
		case wasm.OpcodeAtomicI32RmwSub:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32RmwSubName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpSub),
			)
		case wasm.OpcodeAtomicI64RmwSub:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64RmwSubName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI64, imm, atomicArithmeticOpSub),
			)
		case wasm.OpcodeAtomicI32Rmw8SubU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw8SubUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI32, imm, atomicArithmeticOpSub),
			)
		case wasm.OpcodeAtomicI64Rmw8SubU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw8SubUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI64, imm, atomicArithmeticOpSub),
			)
		case wasm.OpcodeAtomicI32Rmw16SubU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw16SubUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI32, imm, atomicArithmeticOpSub),
			)
		case wasm.OpcodeAtomicI64Rmw16SubU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw16SubUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI64, imm, atomicArithmeticOpSub),
			)
		case wasm.OpcodeAtomicI64Rmw32SubU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw32SubUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpSub),
			)
		case wasm.OpcodeAtomicI32RmwAnd:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32RmwAndName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpAnd),
			)
		case wasm.OpcodeAtomicI64RmwAnd:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64RmwAndName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI64, imm, atomicArithmeticOpAnd),
			)
		case wasm.OpcodeAtomicI32Rmw8AndU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw8AndUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI32, imm, atomicArithmeticOpAnd),
			)
		case wasm.OpcodeAtomicI64Rmw8AndU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw8AndUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI64, imm, atomicArithmeticOpAnd),
			)
		case wasm.OpcodeAtomicI32Rmw16AndU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw16AndUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI32, imm, atomicArithmeticOpAnd),
			)
		case wasm.OpcodeAtomicI64Rmw16AndU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw16AndUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI64, imm, atomicArithmeticOpAnd),
			)
		case wasm.OpcodeAtomicI64Rmw32AndU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw32AndUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpAnd),
			)
		case wasm.OpcodeAtomicI32RmwOr:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32RmwOrName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpOr),
			)
		case wasm.OpcodeAtomicI64RmwOr:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64RmwOrName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI64, imm, atomicArithmeticOpOr),
			)
		case wasm.OpcodeAtomicI32Rmw8OrU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw8OrUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI32, imm, atomicArithmeticOpOr),
			)
		case wasm.OpcodeAtomicI64Rmw8OrU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw8OrUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI64, imm, atomicArithmeticOpOr),
			)
		case wasm.OpcodeAtomicI32Rmw16OrU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw16OrUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI32, imm, atomicArithmeticOpOr),
			)
		case wasm.OpcodeAtomicI64Rmw16OrU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw16OrUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI64, imm, atomicArithmeticOpOr),
			)
		case wasm.OpcodeAtomicI64Rmw32OrU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw32OrUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpOr),
			)
		case wasm.OpcodeAtomicI32RmwXor:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32RmwXorName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpXor),
			)
		case wasm.OpcodeAtomicI64RmwXor:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64RmwXorName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI64, imm, atomicArithmeticOpXor),
			)
		case wasm.OpcodeAtomicI32Rmw8XorU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw8XorUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI32, imm, atomicArithmeticOpXor),
			)
		case wasm.OpcodeAtomicI64Rmw8XorU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw8XorUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI64, imm, atomicArithmeticOpXor),
			)
		case wasm.OpcodeAtomicI32Rmw16XorU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw16XorUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI32, imm, atomicArithmeticOpXor),
			)
		case wasm.OpcodeAtomicI64Rmw16XorU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw16XorUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI64, imm, atomicArithmeticOpXor),
			)
		case wasm.OpcodeAtomicI64Rmw32XorU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw32XorUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpXor),
			)
		case wasm.OpcodeAtomicI32RmwXchg:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32RmwXchgName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpNop),
			)
		case wasm.OpcodeAtomicI64RmwXchg:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64RmwXchgName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI64, imm, atomicArithmeticOpNop),
			)
		case wasm.OpcodeAtomicI32Rmw8XchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw8XchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI32, imm, atomicArithmeticOpNop),
			)
		case wasm.OpcodeAtomicI64Rmw8XchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw8XchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8(unsignedTypeI64, imm, atomicArithmeticOpNop),
			)
		case wasm.OpcodeAtomicI32Rmw16XchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw16XchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI32, imm, atomicArithmeticOpNop),
			)
		case wasm.OpcodeAtomicI64Rmw16XchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw16XchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16(unsignedTypeI64, imm, atomicArithmeticOpNop),
			)
		case wasm.OpcodeAtomicI64Rmw32XchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw32XchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW(unsignedTypeI32, imm, atomicArithmeticOpNop),
			)
		case wasm.OpcodeAtomicI32RmwCmpxchg:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32RmwCmpxchgName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMWCmpxchg(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI64RmwCmpxchg:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64RmwCmpxchgName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMWCmpxchg(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI32Rmw8CmpxchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw8CmpxchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8Cmpxchg(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI64Rmw8CmpxchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw8CmpxchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW8Cmpxchg(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI32Rmw16CmpxchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI32Rmw16CmpxchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16Cmpxchg(unsignedTypeI32, imm),
			)
		case wasm.OpcodeAtomicI64Rmw16CmpxchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw16CmpxchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMW16Cmpxchg(unsignedTypeI64, imm),
			)
		case wasm.OpcodeAtomicI64Rmw32CmpxchgU:
			imm, err := c.readMemoryArg(wasm.OpcodeAtomicI64Rmw32CmpxchgUName)
			if err != nil {
				return err
			}
			c.emit(
				newOperationAtomicRMWCmpxchg(unsignedTypeI32, imm),
			)
		default:
			return fmt.Errorf("unsupported atomic instruction in interpreterir: %s", wasm.AtomicInstructionName(atomicOp))
		}
	default:
		return fmt.Errorf("unsupported instruction in interpreterir: 0x%x", op)
	}

	// Move the program counter to point to the next instruction.
	c.pc++
	return nil
}

func (c *compiler) nextFrameID() (id uint32) {
	id = c.currentFrameID + 1
	c.currentFrameID++
	return
}

func (c *compiler) applyToStack(opcode wasm.Opcode) (index uint32, err error) {
	switch opcode {
	case
		// These are the opcodes that is coupled with "index"　immediate
		// and it DOES affect the signature of opcode.
		wasm.OpcodeCall,
		wasm.OpcodeCallIndirect,
		wasm.OpcodeLocalGet,
		wasm.OpcodeLocalSet,
		wasm.OpcodeLocalTee,
		wasm.OpcodeGlobalGet,
		wasm.OpcodeGlobalSet:
		// Assumes that we are at the opcode now so skip it before read immediates.
		v, num, err := leb128.LoadUint32(c.body[c.pc+1:])
		if err != nil {
			return 0, fmt.Errorf("reading immediates: %w", err)
		}
		c.pc += num
		index = v
	default:
		// Note that other opcodes are free of index
		// as it doesn't affect the signature of opt code.
		// In other words, the "index" argument of wasmOpcodeSignature
		// is ignored there.
	}

	if c.unreachableState.on {
		return 0, nil
	}

	// Retrieve the signature of the opcode.
	s, err := c.wasmOpcodeSignature(opcode, index)
	if err != nil {
		return 0, err
	}

	// Manipulate the stack according to the signature.
	// Note that the following algorithm assumes that
	// the unknown type is unique in the signature,
	// and is determined by the actual type on the stack.
	// The determined type is stored in this typeParam.
	var typeParam unsignedType
	var typeParamFound bool
	for i := range s.in {
		want := s.in[len(s.in)-1-i]
		actual := c.stackPop()
		if want == unsignedTypeUnknown && typeParamFound {
			want = typeParam
		} else if want == unsignedTypeUnknown {
			want = actual
			typeParam = want
			typeParamFound = true
		}
		if want != actual {
			return 0, fmt.Errorf("input signature mismatch: want %s but have %s", want, actual)
		}
	}

	for _, target := range s.out {
		if target == unsignedTypeUnknown && !typeParamFound {
			return 0, fmt.Errorf("cannot determine type of unknown result")
		} else if target == unsignedTypeUnknown {
			c.stackPush(typeParam)
		} else {
			c.stackPush(target)
		}
	}

	return index, nil
}

func (c *compiler) stackPeek() (ret unsignedType) {
	ret = c.stack[len(c.stack)-1]
	return
}

func (c *compiler) stackSwitchAt(frame *controlFrame) {
	c.stack = c.stack[:frame.originalStackLenWithoutParam]
	c.stackLenInUint64 = frame.originalStackLenWithoutParamUint64
}

func (c *compiler) stackPop() (ret unsignedType) {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase.
	ret = c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
	c.stackLenInUint64 -= 1 + int(unsignedTypeV128&ret>>2)
	return
}

func (c *compiler) stackPush(ts unsignedType) {
	c.stack = append(c.stack, ts)
	c.stackLenInUint64 += 1 + int(unsignedTypeV128&ts>>2)
}

// emit adds the operations into the result.
func (c *compiler) emit(op unionOperation) {
	if !c.unreachableState.on {
		switch op.Kind {
		case operationKindDrop:
			// If the drop range is nil,
			// we could remove such operations.
			// That happens when drop operation is unnecessary.
			// i.e. when there's no need to adjust stack before jmp.
			if int64(op.U1) == -1 {
				return
			}
		}
		c.result.Operations = append(c.result.Operations, op)
		if c.needSourceOffset {
			c.result.IROperationSourceOffsetsInWasmBinary = append(c.result.IROperationSourceOffsetsInWasmBinary,
				c.currentOpPC+c.bodyOffsetInCodeSection)
		}
	}
}

// Emit const expression with default values of the given type.
func (c *compiler) emitDefaultValue(t wasm.ValueType) {
	switch t {
	case wasm.ValueTypeI32:
		c.stackPush(unsignedTypeI32)
		c.emit(newOperationConstI32(0))
	case wasm.ValueTypeI64, wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
		c.stackPush(unsignedTypeI64)
		c.emit(newOperationConstI64(0))
	case wasm.ValueTypeF32:
		c.stackPush(unsignedTypeF32)
		c.emit(newOperationConstF32(0))
	case wasm.ValueTypeF64:
		c.stackPush(unsignedTypeF64)
		c.emit(newOperationConstF64(0))
	case wasm.ValueTypeV128:
		c.stackPush(unsignedTypeV128)
		c.emit(newOperationV128Const(0, 0))
	}
}

// Returns the "depth" (starting from top of the stack)
// of the n-th local.
func (c *compiler) localDepth(index wasm.Index) int {
	height := c.localIndexToStackHeightInUint64[index]
	return c.stackLenInUint64 - 1 - height
}

func (c *compiler) localType(index wasm.Index) (t wasm.ValueType) {
	if params := uint32(len(c.sig.Params)); index < params {
		t = c.sig.Params[index]
	} else {
		t = c.localTypes[index-params]
	}
	return
}

// getFrameDropRange returns the range (starting from top of the stack) that spans across the (uint64) stack. The range is
// supposed to be dropped from the stack when the given frame exists or branch into it.
//
// * frame is the control frame which the call-site is trying to branch into or exit.
// * isEnd true if the call-site is handling wasm.OpcodeEnd.
func (c *compiler) getFrameDropRange(frame *controlFrame, isEnd bool) inclusiveRange {
	var start int
	if !isEnd && frame.kind == controlFrameKindLoop {
		// If this is not End and the call-site is trying to branch into the Loop control frame,
		// we have to Start executing from the beginning of the loop block.
		// Therefore, we have to pass the inputs to the frame.
		start = frame.blockType.ParamNumInUint64
	} else {
		start = frame.blockType.ResultNumInUint64
	}
	end := c.stackLenInUint64 - 1 - frame.originalStackLenWithoutParamUint64
	if start <= end {
		return inclusiveRange{Start: int32(start), End: int32(end)}
	} else {
		return nopinclusiveRange
	}
}

func (c *compiler) readMemoryArg(tag string) (memoryArg, error) {
	c.result.UsesMemory = true
	alignment, num, err := leb128.LoadUint32(c.body[c.pc+1:])
	if err != nil {
		return memoryArg{}, fmt.Errorf("reading alignment for %s: %w", tag, err)
	}
	c.pc += num
	offset, num, err := leb128.LoadUint32(c.body[c.pc+1:])
	if err != nil {
		return memoryArg{}, fmt.Errorf("reading offset for %s: %w", tag, err)
	}
	c.pc += num
	return memoryArg{Offset: offset, Alignment: alignment}, nil
}
