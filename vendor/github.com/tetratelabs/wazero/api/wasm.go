// Package api includes constants and interfaces used by both end-users and internal implementations.
package api

import (
	"context"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/internalapi"
)

// ExternType classifies imports and exports with their respective types.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#external-types%E2%91%A0
type ExternType = byte

const (
	ExternTypeFunc   ExternType = 0x00
	ExternTypeTable  ExternType = 0x01
	ExternTypeMemory ExternType = 0x02
	ExternTypeGlobal ExternType = 0x03
)

// The below are exported to consolidate parsing behavior for external types.
const (
	// ExternTypeFuncName is the name of the WebAssembly 1.0 (20191205) Text Format field for ExternTypeFunc.
	ExternTypeFuncName = "func"
	// ExternTypeTableName is the name of the WebAssembly 1.0 (20191205) Text Format field for ExternTypeTable.
	ExternTypeTableName = "table"
	// ExternTypeMemoryName is the name of the WebAssembly 1.0 (20191205) Text Format field for ExternTypeMemory.
	ExternTypeMemoryName = "memory"
	// ExternTypeGlobalName is the name of the WebAssembly 1.0 (20191205) Text Format field for ExternTypeGlobal.
	ExternTypeGlobalName = "global"
)

// ExternTypeName returns the name of the WebAssembly 1.0 (20191205) Text Format field of the given type.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#exports%E2%91%A4
func ExternTypeName(et ExternType) string {
	switch et {
	case ExternTypeFunc:
		return ExternTypeFuncName
	case ExternTypeTable:
		return ExternTypeTableName
	case ExternTypeMemory:
		return ExternTypeMemoryName
	case ExternTypeGlobal:
		return ExternTypeGlobalName
	}
	return fmt.Sprintf("%#x", et)
}

// ValueType describes a parameter or result type mapped to a WebAssembly
// function signature.
//
// The following describes how to convert between Wasm and Golang types:
//
//   - ValueTypeI32 - EncodeU32 DecodeU32 for uint32 / EncodeI32 DecodeI32 for int32
//   - ValueTypeI64 - uint64(int64)
//   - ValueTypeF32 - EncodeF32 DecodeF32 from float32
//   - ValueTypeF64 - EncodeF64 DecodeF64 from float64
//   - ValueTypeExternref - unintptr(unsafe.Pointer(p)) where p is any pointer
//     type in Go (e.g. *string)
//
// e.g. Given a Text Format type use (param i64) (result i64), no conversion is
// necessary.
//
//	results, _ := fn(ctx, input)
//	result := result[0]
//
// e.g. Given a Text Format type use (param f64) (result f64), conversion is
// necessary.
//
//	results, _ := fn(ctx, api.EncodeF64(input))
//	result := api.DecodeF64(result[0])
//
// Note: This is a type alias as it is easier to encode and decode in the
// binary format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-valtype
type ValueType = byte

const (
	// ValueTypeI32 is a 32-bit integer.
	ValueTypeI32 ValueType = 0x7f
	// ValueTypeI64 is a 64-bit integer.
	ValueTypeI64 ValueType = 0x7e
	// ValueTypeF32 is a 32-bit floating point number.
	ValueTypeF32 ValueType = 0x7d
	// ValueTypeF64 is a 64-bit floating point number.
	ValueTypeF64 ValueType = 0x7c

	// ValueTypeExternref is a externref type.
	//
	// Note: in wazero, externref type value are opaque raw 64-bit pointers,
	// and the ValueTypeExternref type in the signature will be translated as
	// uintptr in wazero's API level.
	//
	// For example, given the import function:
	//	(func (import "env" "f") (param externref) (result externref))
	//
	// This can be defined in Go as:
	//  r.NewHostModuleBuilder("env").
	//		NewFunctionBuilder().
	//		WithFunc(func(context.Context, _ uintptr) (_ uintptr) { return }).
	//		Export("f")
	//
	// Note: The usage of this type is toggled with api.CoreFeatureBulkMemoryOperations.
	ValueTypeExternref ValueType = 0x6f
)

