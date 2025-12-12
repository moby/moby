package wazevo

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
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
	compiledModuleWithCount struct {
		*compiledModule
		refCount int
	}
	// engine implements wasm.Engine.
	engine struct {
		wazeroVersion   string
		fileCache       filecache.Cache
		compiledModules map[wasm.ModuleID]*compiledModuleWithCount
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
		// The compiled trampolines executable.
		executable []byte
		// memoryGrowAddress is the address of memory.grow builtin function.
		memoryGrowAddress *byte
		// checkModuleExitCodeAddress is the address of checking module instance exit code.
		// This is used when ensureTermination is true.
		checkModuleExitCodeAddress *byte
		// stackGrowAddress is the address of growing stack builtin function.
		stackGrowAddress *byte
		// tableGrowAddress is the address of table.grow builtin function.
		tableGrowAddress *byte
		// refFuncAddress is the address of ref.func builtin function.
		refFuncAddress *byte
		// memoryWait32Address is the address of memory.wait32 builtin function
		memoryWait32Address *byte
		// memoryWait64Address is the address of memory.wait64 builtin function
		memoryWait64Address *byte
		// memoryNotifyAddress is the address of memory.notify builtin function
		memoryNotifyAddress *byte
		listenerTrampolines listenerTrampolines
	}

	listenerTrampolines = map[*wasm.FunctionType]struct {
		executable []byte
		before     *byte
		after      *byte
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
		executable         []byte
		entryPreambles     []byte
		entryPreamblesPtrs []*byte
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
		compiledModules: make(map[wasm.ModuleID]*compiledModuleWithCount),
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
	if len(m.TypeSection) == 0 {
		return
	}

	var preambles []byte
	sizes := make([]int, len(m.TypeSection))

	for i := range sizes {
		typ := &m.TypeSection[i]
		sig := frontend.SignatureForWasmFunctionType(typ)
		be.Init()
		buf := machine.CompileEntryPreamble(&sig)
		preambles = append(preambles, buf...)
		align := 15 & -len(preambles) // Align 16-bytes boundary.
		preambles = append(preambles, make([]byte, align)...)
		sizes[i] = len(buf) + align
	}

	exec.entryPreambles = mmapExecutable(preambles)
	exec.entryPreamblesPtrs = make([]*byte, len(sizes))

	offset := 0
	for i, size := range sizes {
		ptr := &exec.entryPreambles[offset]
		exec.entryPreamblesPtrs[i] = ptr
		offset += size

		if wazevoapi.PerfMapEnabled {
			typ := &m.TypeSection[i]
			wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(ptr)),
				uint64(size), fmt.Sprintf("entry_preamble::type=%s", typ.String()))
		}
	}
}

