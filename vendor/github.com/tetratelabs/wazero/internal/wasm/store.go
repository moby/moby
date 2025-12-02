package wasm

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/expctxkeys"
	"github.com/tetratelabs/wazero/internal/internalapi"
	"github.com/tetratelabs/wazero/internal/leb128"
	internalsys "github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/sys"
)

// nameToModuleShrinkThreshold is the size the nameToModule map can grow to
// before it starts to be monitored for shrinking.
// The capacity will never be smaller than this once the threshold is met.
const nameToModuleShrinkThreshold = 100

type (
	// Store is the runtime representation of "instantiated" Wasm module and objects.
	// Multiple modules can be instantiated within a single store, and each instance,
	// (e.g. function instance) can be referenced by other module instances in a Store via Module.ImportSection.
	//
	// Every type whose name ends with "Instance" suffix belongs to exactly one store.
	//
	// Note that store is not thread (concurrency) safe, meaning that using single Store
	// via multiple goroutines might result in race conditions. In that case, the invocation
	// and access to any methods and field of Store must be guarded by mutex.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#store%E2%91%A0
	Store struct {
		// moduleList ensures modules are closed in reverse initialization order.
		moduleList *ModuleInstance // guarded by mux

		// nameToModule holds the instantiated Wasm modules by module name from Instantiate.
		// It ensures no race conditions instantiating two modules of the same name.
		nameToModule map[string]*ModuleInstance // guarded by mux

		// nameToModuleCap tracks the growth of the nameToModule map in order to
		// track when to shrink it.
		nameToModuleCap int // guarded by mux

		// EnabledFeatures are read-only to allow optimizations.
		EnabledFeatures api.CoreFeatures

		// Engine is a global context for a Store which is in responsible for compilation and execution of Wasm modules.
		Engine Engine

		// typeIDs maps each FunctionType.String() to a unique FunctionTypeID. This is used at runtime to
		// do type-checks on indirect function calls.
		typeIDs map[string]FunctionTypeID

		// functionMaxTypes represents the limit on the number of function types in a store.
		// Note: this is fixed to 2^27 but have this a field for testability.
		functionMaxTypes uint32

		// mux is used to guard the fields from concurrent access.
		mux sync.RWMutex
	}

	// ModuleInstance represents instantiated wasm module.
	// The difference from the spec is that in wazero, a ModuleInstance holds pointers
	// to the instances, rather than "addresses" (i.e. index to Store.Functions, Globals, etc) for convenience.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-moduleinst
	//
	// This implements api.Module.
	ModuleInstance struct {
		internalapi.WazeroOnlyType

		ModuleName     string
		Exports        map[string]*Export
		Globals        []*GlobalInstance
		MemoryInstance *MemoryInstance
		Tables         []*TableInstance

		// Engine implements function calls for this module.
		Engine ModuleEngine

		// TypeIDs is index-correlated with types and holds typeIDs which is uniquely assigned to a type by store.
		// This is necessary to achieve fast runtime type checking for indirect function calls at runtime.
		TypeIDs []FunctionTypeID

		// DataInstances holds data segments bytes of the module.
		// This is only used by bulk memory operations.
		//
		// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/runtime.html#data-instances
		DataInstances []DataInstance

		// ElementInstances holds the element instance, and each holds the references to either functions
		// or external objects (unimplemented).
		ElementInstances []ElementInstance

		// Sys is exposed for use in special imports such as WASI, assemblyscript.
		//
		// # Notes
		//
		//   - This is a part of ModuleInstance so that scope and Close is coherent.
		//   - This is not exposed outside this repository (as a host function
		//	  parameter) because we haven't thought through capabilities based
		//	  security implications.
		Sys *internalsys.Context

		// Closed is used both to guard moduleEngine.CloseWithExitCode and to store the exit code.
		//
		// The update value is closedType + exitCode << 32. This ensures an exit code of zero isn't mistaken for never closed.
		//
		// Note: Exclusively reading and updating this with atomics guarantees cross-goroutine observations.
		// See /RATIONALE.md
		Closed atomic.Uint64

		// CodeCloser is non-nil when the code should be closed after this module.
		CodeCloser api.Closer

		// s is the Store on which this module is instantiated.
		s *Store
		// prev and next hold the nodes in the linked list of ModuleInstance held by Store.
		prev, next *ModuleInstance
		// Source is a pointer to the Module from which this ModuleInstance derives.
		Source *Module

		// CloseNotifier is an experimental hook called once on close.
		CloseNotifier experimental.CloseNotifier
	}

	// DataInstance holds bytes corresponding to the data segment in a module.
	//
	// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/runtime.html#data-instances
	DataInstance = []byte

	// GlobalInstance represents a global instance in a store.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#global-instances%E2%91%A0
	GlobalInstance struct {
		Type GlobalType
		// Val holds a 64-bit representation of the actual value.
		// If me is non-nil, the value will not be updated and the current value is stored in the module engine.
		Val uint64
		// ValHi is only used for vector type globals, and holds the higher bits of the vector.
		// If me is non-nil, the value will not be updated and the current value is stored in the module engine.
		ValHi uint64
		// Me is the module engine that owns this global instance.
		// The .Val and .ValHi fields are only valid when me is nil.
		// If me is non-nil, the value is stored in the module engine.
		Me    ModuleEngine
		Index Index
	}

	// FunctionTypeID is a uniquely assigned integer for a function type.
	// This is wazero specific runtime object and specific to a store,
	// and used at runtime to do type-checks on indirect function calls.
	FunctionTypeID uint32
)

