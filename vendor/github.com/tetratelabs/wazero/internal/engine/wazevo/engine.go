package wazevo

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/frontend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
	"github.com/tetratelabs/wazero/internal/filecache"
	"github.com/tetratelabs/wazero/internal/platform"
	"github.com/tetratelabs/wazero/internal/version"
	"github.com/tetratelabs/wazero/internal/wasm"
)

type (
	// engine implements wasm.Engine.
	engine struct {
		wazeroVersion   string
		fileCache       filecache.Cache
		compiledModules map[wasm.ModuleID]*compiledModule
		// sortedCompiledModules is a list of compiled modules sorted by the initial address of the executable.
		sortedCompiledModules []*compiledModule
		mux                   sync.RWMutex
		// sharedFunctions is compiled functions shared by all modules.
		sharedFunctions *sharedFunctions
		// setFinalizer defaults to runtime.SetFinalizer, but overridable for tests.
		setFinalizer func(obj interface{}, finalizer interface{})

		// The followings are reused for compiling shared functions.
		machine backend.Machine
		be      backend.Compiler
	}

	sharedFunctions struct {
		// memoryGrowExecutable is a compiled trampoline executable for memory.grow builtin function.
		memoryGrowExecutable []byte
		// checkModuleExitCode is a compiled trampoline executable for checking module instance exit code. This
		// is used when ensureTermination is true.
		checkModuleExitCode []byte
		// stackGrowExecutable is a compiled executable for growing stack builtin function.
		stackGrowExecutable []byte
		// tableGrowExecutable is a compiled trampoline executable for table.grow builtin function.
		tableGrowExecutable []byte
		// refFuncExecutable is a compiled trampoline executable for ref.func builtin function.
		refFuncExecutable []byte
		// memoryWait32Executable is a compiled trampoline executable for memory.wait32 builtin function
		memoryWait32Executable []byte
		// memoryWait64Executable is a compiled trampoline executable for memory.wait64 builtin function
		memoryWait64Executable []byte
		// memoryNotifyExecutable is a compiled trampoline executable for memory.notify builtin function
		memoryNotifyExecutable    []byte
		listenerBeforeTrampolines map[*wasm.FunctionType][]byte
		listenerAfterTrampolines  map[*wasm.FunctionType][]byte
	}

	// compiledModule is a compiled variant of a wasm.Module and ready to be used for instantiation.
	compiledModule struct {
		*executables
		// functionOffsets maps a local function index to the offset in the executable.
		functionOffsets           []int
		parent                    *engine
		module                    *wasm.Module
		ensureTermination         bool
		listeners                 []experimental.FunctionListener
		listenerBeforeTrampolines []*byte
		listenerAfterTrampolines  []*byte

		// The followings are only available for non host modules.

		offsets         wazevoapi.ModuleContextOffsetData
		sharedFunctions *sharedFunctions
		sourceMap       sourceMap
	}

	executables struct {
		executable     []byte
		entryPreambles [][]byte
	}
)

// sourceMap is a mapping from the offset of the executable to the offset of the original wasm binary.
type sourceMap struct {
	// executableOffsets is a sorted list of offsets of the executable. This is index-correlated with wasmBinaryOffsets,
	// in other words executableOffsets[i] is the offset of the executable which corresponds to the offset of a Wasm
	// binary pointed by wasmBinaryOffsets[i].
	executableOffsets []uintptr
	// wasmBinaryOffsets is the counterpart of executableOffsets.
	wasmBinaryOffsets []uint64
}

var _ wasm.Engine = (*engine)(nil)

// NewEngine returns the implementation of wasm.Engine.
func NewEngine(ctx context.Context, _ api.CoreFeatures, fc filecache.Cache) wasm.Engine {
	machine := newMachine()
	be := backend.NewCompiler(ctx, machine, ssa.NewBuilder())
	e := &engine{
		compiledModules: make(map[wasm.ModuleID]*compiledModule),
		setFinalizer:    runtime.SetFinalizer,
		machine:         machine,
		be:              be,
		fileCache:       fc,
		wazeroVersion:   version.GetWazeroVersion(),
	}
	e.compileSharedFunctions()
	return e
}

