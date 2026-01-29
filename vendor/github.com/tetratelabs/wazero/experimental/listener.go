package experimental

import (
	"context"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/expctxkeys"
)

// StackIterator allows iterating on each function of the call stack, starting
// from the top. At least one call to Next() is required to start the iteration.
//
// Note: The iterator provides a view of the call stack at the time of
// iteration. As a result, parameter values may be different than the ones their
// function was called with.
type StackIterator interface {
	// Next moves the iterator to the next function in the stack. Returns
	// false if it reached the bottom of the stack.
	Next() bool
	// Function describes the function called by the current frame.
	Function() InternalFunction
	// ProgramCounter returns the program counter associated with the
	// function call.
	ProgramCounter() ProgramCounter
}

// WithFunctionListenerFactory registers a FunctionListenerFactory
// with the context.
func WithFunctionListenerFactory(ctx context.Context, factory FunctionListenerFactory) context.Context {
	return context.WithValue(ctx, expctxkeys.FunctionListenerFactoryKey{}, factory)
}

// FunctionListenerFactory returns FunctionListeners to be notified when a
// function is called.
type FunctionListenerFactory interface {
	// NewFunctionListener returns a FunctionListener for a defined function.
	// If nil is returned, no listener will be notified.
	NewFunctionListener(api.FunctionDefinition) FunctionListener
	// ^^ A single instance can be returned to avoid instantiating a listener
	// per function, especially as they may be thousands of functions. Shared
	// listeners use their FunctionDefinition parameter to clarify.
}

// FunctionListener can be registered for any function via
// FunctionListenerFactory to be notified when the function is called.
type FunctionListener interface {
	// Before is invoked before a function is called.
	//
	// There is always one corresponding call to After or Abort for each call to
	// Before. This guarantee allows the listener to maintain an internal stack
	// to perform correlations between the entry and exit of functions.
	//
	// # Params
	//
	//   - ctx: the context of the caller function which must be the same
	//	   instance or parent of the result.
	//   - mod: the calling module.
	//   - def: the function definition.
	//   - params:  api.ValueType encoded parameters.
	//   - stackIterator: iterator on the call stack. At least one entry is
	//     guaranteed (the called function), whose Args() will be equal to
	//     params. The iterator will be reused between calls to Before.
	//
	// Note: api.Memory is meant for inspection, not modification.
	// mod can be cast to InternalModule to read non-exported globals.
	Before(ctx context.Context, mod api.Module, def api.FunctionDefinition, params []uint64, stackIterator StackIterator)

	// After is invoked after a function is called.
	//
	// # Params
	//
	//   - ctx: the context of the caller function.
	//   - mod: the calling module.
	//   - def: the function definition.
	//   - results: api.ValueType encoded results.
	//
	// # Notes
	//
	//   - api.Memory is meant for inspection, not modification.
	//   - This is not called when a host function panics, or a guest function traps.
	//      See Abort for more details.
	After(ctx context.Context, mod api.Module, def api.FunctionDefinition, results []uint64)

	// Abort is invoked when a function does not return due to a trap or panic.
	//
	// # Params
	//
	//   - ctx: the context of the caller function.
	//   - mod: the calling module.
	//   - def: the function definition.
	//   - err: the error value representing the reason why the function aborted.
	//
	// # Notes
	//
	//   - api.Memory is meant for inspection, not modification.
	Abort(ctx context.Context, mod api.Module, def api.FunctionDefinition, err error)
}

// FunctionListenerFunc is a function type implementing the FunctionListener
// interface, making it possible to use regular functions and methods as
// listeners of function invocation.
//
// The FunctionListener interface declares two methods (Before and After),
// but this type invokes its value only when Before is called. It is best
// suites for cases where the host does not need to perform correlation
// between the start and end of the function call.
type FunctionListenerFunc func(context.Context, api.Module, api.FunctionDefinition, []uint64, StackIterator)

// Before satisfies the FunctionListener interface, calls f.
func (f FunctionListenerFunc) Before(ctx context.Context, mod api.Module, def api.FunctionDefinition, params []uint64, stackIterator StackIterator) {
	f(ctx, mod, def, params, stackIterator)
}

// After is declared to satisfy the FunctionListener interface, but it does
// nothing.
func (f FunctionListenerFunc) After(context.Context, api.Module, api.FunctionDefinition, []uint64) {
}

// Abort is declared to satisfy the FunctionListener interface, but it does
// nothing.
func (f FunctionListenerFunc) Abort(context.Context, api.Module, api.FunctionDefinition, error) {
}

// FunctionListenerFactoryFunc is a function type implementing the
// FunctionListenerFactory interface, making it possible to use regular
// functions and methods as factory of function listeners.
type FunctionListenerFactoryFunc func(api.FunctionDefinition) FunctionListener

// NewFunctionListener satisfies the FunctionListenerFactory interface, calls f.
func (f FunctionListenerFactoryFunc) NewFunctionListener(def api.FunctionDefinition) FunctionListener {
	return f(def)
}

// MultiFunctionListenerFactory constructs a FunctionListenerFactory which
// combines the listeners created by each of the factories passed as arguments.
//
// This function is useful when multiple listeners need to be hooked to a module
// because the propagation mechanism based on installing a listener factory in
// the context.Context used when instantiating modules allows for a single
// listener to be installed.
//
// The stack iterator passed to the Before method is reset so that each listener
// can iterate the call stack independently without impacting the ability of
// other listeners to do so.
func MultiFunctionListenerFactory(factories ...FunctionListenerFactory) FunctionListenerFactory {
	multi := make(multiFunctionListenerFactory, len(factories))
	copy(multi, factories)
	return multi
}

