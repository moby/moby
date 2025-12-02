package wazevo

import (
	"encoding/binary"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wasmruntime"
)

type (
	// moduleEngine implements wasm.ModuleEngine.
	moduleEngine struct {
		// opaquePtr equals &opaque[0].
		opaquePtr              *byte
		parent                 *compiledModule
		module                 *wasm.ModuleInstance
		opaque                 moduleContextOpaque
		localFunctionInstances []*functionInstance
		importedFunctions      []importedFunction
		listeners              []experimental.FunctionListener
	}

	functionInstance struct {
		executable             *byte
		moduleContextOpaquePtr *byte
		typeID                 wasm.FunctionTypeID
		indexInModule          wasm.Index
	}

	importedFunction struct {
		me            *moduleEngine
		indexInModule wasm.Index
	}

	// moduleContextOpaque is the opaque byte slice of Module instance specific contents whose size
	// is only Wasm-compile-time known, hence dynamic. Its contents are basically the pointers to the module instance,
	// specific objects as well as functions. This is sometimes called "VMContext" in other Wasm runtimes.
	//
	// Internally, the buffer is structured as follows:
	//
	// 	type moduleContextOpaque struct {
	// 	    moduleInstance                            *wasm.ModuleInstance
	// 	    localMemoryBufferPtr                      *byte                (optional)
	// 	    localMemoryLength                         uint64               (optional)
	// 	    importedMemoryInstance                    *wasm.MemoryInstance (optional)
	// 	    importedMemoryOwnerOpaqueCtx              *byte                (optional)
	// 	    importedFunctions                         [# of importedFunctions]functionInstance
	//      importedGlobals                           []ImportedGlobal       (optional)
	//      localGlobals                              []Global               (optional)
	//      typeIDsBegin                              &wasm.ModuleInstance.TypeIDs[0]  (optional)
	//      tables                                    []*wasm.TableInstance  (optional)
	// 	    beforeListenerTrampolines1stElement       **byte                 (optional)
	// 	    afterListenerTrampolines1stElement        **byte                 (optional)
	//      dataInstances1stElement                   []wasm.DataInstance    (optional)
	//      elementInstances1stElement                []wasm.ElementInstance (optional)
	// 	}
	//
	//  type ImportedGlobal struct {
	// 		*Global
	// 		_ uint64 // padding
	//  }
	//
	//  type Global struct {
	// 		Val, ValHi uint64
	//  }
	//
	// See wazevoapi.NewModuleContextOffsetData for the details of the offsets.
	//
	// Note that for host modules, the structure is entirely different. See buildHostModuleOpaque.
	moduleContextOpaque []byte
)

func newAlignedOpaque(size int) moduleContextOpaque {
	// Check if the size is a multiple of 16.
	if size%16 != 0 {
		panic("size must be a multiple of 16")
	}
	buf := make([]byte, size+16)
	// Align the buffer to 16 bytes.
	rem := uintptr(unsafe.Pointer(&buf[0])) % 16
	buf = buf[16-rem:]
	return buf
}