// The wazero specific limitations described at RATIONALE.md.
const maximumFunctionTypes = 1 << 27

// GetFunctionTypeID is used by emscripten.
func (m *ModuleInstance) GetFunctionTypeID(t *FunctionType) FunctionTypeID {
	id, err := m.s.GetFunctionTypeID(t)
	if err != nil {
		// This is not recoverable in practice since the only error GetFunctionTypeID returns is
		// when there's too many function types in the store.
		panic(err)
	}
	return id
}

func (m *ModuleInstance) buildElementInstances(elements []ElementSegment) {
	m.ElementInstances = make([][]Reference, len(elements))
	for i, elm := range elements {
		if elm.Type == RefTypeFuncref && elm.Mode == ElementModePassive {
			// Only passive elements can be access as element instances.
			// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/syntax/modules.html#element-segments
			inits := elm.Init
			inst := make([]Reference, len(inits))
			m.ElementInstances[i] = inst
			for j, idx := range inits {
				if index, ok := unwrapElementInitGlobalReference(idx); ok {
					global := m.Globals[index]
					inst[j] = Reference(global.Val)
				} else {
					if idx != ElementInitNullReference {
						inst[j] = m.Engine.FunctionInstanceReference(idx)
					}
				}
			}
		}
	}
}

func (m *ModuleInstance) applyElements(elems []ElementSegment) {
	for elemI := range elems {
		elem := &elems[elemI]
		if !elem.IsActive() ||
			// Per https://github.com/WebAssembly/spec/issues/1427 init can be no-op.
			len(elem.Init) == 0 {
			continue
		}
		var offset uint32
		if elem.OffsetExpr.Opcode == OpcodeGlobalGet {
			// Ignore error as it's already validated.
			globalIdx, _, _ := leb128.LoadUint32(elem.OffsetExpr.Data)
			global := m.Globals[globalIdx]
			offset = uint32(global.Val)
		} else {
			// Ignore error as it's already validated.
			o, _, _ := leb128.LoadInt32(elem.OffsetExpr.Data)
			offset = uint32(o)
		}

		table := m.Tables[elem.TableIndex]
		references := table.References
		if int(offset)+len(elem.Init) > len(references) {
			// ErrElementOffsetOutOfBounds is the error raised when the active element offset exceeds the table length.
			// Before CoreFeatureReferenceTypes, this was checked statically before instantiation, after the proposal,
			// this must be raised as runtime error (as in assert_trap in spectest), not even an instantiation error.
			// https://github.com/WebAssembly/spec/blob/d39195773112a22b245ffbe864bab6d1182ccb06/test/core/linking.wast#L264-L274
			//
			// In wazero, we ignore it since in any way, the instantiated module and engines are fine and can be used
			// for function invocations.
			return
		}

		if table.Type == RefTypeExternref {
			for i := 0; i < len(elem.Init); i++ {
				references[offset+uint32(i)] = Reference(0)
			}
		} else {
			for i, init := range elem.Init {
				if init == ElementInitNullReference {
					continue
				}

				var ref Reference
				if index, ok := unwrapElementInitGlobalReference(init); ok {
					global := m.Globals[index]
					ref = Reference(global.Val)
				} else {
					ref = m.Engine.FunctionInstanceReference(index)
				}
				references[offset+uint32(i)] = ref
			}
		}
	}
}