// ValueTypeName returns the type name of the given ValueType as a string.
// These type names match the names used in the WebAssembly text format.
//
// Note: This returns "unknown", if an undefined ValueType value is passed.
func ValueTypeName(t ValueType) string {
	switch t {
	case ValueTypeI32:
		return "i32"
	case ValueTypeI64:
		return "i64"
	case ValueTypeF32:
		return "f32"
	case ValueTypeF64:
		return "f64"
	case ValueTypeExternref:
		return "externref"
	}
	return "unknown"
}

// Module is a sandboxed, ready to execute Wasm module. This can be used to get exported functions, etc.
//
// In WebAssembly terminology, this corresponds to a "Module Instance", but wazero calls pre-instantiation module as
// "Compiled Module" as in wazero.CompiledModule, therefore we call this post-instantiation module simply "Module".
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#module-instances%E2%91%A0
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
//   - Closing the wazero.Runtime closes any Module it instantiated.
type Module interface {
	fmt.Stringer

	// Name is the name this module was instantiated with. Exported functions can be imported with this name.
	Name() string

	// Memory returns a memory defined in this module or nil if there are none wasn't.
	Memory() Memory

	// ExportedFunction returns a function exported from this module or nil if it wasn't.
	//
	// # Notes
	//   - The default wazero.ModuleConfig attempts to invoke `_start`, which
	//     in rare cases can close the module. When in doubt, check IsClosed prior
	//     to invoking a function export after instantiation.
	//   - The semantics of host functions assumes the existence of an "importing module" because, for example, the host function needs access to
	//     the memory of the importing module. Therefore, direct use of ExportedFunction is forbidden for host modules.
	//     Practically speaking, it is usually meaningless to directly call a host function from Go code as it is already somewhere in Go code.
	ExportedFunction(name string) Function

	// ExportedFunctionDefinitions returns all the exported function
	// definitions in this module, keyed on export name.
	ExportedFunctionDefinitions() map[string]FunctionDefinition

	// TODO: Table

	// ExportedMemory returns a memory exported from this module or nil if it wasn't.
	//
	// WASI modules require exporting a Memory named "memory". This means that a module successfully initialized
	// as a WASI Command or Reactor will never return nil for this name.
	//
	// See https://github.com/WebAssembly/WASI/blob/snapshot-01/design/application-abi.md#current-unstable-abi
	ExportedMemory(name string) Memory

	// ExportedMemoryDefinitions returns all the exported memory definitions
	// in this module, keyed on export name.
	//
	// Note: As of WebAssembly Core Specification 2.0, there can be at most one
	// memory.
	ExportedMemoryDefinitions() map[string]MemoryDefinition

	// ExportedGlobal a global exported from this module or nil if it wasn't.
	ExportedGlobal(name string) Global

	// CloseWithExitCode releases resources allocated for this Module. Use a non-zero exitCode parameter to indicate a
	// failure to ExportedFunction callers.
	//
	// The error returned here, if present, is about resource de-allocation (such as I/O errors). Only the last error is
	// returned, so a non-nil return means at least one error happened. Regardless of error, this Module will
	// be removed, making its name available again.
	//
	// Calling this inside a host function is safe, and may cause ExportedFunction callers to receive a sys.ExitError
	// with the exitCode.
	CloseWithExitCode(ctx context.Context, exitCode uint32) error

	// Closer closes this module by delegating to CloseWithExitCode with an exit code of zero.
	Closer

	// IsClosed returns true if the module is closed, so no longer usable.
	//
	// This can happen for the following reasons:
	//   - Closer was called directly.
	//   - A guest function called Closer indirectly, such as `_start` calling
	//     `proc_exit`, which internally closed the module.
	//   - wazero.RuntimeConfig `WithCloseOnContextDone` was enabled and a
	//     context completion closed the module.
	//
	// Where any of the above are possible, check this value before calling an
	// ExportedFunction, even if you didn't formerly receive a sys.ExitError.
	// sys.ExitError is only returned on non-zero code, something that closes
	// the module successfully will not result it one.
	IsClosed() bool

	internalapi.WazeroOnly
}

// Closer closes a resource.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type Closer interface {
	// Close closes the resource.
	//
	// Note: The context parameter is used for value lookup, such as for
	// logging. A canceled or otherwise done context will not prevent Close
	// from succeeding.
	Close(context.Context) error
}