// CompileModule implements wasm.Engine.
func (e *engine) CompileModule(ctx context.Context, module *wasm.Module, listeners []experimental.FunctionListener, ensureTermination bool) (err error) {
	if wazevoapi.PerfMapEnabled {
		wazevoapi.PerfMap.Lock()
		defer wazevoapi.PerfMap.Unlock()
	}

	if _, ok, err := e.getCompiledModule(module, listeners, ensureTermination); ok { // cache hit!
		return nil
	} else if err != nil {
		return err
	}

	if wazevoapi.DeterministicCompilationVerifierEnabled {
		ctx = wazevoapi.NewDeterministicCompilationVerifierContext(ctx, len(module.CodeSection))
	}
	cm, err := e.compileModule(ctx, module, listeners, ensureTermination)
	if err != nil {
		return err
	}
	if err = e.addCompiledModule(module, cm); err != nil {
		return err
	}

	if wazevoapi.DeterministicCompilationVerifierEnabled {
		for i := 0; i < wazevoapi.DeterministicCompilationVerifyingIter; i++ {
			_, err := e.compileModule(ctx, module, listeners, ensureTermination)
			if err != nil {
				return err
			}
		}
	}

	if len(listeners) > 0 {
		cm.listeners = listeners
		cm.listenerBeforeTrampolines = make([]*byte, len(module.TypeSection))
		cm.listenerAfterTrampolines = make([]*byte, len(module.TypeSection))
		for i := range module.TypeSection {
			typ := &module.TypeSection[i]
			before, after := e.getListenerTrampolineForType(typ)
			cm.listenerBeforeTrampolines[i] = before
			cm.listenerAfterTrampolines[i] = after
		}
	}
	return nil
}

func (exec *executables) compileEntryPreambles(m *wasm.Module, machine backend.Machine, be backend.Compiler) {
	exec.entryPreambles = make([][]byte, len(m.TypeSection))
	for i := range m.TypeSection {
		typ := &m.TypeSection[i]
		sig := frontend.SignatureForWasmFunctionType(typ)
		be.Init()
		buf := machine.CompileEntryPreamble(&sig)
		executable := mmapExecutable(buf)
		exec.entryPreambles[i] = executable

		if wazevoapi.PerfMapEnabled {
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&executable[0])),
				uint64(len(executable)), fmt.Sprintf("entry_preamble::type=%s", typ.String()))
		}
	}
}