func (m *moduleEngine) setupOpaque() {
	inst := m.module
	offsets := &m.parent.offsets
	opaque := m.opaque

	binary.LittleEndian.PutUint64(opaque[offsets.ModuleInstanceOffset:],
		uint64(uintptr(unsafe.Pointer(m.module))),
	)

	if lm := offsets.LocalMemoryBegin; lm >= 0 {
		m.putLocalMemory()
	}

	// Note: imported memory is resolved in ResolveImportedFunction.

	// Note: imported functions are resolved in ResolveImportedFunction.

	if globalOffset := offsets.GlobalsBegin; globalOffset >= 0 {
		for i, g := range inst.Globals {
			if i < int(inst.Source.ImportGlobalCount) {
				importedME := g.Me.(*moduleEngine)
				offset := importedME.parent.offsets.GlobalInstanceOffset(g.Index)
				importedMEOpaque := importedME.opaque
				binary.LittleEndian.PutUint64(opaque[globalOffset:],
					uint64(uintptr(unsafe.Pointer(&importedMEOpaque[offset]))))
			} else {
				binary.LittleEndian.PutUint64(opaque[globalOffset:], g.Val)
				binary.LittleEndian.PutUint64(opaque[globalOffset+8:], g.ValHi)
			}
			globalOffset += 16
		}
	}

	if tableOffset := offsets.TablesBegin; tableOffset >= 0 {
		// First we write the first element's address of typeIDs.
		if len(inst.TypeIDs) > 0 {
			binary.LittleEndian.PutUint64(opaque[offsets.TypeIDs1stElement:], uint64(uintptr(unsafe.Pointer(&inst.TypeIDs[0]))))
		}

		// Then we write the table addresses.
		for _, table := range inst.Tables {
			binary.LittleEndian.PutUint64(opaque[tableOffset:], uint64(uintptr(unsafe.Pointer(table))))
			tableOffset += 8
		}
	}

	if beforeListenerOffset := offsets.BeforeListenerTrampolines1stElement; beforeListenerOffset >= 0 {
		binary.LittleEndian.PutUint64(opaque[beforeListenerOffset:], uint64(uintptr(unsafe.Pointer(&m.parent.listenerBeforeTrampolines[0]))))
	}
	if afterListenerOffset := offsets.AfterListenerTrampolines1stElement; afterListenerOffset >= 0 {
		binary.LittleEndian.PutUint64(opaque[afterListenerOffset:], uint64(uintptr(unsafe.Pointer(&m.parent.listenerAfterTrampolines[0]))))
	}
	if len(inst.DataInstances) > 0 {
		binary.LittleEndian.PutUint64(opaque[offsets.DataInstances1stElement:], uint64(uintptr(unsafe.Pointer(&inst.DataInstances[0]))))
	}
	if len(inst.ElementInstances) > 0 {
		binary.LittleEndian.PutUint64(opaque[offsets.ElementInstances1stElement:], uint64(uintptr(unsafe.Pointer(&inst.ElementInstances[0]))))
	}
}

// NewFunction implements wasm.ModuleEngine.
func (m *moduleEngine) NewFunction(index wasm.Index) api.Function {
	if wazevoapi.PrintMachineCodeHexPerFunctionDisassemblable {
		panic("When PrintMachineCodeHexPerFunctionDisassemblable enabled, functions must not be called")
	}

	localIndex := index
	if importedFnCount := m.module.Source.ImportFunctionCount; index < importedFnCount {
		imported := &m.importedFunctions[index]
		return imported.me.NewFunction(imported.indexInModule)
	} else {
		localIndex -= importedFnCount
	}

	src := m.module.Source
	typIndex := src.FunctionSection[localIndex]
	typ := src.TypeSection[typIndex]
	sizeOfParamResultSlice := typ.ResultNumInUint64
	if ps := typ.ParamNumInUint64; ps > sizeOfParamResultSlice {
		sizeOfParamResultSlice = ps
	}
	p := m.parent
	offset := p.functionOffsets[localIndex]

	ce := &callEngine{
		indexInModule:          index,
		executable:             &p.executable[offset],
		parent:                 m,
		preambleExecutable:     &m.parent.entryPreambles[typIndex][0],
		sizeOfParamResultSlice: sizeOfParamResultSlice,
		requiredParams:         typ.ParamNumInUint64,
		numberOfResults:        typ.ResultNumInUint64,
	}

	ce.execCtx.memoryGrowTrampolineAddress = &m.parent.sharedFunctions.memoryGrowExecutable[0]
	ce.execCtx.stackGrowCallTrampolineAddress = &m.parent.sharedFunctions.stackGrowExecutable[0]
	ce.execCtx.checkModuleExitCodeTrampolineAddress = &m.parent.sharedFunctions.checkModuleExitCode[0]
	ce.execCtx.tableGrowTrampolineAddress = &m.parent.sharedFunctions.tableGrowExecutable[0]
	ce.execCtx.refFuncTrampolineAddress = &m.parent.sharedFunctions.refFuncExecutable[0]
	ce.execCtx.memoryWait32TrampolineAddress = &m.parent.sharedFunctions.memoryWait32Executable[0]
	ce.execCtx.memoryWait64TrampolineAddress = &m.parent.sharedFunctions.memoryWait64Executable[0]
	ce.execCtx.memoryNotifyTrampolineAddress = &m.parent.sharedFunctions.memoryNotifyExecutable[0]
	ce.execCtx.memmoveAddress = memmovPtr
	ce.init()
	return ce
}