// ExportDefinition is a WebAssembly type exported in a module
// (wazero.CompiledModule).
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#exports%E2%91%A0
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type ExportDefinition interface {
	// ModuleName is the possibly empty name of the module defining this
	// export.
	//
	// Note: This may be different from Module.Name, because a compiled module
	// can be instantiated multiple times as different names.
	ModuleName() string

	// Index is the position in the module's index, imports first.
	Index() uint32

	// Import returns true with the module and name when this was imported.
	// Otherwise, it returns false.
	//
	// Note: Empty string is valid for both names in the WebAssembly Core
	// Specification, so "" "" is possible.
	Import() (moduleName, name string, isImport bool)

	// ExportNames include all exported names.
	//
	// Note: The empty name is allowed in the WebAssembly Core Specification,
	// so "" is possible.
	ExportNames() []string

	internalapi.WazeroOnly
}

// MemoryDefinition is a WebAssembly memory exported in a module
// (wazero.CompiledModule). Units are in pages (64KB).
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#exports%E2%91%A0
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type MemoryDefinition interface {
	ExportDefinition

	// Min returns the possibly zero initial count of 64KB pages.
	Min() uint32

	// Max returns the possibly zero max count of 64KB pages, or false if
	// unbounded.
	Max() (uint32, bool)

	internalapi.WazeroOnly
}

// FunctionDefinition is a WebAssembly function exported in a module
// (wazero.CompiledModule).
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#exports%E2%91%A0
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type FunctionDefinition interface {
	ExportDefinition

	// Name is the module-defined name of the function, which is not necessarily
	// the same as its export name.
	Name() string

	// DebugName identifies this function based on its Index or Name in the
	// module. This is used for errors and stack traces. e.g. "env.abort".
	//
	// When the function name is empty, a substitute name is generated by
	// prefixing '$' to its position in the index. Ex ".$0" is the
	// first function (possibly imported) in an unnamed module.
	//
	// The format is dot-delimited module and function name, but there are no
	// restrictions on the module and function name. This means either can be
	// empty or include dots. e.g. "x.x.x" could mean module "x" and name "x.x",
	// or it could mean module "x.x" and name "x".
	//
	// Note: This name is stable regardless of import or export. For example,
	// if Import returns true, the value is still based on the Name or Index
	// and not the imported function name.
	DebugName() string

	// GoFunction is non-nil when implemented by the embedder instead of a wasm
	// binary, e.g. via wazero.HostModuleBuilder
	//
	// The expected results are nil, GoFunction or GoModuleFunction.
	GoFunction() interface{}

	// ParamTypes are the possibly empty sequence of value types accepted by a
	// function with this signature.
	//
	// See ValueType documentation for encoding rules.
	ParamTypes() []ValueType

	// ParamNames are index-correlated with ParamTypes or nil if not available
	// for one or more parameters.
	ParamNames() []string

	// ResultTypes are the results of the function.
	//
	// When WebAssembly 1.0 (20191205), there can be at most one result.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#result-types%E2%91%A0
	//
	// See ValueType documentation for encoding rules.
	ResultTypes() []ValueType

	// ResultNames are index-correlated with ResultTypes or nil if not
	// available for one or more results.
	ResultNames() []string

	internalapi.WazeroOnly
}