func (e *engine) compileModule(ctx context.Context, module *wasm.Module, listeners []experimental.FunctionListener, ensureTermination bool) (*compiledModule, error) {
	if module.IsHostModule {
		return e.compileHostModule(ctx, module, listeners)
	}

	withListener := len(listeners) > 0
	cm := &compiledModule{
		offsets: wazevoapi.NewModuleContextOffsetData(module, withListener), parent: e, module: module,
		ensureTermination: ensureTermination,
		executables:       &executables{},
	}

	importedFns, localFns := int(module.ImportFunctionCount), len(module.FunctionSection)
	if localFns == 0 {
		return cm, nil
	}

	machine := newMachine()
	relocator, err := newEngineRelocator(machine, importedFns, localFns)
	if err != nil {
		return nil, err
	}

	needSourceInfo := module.DWARFLines != nil

	ssaBuilder := ssa.NewBuilder()
	be := backend.NewCompiler(ctx, machine, ssaBuilder)
	cm.executables.compileEntryPreambles(module, machine, be)
	cm.functionOffsets = make([]int, localFns)

	var indexes []int
	if wazevoapi.DeterministicCompilationVerifierEnabled {
		// The compilation must be deterministic regardless of the order of functions being compiled.
		indexes = wazevoapi.DeterministicCompilationVerifierRandomizeIndexes(ctx)
	}

	if workers := experimental.GetCompilationWorkers(ctx); workers <= 1 {
		// Compile with a single goroutine.
		fe := frontend.NewFrontendCompiler(module, ssaBuilder, &cm.offsets, ensureTermination, withListener, needSourceInfo)

		for i := range module.CodeSection {
			if wazevoapi.DeterministicCompilationVerifierEnabled {
				i = indexes[i]
			}

			fidx := wasm.Index(i + importedFns)
			fctx := functionContext(ctx, module, i, fidx)

			needListener := len(listeners) > i && listeners[i] != nil
			body, relsPerFunc, err := e.compileLocalWasmFunction(fctx, module, wasm.Index(i), fe, ssaBuilder, be, needListener)
			if err != nil {
				return nil, fmt.Errorf("compile function %d/%d: %v", i, len(module.CodeSection)-1, err)
			}

			relocator.appendFunction(fctx, module, cm, i, fidx, body, relsPerFunc, be.SourceOffsetInfo())
		}
	} else {
		// Compile with N worker goroutines.
		// Collect compiled functions across workers in a slice,
		// to be added to the relocator in-order and resolved serially at the end.
		// This uses more memory and CPU (across cores), but can be significantly faster.
		type compiledFunc struct {
			fctx        context.Context
			fnum        int
			fidx        wasm.Index
			body        []byte
			relsPerFunc []backend.RelocationInfo
			offsPerFunc []backend.SourceOffsetInfo
		}

		compiledFuncs := make([]compiledFunc, len(module.CodeSection))
		ctx, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)

		var count atomic.Uint32
		var wg sync.WaitGroup
		wg.Add(workers)

		for range workers {
			go func() {
				defer wg.Done()

				// Creates new compiler instances which are reused for each function.
				machine := newMachine()
				ssaBuilder := ssa.NewBuilder()
				be := backend.NewCompiler(ctx, machine, ssaBuilder)
				fe := frontend.NewFrontendCompiler(module, ssaBuilder, &cm.offsets, ensureTermination, withListener, needSourceInfo)

				for {
					if err := ctx.Err(); err != nil {
						// Compilation canceled!
						return
					}

					i := int(count.Add(1)) - 1
					if i >= len(module.CodeSection) {
						return
					}

					if wazevoapi.DeterministicCompilationVerifierEnabled {
						i = indexes[i]
					}

					fidx := wasm.Index(i + importedFns)
					fctx := functionContext(ctx, module, i, fidx)

					needListener := len(listeners) > i && listeners[i] != nil
					body, relsPerFunc, err := e.compileLocalWasmFunction(fctx, module, wasm.Index(i), fe, ssaBuilder, be, needListener)
					if err != nil {
						cancel(fmt.Errorf("compile function %d/%d: %v", i, len(module.CodeSection)-1, err))
						return
					}

					compiledFuncs[i] = compiledFunc{
						fctx, i, fidx, body,
						// These slices are internal to the backend compiler and since we are going to buffer them instead
						// of process them immediately we need to copy the memory.
						slices.Clone(relsPerFunc),
						slices.Clone(be.SourceOffsetInfo()),
					}
				}
			}()
		}

		wg.Wait()
		if err := context.Cause(ctx); err != nil {
			return nil, err
		}

		for i := range compiledFuncs {
			fn := &compiledFuncs[i]
			relocator.appendFunction(fn.fctx, module, cm, fn.fnum, fn.fidx, fn.body, fn.relsPerFunc, fn.offsPerFunc)
		}
	}

	// Allocate executable memory and then copy the generated machine code.
	executable, err := platform.MmapCodeSegment(relocator.totalSize)
	if err != nil {
		panic(err)
	}
	cm.executable = executable

	for i, b := range relocator.bodies {
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

	relocator.resolveRelocations(machine, executable, importedFns)

	if err = platform.MprotectRX(executable); err != nil {
		return nil, err
	}
	cm.sharedFunctions = e.sharedFunctions
	e.setFinalizer(cm.executables, executablesFinalizer)
	return cm, nil
}

