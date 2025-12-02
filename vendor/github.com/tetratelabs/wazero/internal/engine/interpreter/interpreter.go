package interpreter

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"sync"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/expctxkeys"
	"github.com/tetratelabs/wazero/internal/filecache"
	"github.com/tetratelabs/wazero/internal/internalapi"
	"github.com/tetratelabs/wazero/internal/moremath"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wasmdebug"
	"github.com/tetratelabs/wazero/internal/wasmruntime"
)

// callStackCeiling is the maximum WebAssembly call frame stack height. This allows wazero to raise
// wasm.ErrCallStackOverflow instead of overflowing the Go runtime.
//
// The default value should suffice for most use cases. Those wishing to change this can via `go build -ldflags`.
var callStackCeiling = 2000

// engine is an interpreter implementation of wasm.Engine
type engine struct {
	enabledFeatures   api.CoreFeatures
	compiledFunctions map[wasm.ModuleID][]compiledFunction // guarded by mutex.
	mux               sync.RWMutex
}

func NewEngine(_ context.Context, enabledFeatures api.CoreFeatures, _ filecache.Cache) wasm.Engine {
	return &engine{
		enabledFeatures:   enabledFeatures,
		compiledFunctions: map[wasm.ModuleID][]compiledFunction{},
	}
}

// Close implements the same method as documented on wasm.Engine.
func (e *engine) Close() (err error) {
	return
}

// CompiledModuleCount implements the same method as documented on wasm.Engine.
func (e *engine) CompiledModuleCount() uint32 {
	return uint32(len(e.compiledFunctions))
}

// DeleteCompiledModule implements the same method as documented on wasm.Engine.
func (e *engine) DeleteCompiledModule(m *wasm.Module) {
	e.deleteCompiledFunctions(m)
}

func (e *engine) deleteCompiledFunctions(module *wasm.Module) {
	e.mux.Lock()
	defer e.mux.Unlock()
	delete(e.compiledFunctions, module.ID)
}

func (e *engine) addCompiledFunctions(module *wasm.Module, fs []compiledFunction) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.compiledFunctions[module.ID] = fs
}

func (e *engine) getCompiledFunctions(module *wasm.Module) (fs []compiledFunction, ok bool) {
	e.mux.RLock()
	defer e.mux.RUnlock()
	fs, ok = e.compiledFunctions[module.ID]
	return
}

// moduleEngine implements wasm.ModuleEngine
type moduleEngine struct {
	// codes are the compiled functions in a module instances.
	// The index is module instance-scoped.
	functions []function

	// parentEngine holds *engine from which this module engine is created from.
	parentEngine *engine
}

// GetGlobalValue implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) GetGlobalValue(wasm.Index) (lo, hi uint64) {
	panic("BUG: GetGlobalValue should never be called on interpreter mode")
}

// SetGlobalValue implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) SetGlobalValue(idx wasm.Index, lo, hi uint64) {
	panic("BUG: SetGlobalValue should never be called on interpreter mode")
}

// OwnsGlobals implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) OwnsGlobals() bool { return false }

// MemoryGrown implements wasm.ModuleEngine.
func (e *moduleEngine) MemoryGrown() {}

// callEngine holds context per moduleEngine.Call, and shared across all the
// function calls originating from the same moduleEngine.Call execution.
//
// This implements api.Function.
type callEngine struct {
	internalapi.WazeroOnlyType

	// stack contains the operands.
	// Note that all the values are represented as uint64.
	stack []uint64

	// frames are the function call stack.
	frames []*callFrame

	// f is the initial function for this call engine.
	f *function

	// stackiterator for Listeners to walk frames and stack.
	stackIterator stackIterator
}

func (e *moduleEngine) newCallEngine(compiled *function) *callEngine {
	return &callEngine{f: compiled}
}

func (ce *callEngine) pushValue(v uint64) {
	ce.stack = append(ce.stack, v)
}

func (ce *callEngine) pushValues(v []uint64) {
	ce.stack = append(ce.stack, v...)
}

func (ce *callEngine) popValue() (v uint64) {
	// No need to check stack bound
	// as we can assume that all the operations
	// are valid thanks to validateFunction
	// at module validation phase
	// and interpreterir translation
	// before compilation.
	stackTopIndex := len(ce.stack) - 1
	v = ce.stack[stackTopIndex]
	ce.stack = ce.stack[:stackTopIndex]
	return
}

func (ce *callEngine) popValues(v []uint64) {
	stackTopIndex := len(ce.stack) - len(v)
	copy(v, ce.stack[stackTopIndex:])
	ce.stack = ce.stack[:stackTopIndex]
}

// peekValues peeks api.ValueType values from the stack and returns them.
func (ce *callEngine) peekValues(count int) []uint64 {
	if count == 0 {
		return nil
	}
	stackLen := len(ce.stack)
	return ce.stack[stackLen-count : stackLen]
}

func (ce *callEngine) drop(raw uint64) {
	r := inclusiveRangeFromU64(raw)
	if r.Start == -1 {
		return
	} else if r.Start == 0 {
		ce.stack = ce.stack[:int32(len(ce.stack))-1-r.End]
	} else {
		newStack := ce.stack[:int32(len(ce.stack))-1-r.End]
		newStack = append(newStack, ce.stack[int32(len(ce.stack))-r.Start:]...)
		ce.stack = newStack
	}
}

func (ce *callEngine) pushFrame(frame *callFrame) {
	if callStackCeiling <= len(ce.frames) {
		panic(wasmruntime.ErrRuntimeStackOverflow)
	}
	ce.frames = append(ce.frames, frame)
}

func (ce *callEngine) popFrame() (frame *callFrame) {
	// No need to check stack bound as we can assume that all the operations are valid thanks to validateFunction at
	// module validation phase and interpreterir translation before compilation.
	oneLess := len(ce.frames) - 1
	frame = ce.frames[oneLess]
	ce.frames = ce.frames[:oneLess]
	return
}

type callFrame struct {
	// pc is the program counter representing the current position in code.body.
	pc uint64
	// f is the compiled function used in this function frame.
	f *function
	// base index in the frame of this function, used to detect the count of
	// values on the stack.
	base int
}

type compiledFunction struct {
	source              *wasm.Module
	body                []unionOperation
	listener            experimental.FunctionListener
	offsetsInWasmBinary []uint64
	hostFn              interface{}
	ensureTermination   bool
	index               wasm.Index
}

type function struct {
	funcType       *wasm.FunctionType
	moduleInstance *wasm.ModuleInstance
	typeID         wasm.FunctionTypeID
	parent         *compiledFunction
}

// functionFromUintptr resurrects the original *function from the given uintptr
// which comes from either funcref table or OpcodeRefFunc instruction.
func functionFromUintptr(ptr uintptr) *function {
	// Wraps ptrs as the double pointer in order to avoid the unsafe access as detected by race detector.
	//
	// For example, if we have (*function)(unsafe.Pointer(ptr)) instead, then the race detector's "checkptr"
	// subroutine wanrs as "checkptr: pointer arithmetic result points to invalid allocation"
	// https://github.com/golang/go/blob/1ce7fcf139417d618c2730010ede2afb41664211/src/runtime/checkptr.go#L69
	var wrapped *uintptr = &ptr
	return *(**function)(unsafe.Pointer(wrapped))
}

type snapshot struct {
	stack  []uint64
	frames []*callFrame
	pc     uint64

	ret []uint64

	ce *callEngine
}

// Snapshot implements the same method as documented on experimental.Snapshotter.
func (ce *callEngine) Snapshot() experimental.Snapshot {
	stack := make([]uint64, len(ce.stack))
	copy(stack, ce.stack)

	frames := make([]*callFrame, len(ce.frames))
	copy(frames, ce.frames)

	return &snapshot{
		stack:  stack,
		frames: frames,
		ce:     ce,
	}
}

// Restore implements the same method as documented on experimental.Snapshot.
func (s *snapshot) Restore(ret []uint64) {
	s.ret = ret
	panic(s)
}

func (s *snapshot) doRestore() {
	ce := s.ce

	ce.stack = s.stack
	ce.frames = s.frames
	ce.frames[len(ce.frames)-1].pc = s.pc

	copy(ce.stack[len(ce.stack)-len(s.ret):], s.ret)
}

// Error implements the same method on error.
func (s *snapshot) Error() string {
	return "unhandled snapshot restore, this generally indicates restore was called from a different " +
		"exported function invocation than snapshot"
}

// stackIterator implements experimental.StackIterator.
type stackIterator struct {
	stack   []uint64
	frames  []*callFrame
	started bool
	fn      *function
	pc      uint64
}

func (si *stackIterator) reset(stack []uint64, frames []*callFrame, f *function) {
	si.fn = f
	si.pc = 0
	si.stack = stack
	si.frames = frames
	si.started = false
}

func (si *stackIterator) clear() {
	si.stack = nil
	si.frames = nil
	si.started = false
	si.fn = nil
}

// Next implements the same method as documented on experimental.StackIterator.
func (si *stackIterator) Next() bool {
	if !si.started {
		si.started = true
		return true
	}

	if len(si.frames) == 0 {
		return false
	}

	frame := si.frames[len(si.frames)-1]
	si.stack = si.stack[:frame.base]
	si.fn = frame.f
	si.pc = frame.pc
	si.frames = si.frames[:len(si.frames)-1]
	return true
}

// Function implements the same method as documented on
// experimental.StackIterator.
func (si *stackIterator) Function() experimental.InternalFunction {
	return internalFunction{si.fn}
}

// ProgramCounter implements the same method as documented on
// experimental.StackIterator.
func (si *stackIterator) ProgramCounter() experimental.ProgramCounter {
	return experimental.ProgramCounter(si.pc)
}

// internalFunction implements experimental.InternalFunction.
type internalFunction struct{ *function }

// Definition implements the same method as documented on
// experimental.InternalFunction.
func (f internalFunction) Definition() api.FunctionDefinition {
	return f.definition()
}

// SourceOffsetForPC implements the same method as documented on
// experimental.InternalFunction.
func (f internalFunction) SourceOffsetForPC(pc experimental.ProgramCounter) uint64 {
	offsetsMap := f.parent.offsetsInWasmBinary
	if uint64(pc) < uint64(len(offsetsMap)) {
		return offsetsMap[pc]
	}
	return 0
}

// interpreter mode doesn't maintain call frames in the stack, so pass the zero size to the IR.
const callFrameStackSize = 0

// CompileModule implements the same method as documented on wasm.Engine.
func (e *engine) CompileModule(_ context.Context, module *wasm.Module, listeners []experimental.FunctionListener, ensureTermination bool) error {
	if _, ok := e.getCompiledFunctions(module); ok { // cache hit!
		return nil
	}

	funcs := make([]compiledFunction, len(module.FunctionSection))
	irCompiler, err := newCompiler(e.enabledFeatures, callFrameStackSize, module, ensureTermination)
	if err != nil {
		return err
	}
	imported := module.ImportFunctionCount
	for i := range module.CodeSection {
		var lsn experimental.FunctionListener
		if i < len(listeners) {
			lsn = listeners[i]
		}

		compiled := &funcs[i]
		// If this is the host function, there's nothing to do as the runtime representation of
		// host function in interpreter is its Go function itself as opposed to Wasm functions,
		// which need to be compiled down to
		if codeSeg := &module.CodeSection[i]; codeSeg.GoFunc != nil {
			compiled.hostFn = codeSeg.GoFunc
		} else {
			ir, err := irCompiler.Next()
			if err != nil {
				return err
			}
			err = e.lowerIR(ir, compiled)
			if err != nil {
				def := module.FunctionDefinition(uint32(i) + module.ImportFunctionCount)
				return fmt.Errorf("failed to lower func[%s] to interpreterir: %w", def.DebugName(), err)
			}
		}
		compiled.source = module
		compiled.ensureTermination = ensureTermination
		compiled.listener = lsn
		compiled.index = imported + uint32(i)
	}
	e.addCompiledFunctions(module, funcs)
	return nil
}

// NewModuleEngine implements the same method as documented on wasm.Engine.
func (e *engine) NewModuleEngine(module *wasm.Module, instance *wasm.ModuleInstance) (wasm.ModuleEngine, error) {
	me := &moduleEngine{
		parentEngine: e,
		functions:    make([]function, len(module.FunctionSection)+int(module.ImportFunctionCount)),
	}

	codes, ok := e.getCompiledFunctions(module)
	if !ok {
		return nil, errors.New("source module must be compiled before instantiation")
	}

	for i := range codes {
		c := &codes[i]
		offset := i + int(module.ImportFunctionCount)
		typeIndex := module.FunctionSection[i]
		me.functions[offset] = function{
			moduleInstance: instance,
			typeID:         instance.TypeIDs[typeIndex],
			funcType:       &module.TypeSection[typeIndex],
			parent:         c,
		}
	}
	return me, nil
}

// lowerIR lowers the interpreterir operations to engine friendly struct.
func (e *engine) lowerIR(ir *compilationResult, ret *compiledFunction) error {
	// Copy the body from the result.
	ret.body = make([]unionOperation, len(ir.Operations))
	copy(ret.body, ir.Operations)
	// Also copy the offsets if necessary.
	if offsets := ir.IROperationSourceOffsetsInWasmBinary; len(offsets) > 0 {
		ret.offsetsInWasmBinary = make([]uint64, len(offsets))
		copy(ret.offsetsInWasmBinary, offsets)
	}

	labelAddressResolutions := [labelKindNum][]uint64{}

	// First, we iterate all labels, and resolve the address.
	for i := range ret.body {
		op := &ret.body[i]
		switch op.Kind {
		case operationKindLabel:
			label := label(op.U1)
			address := uint64(i)

			kind, fid := label.Kind(), label.FrameID()
			frameToAddresses := labelAddressResolutions[label.Kind()]
			// Expand the slice if necessary.
			if diff := fid - len(frameToAddresses) + 1; diff > 0 {
				for j := 0; j < diff; j++ {
					frameToAddresses = append(frameToAddresses, 0)
				}
			}
			frameToAddresses[fid] = address
			labelAddressResolutions[kind] = frameToAddresses
		}
	}

	// Then resolve the label as the index to the body.
	for i := range ret.body {
		op := &ret.body[i]
		switch op.Kind {
		case operationKindBr:
			e.setLabelAddress(&op.U1, label(op.U1), labelAddressResolutions)
		case operationKindBrIf:
			e.setLabelAddress(&op.U1, label(op.U1), labelAddressResolutions)
			e.setLabelAddress(&op.U2, label(op.U2), labelAddressResolutions)
		case operationKindBrTable:
			for j := 0; j < len(op.Us); j += 2 {
				target := op.Us[j]
				e.setLabelAddress(&op.Us[j], label(target), labelAddressResolutions)
			}
		}
	}
	return nil
}

func (e *engine) setLabelAddress(op *uint64, label label, labelAddressResolutions [labelKindNum][]uint64) {
	if label.IsReturnTarget() {
		// Jmp to the end of the possible binary.
		*op = math.MaxUint64
	} else {
		*op = labelAddressResolutions[label.Kind()][label.FrameID()]
	}
}

// ResolveImportedFunction implements wasm.ModuleEngine.
func (e *moduleEngine) ResolveImportedFunction(index, descFunc, indexInImportedModule wasm.Index, importedModuleEngine wasm.ModuleEngine) {
	imported := importedModuleEngine.(*moduleEngine)
	e.functions[index] = imported.functions[indexInImportedModule]
}

// ResolveImportedMemory implements wasm.ModuleEngine.
func (e *moduleEngine) ResolveImportedMemory(wasm.ModuleEngine) {}

// DoneInstantiation implements wasm.ModuleEngine.
func (e *moduleEngine) DoneInstantiation() {}

// FunctionInstanceReference implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) FunctionInstanceReference(funcIndex wasm.Index) wasm.Reference {
	return uintptr(unsafe.Pointer(&e.functions[funcIndex]))
}

// NewFunction implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) NewFunction(index wasm.Index) (ce api.Function) {
	// Note: The input parameters are pre-validated, so a compiled function is only absent on close. Updates to
	// code on close aren't locked, neither is this read.
	compiled := &e.functions[index]
	return e.newCallEngine(compiled)
}

// LookupFunction implements the same method as documented on wasm.ModuleEngine.
func (e *moduleEngine) LookupFunction(t *wasm.TableInstance, typeId wasm.FunctionTypeID, tableOffset wasm.Index) (*wasm.ModuleInstance, wasm.Index) {
	if tableOffset >= uint32(len(t.References)) {
		panic(wasmruntime.ErrRuntimeInvalidTableAccess)
	}
	rawPtr := t.References[tableOffset]
	if rawPtr == 0 {
		panic(wasmruntime.ErrRuntimeInvalidTableAccess)
	}

	tf := functionFromUintptr(rawPtr)
	if tf.typeID != typeId {
		panic(wasmruntime.ErrRuntimeIndirectCallTypeMismatch)
	}
	return tf.moduleInstance, tf.parent.index
}

// Definition implements the same method as documented on api.Function.
func (ce *callEngine) Definition() api.FunctionDefinition {
	return ce.f.definition()
}

func (f *function) definition() api.FunctionDefinition {
	compiled := f.parent
	return compiled.source.FunctionDefinition(compiled.index)
}

// Call implements the same method as documented on api.Function.
func (ce *callEngine) Call(ctx context.Context, params ...uint64) (results []uint64, err error) {
	ft := ce.f.funcType
	if n := ft.ParamNumInUint64; n != len(params) {
		return nil, fmt.Errorf("expected %d params, but passed %d", n, len(params))
	}
	return ce.call(ctx, params, nil)
}

// CallWithStack implements the same method as documented on api.Function.
func (ce *callEngine) CallWithStack(ctx context.Context, stack []uint64) error {
	params, results, err := wasm.SplitCallStack(ce.f.funcType, stack)
	if err != nil {
		return err
	}
	_, err = ce.call(ctx, params, results)
	return err
}

func (ce *callEngine) call(ctx context.Context, params, results []uint64) (_ []uint64, err error) {
	m := ce.f.moduleInstance
	if ce.f.parent.ensureTermination {
		select {
		case <-ctx.Done():
			// If the provided context is already done, close the call context
			// and return the error.
			m.CloseWithCtxErr(ctx)
			return nil, m.FailIfClosed()
		default:
		}
	}

	if ctx.Value(expctxkeys.EnableSnapshotterKey{}) != nil {
		ctx = context.WithValue(ctx, expctxkeys.SnapshotterKey{}, ce)
	}

	defer func() {
		// If the module closed during the call, and the call didn't err for another reason, set an ExitError.
		if err == nil {
			err = m.FailIfClosed()
		}
		// TODO: ^^ Will not fail if the function was imported from a closed module.

		if v := recover(); v != nil {
			err = ce.recoverOnCall(ctx, m, v)
		}
	}()

	ce.pushValues(params)

	if ce.f.parent.ensureTermination {
		done := m.CloseModuleOnCanceledOrTimeout(ctx)
		defer done()
	}

	ce.callFunction(ctx, m, ce.f)

	// This returns a safe copy of the results, instead of a slice view. If we
	// returned a re-slice, the caller could accidentally or purposefully
	// corrupt the stack of subsequent calls.
	ft := ce.f.funcType
	if results == nil && ft.ResultNumInUint64 > 0 {
		results = make([]uint64, ft.ResultNumInUint64)
	}
	ce.popValues(results)
	return results, nil
}

// functionListenerInvocation captures arguments needed to perform function
// listener invocations when unwinding the call stack.
type functionListenerInvocation struct {
	experimental.FunctionListener
	def api.FunctionDefinition
}

// recoverOnCall takes the recovered value `recoverOnCall`, and wraps it
// with the call frame stack traces. Also, reset the state of callEngine
// so that it can be used for the subsequent calls.
func (ce *callEngine) recoverOnCall(ctx context.Context, m *wasm.ModuleInstance, v interface{}) (err error) {
	if s, ok := v.(*snapshot); ok {
		// A snapshot that wasn't handled was created by a different call engine possibly from a nested wasm invocation,
		// let it propagate up to be handled by the caller.
		panic(s)
	}

	builder := wasmdebug.NewErrorBuilder()
	frameCount := len(ce.frames)
	functionListeners := make([]functionListenerInvocation, 0, 16)

	if frameCount > wasmdebug.MaxFrames {
		frameCount = wasmdebug.MaxFrames
	}
	for i := 0; i < frameCount; i++ {
		frame := ce.popFrame()
		f := frame.f
		def := f.definition()
		var sources []string
		if parent := frame.f.parent; parent.body != nil && len(parent.offsetsInWasmBinary) > 0 {
			sources = parent.source.DWARFLines.Line(parent.offsetsInWasmBinary[frame.pc])
		}
		builder.AddFrame(def.DebugName(), def.ParamTypes(), def.ResultTypes(), sources)
		if f.parent.listener != nil {
			functionListeners = append(functionListeners, functionListenerInvocation{
				FunctionListener: f.parent.listener,
				def:              f.definition(),
			})
		}
	}

	err = builder.FromRecovered(v)
	for i := range functionListeners {
		functionListeners[i].Abort(ctx, m, functionListeners[i].def, err)
	}

	// Allows the reuse of CallEngine.
	ce.stack, ce.frames = ce.stack[:0], ce.frames[:0]
	return
}