// Function is a WebAssembly function exported from an instantiated module
// (wazero.Runtime InstantiateModule).
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-func
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type Function interface {
	// Definition is metadata about this function from its defining module.
	Definition() FunctionDefinition

	// Call invokes the function with the given parameters and returns any
	// results or an error for any failure looking up or invoking the function.
	//
	// Encoding is described in Definition, and supplying an incorrect count of
	// parameters vs FunctionDefinition.ParamTypes is an error.
	//
	// If the exporting Module was closed during this call, the error returned
	// may be a sys.ExitError. See Module.CloseWithExitCode for details.
	//
	// Call is not goroutine-safe, therefore it is recommended to create
	// another Function if you want to invoke the same function concurrently.
	// On the other hand, sequential invocations of Call is allowed.
	// However, this should not be called multiple times until the previous Call returns.
	//
	// To safely encode/decode params/results expressed as uint64, users are encouraged to
	// use api.EncodeXXX or DecodeXXX functions. See the docs on api.ValueType.
	//
	// When RuntimeConfig.WithCloseOnContextDone is toggled, the invocation of this Call method is ensured to be closed
	// whenever one of the three conditions is met. In the event of close, sys.ExitError will be returned and
	// the api.Module from which this api.Function is derived will be made closed. See the documentation of
	// WithCloseOnContextDone on wazero.RuntimeConfig for detail. See examples in context_done_example_test.go for
	// the end-to-end demonstrations of how these terminations can be performed.
	Call(ctx context.Context, params ...uint64) ([]uint64, error)

	// CallWithStack is an optimized variation of Call that saves memory
	// allocations when the stack slice is reused across calls.
	//
	// Stack length must be at least the max of parameter or result length.
	// The caller adds parameters in order to the stack, and reads any results
	// in order from the stack, except in the error case.
	//
	// For example, the following reuses the same stack slice to call searchFn
	// repeatedly saving one allocation per iteration:
	//
	//	stack := make([]uint64, 4)
	//	for i, search := range searchParams {
	//		// copy the next params to the stack
	//		copy(stack, search)
	//		if err := searchFn.CallWithStack(ctx, stack); err != nil {
	//			return err
	//		} else if stack[0] == 1 { // found
	//			return i // searchParams[i] matched!
	//		}
	//	}
	//
	// # Notes
	//
	//   - This is similar to GoModuleFunction, except for using calling functions
	//     instead of implementing them. Moreover, this is used regardless of
	//     whether the callee is a host or wasm defined function.
	CallWithStack(ctx context.Context, stack []uint64) error

	internalapi.WazeroOnly
}

// GoModuleFunction is a Function implemented in Go instead of a wasm binary.
// The Module parameter is the calling module, used to access memory or
// exported functions. See GoModuleFunc for an example.
//
// The stack is includes any parameters encoded according to their ValueType.
// Its length is the max of parameter or result length. When there are results,
// write them in order beginning at index zero. Do not use the stack after the
// function returns.
//
// Here's a typical way to read three parameters and write back one.
//
//	// read parameters off the stack in index order
//	argv, argvBuf := api.DecodeU32(stack[0]), api.DecodeU32(stack[1])
//
//	// write results back to the stack in index order
//	stack[0] = api.EncodeU32(ErrnoSuccess)
//
// This function can be non-deterministic or cause side effects. It also
// has special properties not defined in the WebAssembly Core specification.
// Notably, this uses the caller's memory (via Module.Memory). See
// https://www.w3.org/TR/wasm-core-1/#host-functions%E2%91%A0
//
// Most end users will not define functions directly with this, as they will
// use reflection or code generators instead. These approaches are more
// idiomatic as they can map go types to ValueType. This type is exposed for
// those willing to trade usability and safety for performance.
//
// To safely decode/encode values from/to the uint64 stack, users are encouraged to use
// api.EncodeXXX or api.DecodeXXX functions. See the docs on api.ValueType.
type GoModuleFunction interface {
	Call(ctx context.Context, mod Module, stack []uint64)
}

// GoModuleFunc is a convenience for defining an inlined function.
//
// For example, the following returns an uint32 value read from parameter zero:
//
//	api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
//		offset := api.DecodeU32(stack[0]) // read the parameter from the stack
//
//		ret, ok := mod.Memory().ReadUint32Le(offset)
//		if !ok {
//			panic("out of memory")
//		}
//
//		stack[0] = api.EncodeU32(ret) // add the result back to the stack.
//	})
type GoModuleFunc func(ctx context.Context, mod Module, stack []uint64)

// Call implements GoModuleFunction.Call.
func (f GoModuleFunc) Call(ctx context.Context, mod Module, stack []uint64) {
	f(ctx, mod, stack)
}

// GoFunction is an optimized form of GoModuleFunction which doesn't require
// the Module parameter. See GoFunc for an example.
//
// For example, this function does not need to use the importing module's
// memory or exported functions.
type GoFunction interface {
	Call(ctx context.Context, stack []uint64)
}

// GoFunc is a convenience for defining an inlined function.
//
// For example, the following returns the sum of two uint32 parameters:
//
//	api.GoFunc(func(ctx context.Context, stack []uint64) {
//		x, y := api.DecodeU32(stack[0]), api.DecodeU32(stack[1])
//		stack[0] = api.EncodeU32(x + y)
//	})
type GoFunc func(ctx context.Context, stack []uint64)