func (e *engine) compileModule(ctx context.Context, module *wasm.Module, listeners []experimental.FunctionListener, ensureTermination bool) (*compiledModule, error) {
	withListener := len(listeners) > 0
	cm := &compiledModule{
		offsets: wazevoapi.NewModuleContextOffsetData(module, withListener), parent: e, module: module,
		ensureTermination: ensureTermination,
		executables:       &executables{},
	}

	if module.IsHostModule {
		return e.compileHostModule(ctx, module, listeners)
	}

	importedFns, localFns := int(module.ImportFunctionCount), len(module.FunctionSection)
	if localFns == 0 {
		return cm, nil
	}

	rels := make([]backend.RelocationInfo, 0)
	refToBinaryOffset := make([]int, importedFns+localFns)

	if wazevoapi.DeterministicCompilationVerifierEnabled {
		// The compilation must be deterministic regardless of the order of functions being compiled.
		wazevoapi.DeterministicCompilationVerifierRandomizeIndexes(ctx)
	}

	needSourceInfo := module.DWARFLines != nil

	// Creates new compiler instances which are reused for each function.
	ssaBuilder := ssa.NewBuilder()
	fe := frontend.NewFrontendCompiler(module, ssaBuilder, &cm.offsets, ensureTermination, withListener, needSourceInfo)
	machine := newMachine()
	be := backend.NewCompiler(ctx, machine, ssaBuilder)

	cm.executables.compileEntryPreambles(module, machine, be)

	totalSize := 0 // Total binary size of the executable.
	cm.functionOffsets = make([]int, localFns)
	bodies := make([][]byte, localFns)

	// Trampoline relocation related variables.
	trampolineInterval, callTrampolineIslandSize, err := machine.CallTrampolineIslandInfo(localFns)
	if err != nil {
		return nil, err
	}
	needCallTrampoline := callTrampolineIslandSize > 0
	var callTrampolineIslandOffsets []int // Holds the offsets of trampoline islands.

	for i := range module.CodeSection {
		if wazevoapi.DeterministicCompilationVerifierEnabled {
			i = wazevoapi.DeterministicCompilationVerifierGetRandomizedLocalFunctionIndex(ctx, i)
		}

		fidx := wasm.Index(i + importedFns)

		if wazevoapi.NeedFunctionNameInContext {
			def := module.FunctionDefinition(fidx)
			name := def.DebugName()
			if len(def.ExportNames()) > 0 {
				name = def.ExportNames()[0]
			}
			ctx = wazevoapi.SetCurrentFunctionName(ctx, i, fmt.Sprintf("[%d/%d]%s", i, len(module.CodeSection)-1, name))
		}

		needListener := len(listeners) > 0 && listeners[i] != nil
		body, relsPerFunc, err := e.compileLocalWasmFunction(ctx, module, wasm.Index(i), fe, ssaBuilder, be, needListener)
		if err != nil {
			return nil, fmt.Errorf("compile function %d/%d: %v", i, len(module.CodeSection)-1, err)
		}

		// Align 16-bytes boundary.
		totalSize = (totalSize + 15) &^ 15
		cm.functionOffsets[i] = totalSize

		if needSourceInfo {
			// At the beginning of the function, we add the offset of the function body so that
			// we can resolve the source location of the call site of before listener call.
			cm.sourceMap.executableOffsets = append(cm.sourceMap.executableOffsets, uintptr(totalSize))
			cm.sourceMap.wasmBinaryOffsets = append(cm.sourceMap.wasmBinaryOffsets, module.CodeSection[i].BodyOffsetInCodeSection)

			for _, info := range be.SourceOffsetInfo() {
				cm.sourceMap.executableOffsets = append(cm.sourceMap.executableOffsets, uintptr(totalSize)+uintptr(info.ExecutableOffset))
				cm.sourceMap.wasmBinaryOffsets = append(cm.sourceMap.wasmBinaryOffsets, uint64(info.SourceOffset))
			}
		}

		fref := frontend.FunctionIndexToFuncRef(fidx)
		refToBinaryOffset[fref] = totalSize

		// At this point, relocation offsets are relative to the start of the function body,
		// so we adjust it to the start of the executable.
		for _, r := range relsPerFunc {
			r.Offset += int64(totalSize)
			rels = append(rels, r)
		}

		bodies[i] = body
		totalSize += len(body)
		if wazevoapi.PrintMachineCodeHexPerFunction {
			fmt.Printf("[[[machine code for %s]]]\n%s\n\n", wazevoapi.GetCurrentFunctionName(ctx), hex.EncodeToString(body))
		}

		if needCallTrampoline {
			// If the total size exceeds the trampoline interval, we need to add a trampoline island.
			if totalSize/trampolineInterval > len(callTrampolineIslandOffsets) {
				callTrampolineIslandOffsets = append(callTrampolineIslandOffsets, totalSize)
				totalSize += callTrampolineIslandSize
			}
		}
	}

	// Allocate executable memory and then copy the generated machine code.
	executable, err := platform.MmapCodeSegment(totalSize)
	if err != nil {
		panic(err)
	}
	cm.executable = executable

	for i, b := range bodies {
		offset := cm.functionOffsets[i]
		copy(executable[offset:], b)
	}

	if wazevoapi.PerfMapEnabled {
		wazevoapi.PerfMap.Flush(uintptr(unsafe.Pointer(&executable[0])), cm.functionOffsets)
	}

	if needSourceInfo {
		for i := range cm.sourceMap.executableOffsets {
			cm.sourceMap.executableOffsets[i] += uintptr(unsafe.Pointer(&cm.executable[0]))
		}
	}

	// Resolve relocations for local function calls.
	if len(rels) > 0 {
		machine.ResolveRelocations(refToBinaryOffset, importedFns, executable, rels, callTrampolineIslandOffsets)
	}

	if runtime.GOARCH == "arm64" {
		// On arm64, we cannot give all of rwx at the same time, so we change it to exec.
		if err = platform.MprotectRX(executable); err != nil {
			return nil, err
		}
	}
	cm.sharedFunctions = e.sharedFunctions
	e.setFinalizer(cm.executables, executablesFinalizer)
	return cm, nil
}