func (ce *callEngine) callFunction(ctx context.Context, m *wasm.ModuleInstance, f *function) {
	if f.parent.hostFn != nil {
		ce.callGoFuncWithStack(ctx, m, f)
	} else if lsn := f.parent.listener; lsn != nil {
		ce.callNativeFuncWithListener(ctx, m, f, lsn)
	} else {
		ce.callNativeFunc(ctx, m, f)
	}
}

func (ce *callEngine) callGoFunc(ctx context.Context, m *wasm.ModuleInstance, f *function, stack []uint64) {
	typ := f.funcType
	lsn := f.parent.listener
	if lsn != nil {
		params := stack[:typ.ParamNumInUint64]
		ce.stackIterator.reset(ce.stack, ce.frames, f)
		lsn.Before(ctx, m, f.definition(), params, &ce.stackIterator)
		ce.stackIterator.clear()
	}
	frame := &callFrame{f: f, base: len(ce.stack)}
	ce.pushFrame(frame)

	fn := f.parent.hostFn
	switch fn := fn.(type) {
	case api.GoModuleFunction:
		fn.Call(ctx, m, stack)
	case api.GoFunction:
		fn.Call(ctx, stack)
	}

	ce.popFrame()
	if lsn != nil {
		// TODO: This doesn't get the error due to use of panic to propagate them.
		results := stack[:typ.ResultNumInUint64]
		lsn.After(ctx, m, f.definition(), results)
	}
}