// GetGlobalValue implements the same method as documented on wasm.ModuleEngine.
func (m *moduleEngine) GetGlobalValue(i wasm.Index) (lo, hi uint64) {
	offset := m.parent.offsets.GlobalInstanceOffset(i)
	buf := m.opaque[offset:]
	if i < m.module.Source.ImportGlobalCount {
		panic("GetGlobalValue should not be called for imported globals")
	}
	return binary.LittleEndian.Uint64(buf), binary.LittleEndian.Uint64(buf[8:])
}

// SetGlobalValue implements the same method as documented on wasm.ModuleEngine.
func (m *moduleEngine) SetGlobalValue(i wasm.Index, lo, hi uint64) {
	offset := m.parent.offsets.GlobalInstanceOffset(i)
	buf := m.opaque[offset:]
	if i < m.module.Source.ImportGlobalCount {
		panic("GetGlobalValue should not be called for imported globals")
	}
	binary.LittleEndian.PutUint64(buf, lo)
	binary.LittleEndian.PutUint64(buf[8:], hi)
}

// OwnsGlobals implements the same method as documented on wasm.ModuleEngine.
func (m *moduleEngine) OwnsGlobals() bool { return true }

// MemoryGrown implements wasm.ModuleEngine.
func (m *moduleEngine) MemoryGrown() {
	m.putLocalMemory()
}

// putLocalMemory writes the local memory buffer pointer and length to the opaque buffer.
func (m *moduleEngine) putLocalMemory() {
	mem := m.module.MemoryInstance
	offset := m.parent.offsets.LocalMemoryBegin

	s := uint64(len(mem.Buffer))
	var b uint64
	if len(mem.Buffer) > 0 {
		b = uint64(uintptr(unsafe.Pointer(&mem.Buffer[0])))
	}
	binary.LittleEndian.PutUint64(m.opaque[offset:], b)
	binary.LittleEndian.PutUint64(m.opaque[offset+8:], s)
}

// ResolveImportedFunction implements wasm.ModuleEngine.
func (m *moduleEngine) ResolveImportedFunction(index, descFunc, indexInImportedModule wasm.Index, importedModuleEngine wasm.ModuleEngine) {
	executableOffset, moduleCtxOffset, typeIDOffset := m.parent.offsets.ImportedFunctionOffset(index)
	importedME := importedModuleEngine.(*moduleEngine)

	if int(indexInImportedModule) >= len(importedME.importedFunctions) {
		indexInImportedModule -= wasm.Index(len(importedME.importedFunctions))
	} else {
		imported := &importedME.importedFunctions[indexInImportedModule]
		m.ResolveImportedFunction(index, descFunc, imported.indexInModule, imported.me)
		return // Recursively resolve the imported function.
	}

	offset := importedME.parent.functionOffsets[indexInImportedModule]
	typeID := m.module.TypeIDs[descFunc]
	executable := &importedME.parent.executable[offset]
	// Write functionInstance.
	binary.LittleEndian.PutUint64(m.opaque[executableOffset:], uint64(uintptr(unsafe.Pointer(executable))))
	binary.LittleEndian.PutUint64(m.opaque[moduleCtxOffset:], uint64(uintptr(unsafe.Pointer(importedME.opaquePtr))))
	binary.LittleEndian.PutUint64(m.opaque[typeIDOffset:], uint64(typeID))

	// Write importedFunction so that it can be used by NewFunction.
	m.importedFunctions[index] = importedFunction{me: importedME, indexInModule: indexInImportedModule}
}