// Call implements GoFunction.Call.
func (f GoFunc) Call(ctx context.Context, stack []uint64) {
	f(ctx, stack)
}

// Global is a WebAssembly 1.0 (20191205) global exported from an instantiated module (wazero.Runtime InstantiateModule).
//
// For example, if the value is not mutable, you can read it once:
//
//	offset := module.ExportedGlobal("memory.offset").Get()
//
// Globals are allowed by specification to be mutable. However, this can be disabled by configuration. When in doubt,
// safe cast to find out if the value can change. Here's an example:
//
//	offset := module.ExportedGlobal("memory.offset")
//	if _, ok := offset.(api.MutableGlobal); ok {
//		// value can change
//	} else {
//		// value is constant
//	}
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#globals%E2%91%A0
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type Global interface {
	fmt.Stringer

	// Type describes the numeric type of the global.
	Type() ValueType

	// Get returns the last known value of this global.
	//
	// See Type for how to decode this value to a Go type.
	Get() uint64
}

// MutableGlobal is a Global whose value can be updated at runtime (variable).
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type MutableGlobal interface {
	Global

	// Set updates the value of this global.
	//
	// See Global.Type for how to encode this value from a Go type.
	Set(v uint64)

	internalapi.WazeroOnly
}

// Memory allows restricted access to a module's memory. Notably, this does not allow growing.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#storage%E2%91%A0
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
//   - This includes all value types available in WebAssembly 1.0 (20191205) and all are encoded little-endian.
type Memory interface {
	// Definition is metadata about this memory from its defining module.
	Definition() MemoryDefinition

	// Size returns the memory size in bytes available.
	// e.g. If the underlying memory has 1 page: 65536
	//
	// # Notes
	//
	//   - This overflows (returns zero) if the memory has the maximum 65536 pages.
	// 	   As a workaround until wazero v2 to fix the return type, use Grow(0) to obtain the current pages and
	//     multiply by 65536.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#-hrefsyntax-instr-memorymathsfmemorysize%E2%91%A0
	Size() uint32

	// Grow increases memory by the delta in pages (65536 bytes per page).
	// The return val is the previous memory size in pages, or false if the
	// delta was ignored as it exceeds MemoryDefinition.Max.
	//
	// # Notes
	//
	//   - This is the same as the "memory.grow" instruction defined in the
	//	   WebAssembly Core Specification, except returns false instead of -1.
	//   - When this returns true, any shared views via Read must be refreshed.
	//
	// See MemorySizer Read and https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#grow-mem
	Grow(deltaPages uint32) (previousPages uint32, ok bool)

	// ReadByte reads a single byte from the underlying buffer at the offset or returns false if out of range.
	ReadByte(offset uint32) (byte, bool)

	// ReadUint16Le reads a uint16 in little-endian encoding from the underlying buffer at the offset in or returns
	// false if out of range.
	ReadUint16Le(offset uint32) (uint16, bool)

	// ReadUint32Le reads a uint32 in little-endian encoding from the underlying buffer at the offset in or returns
	// false if out of range.
	ReadUint32Le(offset uint32) (uint32, bool)

	// ReadFloat32Le reads a float32 from 32 IEEE 754 little-endian encoded bits in the underlying buffer at the offset
	// or returns false if out of range.
	// See math.Float32bits
	ReadFloat32Le(offset uint32) (float32, bool)

	// ReadUint64Le reads a uint64 in little-endian encoding from the underlying buffer at the offset or returns false
	// if out of range.
	ReadUint64Le(offset uint32) (uint64, bool)

	// ReadFloat64Le reads a float64 from 64 IEEE 754 little-endian encoded bits in the underlying buffer at the offset
	// or returns false if out of range.
	//
	// See math.Float64bits
	ReadFloat64Le(offset uint32) (float64, bool)

	// Read reads byteCount bytes from the underlying buffer at the offset or
	// returns false if out of range.
	//
	// For example, to search for a NUL-terminated string:
	//	buf, _ = memory.Read(offset, byteCount)
	//	n := bytes.IndexByte(buf, 0)
	//	if n < 0 {
	//		// Not found!
	//	}
	//
	// Write-through
	//
	// This returns a view of the underlying memory, not a copy. This means any
	// writes to the slice returned are visible to Wasm, and any updates from
	// Wasm are visible reading the returned slice.
	//
	// For example:
	//	buf, _ = memory.Read(offset, byteCount)
	//	buf[1] = 'a' // writes through to memory, meaning Wasm code see 'a'.
	//
	// If you don't intend-write through, make a copy of the returned slice.
	//
	// When to refresh Read
	//
	// The returned slice disconnects on any capacity change. For example,
	// `buf = append(buf, 'a')` might result in a slice that is no longer
	// shared. The same exists Wasm side. For example, if Wasm changes its
	// memory capacity, ex via "memory.grow"), the host slice is no longer
	// shared. Those who need a stable view must set Wasm memory min=max, or
	// use wazero.RuntimeConfig WithMemoryCapacityPages to ensure max is always
	// allocated.
	Read(offset, byteCount uint32) ([]byte, bool)

	// WriteByte writes a single byte to the underlying buffer at the offset in or returns false if out of range.
	WriteByte(offset uint32, v byte) bool

	// WriteUint16Le writes the value in little-endian encoding to the underlying buffer at the offset in or returns
	// false if out of range.
	WriteUint16Le(offset uint32, v uint16) bool

	// WriteUint32Le writes the value in little-endian encoding to the underlying buffer at the offset in or returns
	// false if out of range.
	WriteUint32Le(offset, v uint32) bool

	// WriteFloat32Le writes the value in 32 IEEE 754 little-endian encoded bits to the underlying buffer at the offset
	// or returns false if out of range.
	//
	// See math.Float32bits
	WriteFloat32Le(offset uint32, v float32) bool

	// WriteUint64Le writes the value in little-endian encoding to the underlying buffer at the offset in or returns
	// false if out of range.
	WriteUint64Le(offset uint32, v uint64) bool

	// WriteFloat64Le writes the value in 64 IEEE 754 little-endian encoded bits to the underlying buffer at the offset
	// or returns false if out of range.
	//
	// See math.Float64bits
	WriteFloat64Le(offset uint32, v float64) bool

	// Write writes the slice to the underlying buffer at the offset or returns false if out of range.
	Write(offset uint32, v []byte) bool

	// WriteString writes the string to the underlying buffer at the offset or returns false if out of range.
	WriteString(offset uint32, v string) bool

	internalapi.WazeroOnly
}