func (e *engine) compileLocalWasmFunction(
	ctx context.Context,
	module *wasm.Module,
	localFunctionIndex wasm.Index,
	fe *frontend.Compiler,
	ssaBuilder ssa.Builder,
	be backend.Compiler,
	needListener bool,
) (body []byte, rels []backend.RelocationInfo, err error) {
	typIndex := module.FunctionSection[localFunctionIndex]
	typ := &module.TypeSection[typIndex]
	codeSeg := &module.CodeSection[localFunctionIndex]

	// Initializes both frontend and backend compilers.
	fe.Init(localFunctionIndex, typIndex, typ, codeSeg.LocalTypes, codeSeg.Body, needListener, codeSeg.BodyOffsetInCodeSection)
	be.Init()

	// Lower Wasm to SSA.
	fe.LowerToSSA()
	if wazevoapi.PrintSSA && wazevoapi.PrintEnabledIndex(ctx) {
		fmt.Printf("[[[SSA for %s]]]%s\n", wazevoapi.GetCurrentFunctionName(ctx), ssaBuilder.Format())
	}

	if wazevoapi.DeterministicCompilationVerifierEnabled {
		wazevoapi.VerifyOrSetDeterministicCompilationContextValue(ctx, "SSA", ssaBuilder.Format())
	}

	// Run SSA-level optimization passes.
	ssaBuilder.RunPasses()

	if wazevoapi.PrintOptimizedSSA && wazevoapi.PrintEnabledIndex(ctx) {
		fmt.Printf("[[[Optimized SSA for %s]]]%s\n", wazevoapi.GetCurrentFunctionName(ctx), ssaBuilder.Format())
	}

	if wazevoapi.DeterministicCompilationVerifierEnabled {
		wazevoapi.VerifyOrSetDeterministicCompilationContextValue(ctx, "Optimized SSA", ssaBuilder.Format())
	}

	// Now our ssaBuilder contains the necessary information to further lower them to
	// machine code.
	original, rels, err := be.Compile(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("ssa->machine code: %v", err)
	}

	// TODO: optimize as zero copy.
	copied := make([]byte, len(original))
	copy(copied, original)
	return copied, rels, nil
}