func functionContext(ctx context.Context, module *wasm.Module, fnum int, fidx wasm.Index) context.Context {
	if wazevoapi.NeedFunctionNameInContext {
		def := module.FunctionDefinition(fidx)
		name := def.DebugName()
		if len(def.ExportNames()) > 0 {
			name = def.ExportNames()[0]
		}
		ctx = wazevoapi.SetCurrentFunctionName(ctx, fnum, fmt.Sprintf("[%d/%d]%s", fnum, len(module.CodeSection)-1, name))
	}
	return ctx
}

type engineRelocator struct {
	bodies                      [][]byte
	refToBinaryOffset           []int
	rels                        []backend.RelocationInfo
	totalSize                   int // Total binary size of the executable.
	trampolineInterval          int
	callTrampolineIslandSize    int
	callTrampolineIslandOffsets []int // Holds the offsets of trampoline islands.
}

func newEngineRelocator(
	machine backend.Machine,
	importedFns, localFns int,
) (r engineRelocator, err error) {
	// Trampoline relocation related variables.
	r.trampolineInterval, r.callTrampolineIslandSize, err = machine.CallTrampolineIslandInfo(localFns)
	r.refToBinaryOffset = make([]int, importedFns+localFns)
	r.bodies = make([][]byte, 0, localFns)
	return
}

func (r *engineRelocator) resolveRelocations(machine backend.Machine, executable []byte, importedFns int) {
	// Resolve relocations for local function calls.
	if len(r.rels) > 0 {
		machine.ResolveRelocations(r.refToBinaryOffset, importedFns, executable, r.rels, r.callTrampolineIslandOffsets)
	}
}