type multiFunctionListenerFactory []FunctionListenerFactory

func (multi multiFunctionListenerFactory) NewFunctionListener(def api.FunctionDefinition) FunctionListener {
	var lstns []FunctionListener
	for _, factory := range multi {
		if lstn := factory.NewFunctionListener(def); lstn != nil {
			lstns = append(lstns, lstn)
		}
	}
	switch len(lstns) {
	case 0:
		return nil
	case 1:
		return lstns[0]
	default:
		return &multiFunctionListener{lstns: lstns}
	}
}

type multiFunctionListener struct {
	lstns []FunctionListener
	stack stackIterator
}

func (multi *multiFunctionListener) Before(ctx context.Context, mod api.Module, def api.FunctionDefinition, params []uint64, si StackIterator) {
	multi.stack.base = si
	for _, lstn := range multi.lstns {
		multi.stack.index = -1
		lstn.Before(ctx, mod, def, params, &multi.stack)
	}
}

func (multi *multiFunctionListener) After(ctx context.Context, mod api.Module, def api.FunctionDefinition, results []uint64) {
	for _, lstn := range multi.lstns {
		lstn.After(ctx, mod, def, results)
	}
}

func (multi *multiFunctionListener) Abort(ctx context.Context, mod api.Module, def api.FunctionDefinition, err error) {
	for _, lstn := range multi.lstns {
		lstn.Abort(ctx, mod, def, err)
	}
}

type stackIterator struct {
	base  StackIterator
	index int
	pcs   []uint64
	fns   []InternalFunction
}

func (si *stackIterator) Next() bool {
	if si.base != nil {
		si.pcs = si.pcs[:0]
		si.fns = si.fns[:0]

		for si.base.Next() {
			si.pcs = append(si.pcs, uint64(si.base.ProgramCounter()))
			si.fns = append(si.fns, si.base.Function())
		}

		si.base = nil
	}
	si.index++
	return si.index < len(si.pcs)
}

func (si *stackIterator) ProgramCounter() ProgramCounter {
	return ProgramCounter(si.pcs[si.index])
}

func (si *stackIterator) Function() InternalFunction {
	return si.fns[si.index]
}

// StackFrame represents a frame on the call stack.
type StackFrame struct {
	Function     api.Function
	Params       []uint64
	Results      []uint64
	PC           uint64
	SourceOffset uint64
}

type internalFunction struct {
	definition   api.FunctionDefinition
	sourceOffset uint64
}

func (f internalFunction) Definition() api.FunctionDefinition {
	return f.definition
}

func (f internalFunction) SourceOffsetForPC(pc ProgramCounter) uint64 {
	return f.sourceOffset
}

// stackFrameIterator is an implementation of the experimental.stackFrameIterator
// interface.
type stackFrameIterator struct {
	index int
	stack []StackFrame
	fndef []api.FunctionDefinition
}

func (si *stackFrameIterator) Next() bool {
	si.index++
	return si.index < len(si.stack)
}

func (si *stackFrameIterator) Function() InternalFunction {
	return internalFunction{
		definition:   si.fndef[si.index],
		sourceOffset: si.stack[si.index].SourceOffset,
	}
}

func (si *stackFrameIterator) ProgramCounter() ProgramCounter {
	return ProgramCounter(si.stack[si.index].PC)
}

// NewStackIterator constructs a stack iterator from a list of stack frames.
// The top most frame is the last one.
func NewStackIterator(stack ...StackFrame) StackIterator {
	si := &stackFrameIterator{
		index: -1,
		stack: make([]StackFrame, len(stack)),
		fndef: make([]api.FunctionDefinition, len(stack)),
	}
	for i := range stack {
		si.stack[i] = stack[len(stack)-(i+1)]
	}
	// The size of function definition is only one pointer which should allow
	// the compiler to optimize the conversion to api.FunctionDefinition; but
	// the presence of internal.WazeroOnlyType, despite being defined as an
	// empty struct, forces a heap allocation that we amortize by caching the
	// result.
	for i, frame := range stack {
		si.fndef[i] = frame.Function.Definition()
	}
	return si
}

// BenchmarkFunctionListener implements a benchmark for function listeners.
//
// The benchmark calls Before and After methods repeatedly using the provided
// module an stack frames to invoke the methods.
//
// The stack frame is a representation of the call stack that the Before method
// will be invoked with. The top of the stack is stored at index zero. The stack
// must contain at least one frame or the benchmark will fail.
func BenchmarkFunctionListener(n int, module api.Module, stack []StackFrame, listener FunctionListener) {
	if len(stack) == 0 {
		panic("cannot benchmark function listener with an empty stack")
	}

	ctx := context.Background()
	def := stack[0].Function.Definition()
	params := stack[0].Params
	results := stack[0].Results
	stackIterator := &stackIterator{base: NewStackIterator(stack...)}

	for i := 0; i < n; i++ {
		stackIterator.index = -1
		listener.Before(ctx, module, def, params, stackIterator)
		listener.After(ctx, module, def, results)
	}
}

// TODO: the calls to Abort are not yet tested in internal/testing/enginetest,
// but they are validated indirectly in tests which exercise host logging,
// like Test_procExit in imports/wasi_snapshot_preview1. Eventually we should
// add dedicated tests to validate the behavior of the interpreter and compiler
// engines independently.
