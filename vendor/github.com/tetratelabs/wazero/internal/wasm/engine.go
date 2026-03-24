package wasm

import (
	"context"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// Engine is a Store-scoped mechanism to compile functions declared or imported by a module.
// This is a top-level type implemented by an interpreter or compiler.
type Engine interface {
	// Close closes this engine, and releases all the compiled cache.
	Close() (err error)

	// CompileModule implements the same method as documented on wasm.Engine.
	CompileModule(ctx context.Context, module *Module, listeners []experimental.FunctionListener, ensureTermination bool) error

	// CompiledModuleCount is exported for testing, to track the size of the compilation cache.
	CompiledModuleCount() uint32

	// DeleteCompiledModule releases compilation caches for the given module (source).
	// Note: it is safe to call this function for a module from which module instances are instantiated even when these
	// module instances have outstanding calls.
	DeleteCompiledModule(module *Module)

	// NewModuleEngine compiles down the function instances in a module, and returns ModuleEngine for the module.
	//
	// * module is the source module from which moduleFunctions are instantiated. This is used for caching.
	// * instance is the *ModuleInstance which is created from `module`.
	//
	// Note: Input parameters must be pre-validated with wasm.Module Validate, to ensure no fields are invalid
	// due to reasons such as out-of-bounds.
	NewModuleEngine(module *Module, instance *ModuleInstance) (ModuleEngine, error)
}

// ModuleEngine implements function calls for a given module.
type ModuleEngine interface {
	// DoneInstantiation is called at the end of the instantiation of the module.
	DoneInstantiation()

	// NewFunction returns an api.Function for the given function pointed by the given Index.
	NewFunction(index Index) api.Function

	// ResolveImportedFunction is used to add imported functions needed to make this ModuleEngine fully functional.
	// 	- `index` is the function Index of this imported function.
	// 	- `descFunc` is the type Index in Module.TypeSection of this imported function. It corresponds to Import.DescFunc.
	// 	- `indexInImportedModule` is the function Index of the imported function in the imported module.
	//	- `importedModuleEngine` is the ModuleEngine for the imported ModuleInstance.
	ResolveImportedFunction(index, descFunc, indexInImportedModule Index, importedModuleEngine ModuleEngine)

	// ResolveImportedMemory is called when this module imports a memory from another module.
	ResolveImportedMemory(importedModuleEngine ModuleEngine)

	// LookupFunction returns the FunctionModule and the Index of the function in the returned ModuleInstance at the given offset in the table.
	LookupFunction(t *TableInstance, typeId FunctionTypeID, tableOffset Index) (*ModuleInstance, Index)

	// GetGlobalValue returns the value of the global variable at the given Index.
	// Only called when OwnsGlobals() returns true, and must not be called for imported globals
	GetGlobalValue(idx Index) (lo, hi uint64)

	// SetGlobalValue sets the value of the global variable at the given Index.
	// Only called when OwnsGlobals() returns true, and must not be called for imported globals
	SetGlobalValue(idx Index, lo, hi uint64)

	// OwnsGlobals returns true if this ModuleEngine owns the global variables. If true, wasm.GlobalInstance's Val,ValHi should
	// not be accessed directly.
	OwnsGlobals() bool

	// FunctionInstanceReference returns Reference for the given Index for a FunctionInstance. The returned values are used by
	// the initialization via ElementSegment.
	FunctionInstanceReference(funcIndex Index) Reference

	// MemoryGrown notifies the engine that the memory has grown.
	MemoryGrown()
}