// validateData ensures that data segments are valid in terms of memory boundary.
// Note: this is used only when bulk-memory/reference type feature is disabled.
func (m *ModuleInstance) validateData(data []DataSegment) (err error) {
	for i := range data {
		d := &data[i]
		if !d.IsPassive() {
			offset := int(executeConstExpressionI32(m.Globals, &d.OffsetExpression))
			ceil := offset + len(d.Init)
			if offset < 0 || ceil > len(m.MemoryInstance.Buffer) {
				return fmt.Errorf("%s[%d]: out of bounds memory access", SectionIDName(SectionIDData), i)
			}
		}
	}
	return
}

// applyData uses the given data segments and mutate the memory according to the initial contents on it
// and populate the `DataInstances`. This is called after all the validation phase passes and out of
// bounds memory access error here is not a validation error, but rather a runtime error.
func (m *ModuleInstance) applyData(data []DataSegment) error {
	m.DataInstances = make([][]byte, len(data))
	for i := range data {
		d := &data[i]
		m.DataInstances[i] = d.Init
		if !d.IsPassive() {
			offset := executeConstExpressionI32(m.Globals, &d.OffsetExpression)
			if offset < 0 || int(offset)+len(d.Init) > len(m.MemoryInstance.Buffer) {
				return fmt.Errorf("%s[%d]: out of bounds memory access", SectionIDName(SectionIDData), i)
			}
			copy(m.MemoryInstance.Buffer[offset:], d.Init)
		}
	}
	return nil
}

// GetExport returns an export of the given name and type or errs if not exported or the wrong type.
func (m *ModuleInstance) getExport(name string, et ExternType) (*Export, error) {
	exp, ok := m.Exports[name]
	if !ok {
		return nil, fmt.Errorf("%q is not exported in module %q", name, m.ModuleName)
	}
	if exp.Type != et {
		return nil, fmt.Errorf("export %q in module %q is a %s, not a %s", name, m.ModuleName, ExternTypeName(exp.Type), ExternTypeName(et))
	}
	return exp, nil
}

func NewStore(enabledFeatures api.CoreFeatures, engine Engine) *Store {
	return &Store{
		nameToModule:     map[string]*ModuleInstance{},
		nameToModuleCap:  nameToModuleShrinkThreshold,
		EnabledFeatures:  enabledFeatures,
		Engine:           engine,
		typeIDs:          map[string]FunctionTypeID{},
		functionMaxTypes: maximumFunctionTypes,
	}
}

// Instantiate uses name instead of the Module.NameSection ModuleName as it allows instantiating the same module under
// different names safely and concurrently.
//
// * ctx: the default context used for function calls.
// * name: the name of the module.
// * sys: the system context, which will be closed (SysContext.Close) on ModuleInstance.Close.
//
// Note: Module.Validate must be called prior to instantiation.
func (s *Store) Instantiate(
	ctx context.Context,
	module *Module,
	name string,
	sys *internalsys.Context,
	typeIDs []FunctionTypeID,
) (*ModuleInstance, error) {
	// Instantiate the module and add it to the store so that other modules can import it.
	m, err := s.instantiate(ctx, module, name, sys, typeIDs)
	if err != nil {
		return nil, err
	}

	// Now that the instantiation is complete without error, add it.
	if err = s.registerModule(m); err != nil {
		_ = m.Close(ctx)
		return nil, err
	}
	return m, nil
}