func (e *engine) compileHostModule(ctx context.Context, module *wasm.Module, listeners []experimental.FunctionListener) (*compiledModule, error) {
	machine := newMachine()
	be := backend.NewCompiler(ctx, machine, ssa.NewBuilder())

	num := len(module.CodeSection)
	cm := &compiledModule{module: module, listeners: listeners, executables: &executables{}}
	cm.functionOffsets = make([]int, num)
	totalSize := 0 // Total binary size of the executable.
	bodies := make([][]byte, num)
	var sig ssa.Signature
	for i := range module.CodeSection {
		totalSize = (totalSize + 15) &^ 15
		cm.functionOffsets[i] = totalSize

		typIndex := module.FunctionSection[i]
		typ := &module.TypeSection[typIndex]

		// We can relax until the index fits together in ExitCode as we do in wazevoapi.ExitCodeCallGoModuleFunctionWithIndex.
		// However, 1 << 16 should be large enough for a real use case.
		const hostFunctionNumMaximum = 1 << 16
		if i >= hostFunctionNumMaximum {
			return nil, fmt.Errorf("too many host functions (maximum %d)", hostFunctionNumMaximum)
		}

		sig.ID = ssa.SignatureID(typIndex) // This is important since we reuse the `machine` which caches the ABI based on the SignatureID.
		sig.Params = append(sig.Params[:0],
			ssa.TypeI64, // First argument must be exec context.
			ssa.TypeI64, // The second argument is the moduleContextOpaque of this host module.
		)
		for _, t := range typ.Params {
			sig.Params = append(sig.Params, frontend.WasmTypeToSSAType(t))
		}

		sig.Results = sig.Results[:0]
		for _, t := range typ.Results {
			sig.Results = append(sig.Results, frontend.WasmTypeToSSAType(t))
		}

		c := &module.CodeSection[i]
		if c.GoFunc == nil {
			panic("BUG: GoFunc must be set for host module")
		}

		withListener := len(listeners) > 0 && listeners[i] != nil
		var exitCode wazevoapi.ExitCode
		fn := c.GoFunc
		switch fn.(type) {
		case api.GoModuleFunction:
			exitCode = wazevoapi.ExitCodeCallGoModuleFunctionWithIndex(i, withListener)
		case api.GoFunction:
			exitCode = wazevoapi.ExitCodeCallGoFunctionWithIndex(i, withListener)
		}

		be.Init()
		machine.CompileGoFunctionTrampoline(exitCode, &sig, true)
		if err := be.Finalize(ctx); err != nil {
			return nil, err
		}
		body := be.Buf()

		if wazevoapi.PerfMapEnabled {
			name := module.FunctionDefinition(wasm.Index(i)).DebugName()
			wazevoapi.PerfMap.AddModuleEntry(i,
				int64(totalSize),
				uint64(len(body)),
				fmt.Sprintf("trampoline:%s", name))
		}

		// TODO: optimize as zero copy.
		copied := make([]byte, len(body))
		copy(copied, body)
		bodies[i] = copied
		totalSize += len(body)
	}

	if totalSize == 0 {
		// Empty module.
		return cm, nil
	}

	// Allocate executable memory and then copy the generated machine code.
	executable, err := platform.MmapCodeSegment(totalSize)
	if err != nil {
		panic(err)
	}
	cm.executable = executable

	for i, b := range bodies {
		offset := cm.functionOffsets[i]
		copy(executable[offset:], b)
	}

	if wazevoapi.PerfMapEnabled {
		wazevoapi.PerfMap.Flush(uintptr(unsafe.Pointer(&executable[0])), cm.functionOffsets)
	}

	if runtime.GOARCH == "arm64" {
		// On arm64, we cannot give all of rwx at the same time, so we change it to exec.
		if err = platform.MprotectRX(executable); err != nil {
			return nil, err
		}
	}
	e.setFinalizer(cm.executables, executablesFinalizer)
	return cm, nil
}

// Close implements wasm.Engine.
func (e *engine) Close() (err error) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.sortedCompiledModules = nil
	e.compiledModules = nil
	e.sharedFunctions = nil
	return nil
}

// CompiledModuleCount implements wasm.Engine.
func (e *engine) CompiledModuleCount() uint32 {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return uint32(len(e.compiledModules))
}

// DeleteCompiledModule implements wasm.Engine.
func (e *engine) DeleteCompiledModule(m *wasm.Module) {
	e.mux.Lock()
	defer e.mux.Unlock()
	cm, ok := e.compiledModules[m.ID]
	if ok {
		if len(cm.executable) > 0 {
			e.deleteCompiledModuleFromSortedList(cm)
		}
		delete(e.compiledModules, m.ID)
	}
}

func (e *engine) addCompiledModuleToSortedList(cm *compiledModule) {
	ptr := uintptr(unsafe.Pointer(&cm.executable[0]))

	index := sort.Search(len(e.sortedCompiledModules), func(i int) bool {
		return uintptr(unsafe.Pointer(&e.sortedCompiledModules[i].executable[0])) >= ptr
	})
	e.sortedCompiledModules = append(e.sortedCompiledModules, nil)
	copy(e.sortedCompiledModules[index+1:], e.sortedCompiledModules[index:])
	e.sortedCompiledModules[index] = cm
}