// ResolveImportedMemory implements wasm.ModuleEngine.
func (m *moduleEngine) ResolveImportedMemory(importedModuleEngine wasm.ModuleEngine) {
	importedME := importedModuleEngine.(*moduleEngine)
	inst := importedME.module

	var memInstPtr uint64
	var memOwnerOpaquePtr uint64
	if offs := importedME.parent.offsets; offs.ImportedMemoryBegin >= 0 {
		offset := offs.ImportedMemoryBegin
		memInstPtr = binary.LittleEndian.Uint64(importedME.opaque[offset:])
		memOwnerOpaquePtr = binary.LittleEndian.Uint64(importedME.opaque[offset+8:])
	} else {
		memInstPtr = uint64(uintptr(unsafe.Pointer(inst.MemoryInstance)))
		memOwnerOpaquePtr = uint64(uintptr(unsafe.Pointer(importedME.opaquePtr)))
	}
	offset := m.parent.offsets.ImportedMemoryBegin
	binary.LittleEndian.PutUint64(m.opaque[offset:], memInstPtr)
	binary.LittleEndian.PutUint64(m.opaque[offset+8:], memOwnerOpaquePtr)
}

// DoneInstantiation implements wasm.ModuleEngine.
func (m *moduleEngine) DoneInstantiation() {
	if !m.module.Source.IsHostModule {
		m.setupOpaque()
	}
}

// FunctionInstanceReference implements wasm.ModuleEngine.
func (m *moduleEngine) FunctionInstanceReference(funcIndex wasm.Index) wasm.Reference {
	if funcIndex < m.module.Source.ImportFunctionCount {
		begin, _, _ := m.parent.offsets.ImportedFunctionOffset(funcIndex)
		return uintptr(unsafe.Pointer(&m.opaque[begin]))
	}
	localIndex := funcIndex - m.module.Source.ImportFunctionCount
	p := m.parent
	executable := &p.executable[p.functionOffsets[localIndex]]
	typeID := m.module.TypeIDs[m.module.Source.FunctionSection[localIndex]]

	lf := &functionInstance{
		executable:             executable,
		moduleContextOpaquePtr: m.opaquePtr,
		typeID:                 typeID,
		indexInModule:          funcIndex,
	}
	m.localFunctionInstances = append(m.localFunctionInstances, lf)
	return uintptr(unsafe.Pointer(lf))
}

// LookupFunction implements wasm.ModuleEngine.
func (m *moduleEngine) LookupFunction(t *wasm.TableInstance, typeId wasm.FunctionTypeID, tableOffset wasm.Index) (*wasm.ModuleInstance, wasm.Index) {
	if tableOffset >= uint32(len(t.References)) || t.Type != wasm.RefTypeFuncref {
		panic(wasmruntime.ErrRuntimeInvalidTableAccess)
	}
	rawPtr := t.References[tableOffset]
	if rawPtr == 0 {
		panic(wasmruntime.ErrRuntimeInvalidTableAccess)
	}

	tf := wazevoapi.PtrFromUintptr[functionInstance](rawPtr)
	if tf.typeID != typeId {
		panic(wasmruntime.ErrRuntimeIndirectCallTypeMismatch)
	}
	return moduleInstanceFromOpaquePtr(tf.moduleContextOpaquePtr), tf.indexInModule
}

func moduleInstanceFromOpaquePtr(ptr *byte) *wasm.ModuleInstance {
	return *(**wasm.ModuleInstance)(unsafe.Pointer(ptr))
}