func (s *Store) instantiate(
	ctx context.Context,
	module *Module,
	name string,
	sysCtx *internalsys.Context,
	typeIDs []FunctionTypeID,
) (m *ModuleInstance, err error) {
	m = &ModuleInstance{ModuleName: name, TypeIDs: typeIDs, Sys: sysCtx, s: s, Source: module}

	m.Tables = make([]*TableInstance, int(module.ImportTableCount)+len(module.TableSection))
	m.Globals = make([]*GlobalInstance, int(module.ImportGlobalCount)+len(module.GlobalSection))
	m.Engine, err = s.Engine.NewModuleEngine(module, m)
	if err != nil {
		return nil, err
	}

	if err = m.resolveImports(ctx, module); err != nil {
		return nil, err
	}

	err = m.buildTables(module,
		// As of reference-types proposal, boundary check must be done after instantiation.
		s.EnabledFeatures.IsEnabled(api.CoreFeatureReferenceTypes))
	if err != nil {
		return nil, err
	}

	allocator, _ := ctx.Value(expctxkeys.MemoryAllocatorKey{}).(experimental.MemoryAllocator)

	m.buildGlobals(module, m.Engine.FunctionInstanceReference)
	m.buildMemory(module, allocator)
	m.Exports = module.Exports
	for _, exp := range m.Exports {
		if exp.Type == ExternTypeTable {
			t := m.Tables[exp.Index]
			t.involvingModuleInstances = append(t.involvingModuleInstances, m)
		}
	}

	// As of reference types proposal, data segment validation must happen after instantiation,
	// and the side effect must persist even if there's out of bounds error after instantiation.
	// https://github.com/WebAssembly/spec/blob/d39195773112a22b245ffbe864bab6d1182ccb06/test/core/linking.wast#L395-L405
	if !s.EnabledFeatures.IsEnabled(api.CoreFeatureReferenceTypes) {
		if err = m.validateData(module.DataSection); err != nil {
			return nil, err
		}
	}

	// After engine creation, we can create the funcref element instances and initialize funcref type globals.
	m.buildElementInstances(module.ElementSection)

	// Now all the validation passes, we are safe to mutate memory instances (possibly imported ones).
	if err = m.applyData(module.DataSection); err != nil {
		return nil, err
	}

	m.applyElements(module.ElementSection)

	m.Engine.DoneInstantiation()

	// Execute the start function.
	if module.StartSection != nil {
		funcIdx := *module.StartSection
		ce := m.Engine.NewFunction(funcIdx)
		_, err = ce.Call(ctx)
		if exitErr, ok := err.(*sys.ExitError); ok { // Don't wrap an exit error!
			return nil, exitErr
		} else if err != nil {
			return nil, fmt.Errorf("start %s failed: %w", module.funcDesc(SectionIDFunction, funcIdx), err)
		}
	}
	return
}