func (e *engine) deleteCompiledModuleFromSortedList(cm *compiledModule) {
	ptr := uintptr(unsafe.Pointer(&cm.executable[0]))

	index := sort.Search(len(e.sortedCompiledModules), func(i int) bool {
		return uintptr(unsafe.Pointer(&e.sortedCompiledModules[i].executable[0])) >= ptr
	})
	if index >= len(e.sortedCompiledModules) {
		return
	}
	copy(e.sortedCompiledModules[index:], e.sortedCompiledModules[index+1:])
	e.sortedCompiledModules = e.sortedCompiledModules[:len(e.sortedCompiledModules)-1]
}

func (e *engine) compiledModuleOfAddr(addr uintptr) *compiledModule {
	e.mux.RLock()
	defer e.mux.RUnlock()

	index := sort.Search(len(e.sortedCompiledModules), func(i int) bool {
		return uintptr(unsafe.Pointer(&e.sortedCompiledModules[i].executable[0])) > addr
	})
	index -= 1
	if index < 0 {
		return nil
	}
	candidate := e.sortedCompiledModules[index]
	if checkAddrInBytes(addr, candidate.executable) {
		// If a module is already deleted, the found module may have been wrong.
		return candidate
	}
	return nil
}

func checkAddrInBytes(addr uintptr, b []byte) bool {
	return uintptr(unsafe.Pointer(&b[0])) <= addr && addr <= uintptr(unsafe.Pointer(&b[len(b)-1]))
}

// NewModuleEngine implements wasm.Engine.
func (e *engine) NewModuleEngine(m *wasm.Module, mi *wasm.ModuleInstance) (wasm.ModuleEngine, error) {
	me := &moduleEngine{}

	// Note: imported functions are resolved in moduleEngine.ResolveImportedFunction.
	me.importedFunctions = make([]importedFunction, m.ImportFunctionCount)

	compiled, ok := e.getCompiledModuleFromMemory(m)
	if !ok {
		return nil, errors.New("source module must be compiled before instantiation")
	}
	me.parent = compiled
	me.module = mi
	me.listeners = compiled.listeners

	if m.IsHostModule {
		me.opaque = buildHostModuleOpaque(m, compiled.listeners)
		me.opaquePtr = &me.opaque[0]
	} else {
		if size := compiled.offsets.TotalSize; size != 0 {
			opaque := newAlignedOpaque(size)
			me.opaque = opaque
			me.opaquePtr = &opaque[0]
		}
	}
	return me, nil
}