func (r *engineRelocator) appendFunction(
	ctx context.Context,
	module *wasm.Module,
	cm *compiledModule,
	fnum int, fidx wasm.Index,
	body []byte,
	relsPerFunc []backend.RelocationInfo,
	offsPerFunc []backend.SourceOffsetInfo,
) {
	// Align 16-bytes boundary.
	r.totalSize = (r.totalSize + 15) &^ 15
	cm.functionOffsets[fnum] = r.totalSize

	needSourceInfo := module.DWARFLines != nil
	if needSourceInfo {
		// At the beginning of the function, we add the offset of the function body so that
		// we can resolve the source location of the call site of before listener call.
		cm.sourceMap.executableOffsets = append(cm.sourceMap.executableOffsets, uintptr(r.totalSize))
		cm.sourceMap.wasmBinaryOffsets = append(cm.sourceMap.wasmBinaryOffsets, module.CodeSection[fnum].BodyOffsetInCodeSection)

		for _, info := range offsPerFunc {
			cm.sourceMap.executableOffsets = append(cm.sourceMap.executableOffsets, uintptr(r.totalSize)+uintptr(info.ExecutableOffset))
			cm.sourceMap.wasmBinaryOffsets = append(cm.sourceMap.wasmBinaryOffsets, uint64(info.SourceOffset))
		}
	}

	fref := frontend.FunctionIndexToFuncRef(fidx)
	r.refToBinaryOffset[fref] = r.totalSize

	// At this point, relocation offsets are relative to the start of the function body,
	// so we adjust it to the start of the executable.
	r.rels = slices.Grow(r.rels, len(relsPerFunc))
	for _, rel := range relsPerFunc {
		rel.Offset += int64(r.totalSize)
		r.rels = append(r.rels, rel)
	}

	r.totalSize += len(body)
	r.bodies = append(r.bodies, body)
	if wazevoapi.PrintMachineCodeHexPerFunction {
		fmt.Printf("[[[machine code for %s]]]\n%s\n\n", wazevoapi.GetCurrentFunctionName(ctx), hex.EncodeToString(body))
	}

	if r.callTrampolineIslandSize > 0 {
		// If the total size exceeds the trampoline interval, we need to add a trampoline island.
		if r.totalSize/r.trampolineInterval > len(r.callTrampolineIslandOffsets) {
			r.callTrampolineIslandOffsets = append(r.callTrampolineIslandOffsets, r.totalSize)
			r.totalSize += r.callTrampolineIslandSize
		}
	}
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
	return slices.Clone(original), rels, nil
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
		bodies[i] = slices.Clone(body)
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

	if err = platform.MprotectRX(executable); err != nil {
		return nil, err
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
	if !ok {
		return
	}
	cm.refCount--
	if cm.refCount > 0 {
		return
	}
	if len(cm.executable) > 0 {
		e.deleteCompiledModuleFromSortedList(cm.compiledModule)
	}
	delete(e.compiledModules, m.ID)
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

	compiled, ok := e.getCompiledModuleFromMemory(m, false)
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
	var sizes [8]int
	var trampolines []byte

	addTrampoline := func(i int, buf []byte) {
		trampolines = append(trampolines, buf...)
		align := 15 & -len(trampolines) // Align 16-bytes boundary.
		trampolines = append(trampolines, make([]byte, align)...)
		sizes[i] = len(buf) + align
	}

	e.be.Init()
	addTrampoline(0,
		e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeGrowMemory, &ssa.Signature{
			Params:  []ssa.Type{ssa.TypeI64 /* exec context */, ssa.TypeI32},
			Results: []ssa.Type{ssa.TypeI32},
		}, false))

	e.be.Init()
	addTrampoline(1,
		e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeTableGrow, &ssa.Signature{
			Params:  []ssa.Type{ssa.TypeI64 /* exec context */, ssa.TypeI32 /* table index */, ssa.TypeI32 /* num */, ssa.TypeI64 /* ref */},
			Results: []ssa.Type{ssa.TypeI32},
		}, false))

	e.be.Init()
	addTrampoline(2,
		e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeCheckModuleExitCode, &ssa.Signature{
			Params:  []ssa.Type{ssa.TypeI32 /* exec context */},
			Results: []ssa.Type{ssa.TypeI32},
		}, false))

	e.be.Init()
	addTrampoline(3,
		e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeRefFunc, &ssa.Signature{
			Params:  []ssa.Type{ssa.TypeI64 /* exec context */, ssa.TypeI32 /* function index */},
			Results: []ssa.Type{ssa.TypeI64}, // returns the function reference.
		}, false))

	e.be.Init()
	addTrampoline(4, e.machine.CompileStackGrowCallSequence())

	e.be.Init()
	addTrampoline(5,
		e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeMemoryWait32, &ssa.Signature{
			// exec context, timeout, expected, addr
			Params: []ssa.Type{ssa.TypeI64, ssa.TypeI64, ssa.TypeI32, ssa.TypeI64},
			// Returns the status.
			Results: []ssa.Type{ssa.TypeI32},
		}, false))

	e.be.Init()
	addTrampoline(6,
		e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeMemoryWait64, &ssa.Signature{
			// exec context, timeout, expected, addr
			Params: []ssa.Type{ssa.TypeI64, ssa.TypeI64, ssa.TypeI64, ssa.TypeI64},
			// Returns the status.
			Results: []ssa.Type{ssa.TypeI32},
		}, false))

	e.be.Init()
	addTrampoline(7,
		e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeMemoryNotify, &ssa.Signature{
			// exec context, count, addr
			Params: []ssa.Type{ssa.TypeI64, ssa.TypeI32, ssa.TypeI64},
			// Returns the number notified.
			Results: []ssa.Type{ssa.TypeI32},
		}, false))

	fns := &sharedFunctions{
		executable:          mmapExecutable(trampolines),
		listenerTrampolines: make(listenerTrampolines),
	}
	e.setFinalizer(fns, sharedFunctionsFinalizer)

	offset := 0
	fns.memoryGrowAddress = &fns.executable[offset]
	offset += sizes[0]
	fns.tableGrowAddress = &fns.executable[offset]
	offset += sizes[1]
	fns.checkModuleExitCodeAddress = &fns.executable[offset]
	offset += sizes[2]
	fns.refFuncAddress = &fns.executable[offset]
	offset += sizes[3]
	fns.stackGrowAddress = &fns.executable[offset]
	offset += sizes[4]
	fns.memoryWait32Address = &fns.executable[offset]
	offset += sizes[5]
	fns.memoryWait64Address = &fns.executable[offset]
	offset += sizes[6]
	fns.memoryNotifyAddress = &fns.executable[offset]

	if wazevoapi.PerfMapEnabled {
		wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(fns.memoryGrowAddress)), uint64(sizes[0]), "memory_grow_trampoline")
		wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(fns.tableGrowAddress)), uint64(sizes[1]), "table_grow_trampoline")
		wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(fns.checkModuleExitCodeAddress)), uint64(sizes[2]), "check_module_exit_code_trampoline")
		wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(fns.refFuncAddress)), uint64(sizes[3]), "ref_func_trampoline")
		wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(fns.stackGrowAddress)), uint64(sizes[4]), "stack_grow_trampoline")
		wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(fns.memoryWait32Address)), uint64(sizes[5]), "memory_wait32_trampoline")
		wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(fns.memoryWait64Address)), uint64(sizes[6]), "memory_wait64_trampoline")
		wazevoapi.PerfMap.AddEntry(uintptr(unsafe.Pointer(fns.memoryNotifyAddress)), uint64(sizes[7]), "memory_notify_trampoline")
	}

	e.sharedFunctions = fns
}