func (m *ModuleInstance) resolveImports(ctx context.Context, module *Module) (err error) {
	// Check if ctx contains an ImportResolver.
	resolveImport, _ := ctx.Value(expctxkeys.ImportResolverKey{}).(experimental.ImportResolver)

	for moduleName, imports := range module.ImportPerModule {
		var importedModule *ModuleInstance
		if resolveImport != nil {
			if v := resolveImport(moduleName); v != nil {
				importedModule = v.(*ModuleInstance)
			}
		}
		if importedModule == nil {
			importedModule, err = m.s.module(moduleName)
			if err != nil {
				return err
			}
		}

		for _, i := range imports {
			var imported *Export
			imported, err = importedModule.getExport(i.Name, i.Type)
			if err != nil {
				return
			}

			switch i.Type {
			case ExternTypeFunc:
				expectedType := &module.TypeSection[i.DescFunc]
				src := importedModule.Source
				actual := src.typeOfFunction(imported.Index)
				if !actual.EqualsSignature(expectedType.Params, expectedType.Results) {
					err = errorInvalidImport(i, fmt.Errorf("signature mismatch: %s != %s", expectedType, actual))
					return
				}

				m.Engine.ResolveImportedFunction(i.IndexPerType, i.DescFunc, imported.Index, importedModule.Engine)
			case ExternTypeTable:
				expected := i.DescTable
				importedTable := importedModule.Tables[imported.Index]
				if expected.Type != importedTable.Type {
					err = errorInvalidImport(i, fmt.Errorf("table type mismatch: %s != %s",
						RefTypeName(expected.Type), RefTypeName(importedTable.Type)))
					return
				}

				if expected.Min > importedTable.Min {
					err = errorMinSizeMismatch(i, expected.Min, importedTable.Min)
					return
				}

				if expected.Max != nil {
					expectedMax := *expected.Max
					if importedTable.Max == nil {
						err = errorNoMax(i, expectedMax)
						return
					} else if expectedMax < *importedTable.Max {
						err = errorMaxSizeMismatch(i, expectedMax, *importedTable.Max)
						return
					}
				}
				m.Tables[i.IndexPerType] = importedTable
				importedTable.involvingModuleInstancesMutex.Lock()
				if len(importedTable.involvingModuleInstances) == 0 {
					panic("BUG: involvingModuleInstances must not be nil when it's imported")
				}
				importedTable.involvingModuleInstances = append(importedTable.involvingModuleInstances, m)
				importedTable.involvingModuleInstancesMutex.Unlock()
			case ExternTypeMemory:
				expected := i.DescMem
				importedMemory := importedModule.MemoryInstance

				if expected.Min > memoryBytesNumToPages(uint64(len(importedMemory.Buffer))) {
					err = errorMinSizeMismatch(i, expected.Min, importedMemory.Min)
					return
				}

				if expected.Max < importedMemory.Max {
					err = errorMaxSizeMismatch(i, expected.Max, importedMemory.Max)
					return
				}
				m.MemoryInstance = importedMemory
				m.Engine.ResolveImportedMemory(importedModule.Engine)
			case ExternTypeGlobal:
				expected := i.DescGlobal
				importedGlobal := importedModule.Globals[imported.Index]

				if expected.Mutable != importedGlobal.Type.Mutable {
					err = errorInvalidImport(i, fmt.Errorf("mutability mismatch: %t != %t",
						expected.Mutable, importedGlobal.Type.Mutable))
					return
				}

				if expected.ValType != importedGlobal.Type.ValType {
					err = errorInvalidImport(i, fmt.Errorf("value type mismatch: %s != %s",
						ValueTypeName(expected.ValType), ValueTypeName(importedGlobal.Type.ValType)))
					return
				}
				m.Globals[i.IndexPerType] = importedGlobal
			}
		}
	}
	return
}

func errorMinSizeMismatch(i *Import, expected, actual uint32) error {
	return errorInvalidImport(i, fmt.Errorf("minimum size mismatch: %d > %d", expected, actual))
}

func errorNoMax(i *Import, expected uint32) error {
	return errorInvalidImport(i, fmt.Errorf("maximum size mismatch: %d, but actual has no max", expected))
}

func errorMaxSizeMismatch(i *Import, expected, actual uint32) error {
	return errorInvalidImport(i, fmt.Errorf("maximum size mismatch: %d < %d", expected, actual))
}

func errorInvalidImport(i *Import, err error) error {
	return fmt.Errorf("import %s[%s.%s]: %w", ExternTypeName(i.Type), i.Module, i.Name, err)
}

// executeConstExpressionI32 executes the ConstantExpression which returns ValueTypeI32.
// The validity of the expression is ensured when calling this function as this is only called
// during instantiation phrase, and the validation happens in compilation (validateConstExpression).
func executeConstExpressionI32(importedGlobals []*GlobalInstance, expr *ConstantExpression) (ret int32) {
	switch expr.Opcode {
	case OpcodeI32Const:
		ret, _, _ = leb128.LoadInt32(expr.Data)
	case OpcodeGlobalGet:
		id, _, _ := leb128.LoadUint32(expr.Data)
		g := importedGlobals[id]
		ret = int32(g.Val)
	}
	return
}