func (e *engine) compileSharedFunctions() {
	e.sharedFunctions = &sharedFunctions{
		listenerBeforeTrampolines: make(map[*wasm.FunctionType][]byte),
		listenerAfterTrampolines:  make(map[*wasm.FunctionType][]byte),
	}

	e.be.Init()
	{
		src := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeGrowMemory, &ssa.Signature{
			Params:  []ssa.Type{ssa.TypeI64 /* exec context */, ssa.TypeI32},
			Results: []ssa.Type{ssa.TypeI32},
		}, false)
		e.sharedFunctions.memoryGrowExecutable = mmapExecutable(src)
		if wazevoapi.PerfMapEnabled {
			exe := e.sharedFunctions.memoryGrowExecutable
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&exe[0])), uint64(len(exe)), "memory_grow_trampoline")
		}
	}

	e.be.Init()
	{
		src := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeTableGrow, &ssa.Signature{
			Params:  []ssa.Type{ssa.TypeI64 /* exec context */, ssa.TypeI32 /* table index */, ssa.TypeI32 /* num */, ssa.TypeI64 /* ref */},
			Results: []ssa.Type{ssa.TypeI32},
		}, false)
		e.sharedFunctions.tableGrowExecutable = mmapExecutable(src)
		if wazevoapi.PerfMapEnabled {
			exe := e.sharedFunctions.tableGrowExecutable
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&exe[0])), uint64(len(exe)), "table_grow_trampoline")
		}
	}

	e.be.Init()
	{
		src := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeCheckModuleExitCode, &ssa.Signature{
			Params:  []ssa.Type{ssa.TypeI32 /* exec context */},
			Results: []ssa.Type{ssa.TypeI32},
		}, false)
		e.sharedFunctions.checkModuleExitCode = mmapExecutable(src)
		if wazevoapi.PerfMapEnabled {
			exe := e.sharedFunctions.checkModuleExitCode
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&exe[0])), uint64(len(exe)), "check_module_exit_code_trampoline")
		}
	}

	e.be.Init()
	{
		src := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeRefFunc, &ssa.Signature{
			Params:  []ssa.Type{ssa.TypeI64 /* exec context */, ssa.TypeI32 /* function index */},
			Results: []ssa.Type{ssa.TypeI64}, // returns the function reference.
		}, false)
		e.sharedFunctions.refFuncExecutable = mmapExecutable(src)
		if wazevoapi.PerfMapEnabled {
			exe := e.sharedFunctions.refFuncExecutable
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&exe[0])), uint64(len(exe)), "ref_func_trampoline")
		}
	}

	e.be.Init()
	{
		src := e.machine.CompileStackGrowCallSequence()
		e.sharedFunctions.stackGrowExecutable = mmapExecutable(src)
		if wazevoapi.PerfMapEnabled {
			exe := e.sharedFunctions.stackGrowExecutable
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&exe[0])), uint64(len(exe)), "stack_grow_trampoline")
		}
	}

	e.be.Init()
	{
		src := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeMemoryWait32, &ssa.Signature{
			// exec context, timeout, expected, addr
			Params: []ssa.Type{ssa.TypeI64, ssa.TypeI64, ssa.TypeI32, ssa.TypeI64},
			// Returns the status.
			Results: []ssa.Type{ssa.TypeI32},
		}, false)
		e.sharedFunctions.memoryWait32Executable = mmapExecutable(src)
		if wazevoapi.PerfMapEnabled {
			exe := e.sharedFunctions.memoryWait32Executable
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&exe[0])), uint64(len(exe)), "memory_wait32_trampoline")
		}
	}

	e.be.Init()
	{
		src := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeMemoryWait64, &ssa.Signature{
			// exec context, timeout, expected, addr
			Params: []ssa.Type{ssa.TypeI64, ssa.TypeI64, ssa.TypeI64, ssa.TypeI64},
			// Returns the status.
			Results: []ssa.Type{ssa.TypeI32},
		}, false)
		e.sharedFunctions.memoryWait64Executable = mmapExecutable(src)
		if wazevoapi.PerfMapEnabled {
			exe := e.sharedFunctions.memoryWait64Executable
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&exe[0])), uint64(len(exe)), "memory_wait64_trampoline")
		}
	}

	e.be.Init()
	{
		src := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeMemoryNotify, &ssa.Signature{
			// exec context, count, addr
			Params: []ssa.Type{ssa.TypeI64, ssa.TypeI32, ssa.TypeI64},
			// Returns the number notified.
			Results: []ssa.Type{ssa.TypeI32},
		}, false)
		e.sharedFunctions.memoryNotifyExecutable = mmapExecutable(src)
		if wazevoapi.PerfMapEnabled {
			exe := e.sharedFunctions.memoryNotifyExecutable
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(&exe[0])), uint64(len(exe)), "memory_notify_trampoline")
		}
	}

	e.setFinalizer(e.sharedFunctions, sharedFunctionsFinalizer)
}

