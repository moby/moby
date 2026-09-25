package wazevoapi

import (
	"github.com/tetratelabs/wazero/internal/wasm"
)

const (
	// FunctionInstanceSize is the size of wazevo.functionInstance.
	FunctionInstanceSize = 24
	// FunctionInstanceExecutableOffset is an offset of `executable` field in wazevo.functionInstance
	FunctionInstanceExecutableOffset = 0
	// FunctionInstanceModuleContextOpaquePtrOffset is an offset of `moduleContextOpaquePtr` field in wazevo.functionInstance
	FunctionInstanceModuleContextOpaquePtrOffset = 8
	// FunctionInstanceTypeIDOffset is an offset of `typeID` field in wazevo.functionInstance
	FunctionInstanceTypeIDOffset = 16
)

const (
	// ExecutionContextOffsetExitCodeOffset is an offset of `exitCode` field in wazevo.executionContext
	ExecutionContextOffsetExitCodeOffset Offset = 0
	// ExecutionContextOffsetCallerModuleContextPtr is an offset of `callerModuleContextPtr` field in wazevo.executionContext
	ExecutionContextOffsetCallerModuleContextPtr Offset = 8
	// ExecutionContextOffsetOriginalFramePointer is an offset of `originalFramePointer` field in wazevo.executionContext
	ExecutionContextOffsetOriginalFramePointer Offset = 16
	// ExecutionContextOffsetOriginalStackPointer is an offset of `originalStackPointer` field in wazevo.executionContext
	ExecutionContextOffsetOriginalStackPointer Offset = 24
	// ExecutionContextOffsetGoReturnAddress is an offset of `goReturnAddress` field in wazevo.executionContext
	ExecutionContextOffsetGoReturnAddress Offset = 32
	// ExecutionContextOffsetStackBottomPtr is an offset of `stackBottomPtr` field in wazevo.executionContext
	ExecutionContextOffsetStackBottomPtr Offset = 40
	// ExecutionContextOffsetGoCallReturnAddress is an offset of `goCallReturnAddress` field in wazevo.executionContext
	ExecutionContextOffsetGoCallReturnAddress Offset = 48
	// ExecutionContextOffsetStackPointerBeforeGoCall is an offset of `StackPointerBeforeGoCall` field in wazevo.executionContext
	ExecutionContextOffsetStackPointerBeforeGoCall Offset = 56
	// ExecutionContextOffsetStackGrowRequiredSize is an offset of `stackGrowRequiredSize` field in wazevo.executionContext
	ExecutionContextOffsetStackGrowRequiredSize Offset = 64
	// ExecutionContextOffsetMemoryGrowTrampolineAddress is an offset of `memoryGrowTrampolineAddress` field in wazevo.executionContext
	ExecutionContextOffsetMemoryGrowTrampolineAddress Offset = 72
	// ExecutionContextOffsetStackGrowCallTrampolineAddress is an offset of `stackGrowCallTrampolineAddress` field in wazevo.executionContext.
	ExecutionContextOffsetStackGrowCallTrampolineAddress Offset = 80
	// ExecutionContextOffsetCheckModuleExitCodeTrampolineAddress is an offset of `checkModuleExitCodeTrampolineAddress` field in wazevo.executionContext.
	ExecutionContextOffsetCheckModuleExitCodeTrampolineAddress Offset = 88
	// ExecutionContextOffsetSavedRegistersBegin is an offset of the first element of `savedRegisters` field in wazevo.executionContext
	ExecutionContextOffsetSavedRegistersBegin Offset = 96
	// ExecutionContextOffsetGoFunctionCallCalleeModuleContextOpaque is an offset of `goFunctionCallCalleeModuleContextOpaque` field in wazevo.executionContext
	ExecutionContextOffsetGoFunctionCallCalleeModuleContextOpaque Offset = 1120
	// ExecutionContextOffsetTableGrowTrampolineAddress is an offset of `tableGrowTrampolineAddress` field in wazevo.executionContext
	ExecutionContextOffsetTableGrowTrampolineAddress Offset = 1128
	// ExecutionContextOffsetRefFuncTrampolineAddress is an offset of `refFuncTrampolineAddress` field in wazevo.executionContext
	ExecutionContextOffsetRefFuncTrampolineAddress      Offset = 1136
	ExecutionContextOffsetMemmoveAddress                Offset = 1144
	ExecutionContextOffsetFramePointerBeforeGoCall      Offset = 1152
	ExecutionContextOffsetMemoryWait32TrampolineAddress Offset = 1160
	ExecutionContextOffsetMemoryWait64TrampolineAddress Offset = 1168
	ExecutionContextOffsetMemoryNotifyTrampolineAddress Offset = 1176
)

// ModuleContextOffsetData allows the compilers to get the information about offsets to the fields of wazevo.moduleContextOpaque,
// This is unique per module.
type ModuleContextOffsetData struct {
	TotalSize int
	ModuleInstanceOffset,
	LocalMemoryBegin,
	ImportedMemoryBegin,
	ImportedFunctionsBegin,
	GlobalsBegin,
	TypeIDs1stElement,
	TablesBegin,
	BeforeListenerTrampolines1stElement,
	AfterListenerTrampolines1stElement,
	DataInstances1stElement,
	ElementInstances1stElement Offset
}