// initialize initializes the value of this global instance given the const expr and imported globals.
// funcRefResolver is called to get the actual funcref (engine specific) from the OpcodeRefFunc const expr.
//
// Global initialization constant expression can only reference the imported globals.
// See the note on https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#constant-expressions%E2%91%A0
func (g *GlobalInstance) initialize(importedGlobals []*GlobalInstance, expr *ConstantExpression, funcRefResolver func(funcIndex Index) Reference) {
	switch expr.Opcode {
	case OpcodeI32Const:
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		v, _, _ := leb128.LoadInt32(expr.Data)
		g.Val = uint64(uint32(v))
	case OpcodeI64Const:
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		v, _, _ := leb128.LoadInt64(expr.Data)
		g.Val = uint64(v)
	case OpcodeF32Const:
		g.Val = uint64(binary.LittleEndian.Uint32(expr.Data))
	case OpcodeF64Const:
		g.Val = binary.LittleEndian.Uint64(expr.Data)
	case OpcodeGlobalGet:
		id, _, _ := leb128.LoadUint32(expr.Data)
		importedG := importedGlobals[id]
		switch importedG.Type.ValType {
		case ValueTypeI32:
			g.Val = uint64(uint32(importedG.Val))
		case ValueTypeI64:
			g.Val = importedG.Val
		case ValueTypeF32:
			g.Val = importedG.Val
		case ValueTypeF64:
			g.Val = importedG.Val
		case ValueTypeV128:
			g.Val, g.ValHi = importedG.Val, importedG.ValHi
		case ValueTypeFuncref, ValueTypeExternref:
			g.Val = importedG.Val
		}
	case OpcodeRefNull:
		switch expr.Data[0] {
		case ValueTypeExternref, ValueTypeFuncref:
			g.Val = 0 // Reference types are opaque 64bit pointer at runtime.
		}
	case OpcodeRefFunc:
		v, _, _ := leb128.LoadUint32(expr.Data)
		g.Val = uint64(funcRefResolver(v))
	case OpcodeVecV128Const:
		g.Val, g.ValHi = binary.LittleEndian.Uint64(expr.Data[0:8]), binary.LittleEndian.Uint64(expr.Data[8:16])
	}
}

// String implements api.Global.
func (g *GlobalInstance) String() string {
	switch g.Type.ValType {
	case ValueTypeI32, ValueTypeI64:
		return fmt.Sprintf("global(%d)", g.Val)
	case ValueTypeF32:
		return fmt.Sprintf("global(%f)", api.DecodeF32(g.Val))
	case ValueTypeF64:
		return fmt.Sprintf("global(%f)", api.DecodeF64(g.Val))
	default:
		panic(fmt.Errorf("BUG: unknown value type %X", g.Type.ValType))
	}
}

func (g *GlobalInstance) Value() (uint64, uint64) {
	if g.Me != nil {
		return g.Me.GetGlobalValue(g.Index)
	}
	return g.Val, g.ValHi
}

func (g *GlobalInstance) SetValue(lo, hi uint64) {
	if g.Me != nil {
		g.Me.SetGlobalValue(g.Index, lo, hi)
	} else {
		g.Val, g.ValHi = lo, hi
	}
}

func (s *Store) GetFunctionTypeIDs(ts []FunctionType) ([]FunctionTypeID, error) {
	ret := make([]FunctionTypeID, len(ts))
	for i := range ts {
		t := &ts[i]
		inst, err := s.GetFunctionTypeID(t)
		if err != nil {
			return nil, err
		}
		ret[i] = inst
	}
	return ret, nil
}

func (s *Store) GetFunctionTypeID(t *FunctionType) (FunctionTypeID, error) {
	s.mux.RLock()
	key := t.key()
	id, ok := s.typeIDs[key]
	s.mux.RUnlock()
	if !ok {
		s.mux.Lock()
		defer s.mux.Unlock()
		// Check again in case another goroutine has already added the type.
		if id, ok = s.typeIDs[key]; ok {
			return id, nil
		}
		l := len(s.typeIDs)
		if uint32(l) >= s.functionMaxTypes {
			return 0, fmt.Errorf("too many function types in a store")
		}
		id = FunctionTypeID(l)
		s.typeIDs[key] = id
	}
	return id, nil
}

// CloseWithExitCode implements the same method as documented on wazero.Runtime.
func (s *Store) CloseWithExitCode(ctx context.Context, exitCode uint32) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	// Close modules in reverse initialization order.
	var errs []error
	for m := s.moduleList; m != nil; m = m.next {
		// If closing this module errs, proceed anyway to close the others.
		if err := m.closeWithExitCode(ctx, exitCode); err != nil {
			errs = append(errs, err)
		}
	}
	s.moduleList = nil
	s.nameToModule = nil
	s.nameToModuleCap = 0
	s.typeIDs = nil
	return errors.Join(errs...)
}