func sharedFunctionsFinalizer(sf *sharedFunctions) {
	if err := platform.MunmapCodeSegment(sf.memoryGrowExecutable); err != nil {
		panic(err)
	}
	if err := platform.MunmapCodeSegment(sf.checkModuleExitCode); err != nil {
		panic(err)
	}
	if err := platform.MunmapCodeSegment(sf.stackGrowExecutable); err != nil {
		panic(err)
	}
	if err := platform.MunmapCodeSegment(sf.tableGrowExecutable); err != nil {
		panic(err)
	}
	if err := platform.MunmapCodeSegment(sf.refFuncExecutable); err != nil {
		panic(err)
	}
	if err := platform.MunmapCodeSegment(sf.memoryWait32Executable); err != nil {
		panic(err)
	}
	if err := platform.MunmapCodeSegment(sf.memoryWait64Executable); err != nil {
		panic(err)
	}
	if err := platform.MunmapCodeSegment(sf.memoryNotifyExecutable); err != nil {
		panic(err)
	}
	for _, f := range sf.listenerBeforeTrampolines {
		if err := platform.MunmapCodeSegment(f); err != nil {
			panic(err)
		}
	}
	for _, f := range sf.listenerAfterTrampolines {
		if err := platform.MunmapCodeSegment(f); err != nil {
			panic(err)
		}
	}

	sf.memoryGrowExecutable = nil
	sf.checkModuleExitCode = nil
	sf.stackGrowExecutable = nil
	sf.tableGrowExecutable = nil
	sf.refFuncExecutable = nil
	sf.memoryWait32Executable = nil
	sf.memoryWait64Executable = nil
	sf.memoryNotifyExecutable = nil
	sf.listenerBeforeTrampolines = nil
	sf.listenerAfterTrampolines = nil
}

func executablesFinalizer(exec *executables) {
	if len(exec.executable) > 0 {
		if err := platform.MunmapCodeSegment(exec.executable); err != nil {
			panic(err)
		}
	}
	exec.executable = nil

	for _, f := range exec.entryPreambles {
		if err := platform.MunmapCodeSegment(f); err != nil {
			panic(err)
		}
	}
	exec.entryPreambles = nil
}

func mmapExecutable(src []byte) []byte {
	executable, err := platform.MmapCodeSegment(len(src))
	if err != nil {
		panic(err)
	}

	copy(executable, src)

	if runtime.GOARCH == "arm64" {
		// On arm64, we cannot give all of rwx at the same time, so we change it to exec.
		if err = platform.MprotectRX(executable); err != nil {
			panic(err)
		}
	}
	return executable
}

func (cm *compiledModule) functionIndexOf(addr uintptr) wasm.Index {
	addr -= uintptr(unsafe.Pointer(&cm.executable[0]))
	offset := cm.functionOffsets
	index := sort.Search(len(offset), func(i int) bool {
		return offset[i] > int(addr)
	})
	index--
	if index < 0 {
		panic("BUG")
	}
	return wasm.Index(index)
}

func (e *engine) getListenerTrampolineForType(functionType *wasm.FunctionType) (before, after *byte) {
	e.mux.Lock()
	defer e.mux.Unlock()

	beforeBuf, ok := e.sharedFunctions.listenerBeforeTrampolines[functionType]
	afterBuf := e.sharedFunctions.listenerAfterTrampolines[functionType]
	if ok {
		return &beforeBuf[0], &afterBuf[0]
	}

	beforeSig, afterSig := frontend.SignatureForListener(functionType)

	e.be.Init()
	buf := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeCallListenerBefore, beforeSig, false)
	beforeBuf = mmapExecutable(buf)

	e.be.Init()
	buf = e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeCallListenerAfter, afterSig, false)
	afterBuf = mmapExecutable(buf)

	e.sharedFunctions.listenerBeforeTrampolines[functionType] = beforeBuf
	e.sharedFunctions.listenerAfterTrampolines[functionType] = afterBuf
	return &beforeBuf[0], &afterBuf[0]
}

func (cm *compiledModule) getSourceOffset(pc uintptr) uint64 {
	offsets := cm.sourceMap.executableOffsets
	if len(offsets) == 0 {
		return 0
	}

	index := sort.Search(len(offsets), func(i int) bool {
		return offsets[i] >= pc
	})

	index--
	if index < 0 {
		return 0
	}
	return cm.sourceMap.wasmBinaryOffsets[index]
}