// ImportedFunctionOffset returns an offset of the i-th imported function.
// Each item is stored as wazevo.functionInstance whose size matches FunctionInstanceSize.
func (m *ModuleContextOffsetData) ImportedFunctionOffset(i wasm.Index) (
	executableOffset, moduleCtxOffset, typeIDOffset Offset,
) {
	base := m.ImportedFunctionsBegin + Offset(i)*FunctionInstanceSize
	return base, base + 8, base + 16
}

// GlobalInstanceOffset returns an offset of the i-th global instance.
func (m *ModuleContextOffsetData) GlobalInstanceOffset(i wasm.Index) Offset {
	return m.GlobalsBegin + Offset(i)*16
}

// Offset represents an offset of a field of a struct.
type Offset int32

// U32 encodes an Offset as uint32 for convenience.
func (o Offset) U32() uint32 {
	return uint32(o)
}

// I64 encodes an Offset as int64 for convenience.
func (o Offset) I64() int64 {
	return int64(o)
}

// U64 encodes an Offset as int64 for convenience.
func (o Offset) U64() uint64 {
	return uint64(o)
}

// LocalMemoryBase returns an offset of the first byte of the local memory.
func (m *ModuleContextOffsetData) LocalMemoryBase() Offset {
	return m.LocalMemoryBegin
}

// LocalMemoryLen returns an offset of the length of the local memory buffer.
func (m *ModuleContextOffsetData) LocalMemoryLen() Offset {
	if l := m.LocalMemoryBegin; l >= 0 {
		return l + 8
	}
	return -1
}

// TableOffset returns an offset of the i-th table instance.
func (m *ModuleContextOffsetData) TableOffset(tableIndex int) Offset {
	return m.TablesBegin + Offset(tableIndex)*8
}

// NewModuleContextOffsetData creates a ModuleContextOffsetData determining the structure of moduleContextOpaque for the given Module.
// The structure is described in the comment of wazevo.moduleContextOpaque.
func NewModuleContextOffsetData(m *wasm.Module, withListener bool) ModuleContextOffsetData {
	ret := ModuleContextOffsetData{}
	var offset Offset

	ret.ModuleInstanceOffset = 0
	offset += 8

	if m.MemorySection != nil {
		ret.LocalMemoryBegin = offset
		// buffer base + memory size.
		const localMemorySizeInOpaqueModuleContext = 16
		offset += localMemorySizeInOpaqueModuleContext
	} else {
		// Indicates that there's no local memory
		ret.LocalMemoryBegin = -1
	}

	if m.ImportMemoryCount > 0 {
		offset = align8(offset)
		// *wasm.MemoryInstance + imported memory's owner (moduleContextOpaque)
		const importedMemorySizeInOpaqueModuleContext = 16
		ret.ImportedMemoryBegin = offset
		offset += importedMemorySizeInOpaqueModuleContext
	} else {
		// Indicates that there's no imported memory
		ret.ImportedMemoryBegin = -1
	}

	if m.ImportFunctionCount > 0 {
		offset = align8(offset)
		ret.ImportedFunctionsBegin = offset
		// Each function is stored wazevo.functionInstance.
		size := int(m.ImportFunctionCount) * FunctionInstanceSize
		offset += Offset(size)
	} else {
		ret.ImportedFunctionsBegin = -1
	}

	if globals := int(m.ImportGlobalCount) + len(m.GlobalSection); globals > 0 {
		// Align to 16 bytes for globals, as f32/f64/v128 might be loaded via SIMD instructions.
		offset = align16(offset)
		ret.GlobalsBegin = offset
		// Pointers to *wasm.GlobalInstance.
		offset += Offset(globals) * 16
	} else {
		ret.GlobalsBegin = -1
	}

	if tables := len(m.TableSection) + int(m.ImportTableCount); tables > 0 {
		offset = align8(offset)
		ret.TypeIDs1stElement = offset
		offset += 8 // First element of TypeIDs.

		ret.TablesBegin = offset
		// Pointers to *wasm.TableInstance.
		offset += Offset(tables) * 8
	} else {
		ret.TypeIDs1stElement = -1
		ret.TablesBegin = -1
	}

	if withListener {
		offset = align8(offset)
		ret.BeforeListenerTrampolines1stElement = offset
		offset += 8 // First element of BeforeListenerTrampolines.

		ret.AfterListenerTrampolines1stElement = offset
		offset += 8 // First element of AfterListenerTrampolines.
	} else {
		ret.BeforeListenerTrampolines1stElement = -1
		ret.AfterListenerTrampolines1stElement = -1
	}

	ret.DataInstances1stElement = offset
	offset += 8 // First element of DataInstances.

	ret.ElementInstances1stElement = offset
	offset += 8 // First element of ElementInstances.

	ret.TotalSize = int(align16(offset))
	return ret
}

func align16(o Offset) Offset {
	return (o + 15) &^ 15
}

func align8(o Offset) Offset {
	return (o + 7) &^ 7
}