// CustomSection contains the name and raw data of a custom section.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type CustomSection interface {
	// Name is the name of the custom section
	Name() string
	// Data is the raw data of the custom section
	Data() []byte

	internalapi.WazeroOnly
}

// EncodeExternref encodes the input as a ValueTypeExternref.
//
// See DecodeExternref
func EncodeExternref(input uintptr) uint64 {
	return uint64(input)
}

// DecodeExternref decodes the input as a ValueTypeExternref.
//
// See EncodeExternref
func DecodeExternref(input uint64) uintptr {
	return uintptr(input)
}

// EncodeI32 encodes the input as a ValueTypeI32.
func EncodeI32(input int32) uint64 {
	return uint64(uint32(input))
}

// DecodeI32 decodes the input as a ValueTypeI32.
func DecodeI32(input uint64) int32 {
	return int32(input)
}

// EncodeU32 encodes the input as a ValueTypeI32.
func EncodeU32(input uint32) uint64 {
	return uint64(input)
}

// DecodeU32 decodes the input as a ValueTypeI32.
func DecodeU32(input uint64) uint32 {
	return uint32(input)
}

// EncodeI64 encodes the input as a ValueTypeI64.
func EncodeI64(input int64) uint64 {
	return uint64(input)
}

// EncodeF32 encodes the input as a ValueTypeF32.
//
// See DecodeF32
func EncodeF32(input float32) uint64 {
	return uint64(math.Float32bits(input))
}

// DecodeF32 decodes the input as a ValueTypeF32.
//
// See EncodeF32
func DecodeF32(input uint64) float32 {
	return math.Float32frombits(uint32(input))
}

// EncodeF64 encodes the input as a ValueTypeF64.
//
// See EncodeF32
func EncodeF64(input float64) uint64 {
	return math.Float64bits(input)
}

// DecodeF64 decodes the input as a ValueTypeF64.
//
// See EncodeF64
func DecodeF64(input uint64) float64 {
	return math.Float64frombits(input)
}