func sharedFunctionsFinalizer(sf *sharedFunctions) {
	if err := platform.MunmapCodeSegment(sf.executable); err != nil {
		panic(err)
	}
	for _, f := range sf.listenerTrampolines {
		if err := platform.MunmapCodeSegment(f.executable); err != nil {
			panic(err)
		}
	}

	sf.executable = nil
	sf.listenerTrampolines = nil
}

func executablesFinalizer(exec *executables) {
	if len(exec.executable) > 0 {
		if err := platform.MunmapCodeSegment(exec.executable); err != nil {
			panic(err)
		}
	}
	exec.executable = nil

	if len(exec.entryPreambles) > 0 {
		if err := platform.MunmapCodeSegment(exec.entryPreambles); err != nil {
			panic(err)
		}
	}
	exec.entryPreambles = nil
	exec.entryPreamblesPtrs = nil
}

func mmapExecutable(src []byte) []byte {
	executable, err := platform.MmapCodeSegment(len(src))
	if err != nil {
		panic(err)
	}

	copy(executable, src)

	if err = platform.MprotectRX(executable); err != nil {
		panic(err)
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

	trampoline, ok := e.sharedFunctions.listenerTrampolines[functionType]
	if !ok {
		var executable []byte
		beforeSig, afterSig := frontend.SignatureForListener(functionType)

		e.be.Init()
		buf := e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeCallListenerBefore, beforeSig, false)
		executable = append(executable, buf...)

		align := 15 & -len(executable) // Align 16-bytes boundary.
		executable = append(executable, make([]byte, align)...)
		offset := len(executable)

		e.be.Init()
		buf = e.machine.CompileGoFunctionTrampoline(wazevoapi.ExitCodeCallListenerAfter, afterSig, false)
		executable = append(executable, buf...)

		trampoline.executable = mmapExecutable(executable)
		trampoline.before = &trampoline.executable[0]
		trampoline.after = &trampoline.executable[offset]

		e.sharedFunctions.listenerTrampolines[functionType] = trampoline
	}
	return trampoline.before, trampoline.after
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