func (ce *callEngine) callNativeFunc(ctx context.Context, m *wasm.ModuleInstance, f *function) {
	frame := &callFrame{f: f, base: len(ce.stack)}
	moduleInst := f.moduleInstance
	functions := moduleInst.Engine.(*moduleEngine).functions
	memoryInst := moduleInst.MemoryInstance
	globals := moduleInst.Globals
	tables := moduleInst.Tables
	typeIDs := moduleInst.TypeIDs
	dataInstances := moduleInst.DataInstances
	elementInstances := moduleInst.ElementInstances
	ce.pushFrame(frame)
	body := frame.f.parent.body
	bodyLen := uint64(len(body))
	for frame.pc < bodyLen {
		op := &body[frame.pc]
		// TODO: add description of each operation/case
		// on, for example, how many args are used,
		// how the stack is modified, etc.
		switch op.Kind {
		case operationKindBuiltinFunctionCheckExitCode:
			if err := m.FailIfClosed(); err != nil {
				panic(err)
			}
			frame.pc++
		case operationKindUnreachable:
			panic(wasmruntime.ErrRuntimeUnreachable)
		case operationKindBr:
			frame.pc = op.U1
		case operationKindBrIf:
			if ce.popValue() > 0 {
				ce.drop(op.U3)
				frame.pc = op.U1
			} else {
				frame.pc = op.U2
			}
		case operationKindBrTable:
			v := ce.popValue()
			defaultAt := uint64(len(op.Us))/2 - 1
			if v > defaultAt {
				v = defaultAt
			}
			v *= 2
			ce.drop(op.Us[v+1])
			frame.pc = op.Us[v]
		case operationKindCall:
			func() {
				if ctx.Value(expctxkeys.EnableSnapshotterKey{}) != nil {
					defer func() {
						if r := recover(); r != nil {
							if s, ok := r.(*snapshot); ok && s.ce == ce {
								s.doRestore()
								frame = ce.frames[len(ce.frames)-1]
								body = frame.f.parent.body
								bodyLen = uint64(len(body))
							} else {
								panic(r)
							}
						}
					}()
				}
				ce.callFunction(ctx, f.moduleInstance, &functions[op.U1])
			}()
			frame.pc++
		case operationKindCallIndirect:
			offset := ce.popValue()
			table := tables[op.U2]
			if offset >= uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			}
			rawPtr := table.References[offset]
			if rawPtr == 0 {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			}

			tf := functionFromUintptr(rawPtr)
			if tf.typeID != typeIDs[op.U1] {
				panic(wasmruntime.ErrRuntimeIndirectCallTypeMismatch)
			}

			ce.callFunction(ctx, f.moduleInstance, tf)
			frame.pc++
		case operationKindDrop:
			ce.drop(op.U1)
			frame.pc++
		case operationKindSelect:
			c := ce.popValue()
			if op.B3 { // Target is vector.
				x2Hi, x2Lo := ce.popValue(), ce.popValue()
				if c == 0 {
					_, _ = ce.popValue(), ce.popValue() // discard the x1's lo and hi bits.
					ce.pushValue(x2Lo)
					ce.pushValue(x2Hi)
				}
			} else {
				v2 := ce.popValue()
				if c == 0 {
					_ = ce.popValue()
					ce.pushValue(v2)
				}
			}
			frame.pc++
		case operationKindPick:
			index := len(ce.stack) - 1 - int(op.U1)
			ce.pushValue(ce.stack[index])
			if op.B3 { // V128 value target.
				ce.pushValue(ce.stack[index+1])
			}
			frame.pc++
		case operationKindSet:
			if op.B3 { // V128 value target.
				lowIndex := len(ce.stack) - 1 - int(op.U1)
				highIndex := lowIndex + 1
				hi, lo := ce.popValue(), ce.popValue()
				ce.stack[lowIndex], ce.stack[highIndex] = lo, hi
			} else {
				index := len(ce.stack) - 1 - int(op.U1)
				ce.stack[index] = ce.popValue()
			}
			frame.pc++
		case operationKindGlobalGet:
			g := globals[op.U1]
			ce.pushValue(g.Val)
			if g.Type.ValType == wasm.ValueTypeV128 {
				ce.pushValue(g.ValHi)
			}
			frame.pc++
		case operationKindGlobalSet:
			g := globals[op.U1]
			if g.Type.ValType == wasm.ValueTypeV128 {
				g.ValHi = ce.popValue()
			}
			g.Val = ce.popValue()
			frame.pc++
		case operationKindLoad:
			offset := ce.popMemoryOffset(op)
			switch unsignedType(op.B1) {
			case unsignedTypeI32, unsignedTypeF32:
				if val, ok := memoryInst.ReadUint32Le(offset); !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				} else {
					ce.pushValue(uint64(val))
				}
			case unsignedTypeI64, unsignedTypeF64:
				if val, ok := memoryInst.ReadUint64Le(offset); !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				} else {
					ce.pushValue(val)
				}
			}
			frame.pc++
		case operationKindLoad8:
			val, ok := memoryInst.ReadByte(ce.popMemoryOffset(op))
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}

			switch signedInt(op.B1) {
			case signedInt32:
				ce.pushValue(uint64(uint32(int8(val))))
			case signedInt64:
				ce.pushValue(uint64(int8(val)))
			case signedUint32, signedUint64:
				ce.pushValue(uint64(val))
			}
			frame.pc++
		case operationKindLoad16:

			val, ok := memoryInst.ReadUint16Le(ce.popMemoryOffset(op))
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}

			switch signedInt(op.B1) {
			case signedInt32:
				ce.pushValue(uint64(uint32(int16(val))))
			case signedInt64:
				ce.pushValue(uint64(int16(val)))
			case signedUint32, signedUint64:
				ce.pushValue(uint64(val))
			}
			frame.pc++
		case operationKindLoad32:
			val, ok := memoryInst.ReadUint32Le(ce.popMemoryOffset(op))
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}

			if op.B1 == 1 { // Signed
				ce.pushValue(uint64(int32(val)))
			} else {
				ce.pushValue(uint64(val))
			}
			frame.pc++
		case operationKindStore:
			val := ce.popValue()
			offset := ce.popMemoryOffset(op)
			switch unsignedType(op.B1) {
			case unsignedTypeI32, unsignedTypeF32:
				if !memoryInst.WriteUint32Le(offset, uint32(val)) {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
			case unsignedTypeI64, unsignedTypeF64:
				if !memoryInst.WriteUint64Le(offset, val) {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
			}
			frame.pc++
		case operationKindStore8:
			val := byte(ce.popValue())
			offset := ce.popMemoryOffset(op)
			if !memoryInst.WriteByte(offset, val) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case operationKindStore16:
			val := uint16(ce.popValue())
			offset := ce.popMemoryOffset(op)
			if !memoryInst.WriteUint16Le(offset, val) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case operationKindStore32:
			val := uint32(ce.popValue())
			offset := ce.popMemoryOffset(op)
			if !memoryInst.WriteUint32Le(offset, val) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case operationKindMemorySize:
			ce.pushValue(uint64(memoryInst.Pages()))
			frame.pc++
		case operationKindMemoryGrow:
			n := ce.popValue()
			if res, ok := memoryInst.Grow(uint32(n)); !ok {
				ce.pushValue(uint64(0xffffffff)) // = -1 in signed 32-bit integer.
			} else {
				ce.pushValue(uint64(res))
			}
			frame.pc++
		case operationKindConstI32, operationKindConstI64,
			operationKindConstF32, operationKindConstF64:
			ce.pushValue(op.U1)
			frame.pc++
		case operationKindEq:
			var b bool
			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				v2, v1 := ce.popValue(), ce.popValue()
				b = uint32(v1) == uint32(v2)
			case unsignedTypeI64:
				v2, v1 := ce.popValue(), ce.popValue()
				b = v1 == v2
			case unsignedTypeF32:
				v2, v1 := ce.popValue(), ce.popValue()
				b = math.Float32frombits(uint32(v2)) == math.Float32frombits(uint32(v1))
			case unsignedTypeF64:
				v2, v1 := ce.popValue(), ce.popValue()
				b = math.Float64frombits(v2) == math.Float64frombits(v1)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindNe:
			var b bool
			switch unsignedType(op.B1) {
			case unsignedTypeI32, unsignedTypeI64:
				v2, v1 := ce.popValue(), ce.popValue()
				b = v1 != v2
			case unsignedTypeF32:
				v2, v1 := ce.popValue(), ce.popValue()
				b = math.Float32frombits(uint32(v2)) != math.Float32frombits(uint32(v1))
			case unsignedTypeF64:
				v2, v1 := ce.popValue(), ce.popValue()
				b = math.Float64frombits(v2) != math.Float64frombits(v1)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindEqz:
			if ce.popValue() == 0 {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindLt:
			v2 := ce.popValue()
			v1 := ce.popValue()
			var b bool
			switch signedType(op.B1) {
			case signedTypeInt32:
				b = int32(v1) < int32(v2)
			case signedTypeInt64:
				b = int64(v1) < int64(v2)
			case signedTypeUint32, signedTypeUint64:
				b = v1 < v2
			case signedTypeFloat32:
				b = math.Float32frombits(uint32(v1)) < math.Float32frombits(uint32(v2))
			case signedTypeFloat64:
				b = math.Float64frombits(v1) < math.Float64frombits(v2)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindGt:
			v2 := ce.popValue()
			v1 := ce.popValue()
			var b bool
			switch signedType(op.B1) {
			case signedTypeInt32:
				b = int32(v1) > int32(v2)
			case signedTypeInt64:
				b = int64(v1) > int64(v2)
			case signedTypeUint32, signedTypeUint64:
				b = v1 > v2
			case signedTypeFloat32:
				b = math.Float32frombits(uint32(v1)) > math.Float32frombits(uint32(v2))
			case signedTypeFloat64:
				b = math.Float64frombits(v1) > math.Float64frombits(v2)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindLe:
			v2 := ce.popValue()
			v1 := ce.popValue()
			var b bool
			switch signedType(op.B1) {
			case signedTypeInt32:
				b = int32(v1) <= int32(v2)
			case signedTypeInt64:
				b = int64(v1) <= int64(v2)
			case signedTypeUint32, signedTypeUint64:
				b = v1 <= v2
			case signedTypeFloat32:
				b = math.Float32frombits(uint32(v1)) <= math.Float32frombits(uint32(v2))
			case signedTypeFloat64:
				b = math.Float64frombits(v1) <= math.Float64frombits(v2)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindGe:
			v2 := ce.popValue()
			v1 := ce.popValue()
			var b bool
			switch signedType(op.B1) {
			case signedTypeInt32:
				b = int32(v1) >= int32(v2)
			case signedTypeInt64:
				b = int64(v1) >= int64(v2)
			case signedTypeUint32, signedTypeUint64:
				b = v1 >= v2
			case signedTypeFloat32:
				b = math.Float32frombits(uint32(v1)) >= math.Float32frombits(uint32(v2))
			case signedTypeFloat64:
				b = math.Float64frombits(v1) >= math.Float64frombits(v2)
			}
			if b {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindAdd:
			v2 := ce.popValue()
			v1 := ce.popValue()
			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				v := uint32(v1) + uint32(v2)
				ce.pushValue(uint64(v))
			case unsignedTypeI64:
				ce.pushValue(v1 + v2)
			case unsignedTypeF32:
				ce.pushValue(addFloat32bits(uint32(v1), uint32(v2)))
			case unsignedTypeF64:
				v := math.Float64frombits(v1) + math.Float64frombits(v2)
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case operationKindSub:
			v2 := ce.popValue()
			v1 := ce.popValue()
			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				ce.pushValue(uint64(uint32(v1) - uint32(v2)))
			case unsignedTypeI64:
				ce.pushValue(v1 - v2)
			case unsignedTypeF32:
				ce.pushValue(subFloat32bits(uint32(v1), uint32(v2)))
			case unsignedTypeF64:
				v := math.Float64frombits(v1) - math.Float64frombits(v2)
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case operationKindMul:
			v2 := ce.popValue()
			v1 := ce.popValue()
			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				ce.pushValue(uint64(uint32(v1) * uint32(v2)))
			case unsignedTypeI64:
				ce.pushValue(v1 * v2)
			case unsignedTypeF32:
				ce.pushValue(mulFloat32bits(uint32(v1), uint32(v2)))
			case unsignedTypeF64:
				v := math.Float64frombits(v2) * math.Float64frombits(v1)
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case operationKindClz:
			v := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(bits.LeadingZeros32(uint32(v))))
			} else {
				// unsignedInt64
				ce.pushValue(uint64(bits.LeadingZeros64(v)))
			}
			frame.pc++
		case operationKindCtz:
			v := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(bits.TrailingZeros32(uint32(v))))
			} else {
				// unsignedInt64
				ce.pushValue(uint64(bits.TrailingZeros64(v)))
			}
			frame.pc++
		case operationKindPopcnt:
			v := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(bits.OnesCount32(uint32(v))))
			} else {
				// unsignedInt64
				ce.pushValue(uint64(bits.OnesCount64(v)))
			}
			frame.pc++
		case operationKindDiv:
			// If an integer, check we won't divide by zero.
			t := signedType(op.B1)
			v2, v1 := ce.popValue(), ce.popValue()
			switch t {
			case signedTypeFloat32, signedTypeFloat64: // not integers
			default:
				if v2 == 0 {
					panic(wasmruntime.ErrRuntimeIntegerDivideByZero)
				}
			}

			switch t {
			case signedTypeInt32:
				d := int32(v2)
				n := int32(v1)
				if n == math.MinInt32 && d == -1 {
					panic(wasmruntime.ErrRuntimeIntegerOverflow)
				}
				ce.pushValue(uint64(uint32(n / d)))
			case signedTypeInt64:
				d := int64(v2)
				n := int64(v1)
				if n == math.MinInt64 && d == -1 {
					panic(wasmruntime.ErrRuntimeIntegerOverflow)
				}
				ce.pushValue(uint64(n / d))
			case signedTypeUint32:
				d := uint32(v2)
				n := uint32(v1)
				ce.pushValue(uint64(n / d))
			case signedTypeUint64:
				d := v2
				n := v1
				ce.pushValue(n / d)
			case signedTypeFloat32:
				ce.pushValue(divFloat32bits(uint32(v1), uint32(v2)))
			case signedTypeFloat64:
				ce.pushValue(math.Float64bits(math.Float64frombits(v1) / math.Float64frombits(v2)))
			}
			frame.pc++
		case operationKindRem:
			v2, v1 := ce.popValue(), ce.popValue()
			if v2 == 0 {
				panic(wasmruntime.ErrRuntimeIntegerDivideByZero)
			}
			switch signedInt(op.B1) {
			case signedInt32:
				d := int32(v2)
				n := int32(v1)
				ce.pushValue(uint64(uint32(n % d)))
			case signedInt64:
				d := int64(v2)
				n := int64(v1)
				ce.pushValue(uint64(n % d))
			case signedUint32:
				d := uint32(v2)
				n := uint32(v1)
				ce.pushValue(uint64(n % d))
			case signedUint64:
				d := v2
				n := v1
				ce.pushValue(n % d)
			}
			frame.pc++
		case operationKindAnd:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(uint32(v2) & uint32(v1)))
			} else {
				// unsignedInt64
				ce.pushValue(uint64(v2 & v1))
			}
			frame.pc++
		case operationKindOr:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(uint32(v2) | uint32(v1)))
			} else {
				// unsignedInt64
				ce.pushValue(uint64(v2 | v1))
			}
			frame.pc++
		case operationKindXor:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(uint32(v2) ^ uint32(v1)))
			} else {
				// unsignedInt64
				ce.pushValue(uint64(v2 ^ v1))
			}
			frame.pc++
		case operationKindShl:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(uint32(v1) << (uint32(v2) % 32)))
			} else {
				// unsignedInt64
				ce.pushValue(v1 << (v2 % 64))
			}
			frame.pc++
		case operationKindShr:
			v2 := ce.popValue()
			v1 := ce.popValue()
			switch signedInt(op.B1) {
			case signedInt32:
				ce.pushValue(uint64(uint32(int32(v1) >> (uint32(v2) % 32))))
			case signedInt64:
				ce.pushValue(uint64(int64(v1) >> (v2 % 64)))
			case signedUint32:
				ce.pushValue(uint64(uint32(v1) >> (uint32(v2) % 32)))
			case signedUint64:
				ce.pushValue(v1 >> (v2 % 64))
			}
			frame.pc++
		case operationKindRotl:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(bits.RotateLeft32(uint32(v1), int(v2))))
			} else {
				// unsignedInt64
				ce.pushValue(uint64(bits.RotateLeft64(v1, int(v2))))
			}
			frame.pc++
		case operationKindRotr:
			v2 := ce.popValue()
			v1 := ce.popValue()
			if op.B1 == 0 {
				// unsignedInt32
				ce.pushValue(uint64(bits.RotateLeft32(uint32(v1), -int(v2))))
			} else {
				// unsignedInt64
				ce.pushValue(uint64(bits.RotateLeft64(v1, -int(v2))))
			}
			frame.pc++
		case operationKindAbs:
			if op.B1 == 0 {
				// float32
				const mask uint32 = 1 << 31
				ce.pushValue(uint64(uint32(ce.popValue()) &^ mask))
			} else {
				// float64
				const mask uint64 = 1 << 63
				ce.pushValue(ce.popValue() &^ mask)
			}
			frame.pc++
		case operationKindNeg:
			if op.B1 == 0 {
				// float32
				v := -math.Float32frombits(uint32(ce.popValue()))
				ce.pushValue(uint64(math.Float32bits(v)))
			} else {
				// float64
				v := -math.Float64frombits(ce.popValue())
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case operationKindCeil:
			if op.B1 == 0 {
				// float32
				v := moremath.WasmCompatCeilF32(math.Float32frombits(uint32(ce.popValue())))
				ce.pushValue(uint64(math.Float32bits(v)))
			} else {
				// float64
				v := moremath.WasmCompatCeilF64(math.Float64frombits(ce.popValue()))
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case operationKindFloor:
			if op.B1 == 0 {
				// float32
				v := moremath.WasmCompatFloorF32(math.Float32frombits(uint32(ce.popValue())))
				ce.pushValue(uint64(math.Float32bits(v)))
			} else {
				// float64
				v := moremath.WasmCompatFloorF64(math.Float64frombits(ce.popValue()))
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case operationKindTrunc:
			if op.B1 == 0 {
				// float32
				v := moremath.WasmCompatTruncF32(math.Float32frombits(uint32(ce.popValue())))
				ce.pushValue(uint64(math.Float32bits(v)))
			} else {
				// float64
				v := moremath.WasmCompatTruncF64(math.Float64frombits(ce.popValue()))
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case operationKindNearest:
			if op.B1 == 0 {
				// float32
				f := math.Float32frombits(uint32(ce.popValue()))
				ce.pushValue(uint64(math.Float32bits(moremath.WasmCompatNearestF32(f))))
			} else {
				// float64
				f := math.Float64frombits(ce.popValue())
				ce.pushValue(math.Float64bits(moremath.WasmCompatNearestF64(f)))
			}
			frame.pc++
		case operationKindSqrt:
			if op.B1 == 0 {
				// float32
				v := math.Sqrt(float64(math.Float32frombits(uint32(ce.popValue()))))
				ce.pushValue(uint64(math.Float32bits(float32(v))))
			} else {
				// float64
				v := math.Sqrt(math.Float64frombits(ce.popValue()))
				ce.pushValue(math.Float64bits(v))
			}
			frame.pc++
		case operationKindMin:
			if op.B1 == 0 {
				// float32
				ce.pushValue(wasmCompatMin32bits(uint32(ce.popValue()), uint32(ce.popValue())))
			} else {
				v2 := math.Float64frombits(ce.popValue())
				v1 := math.Float64frombits(ce.popValue())
				ce.pushValue(math.Float64bits(moremath.WasmCompatMin64(v1, v2)))
			}
			frame.pc++
		case operationKindMax:
			if op.B1 == 0 {
				ce.pushValue(wasmCompatMax32bits(uint32(ce.popValue()), uint32(ce.popValue())))
			} else {
				// float64
				v2 := math.Float64frombits(ce.popValue())
				v1 := math.Float64frombits(ce.popValue())
				ce.pushValue(math.Float64bits(moremath.WasmCompatMax64(v1, v2)))
			}
			frame.pc++
		case operationKindCopysign:
			if op.B1 == 0 {
				// float32
				v2 := uint32(ce.popValue())
				v1 := uint32(ce.popValue())
				const signbit = 1 << 31
				ce.pushValue(uint64(v1&^signbit | v2&signbit))
			} else {
				// float64
				v2 := ce.popValue()
				v1 := ce.popValue()
				const signbit = 1 << 63
				ce.pushValue(v1&^signbit | v2&signbit)
			}
			frame.pc++
		case operationKindI32WrapFromI64:
			ce.pushValue(uint64(uint32(ce.popValue())))
			frame.pc++
		case operationKindITruncFromF:
			if op.B1 == 0 {
				// float32
				switch signedInt(op.B2) {
				case signedInt32:
					v := math.Trunc(float64(math.Float32frombits(uint32(ce.popValue()))))
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.B3 {
							// non-trapping conversion must cast nan to zero.
							v = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < math.MinInt32 || v > math.MaxInt32 {
						if op.B3 {
							// non-trapping conversion must "saturate" the value for overflowing sources.
							if v < 0 {
								v = math.MinInt32
							} else {
								v = math.MaxInt32
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(uint32(int32(v))))
				case signedInt64:
					v := math.Trunc(float64(math.Float32frombits(uint32(ce.popValue()))))
					res := int64(v)
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.B3 {
							// non-trapping conversion must cast nan to zero.
							res = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < math.MinInt64 || v >= math.MaxInt64 {
						// Note: math.MaxInt64 is rounded up to math.MaxInt64+1 in 64-bit float representation,
						// and that's why we use '>=' not '>' to check overflow.
						if op.B3 {
							// non-trapping conversion must "saturate" the value for overflowing sources.
							if v < 0 {
								res = math.MinInt64
							} else {
								res = math.MaxInt64
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(res))
				case signedUint32:
					v := math.Trunc(float64(math.Float32frombits(uint32(ce.popValue()))))
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.B3 {
							// non-trapping conversion must cast nan to zero.
							v = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < 0 || v > math.MaxUint32 {
						if op.B3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								v = 0
							} else {
								v = math.MaxUint32
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(uint32(v)))
				case signedUint64:
					v := math.Trunc(float64(math.Float32frombits(uint32(ce.popValue()))))
					res := uint64(v)
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.B3 {
							// non-trapping conversion must cast nan to zero.
							res = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < 0 || v >= math.MaxUint64 {
						// Note: math.MaxUint64 is rounded up to math.MaxUint64+1 in 64-bit float representation,
						// and that's why we use '>=' not '>' to check overflow.
						if op.B3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								res = 0
							} else {
								res = math.MaxUint64
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(res)
				}
			} else {
				// float64
				switch signedInt(op.B2) {
				case signedInt32:
					v := math.Trunc(math.Float64frombits(ce.popValue()))
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.B3 {
							// non-trapping conversion must cast nan to zero.
							v = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < math.MinInt32 || v > math.MaxInt32 {
						if op.B3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								v = math.MinInt32
							} else {
								v = math.MaxInt32
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(uint32(int32(v))))
				case signedInt64:
					v := math.Trunc(math.Float64frombits(ce.popValue()))
					res := int64(v)
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.B3 {
							// non-trapping conversion must cast nan to zero.
							res = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < math.MinInt64 || v >= math.MaxInt64 {
						// Note: math.MaxInt64 is rounded up to math.MaxInt64+1 in 64-bit float representation,
						// and that's why we use '>=' not '>' to check overflow.
						if op.B3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								res = math.MinInt64
							} else {
								res = math.MaxInt64
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(res))
				case signedUint32:
					v := math.Trunc(math.Float64frombits(ce.popValue()))
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.B3 {
							// non-trapping conversion must cast nan to zero.
							v = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < 0 || v > math.MaxUint32 {
						if op.B3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								v = 0
							} else {
								v = math.MaxUint32
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(uint64(uint32(v)))
				case signedUint64:
					v := math.Trunc(math.Float64frombits(ce.popValue()))
					res := uint64(v)
					if math.IsNaN(v) { // NaN cannot be compared with themselves, so we have to use IsNaN
						if op.B3 {
							// non-trapping conversion must cast nan to zero.
							res = 0
						} else {
							panic(wasmruntime.ErrRuntimeInvalidConversionToInteger)
						}
					} else if v < 0 || v >= math.MaxUint64 {
						// Note: math.MaxUint64 is rounded up to math.MaxUint64+1 in 64-bit float representation,
						// and that's why we use '>=' not '>' to check overflow.
						if op.B3 {
							// non-trapping conversion must "saturate" the value for overflowing source.
							if v < 0 {
								res = 0
							} else {
								res = math.MaxUint64
							}
						} else {
							panic(wasmruntime.ErrRuntimeIntegerOverflow)
						}
					}
					ce.pushValue(res)
				}
			}
			frame.pc++
		case operationKindFConvertFromI:
			switch signedInt(op.B1) {
			case signedInt32:
				if op.B2 == 0 {
					// float32
					v := float32(int32(ce.popValue()))
					ce.pushValue(uint64(math.Float32bits(v)))
				} else {
					// float64
					v := float64(int32(ce.popValue()))
					ce.pushValue(math.Float64bits(v))
				}
			case signedInt64:
				if op.B2 == 0 {
					// float32
					v := float32(int64(ce.popValue()))
					ce.pushValue(uint64(math.Float32bits(v)))
				} else {
					// float64
					v := float64(int64(ce.popValue()))
					ce.pushValue(math.Float64bits(v))
				}
			case signedUint32:
				if op.B2 == 0 {
					// float32
					v := float32(uint32(ce.popValue()))
					ce.pushValue(uint64(math.Float32bits(v)))
				} else {
					// float64
					v := float64(uint32(ce.popValue()))
					ce.pushValue(math.Float64bits(v))
				}
			case signedUint64:
				if op.B2 == 0 {
					// float32
					v := float32(ce.popValue())
					ce.pushValue(uint64(math.Float32bits(v)))
				} else {
					// float64
					v := float64(ce.popValue())
					ce.pushValue(math.Float64bits(v))
				}
			}
			frame.pc++
		case operationKindF32DemoteFromF64:
			v := float32(math.Float64frombits(ce.popValue()))
			ce.pushValue(uint64(math.Float32bits(v)))
			frame.pc++
		case operationKindF64PromoteFromF32:
			v := float64(math.Float32frombits(uint32(ce.popValue())))
			ce.pushValue(math.Float64bits(v))
			frame.pc++
		case operationKindExtend:
			if op.B1 == 1 {
				// Signed.
				v := int64(int32(ce.popValue()))
				ce.pushValue(uint64(v))
			} else {
				v := uint64(uint32(ce.popValue()))
				ce.pushValue(v)
			}
			frame.pc++
		case operationKindSignExtend32From8:
			v := uint32(int8(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case operationKindSignExtend32From16:
			v := uint32(int16(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case operationKindSignExtend64From8:
			v := int64(int8(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case operationKindSignExtend64From16:
			v := int64(int16(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case operationKindSignExtend64From32:
			v := int64(int32(ce.popValue()))
			ce.pushValue(uint64(v))
			frame.pc++
		case operationKindMemoryInit:
			dataInstance := dataInstances[op.U1]
			copySize := ce.popValue()
			inDataOffset := ce.popValue()
			inMemoryOffset := ce.popValue()
			if inDataOffset+copySize > uint64(len(dataInstance)) ||
				inMemoryOffset+copySize > uint64(len(memoryInst.Buffer)) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			} else if copySize != 0 {
				copy(memoryInst.Buffer[inMemoryOffset:inMemoryOffset+copySize], dataInstance[inDataOffset:])
			}
			frame.pc++
		case operationKindDataDrop:
			dataInstances[op.U1] = nil
			frame.pc++
		case operationKindMemoryCopy:
			memLen := uint64(len(memoryInst.Buffer))
			copySize := ce.popValue()
			sourceOffset := ce.popValue()
			destinationOffset := ce.popValue()
			if sourceOffset+copySize > memLen || destinationOffset+copySize > memLen {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			} else if copySize != 0 {
				copy(memoryInst.Buffer[destinationOffset:],
					memoryInst.Buffer[sourceOffset:sourceOffset+copySize])
			}
			frame.pc++
		case operationKindMemoryFill:
			fillSize := ce.popValue()
			value := byte(ce.popValue())
			offset := ce.popValue()
			if fillSize+offset > uint64(len(memoryInst.Buffer)) {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			} else if fillSize != 0 {
				// Uses the copy trick for faster filling buffer.
				// https://gist.github.com/taylorza/df2f89d5f9ab3ffd06865062a4cf015d
				buf := memoryInst.Buffer[offset : offset+fillSize]
				buf[0] = value
				for i := 1; i < len(buf); i *= 2 {
					copy(buf[i:], buf[:i])
				}
			}
			frame.pc++
		case operationKindTableInit:
			elementInstance := elementInstances[op.U1]
			copySize := ce.popValue()
			inElementOffset := ce.popValue()
			inTableOffset := ce.popValue()
			table := tables[op.U2]
			if inElementOffset+copySize > uint64(len(elementInstance)) ||
				inTableOffset+copySize > uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			} else if copySize != 0 {
				copy(table.References[inTableOffset:inTableOffset+copySize], elementInstance[inElementOffset:])
			}
			frame.pc++
		case operationKindElemDrop:
			elementInstances[op.U1] = nil
			frame.pc++
		case operationKindTableCopy:
			srcTable, dstTable := tables[op.U1].References, tables[op.U2].References
			copySize := ce.popValue()
			sourceOffset := ce.popValue()
			destinationOffset := ce.popValue()
			if sourceOffset+copySize > uint64(len(srcTable)) || destinationOffset+copySize > uint64(len(dstTable)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			} else if copySize != 0 {
				copy(dstTable[destinationOffset:], srcTable[sourceOffset:sourceOffset+copySize])
			}
			frame.pc++
		case operationKindRefFunc:
			ce.pushValue(uint64(uintptr(unsafe.Pointer(&functions[op.U1]))))
			frame.pc++
		case operationKindTableGet:
			table := tables[op.U1]

			offset := ce.popValue()
			if offset >= uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			}

			ce.pushValue(uint64(table.References[offset]))
			frame.pc++
		case operationKindTableSet:
			table := tables[op.U1]
			ref := ce.popValue()

			offset := ce.popValue()
			if offset >= uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			}

			table.References[offset] = uintptr(ref) // externrefs are opaque uint64.
			frame.pc++
		case operationKindTableSize:
			table := tables[op.U1]
			ce.pushValue(uint64(len(table.References)))
			frame.pc++
		case operationKindTableGrow:
			table := tables[op.U1]
			num, ref := ce.popValue(), ce.popValue()
			ret := table.Grow(uint32(num), uintptr(ref))
			ce.pushValue(uint64(ret))
			frame.pc++
		case operationKindTableFill:
			table := tables[op.U1]
			num := ce.popValue()
			ref := uintptr(ce.popValue())
			offset := ce.popValue()
			if num+offset > uint64(len(table.References)) {
				panic(wasmruntime.ErrRuntimeInvalidTableAccess)
			} else if num > 0 {
				// Uses the copy trick for faster filling the region with the value.
				// https://gist.github.com/taylorza/df2f89d5f9ab3ffd06865062a4cf015d
				targetRegion := table.References[offset : offset+num]
				targetRegion[0] = ref
				for i := 1; i < len(targetRegion); i *= 2 {
					copy(targetRegion[i:], targetRegion[:i])
				}
			}
			frame.pc++
		case operationKindV128Const:
			lo, hi := op.U1, op.U2
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Add:
			yHigh, yLow := ce.popValue(), ce.popValue()
			xHigh, xLow := ce.popValue(), ce.popValue()
			switch op.B1 {
			case shapeI8x16:
				ce.pushValue(
					uint64(uint8(xLow>>8)+uint8(yLow>>8))<<8 | uint64(uint8(xLow)+uint8(yLow)) |
						uint64(uint8(xLow>>24)+uint8(yLow>>24))<<24 | uint64(uint8(xLow>>16)+uint8(yLow>>16))<<16 |
						uint64(uint8(xLow>>40)+uint8(yLow>>40))<<40 | uint64(uint8(xLow>>32)+uint8(yLow>>32))<<32 |
						uint64(uint8(xLow>>56)+uint8(yLow>>56))<<56 | uint64(uint8(xLow>>48)+uint8(yLow>>48))<<48,
				)
				ce.pushValue(
					uint64(uint8(xHigh>>8)+uint8(yHigh>>8))<<8 | uint64(uint8(xHigh)+uint8(yHigh)) |
						uint64(uint8(xHigh>>24)+uint8(yHigh>>24))<<24 | uint64(uint8(xHigh>>16)+uint8(yHigh>>16))<<16 |
						uint64(uint8(xHigh>>40)+uint8(yHigh>>40))<<40 | uint64(uint8(xHigh>>32)+uint8(yHigh>>32))<<32 |
						uint64(uint8(xHigh>>56)+uint8(yHigh>>56))<<56 | uint64(uint8(xHigh>>48)+uint8(yHigh>>48))<<48,
				)
			case shapeI16x8:
				ce.pushValue(
					uint64(uint16(xLow>>16+yLow>>16))<<16 | uint64(uint16(xLow)+uint16(yLow)) |
						uint64(uint16(xLow>>48+yLow>>48))<<48 | uint64(uint16(xLow>>32+yLow>>32))<<32,
				)
				ce.pushValue(
					uint64(uint16(xHigh>>16)+uint16(yHigh>>16))<<16 | uint64(uint16(xHigh)+uint16(yHigh)) |
						uint64(uint16(xHigh>>48)+uint16(yHigh>>48))<<48 | uint64(uint16(xHigh>>32)+uint16(yHigh>>32))<<32,
				)
			case shapeI32x4:
				ce.pushValue(uint64(uint32(xLow>>32)+uint32(yLow>>32))<<32 | uint64(uint32(xLow)+uint32(yLow)))
				ce.pushValue(uint64(uint32(xHigh>>32)+uint32(yHigh>>32))<<32 | uint64(uint32(xHigh)+uint32(yHigh)))
			case shapeI64x2:
				ce.pushValue(xLow + yLow)
				ce.pushValue(xHigh + yHigh)
			case shapeF32x4:
				ce.pushValue(
					addFloat32bits(uint32(xLow), uint32(yLow)) | addFloat32bits(uint32(xLow>>32), uint32(yLow>>32))<<32,
				)
				ce.pushValue(
					addFloat32bits(uint32(xHigh), uint32(yHigh)) | addFloat32bits(uint32(xHigh>>32), uint32(yHigh>>32))<<32,
				)
			case shapeF64x2:
				ce.pushValue(math.Float64bits(math.Float64frombits(xLow) + math.Float64frombits(yLow)))
				ce.pushValue(math.Float64bits(math.Float64frombits(xHigh) + math.Float64frombits(yHigh)))
			}
			frame.pc++
		case operationKindV128Sub:
			yHigh, yLow := ce.popValue(), ce.popValue()
			xHigh, xLow := ce.popValue(), ce.popValue()
			switch op.B1 {
			case shapeI8x16:
				ce.pushValue(
					uint64(uint8(xLow>>8)-uint8(yLow>>8))<<8 | uint64(uint8(xLow)-uint8(yLow)) |
						uint64(uint8(xLow>>24)-uint8(yLow>>24))<<24 | uint64(uint8(xLow>>16)-uint8(yLow>>16))<<16 |
						uint64(uint8(xLow>>40)-uint8(yLow>>40))<<40 | uint64(uint8(xLow>>32)-uint8(yLow>>32))<<32 |
						uint64(uint8(xLow>>56)-uint8(yLow>>56))<<56 | uint64(uint8(xLow>>48)-uint8(yLow>>48))<<48,
				)
				ce.pushValue(
					uint64(uint8(xHigh>>8)-uint8(yHigh>>8))<<8 | uint64(uint8(xHigh)-uint8(yHigh)) |
						uint64(uint8(xHigh>>24)-uint8(yHigh>>24))<<24 | uint64(uint8(xHigh>>16)-uint8(yHigh>>16))<<16 |
						uint64(uint8(xHigh>>40)-uint8(yHigh>>40))<<40 | uint64(uint8(xHigh>>32)-uint8(yHigh>>32))<<32 |
						uint64(uint8(xHigh>>56)-uint8(yHigh>>56))<<56 | uint64(uint8(xHigh>>48)-uint8(yHigh>>48))<<48,
				)
			case shapeI16x8:
				ce.pushValue(
					uint64(uint16(xLow>>16)-uint16(yLow>>16))<<16 | uint64(uint16(xLow)-uint16(yLow)) |
						uint64(uint16(xLow>>48)-uint16(yLow>>48))<<48 | uint64(uint16(xLow>>32)-uint16(yLow>>32))<<32,
				)
				ce.pushValue(
					uint64(uint16(xHigh>>16)-uint16(yHigh>>16))<<16 | uint64(uint16(xHigh)-uint16(yHigh)) |
						uint64(uint16(xHigh>>48)-uint16(yHigh>>48))<<48 | uint64(uint16(xHigh>>32)-uint16(yHigh>>32))<<32,
				)
			case shapeI32x4:
				ce.pushValue(uint64(uint32(xLow>>32-yLow>>32))<<32 | uint64(uint32(xLow)-uint32(yLow)))
				ce.pushValue(uint64(uint32(xHigh>>32-yHigh>>32))<<32 | uint64(uint32(xHigh)-uint32(yHigh)))
			case shapeI64x2:
				ce.pushValue(xLow - yLow)
				ce.pushValue(xHigh - yHigh)
			case shapeF32x4:
				ce.pushValue(
					subFloat32bits(uint32(xLow), uint32(yLow)) | subFloat32bits(uint32(xLow>>32), uint32(yLow>>32))<<32,
				)
				ce.pushValue(
					subFloat32bits(uint32(xHigh), uint32(yHigh)) | subFloat32bits(uint32(xHigh>>32), uint32(yHigh>>32))<<32,
				)
			case shapeF64x2:
				ce.pushValue(math.Float64bits(math.Float64frombits(xLow) - math.Float64frombits(yLow)))
				ce.pushValue(math.Float64bits(math.Float64frombits(xHigh) - math.Float64frombits(yHigh)))
			}
			frame.pc++
		case operationKindV128Load:
			offset := ce.popMemoryOffset(op)
			switch op.B1 {
			case v128LoadType128:
				lo, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(lo)
				hi, ok := memoryInst.ReadUint64Le(offset + 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(hi)
			case v128LoadType8x8s:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(
					uint64(uint16(int8(data[3])))<<48 | uint64(uint16(int8(data[2])))<<32 | uint64(uint16(int8(data[1])))<<16 | uint64(uint16(int8(data[0]))),
				)
				ce.pushValue(
					uint64(uint16(int8(data[7])))<<48 | uint64(uint16(int8(data[6])))<<32 | uint64(uint16(int8(data[5])))<<16 | uint64(uint16(int8(data[4]))),
				)
			case v128LoadType8x8u:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(
					uint64(data[3])<<48 | uint64(data[2])<<32 | uint64(data[1])<<16 | uint64(data[0]),
				)
				ce.pushValue(
					uint64(data[7])<<48 | uint64(data[6])<<32 | uint64(data[5])<<16 | uint64(data[4]),
				)
			case v128LoadType16x4s:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(
					uint64(int16(binary.LittleEndian.Uint16(data[2:])))<<32 |
						uint64(uint32(int16(binary.LittleEndian.Uint16(data)))),
				)
				ce.pushValue(
					uint64(uint32(int16(binary.LittleEndian.Uint16(data[6:]))))<<32 |
						uint64(uint32(int16(binary.LittleEndian.Uint16(data[4:])))),
				)
			case v128LoadType16x4u:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(
					uint64(binary.LittleEndian.Uint16(data[2:]))<<32 | uint64(binary.LittleEndian.Uint16(data)),
				)
				ce.pushValue(
					uint64(binary.LittleEndian.Uint16(data[6:]))<<32 | uint64(binary.LittleEndian.Uint16(data[4:])),
				)
			case v128LoadType32x2s:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(uint64(int32(binary.LittleEndian.Uint32(data))))
				ce.pushValue(uint64(int32(binary.LittleEndian.Uint32(data[4:]))))
			case v128LoadType32x2u:
				data, ok := memoryInst.Read(offset, 8)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(uint64(binary.LittleEndian.Uint32(data)))
				ce.pushValue(uint64(binary.LittleEndian.Uint32(data[4:])))
			case v128LoadType8Splat:
				v, ok := memoryInst.ReadByte(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				v8 := uint64(v)<<56 | uint64(v)<<48 | uint64(v)<<40 | uint64(v)<<32 |
					uint64(v)<<24 | uint64(v)<<16 | uint64(v)<<8 | uint64(v)
				ce.pushValue(v8)
				ce.pushValue(v8)
			case v128LoadType16Splat:
				v, ok := memoryInst.ReadUint16Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				v4 := uint64(v)<<48 | uint64(v)<<32 | uint64(v)<<16 | uint64(v)
				ce.pushValue(v4)
				ce.pushValue(v4)
			case v128LoadType32Splat:
				v, ok := memoryInst.ReadUint32Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				vv := uint64(v)<<32 | uint64(v)
				ce.pushValue(vv)
				ce.pushValue(vv)
			case v128LoadType64Splat:
				lo, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(lo)
				ce.pushValue(lo)
			case v128LoadType32zero:
				lo, ok := memoryInst.ReadUint32Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(uint64(lo))
				ce.pushValue(0)
			case v128LoadType64zero:
				lo, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(lo)
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindV128LoadLane:
			hi, lo := ce.popValue(), ce.popValue()
			offset := ce.popMemoryOffset(op)
			switch op.B1 {
			case 8:
				b, ok := memoryInst.ReadByte(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if op.B2 < 8 {
					s := op.B2 << 3
					lo = (lo & ^(0xff << s)) | uint64(b)<<s
				} else {
					s := (op.B2 - 8) << 3
					hi = (hi & ^(0xff << s)) | uint64(b)<<s
				}
			case 16:
				b, ok := memoryInst.ReadUint16Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if op.B2 < 4 {
					s := op.B2 << 4
					lo = (lo & ^(0xff_ff << s)) | uint64(b)<<s
				} else {
					s := (op.B2 - 4) << 4
					hi = (hi & ^(0xff_ff << s)) | uint64(b)<<s
				}
			case 32:
				b, ok := memoryInst.ReadUint32Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if op.B2 < 2 {
					s := op.B2 << 5
					lo = (lo & ^(0xff_ff_ff_ff << s)) | uint64(b)<<s
				} else {
					s := (op.B2 - 2) << 5
					hi = (hi & ^(0xff_ff_ff_ff << s)) | uint64(b)<<s
				}
			case 64:
				b, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if op.B2 == 0 {
					lo = b
				} else {
					hi = b
				}
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Store:
			hi, lo := ce.popValue(), ce.popValue()
			offset := ce.popMemoryOffset(op)
			// Write the upper bytes first to trigger an early error if the memory access is out of bounds.
			// Otherwise, the lower bytes might be written to memory, but the upper bytes might not.
			if uint64(offset)+8 > math.MaxUint32 {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			if ok := memoryInst.WriteUint64Le(offset+8, hi); !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			if ok := memoryInst.WriteUint64Le(offset, lo); !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case operationKindV128StoreLane:
			hi, lo := ce.popValue(), ce.popValue()
			offset := ce.popMemoryOffset(op)
			var ok bool
			switch op.B1 {
			case 8:
				if op.B2 < 8 {
					ok = memoryInst.WriteByte(offset, byte(lo>>(op.B2*8)))
				} else {
					ok = memoryInst.WriteByte(offset, byte(hi>>((op.B2-8)*8)))
				}
			case 16:
				if op.B2 < 4 {
					ok = memoryInst.WriteUint16Le(offset, uint16(lo>>(op.B2*16)))
				} else {
					ok = memoryInst.WriteUint16Le(offset, uint16(hi>>((op.B2-4)*16)))
				}
			case 32:
				if op.B2 < 2 {
					ok = memoryInst.WriteUint32Le(offset, uint32(lo>>(op.B2*32)))
				} else {
					ok = memoryInst.WriteUint32Le(offset, uint32(hi>>((op.B2-2)*32)))
				}
			case 64:
				if op.B2 == 0 {
					ok = memoryInst.WriteUint64Le(offset, lo)
				} else {
					ok = memoryInst.WriteUint64Le(offset, hi)
				}
			}
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case operationKindV128ReplaceLane:
			v := ce.popValue()
			hi, lo := ce.popValue(), ce.popValue()
			switch op.B1 {
			case shapeI8x16:
				if op.B2 < 8 {
					s := op.B2 << 3
					lo = (lo & ^(0xff << s)) | uint64(byte(v))<<s
				} else {
					s := (op.B2 - 8) << 3
					hi = (hi & ^(0xff << s)) | uint64(byte(v))<<s
				}
			case shapeI16x8:
				if op.B2 < 4 {
					s := op.B2 << 4
					lo = (lo & ^(0xff_ff << s)) | uint64(uint16(v))<<s
				} else {
					s := (op.B2 - 4) << 4
					hi = (hi & ^(0xff_ff << s)) | uint64(uint16(v))<<s
				}
			case shapeI32x4, shapeF32x4:
				if op.B2 < 2 {
					s := op.B2 << 5
					lo = (lo & ^(0xff_ff_ff_ff << s)) | uint64(uint32(v))<<s
				} else {
					s := (op.B2 - 2) << 5
					hi = (hi & ^(0xff_ff_ff_ff << s)) | uint64(uint32(v))<<s
				}
			case shapeI64x2, shapeF64x2:
				if op.B2 == 0 {
					lo = v
				} else {
					hi = v
				}
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128ExtractLane:
			hi, lo := ce.popValue(), ce.popValue()
			var v uint64
			switch op.B1 {
			case shapeI8x16:
				var u8 byte
				if op.B2 < 8 {
					u8 = byte(lo >> (op.B2 * 8))
				} else {
					u8 = byte(hi >> ((op.B2 - 8) * 8))
				}
				if op.B3 {
					// sign-extend.
					v = uint64(uint32(int8(u8)))
				} else {
					v = uint64(u8)
				}
			case shapeI16x8:
				var u16 uint16
				if op.B2 < 4 {
					u16 = uint16(lo >> (op.B2 * 16))
				} else {
					u16 = uint16(hi >> ((op.B2 - 4) * 16))
				}
				if op.B3 {
					// sign-extend.
					v = uint64(uint32(int16(u16)))
				} else {
					v = uint64(u16)
				}
			case shapeI32x4, shapeF32x4:
				if op.B2 < 2 {
					v = uint64(uint32(lo >> (op.B2 * 32)))
				} else {
					v = uint64(uint32(hi >> ((op.B2 - 2) * 32)))
				}
			case shapeI64x2, shapeF64x2:
				if op.B2 == 0 {
					v = lo
				} else {
					v = hi
				}
			}
			ce.pushValue(v)
			frame.pc++
		case operationKindV128Splat:
			v := ce.popValue()
			var hi, lo uint64
			switch op.B1 {
			case shapeI8x16:
				v8 := uint64(byte(v))<<56 | uint64(byte(v))<<48 | uint64(byte(v))<<40 | uint64(byte(v))<<32 |
					uint64(byte(v))<<24 | uint64(byte(v))<<16 | uint64(byte(v))<<8 | uint64(byte(v))
				hi, lo = v8, v8
			case shapeI16x8:
				v4 := uint64(uint16(v))<<48 | uint64(uint16(v))<<32 | uint64(uint16(v))<<16 | uint64(uint16(v))
				hi, lo = v4, v4
			case shapeI32x4, shapeF32x4:
				v2 := uint64(uint32(v))<<32 | uint64(uint32(v))
				lo, hi = v2, v2
			case shapeI64x2, shapeF64x2:
				lo, hi = v, v
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Swizzle:
			idxHi, idxLo := ce.popValue(), ce.popValue()
			baseHi, baseLo := ce.popValue(), ce.popValue()
			var newVal [16]byte
			for i := 0; i < 16; i++ {
				var id byte
				if i < 8 {
					id = byte(idxLo >> (i * 8))
				} else {
					id = byte(idxHi >> ((i - 8) * 8))
				}
				if id < 8 {
					newVal[i] = byte(baseLo >> (id * 8))
				} else if id < 16 {
					newVal[i] = byte(baseHi >> ((id - 8) * 8))
				}
			}
			ce.pushValue(binary.LittleEndian.Uint64(newVal[:8]))
			ce.pushValue(binary.LittleEndian.Uint64(newVal[8:]))
			frame.pc++
		case operationKindV128Shuffle:
			xHi, xLo, yHi, yLo := ce.popValue(), ce.popValue(), ce.popValue(), ce.popValue()
			var newVal [16]byte
			for i, l := range op.Us {
				if l < 8 {
					newVal[i] = byte(yLo >> (l * 8))
				} else if l < 16 {
					newVal[i] = byte(yHi >> ((l - 8) * 8))
				} else if l < 24 {
					newVal[i] = byte(xLo >> ((l - 16) * 8))
				} else if l < 32 {
					newVal[i] = byte(xHi >> ((l - 24) * 8))
				}
			}
			ce.pushValue(binary.LittleEndian.Uint64(newVal[:8]))
			ce.pushValue(binary.LittleEndian.Uint64(newVal[8:]))
			frame.pc++
		case operationKindV128AnyTrue:
			hi, lo := ce.popValue(), ce.popValue()
			if hi != 0 || lo != 0 {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindV128AllTrue:
			hi, lo := ce.popValue(), ce.popValue()
			var ret bool
			switch op.B1 {
			case shapeI8x16:
				ret = (uint8(lo) != 0) && (uint8(lo>>8) != 0) && (uint8(lo>>16) != 0) && (uint8(lo>>24) != 0) &&
					(uint8(lo>>32) != 0) && (uint8(lo>>40) != 0) && (uint8(lo>>48) != 0) && (uint8(lo>>56) != 0) &&
					(uint8(hi) != 0) && (uint8(hi>>8) != 0) && (uint8(hi>>16) != 0) && (uint8(hi>>24) != 0) &&
					(uint8(hi>>32) != 0) && (uint8(hi>>40) != 0) && (uint8(hi>>48) != 0) && (uint8(hi>>56) != 0)
			case shapeI16x8:
				ret = (uint16(lo) != 0) && (uint16(lo>>16) != 0) && (uint16(lo>>32) != 0) && (uint16(lo>>48) != 0) &&
					(uint16(hi) != 0) && (uint16(hi>>16) != 0) && (uint16(hi>>32) != 0) && (uint16(hi>>48) != 0)
			case shapeI32x4:
				ret = (uint32(lo) != 0) && (uint32(lo>>32) != 0) &&
					(uint32(hi) != 0) && (uint32(hi>>32) != 0)
			case shapeI64x2:
				ret = (lo != 0) &&
					(hi != 0)
			}
			if ret {
				ce.pushValue(1)
			} else {
				ce.pushValue(0)
			}
			frame.pc++
		case operationKindV128BitMask:
			// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#bitmask-extraction
			hi, lo := ce.popValue(), ce.popValue()
			var res uint64
			switch op.B1 {
			case shapeI8x16:
				for i := 0; i < 8; i++ {
					if int8(lo>>(i*8)) < 0 {
						res |= 1 << i
					}
				}
				for i := 0; i < 8; i++ {
					if int8(hi>>(i*8)) < 0 {
						res |= 1 << (i + 8)
					}
				}
			case shapeI16x8:
				for i := 0; i < 4; i++ {
					if int16(lo>>(i*16)) < 0 {
						res |= 1 << i
					}
				}
				for i := 0; i < 4; i++ {
					if int16(hi>>(i*16)) < 0 {
						res |= 1 << (i + 4)
					}
				}
			case shapeI32x4:
				for i := 0; i < 2; i++ {
					if int32(lo>>(i*32)) < 0 {
						res |= 1 << i
					}
				}
				for i := 0; i < 2; i++ {
					if int32(hi>>(i*32)) < 0 {
						res |= 1 << (i + 2)
					}
				}
			case shapeI64x2:
				if int64(lo) < 0 {
					res |= 0b01
				}
				if int(hi) < 0 {
					res |= 0b10
				}
			}
			ce.pushValue(res)
			frame.pc++
		case operationKindV128And:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(x1Lo & x2Lo)
			ce.pushValue(x1Hi & x2Hi)
			frame.pc++
		case operationKindV128Not:
			hi, lo := ce.popValue(), ce.popValue()
			ce.pushValue(^lo)
			ce.pushValue(^hi)
			frame.pc++
		case operationKindV128Or:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(x1Lo | x2Lo)
			ce.pushValue(x1Hi | x2Hi)
			frame.pc++
		case operationKindV128Xor:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(x1Lo ^ x2Lo)
			ce.pushValue(x1Hi ^ x2Hi)
			frame.pc++
		case operationKindV128Bitselect:
			// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#bitwise-select
			cHi, cLo := ce.popValue(), ce.popValue()
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			// v128.or(v128.and(v1, c), v128.and(v2, v128.not(c)))
			ce.pushValue((x1Lo & cLo) | (x2Lo & (^cLo)))
			ce.pushValue((x1Hi & cHi) | (x2Hi & (^cHi)))
			frame.pc++
		case operationKindV128AndNot:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			ce.pushValue(x1Lo & (^x2Lo))
			ce.pushValue(x1Hi & (^x2Hi))
			frame.pc++
		case operationKindV128Shl:
			s := ce.popValue()
			hi, lo := ce.popValue(), ce.popValue()
			switch op.B1 {
			case shapeI8x16:
				s = s % 8
				lo = uint64(uint8(lo<<s)) |
					uint64(uint8((lo>>8)<<s))<<8 |
					uint64(uint8((lo>>16)<<s))<<16 |
					uint64(uint8((lo>>24)<<s))<<24 |
					uint64(uint8((lo>>32)<<s))<<32 |
					uint64(uint8((lo>>40)<<s))<<40 |
					uint64(uint8((lo>>48)<<s))<<48 |
					uint64(uint8((lo>>56)<<s))<<56
				hi = uint64(uint8(hi<<s)) |
					uint64(uint8((hi>>8)<<s))<<8 |
					uint64(uint8((hi>>16)<<s))<<16 |
					uint64(uint8((hi>>24)<<s))<<24 |
					uint64(uint8((hi>>32)<<s))<<32 |
					uint64(uint8((hi>>40)<<s))<<40 |
					uint64(uint8((hi>>48)<<s))<<48 |
					uint64(uint8((hi>>56)<<s))<<56
			case shapeI16x8:
				s = s % 16
				lo = uint64(uint16(lo<<s)) |
					uint64(uint16((lo>>16)<<s))<<16 |
					uint64(uint16((lo>>32)<<s))<<32 |
					uint64(uint16((lo>>48)<<s))<<48
				hi = uint64(uint16(hi<<s)) |
					uint64(uint16((hi>>16)<<s))<<16 |
					uint64(uint16((hi>>32)<<s))<<32 |
					uint64(uint16((hi>>48)<<s))<<48
			case shapeI32x4:
				s = s % 32
				lo = uint64(uint32(lo<<s)) | uint64(uint32((lo>>32)<<s))<<32
				hi = uint64(uint32(hi<<s)) | uint64(uint32((hi>>32)<<s))<<32
			case shapeI64x2:
				s = s % 64
				lo = lo << s
				hi = hi << s
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Shr:
			s := ce.popValue()
			hi, lo := ce.popValue(), ce.popValue()
			switch op.B1 {
			case shapeI8x16:
				s = s % 8
				if op.B3 { // signed
					lo = uint64(uint8(int8(lo)>>s)) |
						uint64(uint8(int8(lo>>8)>>s))<<8 |
						uint64(uint8(int8(lo>>16)>>s))<<16 |
						uint64(uint8(int8(lo>>24)>>s))<<24 |
						uint64(uint8(int8(lo>>32)>>s))<<32 |
						uint64(uint8(int8(lo>>40)>>s))<<40 |
						uint64(uint8(int8(lo>>48)>>s))<<48 |
						uint64(uint8(int8(lo>>56)>>s))<<56
					hi = uint64(uint8(int8(hi)>>s)) |
						uint64(uint8(int8(hi>>8)>>s))<<8 |
						uint64(uint8(int8(hi>>16)>>s))<<16 |
						uint64(uint8(int8(hi>>24)>>s))<<24 |
						uint64(uint8(int8(hi>>32)>>s))<<32 |
						uint64(uint8(int8(hi>>40)>>s))<<40 |
						uint64(uint8(int8(hi>>48)>>s))<<48 |
						uint64(uint8(int8(hi>>56)>>s))<<56
				} else {
					lo = uint64(uint8(lo)>>s) |
						uint64(uint8(lo>>8)>>s)<<8 |
						uint64(uint8(lo>>16)>>s)<<16 |
						uint64(uint8(lo>>24)>>s)<<24 |
						uint64(uint8(lo>>32)>>s)<<32 |
						uint64(uint8(lo>>40)>>s)<<40 |
						uint64(uint8(lo>>48)>>s)<<48 |
						uint64(uint8(lo>>56)>>s)<<56
					hi = uint64(uint8(hi)>>s) |
						uint64(uint8(hi>>8)>>s)<<8 |
						uint64(uint8(hi>>16)>>s)<<16 |
						uint64(uint8(hi>>24)>>s)<<24 |
						uint64(uint8(hi>>32)>>s)<<32 |
						uint64(uint8(hi>>40)>>s)<<40 |
						uint64(uint8(hi>>48)>>s)<<48 |
						uint64(uint8(hi>>56)>>s)<<56
				}
			case shapeI16x8:
				s = s % 16
				if op.B3 { // signed
					lo = uint64(uint16(int16(lo)>>s)) |
						uint64(uint16(int16(lo>>16)>>s))<<16 |
						uint64(uint16(int16(lo>>32)>>s))<<32 |
						uint64(uint16(int16(lo>>48)>>s))<<48
					hi = uint64(uint16(int16(hi)>>s)) |
						uint64(uint16(int16(hi>>16)>>s))<<16 |
						uint64(uint16(int16(hi>>32)>>s))<<32 |
						uint64(uint16(int16(hi>>48)>>s))<<48
				} else {
					lo = uint64(uint16(lo)>>s) |
						uint64(uint16(lo>>16)>>s)<<16 |
						uint64(uint16(lo>>32)>>s)<<32 |
						uint64(uint16(lo>>48)>>s)<<48
					hi = uint64(uint16(hi)>>s) |
						uint64(uint16(hi>>16)>>s)<<16 |
						uint64(uint16(hi>>32)>>s)<<32 |
						uint64(uint16(hi>>48)>>s)<<48
				}
			case shapeI32x4:
				s = s % 32
				if op.B3 {
					lo = uint64(uint32(int32(lo)>>s)) | uint64(uint32(int32(lo>>32)>>s))<<32
					hi = uint64(uint32(int32(hi)>>s)) | uint64(uint32(int32(hi>>32)>>s))<<32
				} else {
					lo = uint64(uint32(lo)>>s) | uint64(uint32(lo>>32)>>s)<<32
					hi = uint64(uint32(hi)>>s) | uint64(uint32(hi>>32)>>s)<<32
				}
			case shapeI64x2:
				s = s % 64
				if op.B3 { // signed
					lo = uint64(int64(lo) >> s)
					hi = uint64(int64(hi) >> s)
				} else {
					lo = lo >> s
					hi = hi >> s
				}

			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Cmp:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			var result []bool
			switch op.B1 {
			case v128CmpTypeI8x16Eq:
				result = []bool{
					byte(x1Lo>>0) == byte(x2Lo>>0), byte(x1Lo>>8) == byte(x2Lo>>8),
					byte(x1Lo>>16) == byte(x2Lo>>16), byte(x1Lo>>24) == byte(x2Lo>>24),
					byte(x1Lo>>32) == byte(x2Lo>>32), byte(x1Lo>>40) == byte(x2Lo>>40),
					byte(x1Lo>>48) == byte(x2Lo>>48), byte(x1Lo>>56) == byte(x2Lo>>56),
					byte(x1Hi>>0) == byte(x2Hi>>0), byte(x1Hi>>8) == byte(x2Hi>>8),
					byte(x1Hi>>16) == byte(x2Hi>>16), byte(x1Hi>>24) == byte(x2Hi>>24),
					byte(x1Hi>>32) == byte(x2Hi>>32), byte(x1Hi>>40) == byte(x2Hi>>40),
					byte(x1Hi>>48) == byte(x2Hi>>48), byte(x1Hi>>56) == byte(x2Hi>>56),
				}
			case v128CmpTypeI8x16Ne:
				result = []bool{
					byte(x1Lo>>0) != byte(x2Lo>>0), byte(x1Lo>>8) != byte(x2Lo>>8),
					byte(x1Lo>>16) != byte(x2Lo>>16), byte(x1Lo>>24) != byte(x2Lo>>24),
					byte(x1Lo>>32) != byte(x2Lo>>32), byte(x1Lo>>40) != byte(x2Lo>>40),
					byte(x1Lo>>48) != byte(x2Lo>>48), byte(x1Lo>>56) != byte(x2Lo>>56),
					byte(x1Hi>>0) != byte(x2Hi>>0), byte(x1Hi>>8) != byte(x2Hi>>8),
					byte(x1Hi>>16) != byte(x2Hi>>16), byte(x1Hi>>24) != byte(x2Hi>>24),
					byte(x1Hi>>32) != byte(x2Hi>>32), byte(x1Hi>>40) != byte(x2Hi>>40),
					byte(x1Hi>>48) != byte(x2Hi>>48), byte(x1Hi>>56) != byte(x2Hi>>56),
				}
			case v128CmpTypeI8x16LtS:
				result = []bool{
					int8(x1Lo>>0) < int8(x2Lo>>0), int8(x1Lo>>8) < int8(x2Lo>>8),
					int8(x1Lo>>16) < int8(x2Lo>>16), int8(x1Lo>>24) < int8(x2Lo>>24),
					int8(x1Lo>>32) < int8(x2Lo>>32), int8(x1Lo>>40) < int8(x2Lo>>40),
					int8(x1Lo>>48) < int8(x2Lo>>48), int8(x1Lo>>56) < int8(x2Lo>>56),
					int8(x1Hi>>0) < int8(x2Hi>>0), int8(x1Hi>>8) < int8(x2Hi>>8),
					int8(x1Hi>>16) < int8(x2Hi>>16), int8(x1Hi>>24) < int8(x2Hi>>24),
					int8(x1Hi>>32) < int8(x2Hi>>32), int8(x1Hi>>40) < int8(x2Hi>>40),
					int8(x1Hi>>48) < int8(x2Hi>>48), int8(x1Hi>>56) < int8(x2Hi>>56),
				}
			case v128CmpTypeI8x16LtU:
				result = []bool{
					byte(x1Lo>>0) < byte(x2Lo>>0), byte(x1Lo>>8) < byte(x2Lo>>8),
					byte(x1Lo>>16) < byte(x2Lo>>16), byte(x1Lo>>24) < byte(x2Lo>>24),
					byte(x1Lo>>32) < byte(x2Lo>>32), byte(x1Lo>>40) < byte(x2Lo>>40),
					byte(x1Lo>>48) < byte(x2Lo>>48), byte(x1Lo>>56) < byte(x2Lo>>56),
					byte(x1Hi>>0) < byte(x2Hi>>0), byte(x1Hi>>8) < byte(x2Hi>>8),
					byte(x1Hi>>16) < byte(x2Hi>>16), byte(x1Hi>>24) < byte(x2Hi>>24),
					byte(x1Hi>>32) < byte(x2Hi>>32), byte(x1Hi>>40) < byte(x2Hi>>40),
					byte(x1Hi>>48) < byte(x2Hi>>48), byte(x1Hi>>56) < byte(x2Hi>>56),
				}
			case v128CmpTypeI8x16GtS:
				result = []bool{
					int8(x1Lo>>0) > int8(x2Lo>>0), int8(x1Lo>>8) > int8(x2Lo>>8),
					int8(x1Lo>>16) > int8(x2Lo>>16), int8(x1Lo>>24) > int8(x2Lo>>24),
					int8(x1Lo>>32) > int8(x2Lo>>32), int8(x1Lo>>40) > int8(x2Lo>>40),
					int8(x1Lo>>48) > int8(x2Lo>>48), int8(x1Lo>>56) > int8(x2Lo>>56),
					int8(x1Hi>>0) > int8(x2Hi>>0), int8(x1Hi>>8) > int8(x2Hi>>8),
					int8(x1Hi>>16) > int8(x2Hi>>16), int8(x1Hi>>24) > int8(x2Hi>>24),
					int8(x1Hi>>32) > int8(x2Hi>>32), int8(x1Hi>>40) > int8(x2Hi>>40),
					int8(x1Hi>>48) > int8(x2Hi>>48), int8(x1Hi>>56) > int8(x2Hi>>56),
				}
			case v128CmpTypeI8x16GtU:
				result = []bool{
					byte(x1Lo>>0) > byte(x2Lo>>0), byte(x1Lo>>8) > byte(x2Lo>>8),
					byte(x1Lo>>16) > byte(x2Lo>>16), byte(x1Lo>>24) > byte(x2Lo>>24),
					byte(x1Lo>>32) > byte(x2Lo>>32), byte(x1Lo>>40) > byte(x2Lo>>40),
					byte(x1Lo>>48) > byte(x2Lo>>48), byte(x1Lo>>56) > byte(x2Lo>>56),
					byte(x1Hi>>0) > byte(x2Hi>>0), byte(x1Hi>>8) > byte(x2Hi>>8),
					byte(x1Hi>>16) > byte(x2Hi>>16), byte(x1Hi>>24) > byte(x2Hi>>24),
					byte(x1Hi>>32) > byte(x2Hi>>32), byte(x1Hi>>40) > byte(x2Hi>>40),
					byte(x1Hi>>48) > byte(x2Hi>>48), byte(x1Hi>>56) > byte(x2Hi>>56),
				}
			case v128CmpTypeI8x16LeS:
				result = []bool{
					int8(x1Lo>>0) <= int8(x2Lo>>0), int8(x1Lo>>8) <= int8(x2Lo>>8),
					int8(x1Lo>>16) <= int8(x2Lo>>16), int8(x1Lo>>24) <= int8(x2Lo>>24),
					int8(x1Lo>>32) <= int8(x2Lo>>32), int8(x1Lo>>40) <= int8(x2Lo>>40),
					int8(x1Lo>>48) <= int8(x2Lo>>48), int8(x1Lo>>56) <= int8(x2Lo>>56),
					int8(x1Hi>>0) <= int8(x2Hi>>0), int8(x1Hi>>8) <= int8(x2Hi>>8),
					int8(x1Hi>>16) <= int8(x2Hi>>16), int8(x1Hi>>24) <= int8(x2Hi>>24),
					int8(x1Hi>>32) <= int8(x2Hi>>32), int8(x1Hi>>40) <= int8(x2Hi>>40),
					int8(x1Hi>>48) <= int8(x2Hi>>48), int8(x1Hi>>56) <= int8(x2Hi>>56),
				}
			case v128CmpTypeI8x16LeU:
				result = []bool{
					byte(x1Lo>>0) <= byte(x2Lo>>0), byte(x1Lo>>8) <= byte(x2Lo>>8),
					byte(x1Lo>>16) <= byte(x2Lo>>16), byte(x1Lo>>24) <= byte(x2Lo>>24),
					byte(x1Lo>>32) <= byte(x2Lo>>32), byte(x1Lo>>40) <= byte(x2Lo>>40),
					byte(x1Lo>>48) <= byte(x2Lo>>48), byte(x1Lo>>56) <= byte(x2Lo>>56),
					byte(x1Hi>>0) <= byte(x2Hi>>0), byte(x1Hi>>8) <= byte(x2Hi>>8),
					byte(x1Hi>>16) <= byte(x2Hi>>16), byte(x1Hi>>24) <= byte(x2Hi>>24),
					byte(x1Hi>>32) <= byte(x2Hi>>32), byte(x1Hi>>40) <= byte(x2Hi>>40),
					byte(x1Hi>>48) <= byte(x2Hi>>48), byte(x1Hi>>56) <= byte(x2Hi>>56),
				}
			case v128CmpTypeI8x16GeS:
				result = []bool{
					int8(x1Lo>>0) >= int8(x2Lo>>0), int8(x1Lo>>8) >= int8(x2Lo>>8),
					int8(x1Lo>>16) >= int8(x2Lo>>16), int8(x1Lo>>24) >= int8(x2Lo>>24),
					int8(x1Lo>>32) >= int8(x2Lo>>32), int8(x1Lo>>40) >= int8(x2Lo>>40),
					int8(x1Lo>>48) >= int8(x2Lo>>48), int8(x1Lo>>56) >= int8(x2Lo>>56),
					int8(x1Hi>>0) >= int8(x2Hi>>0), int8(x1Hi>>8) >= int8(x2Hi>>8),
					int8(x1Hi>>16) >= int8(x2Hi>>16), int8(x1Hi>>24) >= int8(x2Hi>>24),
					int8(x1Hi>>32) >= int8(x2Hi>>32), int8(x1Hi>>40) >= int8(x2Hi>>40),
					int8(x1Hi>>48) >= int8(x2Hi>>48), int8(x1Hi>>56) >= int8(x2Hi>>56),
				}
			case v128CmpTypeI8x16GeU:
				result = []bool{
					byte(x1Lo>>0) >= byte(x2Lo>>0), byte(x1Lo>>8) >= byte(x2Lo>>8),
					byte(x1Lo>>16) >= byte(x2Lo>>16), byte(x1Lo>>24) >= byte(x2Lo>>24),
					byte(x1Lo>>32) >= byte(x2Lo>>32), byte(x1Lo>>40) >= byte(x2Lo>>40),
					byte(x1Lo>>48) >= byte(x2Lo>>48), byte(x1Lo>>56) >= byte(x2Lo>>56),
					byte(x1Hi>>0) >= byte(x2Hi>>0), byte(x1Hi>>8) >= byte(x2Hi>>8),
					byte(x1Hi>>16) >= byte(x2Hi>>16), byte(x1Hi>>24) >= byte(x2Hi>>24),
					byte(x1Hi>>32) >= byte(x2Hi>>32), byte(x1Hi>>40) >= byte(x2Hi>>40),
					byte(x1Hi>>48) >= byte(x2Hi>>48), byte(x1Hi>>56) >= byte(x2Hi>>56),
				}
			case v128CmpTypeI16x8Eq:
				result = []bool{
					uint16(x1Lo>>0) == uint16(x2Lo>>0), uint16(x1Lo>>16) == uint16(x2Lo>>16),
					uint16(x1Lo>>32) == uint16(x2Lo>>32), uint16(x1Lo>>48) == uint16(x2Lo>>48),
					uint16(x1Hi>>0) == uint16(x2Hi>>0), uint16(x1Hi>>16) == uint16(x2Hi>>16),
					uint16(x1Hi>>32) == uint16(x2Hi>>32), uint16(x1Hi>>48) == uint16(x2Hi>>48),
				}
			case v128CmpTypeI16x8Ne:
				result = []bool{
					uint16(x1Lo>>0) != uint16(x2Lo>>0), uint16(x1Lo>>16) != uint16(x2Lo>>16),
					uint16(x1Lo>>32) != uint16(x2Lo>>32), uint16(x1Lo>>48) != uint16(x2Lo>>48),
					uint16(x1Hi>>0) != uint16(x2Hi>>0), uint16(x1Hi>>16) != uint16(x2Hi>>16),
					uint16(x1Hi>>32) != uint16(x2Hi>>32), uint16(x1Hi>>48) != uint16(x2Hi>>48),
				}
			case v128CmpTypeI16x8LtS:
				result = []bool{
					int16(x1Lo>>0) < int16(x2Lo>>0), int16(x1Lo>>16) < int16(x2Lo>>16),
					int16(x1Lo>>32) < int16(x2Lo>>32), int16(x1Lo>>48) < int16(x2Lo>>48),
					int16(x1Hi>>0) < int16(x2Hi>>0), int16(x1Hi>>16) < int16(x2Hi>>16),
					int16(x1Hi>>32) < int16(x2Hi>>32), int16(x1Hi>>48) < int16(x2Hi>>48),
				}
			case v128CmpTypeI16x8LtU:
				result = []bool{
					uint16(x1Lo>>0) < uint16(x2Lo>>0), uint16(x1Lo>>16) < uint16(x2Lo>>16),
					uint16(x1Lo>>32) < uint16(x2Lo>>32), uint16(x1Lo>>48) < uint16(x2Lo>>48),
					uint16(x1Hi>>0) < uint16(x2Hi>>0), uint16(x1Hi>>16) < uint16(x2Hi>>16),
					uint16(x1Hi>>32) < uint16(x2Hi>>32), uint16(x1Hi>>48) < uint16(x2Hi>>48),
				}
			case v128CmpTypeI16x8GtS:
				result = []bool{
					int16(x1Lo>>0) > int16(x2Lo>>0), int16(x1Lo>>16) > int16(x2Lo>>16),
					int16(x1Lo>>32) > int16(x2Lo>>32), int16(x1Lo>>48) > int16(x2Lo>>48),
					int16(x1Hi>>0) > int16(x2Hi>>0), int16(x1Hi>>16) > int16(x2Hi>>16),
					int16(x1Hi>>32) > int16(x2Hi>>32), int16(x1Hi>>48) > int16(x2Hi>>48),
				}
			case v128CmpTypeI16x8GtU:
				result = []bool{
					uint16(x1Lo>>0) > uint16(x2Lo>>0), uint16(x1Lo>>16) > uint16(x2Lo>>16),
					uint16(x1Lo>>32) > uint16(x2Lo>>32), uint16(x1Lo>>48) > uint16(x2Lo>>48),
					uint16(x1Hi>>0) > uint16(x2Hi>>0), uint16(x1Hi>>16) > uint16(x2Hi>>16),
					uint16(x1Hi>>32) > uint16(x2Hi>>32), uint16(x1Hi>>48) > uint16(x2Hi>>48),
				}
			case v128CmpTypeI16x8LeS:
				result = []bool{
					int16(x1Lo>>0) <= int16(x2Lo>>0), int16(x1Lo>>16) <= int16(x2Lo>>16),
					int16(x1Lo>>32) <= int16(x2Lo>>32), int16(x1Lo>>48) <= int16(x2Lo>>48),
					int16(x1Hi>>0) <= int16(x2Hi>>0), int16(x1Hi>>16) <= int16(x2Hi>>16),
					int16(x1Hi>>32) <= int16(x2Hi>>32), int16(x1Hi>>48) <= int16(x2Hi>>48),
				}
			case v128CmpTypeI16x8LeU:
				result = []bool{
					uint16(x1Lo>>0) <= uint16(x2Lo>>0), uint16(x1Lo>>16) <= uint16(x2Lo>>16),
					uint16(x1Lo>>32) <= uint16(x2Lo>>32), uint16(x1Lo>>48) <= uint16(x2Lo>>48),
					uint16(x1Hi>>0) <= uint16(x2Hi>>0), uint16(x1Hi>>16) <= uint16(x2Hi>>16),
					uint16(x1Hi>>32) <= uint16(x2Hi>>32), uint16(x1Hi>>48) <= uint16(x2Hi>>48),
				}
			case v128CmpTypeI16x8GeS:
				result = []bool{
					int16(x1Lo>>0) >= int16(x2Lo>>0), int16(x1Lo>>16) >= int16(x2Lo>>16),
					int16(x1Lo>>32) >= int16(x2Lo>>32), int16(x1Lo>>48) >= int16(x2Lo>>48),
					int16(x1Hi>>0) >= int16(x2Hi>>0), int16(x1Hi>>16) >= int16(x2Hi>>16),
					int16(x1Hi>>32) >= int16(x2Hi>>32), int16(x1Hi>>48) >= int16(x2Hi>>48),
				}
			case v128CmpTypeI16x8GeU:
				result = []bool{
					uint16(x1Lo>>0) >= uint16(x2Lo>>0), uint16(x1Lo>>16) >= uint16(x2Lo>>16),
					uint16(x1Lo>>32) >= uint16(x2Lo>>32), uint16(x1Lo>>48) >= uint16(x2Lo>>48),
					uint16(x1Hi>>0) >= uint16(x2Hi>>0), uint16(x1Hi>>16) >= uint16(x2Hi>>16),
					uint16(x1Hi>>32) >= uint16(x2Hi>>32), uint16(x1Hi>>48) >= uint16(x2Hi>>48),
				}
			case v128CmpTypeI32x4Eq:
				result = []bool{
					uint32(x1Lo>>0) == uint32(x2Lo>>0), uint32(x1Lo>>32) == uint32(x2Lo>>32),
					uint32(x1Hi>>0) == uint32(x2Hi>>0), uint32(x1Hi>>32) == uint32(x2Hi>>32),
				}
			case v128CmpTypeI32x4Ne:
				result = []bool{
					uint32(x1Lo>>0) != uint32(x2Lo>>0), uint32(x1Lo>>32) != uint32(x2Lo>>32),
					uint32(x1Hi>>0) != uint32(x2Hi>>0), uint32(x1Hi>>32) != uint32(x2Hi>>32),
				}
			case v128CmpTypeI32x4LtS:
				result = []bool{
					int32(x1Lo>>0) < int32(x2Lo>>0), int32(x1Lo>>32) < int32(x2Lo>>32),
					int32(x1Hi>>0) < int32(x2Hi>>0), int32(x1Hi>>32) < int32(x2Hi>>32),
				}
			case v128CmpTypeI32x4LtU:
				result = []bool{
					uint32(x1Lo>>0) < uint32(x2Lo>>0), uint32(x1Lo>>32) < uint32(x2Lo>>32),
					uint32(x1Hi>>0) < uint32(x2Hi>>0), uint32(x1Hi>>32) < uint32(x2Hi>>32),
				}
			case v128CmpTypeI32x4GtS:
				result = []bool{
					int32(x1Lo>>0) > int32(x2Lo>>0), int32(x1Lo>>32) > int32(x2Lo>>32),
					int32(x1Hi>>0) > int32(x2Hi>>0), int32(x1Hi>>32) > int32(x2Hi>>32),
				}
			case v128CmpTypeI32x4GtU:
				result = []bool{
					uint32(x1Lo>>0) > uint32(x2Lo>>0), uint32(x1Lo>>32) > uint32(x2Lo>>32),
					uint32(x1Hi>>0) > uint32(x2Hi>>0), uint32(x1Hi>>32) > uint32(x2Hi>>32),
				}
			case v128CmpTypeI32x4LeS:
				result = []bool{
					int32(x1Lo>>0) <= int32(x2Lo>>0), int32(x1Lo>>32) <= int32(x2Lo>>32),
					int32(x1Hi>>0) <= int32(x2Hi>>0), int32(x1Hi>>32) <= int32(x2Hi>>32),
				}
			case v128CmpTypeI32x4LeU:
				result = []bool{
					uint32(x1Lo>>0) <= uint32(x2Lo>>0), uint32(x1Lo>>32) <= uint32(x2Lo>>32),
					uint32(x1Hi>>0) <= uint32(x2Hi>>0), uint32(x1Hi>>32) <= uint32(x2Hi>>32),
				}
			case v128CmpTypeI32x4GeS:
				result = []bool{
					int32(x1Lo>>0) >= int32(x2Lo>>0), int32(x1Lo>>32) >= int32(x2Lo>>32),
					int32(x1Hi>>0) >= int32(x2Hi>>0), int32(x1Hi>>32) >= int32(x2Hi>>32),
				}
			case v128CmpTypeI32x4GeU:
				result = []bool{
					uint32(x1Lo>>0) >= uint32(x2Lo>>0), uint32(x1Lo>>32) >= uint32(x2Lo>>32),
					uint32(x1Hi>>0) >= uint32(x2Hi>>0), uint32(x1Hi>>32) >= uint32(x2Hi>>32),
				}
			case v128CmpTypeI64x2Eq:
				result = []bool{x1Lo == x2Lo, x1Hi == x2Hi}
			case v128CmpTypeI64x2Ne:
				result = []bool{x1Lo != x2Lo, x1Hi != x2Hi}
			case v128CmpTypeI64x2LtS:
				result = []bool{int64(x1Lo) < int64(x2Lo), int64(x1Hi) < int64(x2Hi)}
			case v128CmpTypeI64x2GtS:
				result = []bool{int64(x1Lo) > int64(x2Lo), int64(x1Hi) > int64(x2Hi)}
			case v128CmpTypeI64x2LeS:
				result = []bool{int64(x1Lo) <= int64(x2Lo), int64(x1Hi) <= int64(x2Hi)}
			case v128CmpTypeI64x2GeS:
				result = []bool{int64(x1Lo) >= int64(x2Lo), int64(x1Hi) >= int64(x2Hi)}
			case v128CmpTypeF32x4Eq:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) == math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) == math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) == math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) == math.Float32frombits(uint32(x2Hi>>32)),
				}
			case v128CmpTypeF32x4Ne:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) != math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) != math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) != math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) != math.Float32frombits(uint32(x2Hi>>32)),
				}
			case v128CmpTypeF32x4Lt:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) < math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) < math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) < math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) < math.Float32frombits(uint32(x2Hi>>32)),
				}
			case v128CmpTypeF32x4Gt:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) > math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) > math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) > math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) > math.Float32frombits(uint32(x2Hi>>32)),
				}
			case v128CmpTypeF32x4Le:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) <= math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) <= math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) <= math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) <= math.Float32frombits(uint32(x2Hi>>32)),
				}
			case v128CmpTypeF32x4Ge:
				result = []bool{
					math.Float32frombits(uint32(x1Lo>>0)) >= math.Float32frombits(uint32(x2Lo>>0)),
					math.Float32frombits(uint32(x1Lo>>32)) >= math.Float32frombits(uint32(x2Lo>>32)),
					math.Float32frombits(uint32(x1Hi>>0)) >= math.Float32frombits(uint32(x2Hi>>0)),
					math.Float32frombits(uint32(x1Hi>>32)) >= math.Float32frombits(uint32(x2Hi>>32)),
				}
			case v128CmpTypeF64x2Eq:
				result = []bool{
					math.Float64frombits(x1Lo) == math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) == math.Float64frombits(x2Hi),
				}
			case v128CmpTypeF64x2Ne:
				result = []bool{
					math.Float64frombits(x1Lo) != math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) != math.Float64frombits(x2Hi),
				}
			case v128CmpTypeF64x2Lt:
				result = []bool{
					math.Float64frombits(x1Lo) < math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) < math.Float64frombits(x2Hi),
				}
			case v128CmpTypeF64x2Gt:
				result = []bool{
					math.Float64frombits(x1Lo) > math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) > math.Float64frombits(x2Hi),
				}
			case v128CmpTypeF64x2Le:
				result = []bool{
					math.Float64frombits(x1Lo) <= math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) <= math.Float64frombits(x2Hi),
				}
			case v128CmpTypeF64x2Ge:
				result = []bool{
					math.Float64frombits(x1Lo) >= math.Float64frombits(x2Lo),
					math.Float64frombits(x1Hi) >= math.Float64frombits(x2Hi),
				}
			}

			var retLo, retHi uint64
			laneNum := len(result)
			switch laneNum {
			case 16:
				for i, b := range result {
					if b {
						if i < 8 {
							retLo |= 0xff << (i * 8)
						} else {
							retHi |= 0xff << ((i - 8) * 8)
						}
					}
				}
			case 8:
				for i, b := range result {
					if b {
						if i < 4 {
							retLo |= 0xffff << (i * 16)
						} else {
							retHi |= 0xffff << ((i - 4) * 16)
						}
					}
				}
			case 4:
				for i, b := range result {
					if b {
						if i < 2 {
							retLo |= 0xffff_ffff << (i * 32)
						} else {
							retHi |= 0xffff_ffff << ((i - 2) * 32)
						}
					}
				}
			case 2:
				if result[0] {
					retLo = ^uint64(0)
				}
				if result[1] {
					retHi = ^uint64(0)
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128AddSat:
			x2hi, x2Lo := ce.popValue(), ce.popValue()
			x1hi, x1Lo := ce.popValue(), ce.popValue()

			var retLo, retHi uint64

			// Lane-wise addition while saturating the overflowing values.
			// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#saturating-integer-addition
			switch op.B1 {
			case shapeI8x16:
				for i := 0; i < 16; i++ {
					var v, w byte
					if i < 8 {
						v, w = byte(x1Lo>>(i*8)), byte(x2Lo>>(i*8))
					} else {
						v, w = byte(x1hi>>((i-8)*8)), byte(x2hi>>((i-8)*8))
					}

					var uv uint64
					if op.B3 { // signed
						if subbed := int64(int8(v)) + int64(int8(w)); subbed < math.MinInt8 {
							uv = uint64(byte(0x80))
						} else if subbed > math.MaxInt8 {
							uv = uint64(byte(0x7f))
						} else {
							uv = uint64(byte(int8(subbed)))
						}
					} else {
						if subbed := int64(v) + int64(w); subbed < 0 {
							uv = uint64(byte(0))
						} else if subbed > math.MaxUint8 {
							uv = uint64(byte(0xff))
						} else {
							uv = uint64(byte(subbed))
						}
					}

					if i < 8 { // first 8 lanes are on lower 64bits.
						retLo |= uv << (i * 8)
					} else {
						retHi |= uv << ((i - 8) * 8)
					}
				}
			case shapeI16x8:
				for i := 0; i < 8; i++ {
					var v, w uint16
					if i < 4 {
						v, w = uint16(x1Lo>>(i*16)), uint16(x2Lo>>(i*16))
					} else {
						v, w = uint16(x1hi>>((i-4)*16)), uint16(x2hi>>((i-4)*16))
					}

					var uv uint64
					if op.B3 { // signed
						if added := int64(int16(v)) + int64(int16(w)); added < math.MinInt16 {
							uv = uint64(uint16(0x8000))
						} else if added > math.MaxInt16 {
							uv = uint64(uint16(0x7fff))
						} else {
							uv = uint64(uint16(int16(added)))
						}
					} else {
						if added := int64(v) + int64(w); added < 0 {
							uv = uint64(uint16(0))
						} else if added > math.MaxUint16 {
							uv = uint64(uint16(0xffff))
						} else {
							uv = uint64(uint16(added))
						}
					}

					if i < 4 { // first 4 lanes are on lower 64bits.
						retLo |= uv << (i * 16)
					} else {
						retHi |= uv << ((i - 4) * 16)
					}
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128SubSat:
			x2hi, x2Lo := ce.popValue(), ce.popValue()
			x1hi, x1Lo := ce.popValue(), ce.popValue()

			var retLo, retHi uint64

			// Lane-wise subtraction while saturating the overflowing values.
			// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#saturating-integer-subtraction
			switch op.B1 {
			case shapeI8x16:
				for i := 0; i < 16; i++ {
					var v, w byte
					if i < 8 {
						v, w = byte(x1Lo>>(i*8)), byte(x2Lo>>(i*8))
					} else {
						v, w = byte(x1hi>>((i-8)*8)), byte(x2hi>>((i-8)*8))
					}

					var uv uint64
					if op.B3 { // signed
						if subbed := int64(int8(v)) - int64(int8(w)); subbed < math.MinInt8 {
							uv = uint64(byte(0x80))
						} else if subbed > math.MaxInt8 {
							uv = uint64(byte(0x7f))
						} else {
							uv = uint64(byte(int8(subbed)))
						}
					} else {
						if subbed := int64(v) - int64(w); subbed < 0 {
							uv = uint64(byte(0))
						} else if subbed > math.MaxUint8 {
							uv = uint64(byte(0xff))
						} else {
							uv = uint64(byte(subbed))
						}
					}

					if i < 8 {
						retLo |= uv << (i * 8)
					} else {
						retHi |= uv << ((i - 8) * 8)
					}
				}
			case shapeI16x8:
				for i := 0; i < 8; i++ {
					var v, w uint16
					if i < 4 {
						v, w = uint16(x1Lo>>(i*16)), uint16(x2Lo>>(i*16))
					} else {
						v, w = uint16(x1hi>>((i-4)*16)), uint16(x2hi>>((i-4)*16))
					}

					var uv uint64
					if op.B3 { // signed
						if subbed := int64(int16(v)) - int64(int16(w)); subbed < math.MinInt16 {
							uv = uint64(uint16(0x8000))
						} else if subbed > math.MaxInt16 {
							uv = uint64(uint16(0x7fff))
						} else {
							uv = uint64(uint16(int16(subbed)))
						}
					} else {
						if subbed := int64(v) - int64(w); subbed < 0 {
							uv = uint64(uint16(0))
						} else if subbed > math.MaxUint16 {
							uv = uint64(uint16(0xffff))
						} else {
							uv = uint64(uint16(subbed))
						}
					}

					if i < 4 {
						retLo |= uv << (i * 16)
					} else {
						retHi |= uv << ((i - 4) * 16)
					}
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Mul:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			switch op.B1 {
			case shapeI16x8:
				retHi = uint64(uint16(x1hi)*uint16(x2hi)) | (uint64(uint16(x1hi>>16)*uint16(x2hi>>16)) << 16) |
					(uint64(uint16(x1hi>>32)*uint16(x2hi>>32)) << 32) | (uint64(uint16(x1hi>>48)*uint16(x2hi>>48)) << 48)
				retLo = uint64(uint16(x1lo)*uint16(x2lo)) | (uint64(uint16(x1lo>>16)*uint16(x2lo>>16)) << 16) |
					(uint64(uint16(x1lo>>32)*uint16(x2lo>>32)) << 32) | (uint64(uint16(x1lo>>48)*uint16(x2lo>>48)) << 48)
			case shapeI32x4:
				retHi = uint64(uint32(x1hi)*uint32(x2hi)) | (uint64(uint32(x1hi>>32)*uint32(x2hi>>32)) << 32)
				retLo = uint64(uint32(x1lo)*uint32(x2lo)) | (uint64(uint32(x1lo>>32)*uint32(x2lo>>32)) << 32)
			case shapeI64x2:
				retHi = x1hi * x2hi
				retLo = x1lo * x2lo
			case shapeF32x4:
				retHi = mulFloat32bits(uint32(x1hi), uint32(x2hi)) | mulFloat32bits(uint32(x1hi>>32), uint32(x2hi>>32))<<32
				retLo = mulFloat32bits(uint32(x1lo), uint32(x2lo)) | mulFloat32bits(uint32(x1lo>>32), uint32(x2lo>>32))<<32
			case shapeF64x2:
				retHi = math.Float64bits(math.Float64frombits(x1hi) * math.Float64frombits(x2hi))
				retLo = math.Float64bits(math.Float64frombits(x1lo) * math.Float64frombits(x2lo))
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Div:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			if op.B1 == shapeF64x2 {
				retHi = math.Float64bits(math.Float64frombits(x1hi) / math.Float64frombits(x2hi))
				retLo = math.Float64bits(math.Float64frombits(x1lo) / math.Float64frombits(x2lo))
			} else {
				retHi = divFloat32bits(uint32(x1hi), uint32(x2hi)) | divFloat32bits(uint32(x1hi>>32), uint32(x2hi>>32))<<32
				retLo = divFloat32bits(uint32(x1lo), uint32(x2lo)) | divFloat32bits(uint32(x1lo>>32), uint32(x2lo>>32))<<32
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Neg:
			hi, lo := ce.popValue(), ce.popValue()
			switch op.B1 {
			case shapeI8x16:
				lo = uint64(-byte(lo)) | (uint64(-byte(lo>>8)) << 8) |
					(uint64(-byte(lo>>16)) << 16) | (uint64(-byte(lo>>24)) << 24) |
					(uint64(-byte(lo>>32)) << 32) | (uint64(-byte(lo>>40)) << 40) |
					(uint64(-byte(lo>>48)) << 48) | (uint64(-byte(lo>>56)) << 56)
				hi = uint64(-byte(hi)) | (uint64(-byte(hi>>8)) << 8) |
					(uint64(-byte(hi>>16)) << 16) | (uint64(-byte(hi>>24)) << 24) |
					(uint64(-byte(hi>>32)) << 32) | (uint64(-byte(hi>>40)) << 40) |
					(uint64(-byte(hi>>48)) << 48) | (uint64(-byte(hi>>56)) << 56)
			case shapeI16x8:
				hi = uint64(-uint16(hi)) | (uint64(-uint16(hi>>16)) << 16) |
					(uint64(-uint16(hi>>32)) << 32) | (uint64(-uint16(hi>>48)) << 48)
				lo = uint64(-uint16(lo)) | (uint64(-uint16(lo>>16)) << 16) |
					(uint64(-uint16(lo>>32)) << 32) | (uint64(-uint16(lo>>48)) << 48)
			case shapeI32x4:
				hi = uint64(-uint32(hi)) | (uint64(-uint32(hi>>32)) << 32)
				lo = uint64(-uint32(lo)) | (uint64(-uint32(lo>>32)) << 32)
			case shapeI64x2:
				hi = -hi
				lo = -lo
			case shapeF32x4:
				hi = uint64(math.Float32bits(-math.Float32frombits(uint32(hi)))) |
					(uint64(math.Float32bits(-math.Float32frombits(uint32(hi>>32)))) << 32)
				lo = uint64(math.Float32bits(-math.Float32frombits(uint32(lo)))) |
					(uint64(math.Float32bits(-math.Float32frombits(uint32(lo>>32)))) << 32)
			case shapeF64x2:
				hi = math.Float64bits(-math.Float64frombits(hi))
				lo = math.Float64bits(-math.Float64frombits(lo))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Sqrt:
			hi, lo := ce.popValue(), ce.popValue()
			if op.B1 == shapeF64x2 {
				hi = math.Float64bits(math.Sqrt(math.Float64frombits(hi)))
				lo = math.Float64bits(math.Sqrt(math.Float64frombits(lo)))
			} else {
				hi = uint64(math.Float32bits(float32(math.Sqrt(float64(math.Float32frombits(uint32(hi))))))) |
					(uint64(math.Float32bits(float32(math.Sqrt(float64(math.Float32frombits(uint32(hi>>32))))))) << 32)
				lo = uint64(math.Float32bits(float32(math.Sqrt(float64(math.Float32frombits(uint32(lo))))))) |
					(uint64(math.Float32bits(float32(math.Sqrt(float64(math.Float32frombits(uint32(lo>>32))))))) << 32)
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Abs:
			hi, lo := ce.popValue(), ce.popValue()
			switch op.B1 {
			case shapeI8x16:
				lo = uint64(i8Abs(byte(lo))) | (uint64(i8Abs(byte(lo>>8))) << 8) |
					(uint64(i8Abs(byte(lo>>16))) << 16) | (uint64(i8Abs(byte(lo>>24))) << 24) |
					(uint64(i8Abs(byte(lo>>32))) << 32) | (uint64(i8Abs(byte(lo>>40))) << 40) |
					(uint64(i8Abs(byte(lo>>48))) << 48) | (uint64(i8Abs(byte(lo>>56))) << 56)
				hi = uint64(i8Abs(byte(hi))) | (uint64(i8Abs(byte(hi>>8))) << 8) |
					(uint64(i8Abs(byte(hi>>16))) << 16) | (uint64(i8Abs(byte(hi>>24))) << 24) |
					(uint64(i8Abs(byte(hi>>32))) << 32) | (uint64(i8Abs(byte(hi>>40))) << 40) |
					(uint64(i8Abs(byte(hi>>48))) << 48) | (uint64(i8Abs(byte(hi>>56))) << 56)
			case shapeI16x8:
				hi = uint64(i16Abs(uint16(hi))) | (uint64(i16Abs(uint16(hi>>16))) << 16) |
					(uint64(i16Abs(uint16(hi>>32))) << 32) | (uint64(i16Abs(uint16(hi>>48))) << 48)
				lo = uint64(i16Abs(uint16(lo))) | (uint64(i16Abs(uint16(lo>>16))) << 16) |
					(uint64(i16Abs(uint16(lo>>32))) << 32) | (uint64(i16Abs(uint16(lo>>48))) << 48)
			case shapeI32x4:
				hi = uint64(i32Abs(uint32(hi))) | (uint64(i32Abs(uint32(hi>>32))) << 32)
				lo = uint64(i32Abs(uint32(lo))) | (uint64(i32Abs(uint32(lo>>32))) << 32)
			case shapeI64x2:
				if int64(hi) < 0 {
					hi = -hi
				}
				if int64(lo) < 0 {
					lo = -lo
				}
			case shapeF32x4:
				hi = hi &^ (1<<31 | 1<<63)
				lo = lo &^ (1<<31 | 1<<63)
			case shapeF64x2:
				hi = hi &^ (1 << 63)
				lo = lo &^ (1 << 63)
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Popcnt:
			hi, lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			for i := 0; i < 16; i++ {
				var v byte
				if i < 8 {
					v = byte(lo >> (i * 8))
				} else {
					v = byte(hi >> ((i - 8) * 8))
				}

				var cnt uint64
				for i := 0; i < 8; i++ {
					if (v>>i)&0b1 != 0 {
						cnt++
					}
				}

				if i < 8 {
					retLo |= cnt << (i * 8)
				} else {
					retHi |= cnt << ((i - 8) * 8)
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Min:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			switch op.B1 {
			case shapeI8x16:
				if op.B3 { // signed
					retLo = uint64(i8MinS(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8MinS(uint8(x1lo), uint8(x2lo))) |
						uint64(i8MinS(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8MinS(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
						uint64(i8MinS(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8MinS(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
						uint64(i8MinS(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8MinS(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
					retHi = uint64(i8MinS(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8MinS(uint8(x1hi), uint8(x2hi))) |
						uint64(i8MinS(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8MinS(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
						uint64(i8MinS(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8MinS(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
						uint64(i8MinS(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8MinS(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
				} else {
					retLo = uint64(i8MinU(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8MinU(uint8(x1lo), uint8(x2lo))) |
						uint64(i8MinU(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8MinU(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
						uint64(i8MinU(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8MinU(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
						uint64(i8MinU(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8MinU(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
					retHi = uint64(i8MinU(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8MinU(uint8(x1hi), uint8(x2hi))) |
						uint64(i8MinU(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8MinU(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
						uint64(i8MinU(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8MinU(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
						uint64(i8MinU(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8MinU(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
				}
			case shapeI16x8:
				if op.B3 { // signed
					retLo = uint64(i16MinS(uint16(x1lo), uint16(x2lo))) |
						uint64(i16MinS(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
						uint64(i16MinS(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
						uint64(i16MinS(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
					retHi = uint64(i16MinS(uint16(x1hi), uint16(x2hi))) |
						uint64(i16MinS(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
						uint64(i16MinS(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
						uint64(i16MinS(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
				} else {
					retLo = uint64(i16MinU(uint16(x1lo), uint16(x2lo))) |
						uint64(i16MinU(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
						uint64(i16MinU(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
						uint64(i16MinU(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
					retHi = uint64(i16MinU(uint16(x1hi), uint16(x2hi))) |
						uint64(i16MinU(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
						uint64(i16MinU(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
						uint64(i16MinU(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
				}
			case shapeI32x4:
				if op.B3 { // signed
					retLo = uint64(i32MinS(uint32(x1lo), uint32(x2lo))) |
						uint64(i32MinS(uint32(x1lo>>32), uint32(x2lo>>32)))<<32
					retHi = uint64(i32MinS(uint32(x1hi), uint32(x2hi))) |
						uint64(i32MinS(uint32(x1hi>>32), uint32(x2hi>>32)))<<32
				} else {
					retLo = uint64(i32MinU(uint32(x1lo), uint32(x2lo))) |
						uint64(i32MinU(uint32(x1lo>>32), uint32(x2lo>>32)))<<32
					retHi = uint64(i32MinU(uint32(x1hi), uint32(x2hi))) |
						uint64(i32MinU(uint32(x1hi>>32), uint32(x2hi>>32)))<<32
				}
			case shapeF32x4:
				retHi = wasmCompatMin32bits(uint32(x1hi), uint32(x2hi)) |
					wasmCompatMin32bits(uint32(x1hi>>32), uint32(x2hi>>32))<<32
				retLo = wasmCompatMin32bits(uint32(x1lo), uint32(x2lo)) |
					wasmCompatMin32bits(uint32(x1lo>>32), uint32(x2lo>>32))<<32
			case shapeF64x2:
				retHi = math.Float64bits(moremath.WasmCompatMin64(
					math.Float64frombits(x1hi),
					math.Float64frombits(x2hi),
				))
				retLo = math.Float64bits(moremath.WasmCompatMin64(
					math.Float64frombits(x1lo),
					math.Float64frombits(x2lo),
				))
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Max:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			switch op.B1 {
			case shapeI8x16:
				if op.B3 { // signed
					retLo = uint64(i8MaxS(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8MaxS(uint8(x1lo), uint8(x2lo))) |
						uint64(i8MaxS(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8MaxS(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
						uint64(i8MaxS(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8MaxS(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
						uint64(i8MaxS(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8MaxS(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
					retHi = uint64(i8MaxS(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8MaxS(uint8(x1hi), uint8(x2hi))) |
						uint64(i8MaxS(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8MaxS(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
						uint64(i8MaxS(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8MaxS(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
						uint64(i8MaxS(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8MaxS(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
				} else {
					retLo = uint64(i8MaxU(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8MaxU(uint8(x1lo), uint8(x2lo))) |
						uint64(i8MaxU(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8MaxU(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
						uint64(i8MaxU(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8MaxU(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
						uint64(i8MaxU(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8MaxU(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
					retHi = uint64(i8MaxU(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8MaxU(uint8(x1hi), uint8(x2hi))) |
						uint64(i8MaxU(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8MaxU(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
						uint64(i8MaxU(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8MaxU(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
						uint64(i8MaxU(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8MaxU(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
				}
			case shapeI16x8:
				if op.B3 { // signed
					retLo = uint64(i16MaxS(uint16(x1lo), uint16(x2lo))) |
						uint64(i16MaxS(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
						uint64(i16MaxS(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
						uint64(i16MaxS(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
					retHi = uint64(i16MaxS(uint16(x1hi), uint16(x2hi))) |
						uint64(i16MaxS(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
						uint64(i16MaxS(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
						uint64(i16MaxS(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
				} else {
					retLo = uint64(i16MaxU(uint16(x1lo), uint16(x2lo))) |
						uint64(i16MaxU(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
						uint64(i16MaxU(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
						uint64(i16MaxU(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
					retHi = uint64(i16MaxU(uint16(x1hi), uint16(x2hi))) |
						uint64(i16MaxU(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
						uint64(i16MaxU(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
						uint64(i16MaxU(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
				}
			case shapeI32x4:
				if op.B3 { // signed
					retLo = uint64(i32MaxS(uint32(x1lo), uint32(x2lo))) |
						uint64(i32MaxS(uint32(x1lo>>32), uint32(x2lo>>32)))<<32
					retHi = uint64(i32MaxS(uint32(x1hi), uint32(x2hi))) |
						uint64(i32MaxS(uint32(x1hi>>32), uint32(x2hi>>32)))<<32
				} else {
					retLo = uint64(i32MaxU(uint32(x1lo), uint32(x2lo))) |
						uint64(i32MaxU(uint32(x1lo>>32), uint32(x2lo>>32)))<<32
					retHi = uint64(i32MaxU(uint32(x1hi), uint32(x2hi))) |
						uint64(i32MaxU(uint32(x1hi>>32), uint32(x2hi>>32)))<<32
				}
			case shapeF32x4:
				retHi = wasmCompatMax32bits(uint32(x1hi), uint32(x2hi)) |
					wasmCompatMax32bits(uint32(x1hi>>32), uint32(x2hi>>32))<<32
				retLo = wasmCompatMax32bits(uint32(x1lo), uint32(x2lo)) |
					wasmCompatMax32bits(uint32(x1lo>>32), uint32(x2lo>>32))<<32
			case shapeF64x2:
				retHi = math.Float64bits(moremath.WasmCompatMax64(
					math.Float64frombits(x1hi),
					math.Float64frombits(x2hi),
				))
				retLo = math.Float64bits(moremath.WasmCompatMax64(
					math.Float64frombits(x1lo),
					math.Float64frombits(x2lo),
				))
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128AvgrU:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			switch op.B1 {
			case shapeI8x16:
				retLo = uint64(i8RoundingAverage(uint8(x1lo>>8), uint8(x2lo>>8)))<<8 | uint64(i8RoundingAverage(uint8(x1lo), uint8(x2lo))) |
					uint64(i8RoundingAverage(uint8(x1lo>>24), uint8(x2lo>>24)))<<24 | uint64(i8RoundingAverage(uint8(x1lo>>16), uint8(x2lo>>16)))<<16 |
					uint64(i8RoundingAverage(uint8(x1lo>>40), uint8(x2lo>>40)))<<40 | uint64(i8RoundingAverage(uint8(x1lo>>32), uint8(x2lo>>32)))<<32 |
					uint64(i8RoundingAverage(uint8(x1lo>>56), uint8(x2lo>>56)))<<56 | uint64(i8RoundingAverage(uint8(x1lo>>48), uint8(x2lo>>48)))<<48
				retHi = uint64(i8RoundingAverage(uint8(x1hi>>8), uint8(x2hi>>8)))<<8 | uint64(i8RoundingAverage(uint8(x1hi), uint8(x2hi))) |
					uint64(i8RoundingAverage(uint8(x1hi>>24), uint8(x2hi>>24)))<<24 | uint64(i8RoundingAverage(uint8(x1hi>>16), uint8(x2hi>>16)))<<16 |
					uint64(i8RoundingAverage(uint8(x1hi>>40), uint8(x2hi>>40)))<<40 | uint64(i8RoundingAverage(uint8(x1hi>>32), uint8(x2hi>>32)))<<32 |
					uint64(i8RoundingAverage(uint8(x1hi>>56), uint8(x2hi>>56)))<<56 | uint64(i8RoundingAverage(uint8(x1hi>>48), uint8(x2hi>>48)))<<48
			case shapeI16x8:
				retLo = uint64(i16RoundingAverage(uint16(x1lo), uint16(x2lo))) |
					uint64(i16RoundingAverage(uint16(x1lo>>16), uint16(x2lo>>16)))<<16 |
					uint64(i16RoundingAverage(uint16(x1lo>>32), uint16(x2lo>>32)))<<32 |
					uint64(i16RoundingAverage(uint16(x1lo>>48), uint16(x2lo>>48)))<<48
				retHi = uint64(i16RoundingAverage(uint16(x1hi), uint16(x2hi))) |
					uint64(i16RoundingAverage(uint16(x1hi>>16), uint16(x2hi>>16)))<<16 |
					uint64(i16RoundingAverage(uint16(x1hi>>32), uint16(x2hi>>32)))<<32 |
					uint64(i16RoundingAverage(uint16(x1hi>>48), uint16(x2hi>>48)))<<48
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Pmin:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			if op.B1 == shapeF32x4 {
				if flt32(math.Float32frombits(uint32(x2lo)), math.Float32frombits(uint32(x1lo))) {
					retLo = x2lo & 0x00000000_ffffffff
				} else {
					retLo = x1lo & 0x00000000_ffffffff
				}
				if flt32(math.Float32frombits(uint32(x2lo>>32)), math.Float32frombits(uint32(x1lo>>32))) {
					retLo |= x2lo & 0xffffffff_00000000
				} else {
					retLo |= x1lo & 0xffffffff_00000000
				}
				if flt32(math.Float32frombits(uint32(x2hi)), math.Float32frombits(uint32(x1hi))) {
					retHi = x2hi & 0x00000000_ffffffff
				} else {
					retHi = x1hi & 0x00000000_ffffffff
				}
				if flt32(math.Float32frombits(uint32(x2hi>>32)), math.Float32frombits(uint32(x1hi>>32))) {
					retHi |= x2hi & 0xffffffff_00000000
				} else {
					retHi |= x1hi & 0xffffffff_00000000
				}
			} else {
				if flt64(math.Float64frombits(x2lo), math.Float64frombits(x1lo)) {
					retLo = x2lo
				} else {
					retLo = x1lo
				}
				if flt64(math.Float64frombits(x2hi), math.Float64frombits(x1hi)) {
					retHi = x2hi
				} else {
					retHi = x1hi
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Pmax:
			x2hi, x2lo := ce.popValue(), ce.popValue()
			x1hi, x1lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			if op.B1 == shapeF32x4 {
				if flt32(math.Float32frombits(uint32(x1lo)), math.Float32frombits(uint32(x2lo))) {
					retLo = x2lo & 0x00000000_ffffffff
				} else {
					retLo = x1lo & 0x00000000_ffffffff
				}
				if flt32(math.Float32frombits(uint32(x1lo>>32)), math.Float32frombits(uint32(x2lo>>32))) {
					retLo |= x2lo & 0xffffffff_00000000
				} else {
					retLo |= x1lo & 0xffffffff_00000000
				}
				if flt32(math.Float32frombits(uint32(x1hi)), math.Float32frombits(uint32(x2hi))) {
					retHi = x2hi & 0x00000000_ffffffff
				} else {
					retHi = x1hi & 0x00000000_ffffffff
				}
				if flt32(math.Float32frombits(uint32(x1hi>>32)), math.Float32frombits(uint32(x2hi>>32))) {
					retHi |= x2hi & 0xffffffff_00000000
				} else {
					retHi |= x1hi & 0xffffffff_00000000
				}
			} else {
				if flt64(math.Float64frombits(x1lo), math.Float64frombits(x2lo)) {
					retLo = x2lo
				} else {
					retLo = x1lo
				}
				if flt64(math.Float64frombits(x1hi), math.Float64frombits(x2hi)) {
					retHi = x2hi
				} else {
					retHi = x1hi
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Ceil:
			hi, lo := ce.popValue(), ce.popValue()
			if op.B1 == shapeF32x4 {
				lo = uint64(math.Float32bits(moremath.WasmCompatCeilF32(math.Float32frombits(uint32(lo))))) |
					(uint64(math.Float32bits(moremath.WasmCompatCeilF32(math.Float32frombits(uint32(lo>>32))))) << 32)
				hi = uint64(math.Float32bits(moremath.WasmCompatCeilF32(math.Float32frombits(uint32(hi))))) |
					(uint64(math.Float32bits(moremath.WasmCompatCeilF32(math.Float32frombits(uint32(hi>>32))))) << 32)
			} else {
				lo = math.Float64bits(moremath.WasmCompatCeilF64(math.Float64frombits(lo)))
				hi = math.Float64bits(moremath.WasmCompatCeilF64(math.Float64frombits(hi)))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Floor:
			hi, lo := ce.popValue(), ce.popValue()
			if op.B1 == shapeF32x4 {
				lo = uint64(math.Float32bits(moremath.WasmCompatFloorF32(math.Float32frombits(uint32(lo))))) |
					(uint64(math.Float32bits(moremath.WasmCompatFloorF32(math.Float32frombits(uint32(lo>>32))))) << 32)
				hi = uint64(math.Float32bits(moremath.WasmCompatFloorF32(math.Float32frombits(uint32(hi))))) |
					(uint64(math.Float32bits(moremath.WasmCompatFloorF32(math.Float32frombits(uint32(hi>>32))))) << 32)
			} else {
				lo = math.Float64bits(moremath.WasmCompatFloorF64(math.Float64frombits(lo)))
				hi = math.Float64bits(moremath.WasmCompatFloorF64(math.Float64frombits(hi)))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Trunc:
			hi, lo := ce.popValue(), ce.popValue()
			if op.B1 == shapeF32x4 {
				lo = uint64(math.Float32bits(moremath.WasmCompatTruncF32(math.Float32frombits(uint32(lo))))) |
					(uint64(math.Float32bits(moremath.WasmCompatTruncF32(math.Float32frombits(uint32(lo>>32))))) << 32)
				hi = uint64(math.Float32bits(moremath.WasmCompatTruncF32(math.Float32frombits(uint32(hi))))) |
					(uint64(math.Float32bits(moremath.WasmCompatTruncF32(math.Float32frombits(uint32(hi>>32))))) << 32)
			} else {
				lo = math.Float64bits(moremath.WasmCompatTruncF64(math.Float64frombits(lo)))
				hi = math.Float64bits(moremath.WasmCompatTruncF64(math.Float64frombits(hi)))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Nearest:
			hi, lo := ce.popValue(), ce.popValue()
			if op.B1 == shapeF32x4 {
				lo = uint64(math.Float32bits(moremath.WasmCompatNearestF32(math.Float32frombits(uint32(lo))))) |
					(uint64(math.Float32bits(moremath.WasmCompatNearestF32(math.Float32frombits(uint32(lo>>32))))) << 32)
				hi = uint64(math.Float32bits(moremath.WasmCompatNearestF32(math.Float32frombits(uint32(hi))))) |
					(uint64(math.Float32bits(moremath.WasmCompatNearestF32(math.Float32frombits(uint32(hi>>32))))) << 32)
			} else {
				lo = math.Float64bits(moremath.WasmCompatNearestF64(math.Float64frombits(lo)))
				hi = math.Float64bits(moremath.WasmCompatNearestF64(math.Float64frombits(hi)))
			}
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128Extend:
			hi, lo := ce.popValue(), ce.popValue()
			var origin uint64
			if op.B3 { // use lower 64 bits
				origin = lo
			} else {
				origin = hi
			}

			signed := op.B2 == 1

			var retHi, retLo uint64
			switch op.B1 {
			case shapeI8x16:
				for i := 0; i < 8; i++ {
					v8 := byte(origin >> (i * 8))

					var v16 uint16
					if signed {
						v16 = uint16(int8(v8))
					} else {
						v16 = uint16(v8)
					}

					if i < 4 {
						retLo |= uint64(v16) << (i * 16)
					} else {
						retHi |= uint64(v16) << ((i - 4) * 16)
					}
				}
			case shapeI16x8:
				for i := 0; i < 4; i++ {
					v16 := uint16(origin >> (i * 16))

					var v32 uint32
					if signed {
						v32 = uint32(int16(v16))
					} else {
						v32 = uint32(v16)
					}

					if i < 2 {
						retLo |= uint64(v32) << (i * 32)
					} else {
						retHi |= uint64(v32) << ((i - 2) * 32)
					}
				}
			case shapeI32x4:
				v32Lo := uint32(origin)
				v32Hi := uint32(origin >> 32)
				if signed {
					retLo = uint64(int32(v32Lo))
					retHi = uint64(int32(v32Hi))
				} else {
					retLo = uint64(v32Lo)
					retHi = uint64(v32Hi)
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128ExtMul:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			var x1, x2 uint64
			if op.B3 { // use lower 64 bits
				x1, x2 = x1Lo, x2Lo
			} else {
				x1, x2 = x1Hi, x2Hi
			}

			signed := op.B2 == 1

			var retLo, retHi uint64
			switch op.B1 {
			case shapeI8x16:
				for i := 0; i < 8; i++ {
					v1, v2 := byte(x1>>(i*8)), byte(x2>>(i*8))

					var v16 uint16
					if signed {
						v16 = uint16(int16(int8(v1)) * int16(int8(v2)))
					} else {
						v16 = uint16(v1) * uint16(v2)
					}

					if i < 4 {
						retLo |= uint64(v16) << (i * 16)
					} else {
						retHi |= uint64(v16) << ((i - 4) * 16)
					}
				}
			case shapeI16x8:
				for i := 0; i < 4; i++ {
					v1, v2 := uint16(x1>>(i*16)), uint16(x2>>(i*16))

					var v32 uint32
					if signed {
						v32 = uint32(int32(int16(v1)) * int32(int16(v2)))
					} else {
						v32 = uint32(v1) * uint32(v2)
					}

					if i < 2 {
						retLo |= uint64(v32) << (i * 32)
					} else {
						retHi |= uint64(v32) << ((i - 2) * 32)
					}
				}
			case shapeI32x4:
				v1Lo, v2Lo := uint32(x1), uint32(x2)
				v1Hi, v2Hi := uint32(x1>>32), uint32(x2>>32)
				if signed {
					retLo = uint64(int64(int32(v1Lo)) * int64(int32(v2Lo)))
					retHi = uint64(int64(int32(v1Hi)) * int64(int32(v2Hi)))
				} else {
					retLo = uint64(v1Lo) * uint64(v2Lo)
					retHi = uint64(v1Hi) * uint64(v2Hi)
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Q15mulrSatS:
			x2hi, x2Lo := ce.popValue(), ce.popValue()
			x1hi, x1Lo := ce.popValue(), ce.popValue()
			var retLo, retHi uint64
			for i := 0; i < 8; i++ {
				var v, w int16
				if i < 4 {
					v, w = int16(uint16(x1Lo>>(i*16))), int16(uint16(x2Lo>>(i*16)))
				} else {
					v, w = int16(uint16(x1hi>>((i-4)*16))), int16(uint16(x2hi>>((i-4)*16)))
				}

				var uv uint64
				// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#saturating-integer-q-format-rounding-multiplication
				if calc := ((int32(v) * int32(w)) + 0x4000) >> 15; calc < math.MinInt16 {
					uv = uint64(uint16(0x8000))
				} else if calc > math.MaxInt16 {
					uv = uint64(uint16(0x7fff))
				} else {
					uv = uint64(uint16(int16(calc)))
				}

				if i < 4 {
					retLo |= uv << (i * 16)
				} else {
					retHi |= uv << ((i - 4) * 16)
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128ExtAddPairwise:
			hi, lo := ce.popValue(), ce.popValue()

			signed := op.B3

			var retLo, retHi uint64
			switch op.B1 {
			case shapeI8x16:
				for i := 0; i < 8; i++ {
					var v1, v2 byte
					if i < 4 {
						v1, v2 = byte(lo>>((i*2)*8)), byte(lo>>((i*2+1)*8))
					} else {
						v1, v2 = byte(hi>>(((i-4)*2)*8)), byte(hi>>(((i-4)*2+1)*8))
					}

					var v16 uint16
					if signed {
						v16 = uint16(int16(int8(v1)) + int16(int8(v2)))
					} else {
						v16 = uint16(v1) + uint16(v2)
					}

					if i < 4 {
						retLo |= uint64(v16) << (i * 16)
					} else {
						retHi |= uint64(v16) << ((i - 4) * 16)
					}
				}
			case shapeI16x8:
				for i := 0; i < 4; i++ {
					var v1, v2 uint16
					if i < 2 {
						v1, v2 = uint16(lo>>((i*2)*16)), uint16(lo>>((i*2+1)*16))
					} else {
						v1, v2 = uint16(hi>>(((i-2)*2)*16)), uint16(hi>>(((i-2)*2+1)*16))
					}

					var v32 uint32
					if signed {
						v32 = uint32(int32(int16(v1)) + int32(int16(v2)))
					} else {
						v32 = uint32(v1) + uint32(v2)
					}

					if i < 2 {
						retLo |= uint64(v32) << (i * 32)
					} else {
						retHi |= uint64(v32) << ((i - 2) * 32)
					}
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128FloatPromote:
			_, toPromote := ce.popValue(), ce.popValue()
			ce.pushValue(math.Float64bits(float64(math.Float32frombits(uint32(toPromote)))))
			ce.pushValue(math.Float64bits(float64(math.Float32frombits(uint32(toPromote >> 32)))))
			frame.pc++
		case operationKindV128FloatDemote:
			hi, lo := ce.popValue(), ce.popValue()
			ce.pushValue(
				uint64(math.Float32bits(float32(math.Float64frombits(lo)))) |
					(uint64(math.Float32bits(float32(math.Float64frombits(hi)))) << 32),
			)
			ce.pushValue(0)
			frame.pc++
		case operationKindV128FConvertFromI:
			hi, lo := ce.popValue(), ce.popValue()
			v1, v2, v3, v4 := uint32(lo), uint32(lo>>32), uint32(hi), uint32(hi>>32)
			signed := op.B3

			var retLo, retHi uint64
			switch op.B1 { // Destination shape.
			case shapeF32x4: // f32x4 from signed/unsigned i32x4
				if signed {
					retLo = uint64(math.Float32bits(float32(int32(v1)))) |
						(uint64(math.Float32bits(float32(int32(v2)))) << 32)
					retHi = uint64(math.Float32bits(float32(int32(v3)))) |
						(uint64(math.Float32bits(float32(int32(v4)))) << 32)
				} else {
					retLo = uint64(math.Float32bits(float32(v1))) |
						(uint64(math.Float32bits(float32(v2))) << 32)
					retHi = uint64(math.Float32bits(float32(v3))) |
						(uint64(math.Float32bits(float32(v4))) << 32)
				}
			case shapeF64x2: // f64x2 from signed/unsigned i32x4
				if signed {
					retLo, retHi = math.Float64bits(float64(int32(v1))), math.Float64bits(float64(int32(v2)))
				} else {
					retLo, retHi = math.Float64bits(float64(v1)), math.Float64bits(float64(v2))
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Narrow:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			signed := op.B3

			var retLo, retHi uint64
			switch op.B1 {
			case shapeI16x8: // signed/unsigned i16x8 to i8x16
				for i := 0; i < 8; i++ {
					var v16 uint16
					if i < 4 {
						v16 = uint16(x1Lo >> (i * 16))
					} else {
						v16 = uint16(x1Hi >> ((i - 4) * 16))
					}

					var v byte
					if signed {
						if s := int16(v16); s > math.MaxInt8 {
							v = math.MaxInt8
						} else if s < math.MinInt8 {
							s = math.MinInt8
							v = byte(s)
						} else {
							v = byte(v16)
						}
					} else {
						if s := int16(v16); s > math.MaxUint8 {
							v = math.MaxUint8
						} else if s < 0 {
							v = 0
						} else {
							v = byte(v16)
						}
					}
					retLo |= uint64(v) << (i * 8)
				}
				for i := 0; i < 8; i++ {
					var v16 uint16
					if i < 4 {
						v16 = uint16(x2Lo >> (i * 16))
					} else {
						v16 = uint16(x2Hi >> ((i - 4) * 16))
					}

					var v byte
					if signed {
						if s := int16(v16); s > math.MaxInt8 {
							v = math.MaxInt8
						} else if s < math.MinInt8 {
							s = math.MinInt8
							v = byte(s)
						} else {
							v = byte(v16)
						}
					} else {
						if s := int16(v16); s > math.MaxUint8 {
							v = math.MaxUint8
						} else if s < 0 {
							v = 0
						} else {
							v = byte(v16)
						}
					}
					retHi |= uint64(v) << (i * 8)
				}
			case shapeI32x4: // signed/unsigned i32x4 to i16x8
				for i := 0; i < 4; i++ {
					var v32 uint32
					if i < 2 {
						v32 = uint32(x1Lo >> (i * 32))
					} else {
						v32 = uint32(x1Hi >> ((i - 2) * 32))
					}

					var v uint16
					if signed {
						if s := int32(v32); s > math.MaxInt16 {
							v = math.MaxInt16
						} else if s < math.MinInt16 {
							s = math.MinInt16
							v = uint16(s)
						} else {
							v = uint16(v32)
						}
					} else {
						if s := int32(v32); s > math.MaxUint16 {
							v = math.MaxUint16
						} else if s < 0 {
							v = 0
						} else {
							v = uint16(v32)
						}
					}
					retLo |= uint64(v) << (i * 16)
				}

				for i := 0; i < 4; i++ {
					var v32 uint32
					if i < 2 {
						v32 = uint32(x2Lo >> (i * 32))
					} else {
						v32 = uint32(x2Hi >> ((i - 2) * 32))
					}

					var v uint16
					if signed {
						if s := int32(v32); s > math.MaxInt16 {
							v = math.MaxInt16
						} else if s < math.MinInt16 {
							s = math.MinInt16
							v = uint16(s)
						} else {
							v = uint16(v32)
						}
					} else {
						if s := int32(v32); s > math.MaxUint16 {
							v = math.MaxUint16
						} else if s < 0 {
							v = 0
						} else {
							v = uint16(v32)
						}
					}
					retHi |= uint64(v) << (i * 16)
				}
			}
			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindV128Dot:
			x2Hi, x2Lo := ce.popValue(), ce.popValue()
			x1Hi, x1Lo := ce.popValue(), ce.popValue()
			lo, hi := v128Dot(x1Hi, x1Lo, x2Hi, x2Lo)
			ce.pushValue(lo)
			ce.pushValue(hi)
			frame.pc++
		case operationKindV128ITruncSatFromF:
			hi, lo := ce.popValue(), ce.popValue()
			signed := op.B3
			var retLo, retHi uint64

			switch op.B1 {
			case shapeF32x4: // f32x4 to i32x4
				for i, f64 := range [4]float64{
					math.Trunc(float64(math.Float32frombits(uint32(lo)))),
					math.Trunc(float64(math.Float32frombits(uint32(lo >> 32)))),
					math.Trunc(float64(math.Float32frombits(uint32(hi)))),
					math.Trunc(float64(math.Float32frombits(uint32(hi >> 32)))),
				} {

					var v uint32
					if math.IsNaN(f64) {
						v = 0
					} else if signed {
						if f64 < math.MinInt32 {
							f64 = math.MinInt32
						} else if f64 > math.MaxInt32 {
							f64 = math.MaxInt32
						}
						v = uint32(int32(f64))
					} else {
						if f64 < 0 {
							f64 = 0
						} else if f64 > math.MaxUint32 {
							f64 = math.MaxUint32
						}
						v = uint32(f64)
					}

					if i < 2 {
						retLo |= uint64(v) << (i * 32)
					} else {
						retHi |= uint64(v) << ((i - 2) * 32)
					}
				}

			case shapeF64x2: // f64x2 to i32x4
				for i, f := range [2]float64{
					math.Trunc(math.Float64frombits(lo)),
					math.Trunc(math.Float64frombits(hi)),
				} {
					var v uint32
					if math.IsNaN(f) {
						v = 0
					} else if signed {
						if f < math.MinInt32 {
							f = math.MinInt32
						} else if f > math.MaxInt32 {
							f = math.MaxInt32
						}
						v = uint32(int32(f))
					} else {
						if f < 0 {
							f = 0
						} else if f > math.MaxUint32 {
							f = math.MaxUint32
						}
						v = uint32(f)
					}

					retLo |= uint64(v) << (i * 32)
				}
			}

			ce.pushValue(retLo)
			ce.pushValue(retHi)
			frame.pc++
		case operationKindAtomicMemoryWait:
			timeout := int64(ce.popValue())
			exp := ce.popValue()
			offset := ce.popMemoryOffset(op)
			// Runtime instead of validation error because the spec intends to allow binaries to include
			// such instructions as long as they are not executed.
			if !memoryInst.Shared {
				panic(wasmruntime.ErrRuntimeExpectedSharedMemory)
			}

			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				if offset%4 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				if int(offset) > len(memoryInst.Buffer)-4 {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(memoryInst.Wait32(offset, uint32(exp), timeout, func(mem *wasm.MemoryInstance, offset uint32) uint32 {
					mem.Mux.Lock()
					defer mem.Mux.Unlock()
					value, _ := mem.ReadUint32Le(offset)
					return value
				}))
			case unsignedTypeI64:
				if offset%8 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				if int(offset) > len(memoryInst.Buffer)-8 {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(memoryInst.Wait64(offset, exp, timeout, func(mem *wasm.MemoryInstance, offset uint32) uint64 {
					mem.Mux.Lock()
					defer mem.Mux.Unlock()
					value, _ := mem.ReadUint64Le(offset)
					return value
				}))
			}
			frame.pc++
		case operationKindAtomicMemoryNotify:
			count := ce.popValue()
			offset := ce.popMemoryOffset(op)
			if offset%4 != 0 {
				panic(wasmruntime.ErrRuntimeUnalignedAtomic)
			}
			// Just a bounds check
			if offset >= memoryInst.Size() {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			res := memoryInst.Notify(offset, uint32(count))
			ce.pushValue(uint64(res))
			frame.pc++
		case operationKindAtomicFence:
			// Memory not required for fence only
			if memoryInst != nil {
				// An empty critical section can be used as a synchronization primitive, which is what
				// fence is. Probably, there are no spectests or defined behavior to confirm this yet.
				memoryInst.Mux.Lock()
				memoryInst.Mux.Unlock() //nolint:staticcheck
			}
			frame.pc++
		case operationKindAtomicLoad:
			offset := ce.popMemoryOffset(op)
			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				if offset%4 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				memoryInst.Mux.Lock()
				val, ok := memoryInst.ReadUint32Le(offset)
				memoryInst.Mux.Unlock()
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(uint64(val))
			case unsignedTypeI64:
				if offset%8 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				memoryInst.Mux.Lock()
				val, ok := memoryInst.ReadUint64Le(offset)
				memoryInst.Mux.Unlock()
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				ce.pushValue(val)
			}
			frame.pc++
		case operationKindAtomicLoad8:
			offset := ce.popMemoryOffset(op)
			memoryInst.Mux.Lock()
			val, ok := memoryInst.ReadByte(offset)
			memoryInst.Mux.Unlock()
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			ce.pushValue(uint64(val))
			frame.pc++
		case operationKindAtomicLoad16:
			offset := ce.popMemoryOffset(op)
			if offset%2 != 0 {
				panic(wasmruntime.ErrRuntimeUnalignedAtomic)
			}
			memoryInst.Mux.Lock()
			val, ok := memoryInst.ReadUint16Le(offset)
			memoryInst.Mux.Unlock()
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			ce.pushValue(uint64(val))
			frame.pc++
		case operationKindAtomicStore:
			val := ce.popValue()
			offset := ce.popMemoryOffset(op)
			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				if offset%4 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				memoryInst.Mux.Lock()
				ok := memoryInst.WriteUint32Le(offset, uint32(val))
				memoryInst.Mux.Unlock()
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
			case unsignedTypeI64:
				if offset%8 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				memoryInst.Mux.Lock()
				ok := memoryInst.WriteUint64Le(offset, val)
				memoryInst.Mux.Unlock()
				if !ok {
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
			}
			frame.pc++
		case operationKindAtomicStore8:
			val := byte(ce.popValue())
			offset := ce.popMemoryOffset(op)
			memoryInst.Mux.Lock()
			ok := memoryInst.WriteByte(offset, val)
			memoryInst.Mux.Unlock()
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case operationKindAtomicStore16:
			val := uint16(ce.popValue())
			offset := ce.popMemoryOffset(op)
			if offset%2 != 0 {
				panic(wasmruntime.ErrRuntimeUnalignedAtomic)
			}
			memoryInst.Mux.Lock()
			ok := memoryInst.WriteUint16Le(offset, val)
			memoryInst.Mux.Unlock()
			if !ok {
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			frame.pc++
		case operationKindAtomicRMW:
			val := ce.popValue()
			offset := ce.popMemoryOffset(op)
			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				if offset%4 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				memoryInst.Mux.Lock()
				old, ok := memoryInst.ReadUint32Le(offset)
				if !ok {
					memoryInst.Mux.Unlock()
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				var newVal uint32
				switch atomicArithmeticOp(op.B2) {
				case atomicArithmeticOpAdd:
					newVal = old + uint32(val)
				case atomicArithmeticOpSub:
					newVal = old - uint32(val)
				case atomicArithmeticOpAnd:
					newVal = old & uint32(val)
				case atomicArithmeticOpOr:
					newVal = old | uint32(val)
				case atomicArithmeticOpXor:
					newVal = old ^ uint32(val)
				case atomicArithmeticOpNop:
					newVal = uint32(val)
				}
				memoryInst.WriteUint32Le(offset, newVal)
				memoryInst.Mux.Unlock()
				ce.pushValue(uint64(old))
			case unsignedTypeI64:
				if offset%8 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				memoryInst.Mux.Lock()
				old, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					memoryInst.Mux.Unlock()
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				var newVal uint64
				switch atomicArithmeticOp(op.B2) {
				case atomicArithmeticOpAdd:
					newVal = old + val
				case atomicArithmeticOpSub:
					newVal = old - val
				case atomicArithmeticOpAnd:
					newVal = old & val
				case atomicArithmeticOpOr:
					newVal = old | val
				case atomicArithmeticOpXor:
					newVal = old ^ val
				case atomicArithmeticOpNop:
					newVal = val
				}
				memoryInst.WriteUint64Le(offset, newVal)
				memoryInst.Mux.Unlock()
				ce.pushValue(old)
			}
			frame.pc++
		case operationKindAtomicRMW8:
			val := ce.popValue()
			offset := ce.popMemoryOffset(op)
			memoryInst.Mux.Lock()
			old, ok := memoryInst.ReadByte(offset)
			if !ok {
				memoryInst.Mux.Unlock()
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			arg := byte(val)
			var newVal byte
			switch atomicArithmeticOp(op.B2) {
			case atomicArithmeticOpAdd:
				newVal = old + arg
			case atomicArithmeticOpSub:
				newVal = old - arg
			case atomicArithmeticOpAnd:
				newVal = old & arg
			case atomicArithmeticOpOr:
				newVal = old | arg
			case atomicArithmeticOpXor:
				newVal = old ^ arg
			case atomicArithmeticOpNop:
				newVal = arg
			}
			memoryInst.WriteByte(offset, newVal)
			memoryInst.Mux.Unlock()
			ce.pushValue(uint64(old))
			frame.pc++
		case operationKindAtomicRMW16:
			val := ce.popValue()
			offset := ce.popMemoryOffset(op)
			if offset%2 != 0 {
				panic(wasmruntime.ErrRuntimeUnalignedAtomic)
			}
			memoryInst.Mux.Lock()
			old, ok := memoryInst.ReadUint16Le(offset)
			if !ok {
				memoryInst.Mux.Unlock()
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			arg := uint16(val)
			var newVal uint16
			switch atomicArithmeticOp(op.B2) {
			case atomicArithmeticOpAdd:
				newVal = old + arg
			case atomicArithmeticOpSub:
				newVal = old - arg
			case atomicArithmeticOpAnd:
				newVal = old & arg
			case atomicArithmeticOpOr:
				newVal = old | arg
			case atomicArithmeticOpXor:
				newVal = old ^ arg
			case atomicArithmeticOpNop:
				newVal = arg
			}
			memoryInst.WriteUint16Le(offset, newVal)
			memoryInst.Mux.Unlock()
			ce.pushValue(uint64(old))
			frame.pc++
		case operationKindAtomicRMWCmpxchg:
			rep := ce.popValue()
			exp := ce.popValue()
			offset := ce.popMemoryOffset(op)
			switch unsignedType(op.B1) {
			case unsignedTypeI32:
				if offset%4 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				memoryInst.Mux.Lock()
				old, ok := memoryInst.ReadUint32Le(offset)
				if !ok {
					memoryInst.Mux.Unlock()
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if old == uint32(exp) {
					memoryInst.WriteUint32Le(offset, uint32(rep))
				}
				memoryInst.Mux.Unlock()
				ce.pushValue(uint64(old))
			case unsignedTypeI64:
				if offset%8 != 0 {
					panic(wasmruntime.ErrRuntimeUnalignedAtomic)
				}
				memoryInst.Mux.Lock()
				old, ok := memoryInst.ReadUint64Le(offset)
				if !ok {
					memoryInst.Mux.Unlock()
					panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
				}
				if old == exp {
					memoryInst.WriteUint64Le(offset, rep)
				}
				memoryInst.Mux.Unlock()
				ce.pushValue(old)
			}
			frame.pc++
		case operationKindAtomicRMW8Cmpxchg:
			rep := byte(ce.popValue())
			exp := byte(ce.popValue())
			offset := ce.popMemoryOffset(op)
			memoryInst.Mux.Lock()
			old, ok := memoryInst.ReadByte(offset)
			if !ok {
				memoryInst.Mux.Unlock()
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			if old == exp {
				memoryInst.WriteByte(offset, rep)
			}
			memoryInst.Mux.Unlock()
			ce.pushValue(uint64(old))
			frame.pc++
		case operationKindAtomicRMW16Cmpxchg:
			rep := uint16(ce.popValue())
			exp := uint16(ce.popValue())
			offset := ce.popMemoryOffset(op)
			if offset%2 != 0 {
				panic(wasmruntime.ErrRuntimeUnalignedAtomic)
			}
			memoryInst.Mux.Lock()
			old, ok := memoryInst.ReadUint16Le(offset)
			if !ok {
				memoryInst.Mux.Unlock()
				panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
			}
			if old == exp {
				memoryInst.WriteUint16Le(offset, rep)
			}
			memoryInst.Mux.Unlock()
			ce.pushValue(uint64(old))
			frame.pc++
		default:
			frame.pc++
		}
	}
	ce.popFrame()
}

func wasmCompatMax32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(moremath.WasmCompatMax32(
		math.Float32frombits(v1),
		math.Float32frombits(v2),
	)))
}

func wasmCompatMin32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(moremath.WasmCompatMin32(
		math.Float32frombits(v1),
		math.Float32frombits(v2),
	)))
}

func addFloat32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(math.Float32frombits(v1) + math.Float32frombits(v2)))
}

func subFloat32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(math.Float32frombits(v1) - math.Float32frombits(v2)))
}

func mulFloat32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(math.Float32frombits(v1) * math.Float32frombits(v2)))
}

func divFloat32bits(v1, v2 uint32) uint64 {
	return uint64(math.Float32bits(math.Float32frombits(v1) / math.Float32frombits(v2)))
}

// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#xref-exec-numerics-op-flt-mathrm-flt-n-z-1-z-2
func flt32(z1, z2 float32) bool {
	if z1 != z1 || z2 != z2 {
		return false
	} else if z1 == z2 {
		return false
	} else if math.IsInf(float64(z1), 1) {
		return false
	} else if math.IsInf(float64(z1), -1) {
		return true
	} else if math.IsInf(float64(z2), 1) {
		return true
	} else if math.IsInf(float64(z2), -1) {
		return false
	}
	return z1 < z2
}

// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/numerics.html#xref-exec-numerics-op-flt-mathrm-flt-n-z-1-z-2
func flt64(z1, z2 float64) bool {
	if z1 != z1 || z2 != z2 {
		return false
	} else if z1 == z2 {
		return false
	} else if math.IsInf(z1, 1) {
		return false
	} else if math.IsInf(z1, -1) {
		return true
	} else if math.IsInf(z2, 1) {
		return true
	} else if math.IsInf(z2, -1) {
		return false
	}
	return z1 < z2
}

func i8RoundingAverage(v1, v2 byte) byte {
	// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#lane-wise-integer-rounding-average
	return byte((uint16(v1) + uint16(v2) + uint16(1)) / 2)
}

func i16RoundingAverage(v1, v2 uint16) uint16 {
	// https://github.com/WebAssembly/spec/blob/wg-2.0.draft1/proposals/simd/SIMD.md#lane-wise-integer-rounding-average
	return uint16((uint32(v1) + uint32(v2) + 1) / 2)
}

func i8Abs(v byte) byte {
	if i := int8(v); i < 0 {
		return byte(-i)
	} else {
		return byte(i)
	}
}

func i8MaxU(v1, v2 byte) byte {
	if v1 < v2 {
		return v2
	} else {
		return v1
	}
}

func i8MinU(v1, v2 byte) byte {
	if v1 > v2 {
		return v2
	} else {
		return v1
	}
}

func i8MaxS(v1, v2 byte) byte {
	if int8(v1) < int8(v2) {
		return v2
	} else {
		return v1
	}
}

func i8MinS(v1, v2 byte) byte {
	if int8(v1) > int8(v2) {
		return v2
	} else {
		return v1
	}
}

func i16MaxU(v1, v2 uint16) uint16 {
	if v1 < v2 {
		return v2
	} else {
		return v1
	}
}

func i16MinU(v1, v2 uint16) uint16 {
	if v1 > v2 {
		return v2
	} else {
		return v1
	}
}

func i16MaxS(v1, v2 uint16) uint16 {
	if int16(v1) < int16(v2) {
		return v2
	} else {
		return v1
	}
}

func i16MinS(v1, v2 uint16) uint16 {
	if int16(v1) > int16(v2) {
		return v2
	} else {
		return v1
	}
}

func i32MaxU(v1, v2 uint32) uint32 {
	if v1 < v2 {
		return v2
	} else {
		return v1
	}
}

func i32MinU(v1, v2 uint32) uint32 {
	if v1 > v2 {
		return v2
	} else {
		return v1
	}
}

func i32MaxS(v1, v2 uint32) uint32 {
	if int32(v1) < int32(v2) {
		return v2
	} else {
		return v1
	}
}

func i32MinS(v1, v2 uint32) uint32 {
	if int32(v1) > int32(v2) {
		return v2
	} else {
		return v1
	}
}

func i16Abs(v uint16) uint16 {
	if i := int16(v); i < 0 {
		return uint16(-i)
	} else {
		return uint16(i)
	}
}

func i32Abs(v uint32) uint32 {
	if i := int32(v); i < 0 {
		return uint32(-i)
	} else {
		return uint32(i)
	}
}

func (ce *callEngine) callNativeFuncWithListener(ctx context.Context, m *wasm.ModuleInstance, f *function, fnl experimental.FunctionListener) context.Context {
	def, typ := f.definition(), f.funcType

	ce.stackIterator.reset(ce.stack, ce.frames, f)
	fnl.Before(ctx, m, def, ce.peekValues(typ.ParamNumInUint64), &ce.stackIterator)
	ce.stackIterator.clear()
	ce.callNativeFunc(ctx, m, f)
	fnl.After(ctx, m, def, ce.peekValues(typ.ResultNumInUint64))
	return ctx
}

// popMemoryOffset takes a memory offset off the stack for use in load and store instructions.
// As the top of stack value is 64-bit, this ensures it is in range before returning it.
func (ce *callEngine) popMemoryOffset(op *unionOperation) uint32 {
	offset := op.U2 + ce.popValue()
	if offset > math.MaxUint32 {
		panic(wasmruntime.ErrRuntimeOutOfBoundsMemoryAccess)
	}
	return uint32(offset)
}

func (ce *callEngine) callGoFuncWithStack(ctx context.Context, m *wasm.ModuleInstance, f *function) {
	typ := f.funcType
	paramLen := typ.ParamNumInUint64
	resultLen := typ.ResultNumInUint64
	stackLen := paramLen

	// In the interpreter engine, ce.stack may only have capacity to store
	// parameters. Grow when there are more results than parameters.
	if growLen := resultLen - paramLen; growLen > 0 {
		for i := 0; i < growLen; i++ {
			ce.stack = append(ce.stack, 0)
		}
		stackLen += growLen
	}

	// Pass the stack elements to the go function.
	stack := ce.stack[len(ce.stack)-stackLen:]
	ce.callGoFunc(ctx, m, f, stack)

	// Shrink the stack when there were more parameters than results.
	if shrinkLen := paramLen - resultLen; shrinkLen > 0 {
		ce.stack = ce.stack[0 : len(ce.stack)-shrinkLen]
	}
}

// v128Dot performs a dot product of two 64-bit vectors.
// Note: for some reason (which I suspect is due to a bug in Go compiler's regalloc),
// inlining this function causes a bug which happens **only when** we run with -race AND arm64 AND Go 1.22.
func v128Dot(x1Hi, x1Lo, x2Hi, x2Lo uint64) (uint64, uint64) {
	r1 := int32(int16(x1Lo>>0)) * int32(int16(x2Lo>>0))
	r2 := int32(int16(x1Lo>>16)) * int32(int16(x2Lo>>16))
	r3 := int32(int16(x1Lo>>32)) * int32(int16(x2Lo>>32))
	r4 := int32(int16(x1Lo>>48)) * int32(int16(x2Lo>>48))
	r5 := int32(int16(x1Hi>>0)) * int32(int16(x2Hi>>0))
	r6 := int32(int16(x1Hi>>16)) * int32(int16(x2Hi>>16))
	r7 := int32(int16(x1Hi>>32)) * int32(int16(x2Hi>>32))
	r8 := int32(int16(x1Hi>>48)) * int32(int16(x2Hi>>48))
	return uint64(uint32(r1+r2)) | (uint64(uint32(r3+r4)) << 32), uint64(uint32(r5+r6)) | (uint64(uint32(r7+r8)) << 32)
}
