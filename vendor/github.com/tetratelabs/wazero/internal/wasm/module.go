package wasm

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/ieee754"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasmdebug"
)

// Module is a WebAssembly binary representation.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#modules%E2%91%A8
//
// Differences from the specification:
// * NameSection is the only key ("name") decoded from the SectionIDCustom.
// * ExportSection is represented as a map for lookup convenience.
// * Code.GoFunc is contains any go `func`. It may be present when Code.Body is not.
type Module struct {
	// TypeSection contains the unique FunctionType of functions imported or defined in this module.
	//
	// Note: Currently, there is no type ambiguity in the index as WebAssembly 1.0 only defines function type.
	// In the future, other types may be introduced to support CoreFeatures such as module linking.
	//
	// Note: In the Binary Format, this is SectionIDType.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#types%E2%91%A0%E2%91%A0
	TypeSection []FunctionType

	// ImportSection contains imported functions, tables, memories or globals required for instantiation
	// (Store.Instantiate).
	//
	// Note: there are no unique constraints relating to the two-level namespace of Import.Module and Import.Name.
	//
	// Note: In the Binary Format, this is SectionIDImport.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#import-section%E2%91%A0
	ImportSection []Import
	// ImportFunctionCount ImportGlobalCount ImportMemoryCount, and ImportTableCount are
	// the cached import count per ExternType set during decoding.
	ImportFunctionCount,
	ImportGlobalCount,
	ImportMemoryCount,
	ImportTableCount Index
	// ImportPerModule maps a module name to the list of Import to be imported from the module.
	// This is used to do fast import resolution during instantiation.
	ImportPerModule map[string][]*Import

	// FunctionSection contains the index in TypeSection of each function defined in this module.
	//
	// Note: The function Index space begins with imported functions and ends with those defined in this module.
	// For example, if there are two imported functions and one defined in this module, the function Index 3 is defined
	// in this module at FunctionSection[0].
	//
	// Note: FunctionSection is index correlated with the CodeSection. If given the same position, e.g. 2, a function
	// type is at TypeSection[FunctionSection[2]], while its locals and body are at CodeSection[2].
	//
	// Note: In the Binary Format, this is SectionIDFunction.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#function-section%E2%91%A0
	FunctionSection []Index

	// TableSection contains each table defined in this module.
	//
	// Note: The table Index space begins with imported tables and ends with those defined in this module.
	// For example, if there are two imported tables and one defined in this module, the table Index 3 is defined in
	// this module at TableSection[0].
	//
	// Note: Version 1.0 (20191205) of the WebAssembly spec allows at most one table definition per module, so the
	// length of the TableSection can be zero or one, and can only be one if there is no imported table.
	//
	// Note: In the Binary Format, this is SectionIDTable.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#table-section%E2%91%A0
	TableSection []Table

	// MemorySection contains each memory defined in this module.
	//
	// Note: The memory Index space begins with imported memories and ends with those defined in this module.
	// For example, if there are two imported memories and one defined in this module, the memory Index 3 is defined in
	// this module at TableSection[0].
	//
	// Note: Version 1.0 (20191205) of the WebAssembly spec allows at most one memory definition per module, so the
	// length of the MemorySection can be zero or one, and can only be one if there is no imported memory.
	//
	// Note: In the Binary Format, this is SectionIDMemory.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#memory-section%E2%91%A0
	MemorySection *Memory

	// GlobalSection contains each global defined in this module.
	//
	// Global indexes are offset by any imported globals because the global index begins with imports, followed by
	// ones defined in this module. For example, if there are two imported globals and three defined in this module, the
	// global at index 3 is defined in this module at GlobalSection[0].
	//
	// Note: In the Binary Format, this is SectionIDGlobal.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#global-section%E2%91%A0
	GlobalSection []Global

	// ExportSection contains each export defined in this module.
	//
	// Note: In the Binary Format, this is SectionIDExport.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#exports%E2%91%A0
	ExportSection []Export
	// Exports maps a name to Export, and is convenient for fast look up of exported instances at runtime.
	// Each item of this map points to an element of ExportSection.
	Exports map[string]*Export

	// StartSection is the index of a function to call before returning from Store.Instantiate.
	//
	// Note: The index here is not the position in the FunctionSection, rather in the function index, which
	// begins with imported functions.
	//
	// Note: In the Binary Format, this is SectionIDStart.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#start-section%E2%91%A0
	StartSection *Index

	// Note: In the Binary Format, this is SectionIDElement.
	ElementSection []ElementSegment

	// CodeSection is index-correlated with FunctionSection and contains each
	// function's locals and body.
	//
	// When present, the HostFunctionSection of the same index must be nil.
	//
	// Note: In the Binary Format, this is SectionIDCode.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#code-section%E2%91%A0
	CodeSection []Code

	// Note: In the Binary Format, this is SectionIDData.
	DataSection []DataSegment

	// NameSection is set when the SectionIDCustom "name" was successfully decoded from the binary format.
	//
	// Note: This is the only SectionIDCustom defined in the WebAssembly 1.0 (20191205) Binary Format.
	// Others are skipped as they are not used in wazero.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#name-section%E2%91%A0
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#custom-section%E2%91%A0
	NameSection *NameSection

	// CustomSections are set when the SectionIDCustom other than "name" were successfully decoded from the binary format.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#custom-section%E2%91%A0
	CustomSections []*CustomSection

	// DataCountSection is the optional section and holds the number of data segments in the data section.
	//
	// Note: This may exist in WebAssembly 2.0 or WebAssembly 1.0 with CoreFeatureBulkMemoryOperations.
	// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#data-count-section
	// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/appendix/changes.html#bulk-memory-and-table-instructions
	DataCountSection *uint32

	// ID is the sha256 value of the source wasm plus the configurations which affect the runtime representation of
	// Wasm binary. This is only used for caching.
	ID ModuleID

	// IsHostModule true if this is the host module, false otherwise.
	IsHostModule bool

	// functionDefinitionSectionInitOnce guards FunctionDefinitionSection so that it is initialized exactly once.
	functionDefinitionSectionInitOnce sync.Once

	// FunctionDefinitionSection is a wazero-specific section.
	FunctionDefinitionSection []FunctionDefinition

	// MemoryDefinitionSection is a wazero-specific section.
	MemoryDefinitionSection []MemoryDefinition

	// DWARFLines is used to emit DWARF based stack trace. This is created from the multiple custom sections
	// as described in https://yurydelendik.github.io/webassembly-dwarf/, though it is not specified in the Wasm
	// specification: https://github.com/WebAssembly/debugging/issues/1
	DWARFLines *wasmdebug.DWARFLines
}

// ModuleID represents sha256 hash value uniquely assigned to Module.
type ModuleID = [sha256.Size]byte

// The wazero specific limitation described at RATIONALE.md.
// TL;DR; We multiply by 8 (to get offsets in bytes) and the multiplication result must be less than 32bit max
const (
	MaximumGlobals       = uint32(1 << 27)
	MaximumFunctionIndex = uint32(1 << 27)
	MaximumTableIndex    = uint32(1 << 27)
)

// AssignModuleID calculates a sha256 checksum on `wasm` and other args, and set Module.ID to the result.
// See the doc on Module.ID on what it's used for.
func (m *Module) AssignModuleID(wasm []byte, listeners []experimental.FunctionListener, withEnsureTermination bool) {
	h := sha256.New()
	h.Write(wasm)
	// Use the pre-allocated space backed by m.ID below.

	// Write the existence of listeners to the checksum per function.
	for i, l := range listeners {
		binary.LittleEndian.PutUint32(m.ID[:], uint32(i))
		m.ID[4] = boolToByte(l != nil)
		h.Write(m.ID[:5])
	}
	// Write the flag of ensureTermination to the checksum.
	m.ID[0] = boolToByte(withEnsureTermination)
	h.Write(m.ID[:1])
	// Get checksum by passing the slice underlying m.ID.
	h.Sum(m.ID[:0])
}

func boolToByte(b bool) (ret byte) {
	if b {
		ret = 1
	}
	return
}

// typeOfFunction returns the wasm.FunctionType for the given function space index or nil.
func (m *Module) typeOfFunction(funcIdx Index) *FunctionType {
	typeSectionLength, importedFunctionCount := uint32(len(m.TypeSection)), m.ImportFunctionCount
	if funcIdx < importedFunctionCount {
		// Imports are not exclusively functions. This is the current function index in the loop.
		cur := Index(0)
		for i := range m.ImportSection {
			imp := &m.ImportSection[i]
			if imp.Type != ExternTypeFunc {
				continue
			}
			if funcIdx == cur {
				if imp.DescFunc >= typeSectionLength {
					return nil
				}
				return &m.TypeSection[imp.DescFunc]
			}
			cur++
		}
	}

	funcSectionIdx := funcIdx - m.ImportFunctionCount
	if funcSectionIdx >= uint32(len(m.FunctionSection)) {
		return nil
	}
	typeIdx := m.FunctionSection[funcSectionIdx]
	if typeIdx >= typeSectionLength {
		return nil
	}
	return &m.TypeSection[typeIdx]
}

func (m *Module) Validate(enabledFeatures api.CoreFeatures) error {
	for i := range m.TypeSection {
		tp := &m.TypeSection[i]
		tp.CacheNumInUint64()
	}

	if err := m.validateStartSection(); err != nil {
		return err
	}

	functions, globals, memory, tables, err := m.AllDeclarations()
	if err != nil {
		return err
	}

	if err = m.validateImports(enabledFeatures); err != nil {
		return err
	}

	if err = m.validateGlobals(globals, uint32(len(functions)), MaximumGlobals); err != nil {
		return err
	}

	if err = m.validateMemory(memory, globals, enabledFeatures); err != nil {
		return err
	}

	if err = m.validateExports(enabledFeatures, functions, globals, memory, tables); err != nil {
		return err
	}

	if m.CodeSection != nil {
		if err = m.validateFunctions(enabledFeatures, functions, globals, memory, tables, MaximumFunctionIndex); err != nil {
			return err
		}
	} // No need to validate host functions as NewHostModule validates

	if err = m.validateTable(enabledFeatures, tables, MaximumTableIndex); err != nil {
		return err
	}

	if err = m.validateDataCountSection(); err != nil {
		return err
	}
	return nil
}

func (m *Module) validateStartSection() error {
	// Check the start function is valid.
	// TODO: this should be verified during decode so that errors have the correct source positions
	if m.StartSection != nil {
		startIndex := *m.StartSection
		ft := m.typeOfFunction(startIndex)
		if ft == nil { // TODO: move this check to decoder so that a module can never be decoded invalidly
			return fmt.Errorf("invalid start function: func[%d] has an invalid type", startIndex)
		}
		if len(ft.Params) > 0 || len(ft.Results) > 0 {
			return fmt.Errorf("invalid start function: func[%d] must have an empty (nullary) signature: %s", startIndex, ft)
		}
	}
	return nil
}

func (m *Module) validateGlobals(globals []GlobalType, numFuncts, maxGlobals uint32) error {
	if uint32(len(globals)) > maxGlobals {
		return fmt.Errorf("too many globals in a module")
	}

	// Global initialization constant expression can only reference the imported globals.
	// See the note on https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#constant-expressions%E2%91%A0
	importedGlobals := globals[:m.ImportGlobalCount]
	for i := range m.GlobalSection {
		g := &m.GlobalSection[i]
		if err := validateConstExpression(importedGlobals, numFuncts, &g.Init, g.Type.ValType); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) validateFunctions(enabledFeatures api.CoreFeatures, functions []Index, globals []GlobalType, memory *Memory, tables []Table, maximumFunctionIndex uint32) error {
	if uint32(len(functions)) > maximumFunctionIndex {
		return fmt.Errorf("too many functions (%d) in a module", len(functions))
	}

	functionCount := m.SectionElementCount(SectionIDFunction)
	codeCount := m.SectionElementCount(SectionIDCode)
	if functionCount == 0 && codeCount == 0 {
		return nil
	}

	typeCount := m.SectionElementCount(SectionIDType)
	if codeCount != functionCount {
		return fmt.Errorf("code count (%d) != function count (%d)", codeCount, functionCount)
	}

	declaredFuncIndexes, err := m.declaredFunctionIndexes()
	if err != nil {
		return err
	}

	// Create bytes.Reader once as it causes allocation, and
	// we frequently need it (e.g. on every If instruction).
	br := bytes.NewReader(nil)
	// Also, we reuse the stacks across multiple function validations to reduce allocations.
	vs := &stacks{}
	for idx, typeIndex := range m.FunctionSection {
		if typeIndex >= typeCount {
			return fmt.Errorf("invalid %s: type section index %d out of range", m.funcDesc(SectionIDFunction, Index(idx)), typeIndex)
		}
		c := &m.CodeSection[idx]
		if c.GoFunc != nil {
			continue
		}
		if err = m.validateFunction(vs, enabledFeatures, Index(idx), functions, globals, memory, tables, declaredFuncIndexes, br); err != nil {
			return fmt.Errorf("invalid %s: %w", m.funcDesc(SectionIDFunction, Index(idx)), err)
		}
	}
	return nil
}

// declaredFunctionIndexes returns a set of function indexes that can be used as an immediate for OpcodeRefFunc instruction.
//
// The criteria for which function indexes can be available for that instruction is vague in the spec:
//
//   - "References: the list of function indices that occur in the module outside functions and can hence be used to form references inside them."
//   - https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/valid/conventions.html#contexts
//   - "Ref is the set funcidx(module with functions=ε, start=ε) , i.e., the set of function indices occurring in the module, except in its functions or start function."
//   - https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/valid/modules.html#valid-module
//
// To clarify, we reverse-engineer logic required to pass the WebAssembly Core specification 2.0 test suite:
// https://github.com/WebAssembly/spec/blob/d39195773112a22b245ffbe864bab6d1182ccb06/test/core/ref_func.wast#L78-L115
//
// To summarize, the function indexes OpcodeRefFunc can refer include:
//   - existing in an element section regardless of its mode (active, passive, declarative).
//   - defined as globals whose value type is ValueRefFunc.
//   - used as an exported function.
//
// See https://github.com/WebAssembly/reference-types/issues/31
// See https://github.com/WebAssembly/reference-types/issues/76
func (m *Module) declaredFunctionIndexes() (ret map[Index]struct{}, err error) {
	ret = map[uint32]struct{}{}

	for i := range m.ExportSection {
		exp := &m.ExportSection[i]
		if exp.Type == ExternTypeFunc {
			ret[exp.Index] = struct{}{}
		}
	}

	for i := range m.GlobalSection {
		g := &m.GlobalSection[i]
		if g.Init.Opcode == OpcodeRefFunc {
			var index uint32
			index, _, err = leb128.LoadUint32(g.Init.Data)
			if err != nil {
				err = fmt.Errorf("%s[%d] failed to initialize: %w", SectionIDName(SectionIDGlobal), i, err)
				return
			}
			ret[index] = struct{}{}
		}
	}

	for i := range m.ElementSection {
		elem := &m.ElementSection[i]
		for _, index := range elem.Init {
			if index != ElementInitNullReference {
				ret[index] = struct{}{}
			}
		}
	}
	return
}

func (m *Module) funcDesc(sectionID SectionID, sectionIndex Index) string {
	// Try to improve the error message by collecting any exports:
	var exportNames []string
	funcIdx := sectionIndex + m.ImportFunctionCount
	for i := range m.ExportSection {
		exp := &m.ExportSection[i]
		if exp.Index == funcIdx && exp.Type == ExternTypeFunc {
			exportNames = append(exportNames, fmt.Sprintf("%q", exp.Name))
		}
	}
	sectionIDName := SectionIDName(sectionID)
	if exportNames == nil {
		return fmt.Sprintf("%s[%d]", sectionIDName, sectionIndex)
	}
	sort.Strings(exportNames) // go map keys do not iterate consistently
	return fmt.Sprintf("%s[%d] export[%s]", sectionIDName, sectionIndex, strings.Join(exportNames, ","))
}

func (m *Module) validateMemory(memory *Memory, globals []GlobalType, _ api.CoreFeatures) error {
	var activeElementCount int
	for i := range m.DataSection {
		d := &m.DataSection[i]
		if !d.IsPassive() {
			activeElementCount++
		}
	}
	if activeElementCount > 0 && memory == nil {
		return fmt.Errorf("unknown memory")
	}

	// Constant expression can only reference imported globals.
	// https://github.com/WebAssembly/spec/blob/5900d839f38641989a9d8df2df4aee0513365d39/test/core/data.wast#L84-L91
	importedGlobals := globals[:m.ImportGlobalCount]
	for i := range m.DataSection {
		d := &m.DataSection[i]
		if !d.IsPassive() {
			if err := validateConstExpression(importedGlobals, 0, &d.OffsetExpression, ValueTypeI32); err != nil {
				return fmt.Errorf("calculate offset: %w", err)
			}
		}
	}
	return nil
}

func (m *Module) validateImports(enabledFeatures api.CoreFeatures) error {
	for i := range m.ImportSection {
		imp := &m.ImportSection[i]
		if imp.Module == "" {
			return fmt.Errorf("import[%d] has an empty module name", i)
		}
		switch imp.Type {
		case ExternTypeFunc:
			if int(imp.DescFunc) >= len(m.TypeSection) {
				return fmt.Errorf("invalid import[%q.%q] function: type index out of range", imp.Module, imp.Name)
			}
		case ExternTypeGlobal:
			if !imp.DescGlobal.Mutable {
				continue
			}
			if err := enabledFeatures.RequireEnabled(api.CoreFeatureMutableGlobal); err != nil {
				return fmt.Errorf("invalid import[%q.%q] global: %w", imp.Module, imp.Name, err)
			}
		}
	}
	return nil
}

func (m *Module) validateExports(enabledFeatures api.CoreFeatures, functions []Index, globals []GlobalType, memory *Memory, tables []Table) error {
	for i := range m.ExportSection {
		exp := &m.ExportSection[i]
		index := exp.Index
		switch exp.Type {
		case ExternTypeFunc:
			if index >= uint32(len(functions)) {
				return fmt.Errorf("unknown function for export[%q]", exp.Name)
			}
		case ExternTypeGlobal:
			if index >= uint32(len(globals)) {
				return fmt.Errorf("unknown global for export[%q]", exp.Name)
			}
			if !globals[index].Mutable {
				continue
			}
			if err := enabledFeatures.RequireEnabled(api.CoreFeatureMutableGlobal); err != nil {
				return fmt.Errorf("invalid export[%q] global[%d]: %w", exp.Name, index, err)
			}
		case ExternTypeMemory:
			if index > 0 || memory == nil {
				return fmt.Errorf("memory for export[%q] out of range", exp.Name)
			}
		case ExternTypeTable:
			if index >= uint32(len(tables)) {
				return fmt.Errorf("table for export[%q] out of range", exp.Name)
			}
		}
	}
	return nil
}

func validateConstExpression(globals []GlobalType, numFuncs uint32, expr *ConstantExpression, expectedType ValueType) (err error) {
	var actualType ValueType
	switch expr.Opcode {
	case OpcodeI32Const:
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		_, _, err = leb128.LoadInt32(expr.Data)
		if err != nil {
			return fmt.Errorf("read i32: %w", err)
		}
		actualType = ValueTypeI32
	case OpcodeI64Const:
		// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
		_, _, err = leb128.LoadInt64(expr.Data)
		if err != nil {
			return fmt.Errorf("read i64: %w", err)
		}
		actualType = ValueTypeI64
	case OpcodeF32Const:
		_, err = ieee754.DecodeFloat32(expr.Data)
		if err != nil {
			return fmt.Errorf("read f32: %w", err)
		}
		actualType = ValueTypeF32
	case OpcodeF64Const:
		_, err = ieee754.DecodeFloat64(expr.Data)
		if err != nil {
			return fmt.Errorf("read f64: %w", err)
		}
		actualType = ValueTypeF64
	case OpcodeGlobalGet:
		id, _, err := leb128.LoadUint32(expr.Data)
		if err != nil {
			return fmt.Errorf("read index of global: %w", err)
		}
		if uint32(len(globals)) <= id {
			return fmt.Errorf("global index out of range")
		}
		actualType = globals[id].ValType
	case OpcodeRefNull:
		if len(expr.Data) == 0 {
			return fmt.Errorf("read reference type for ref.null: %w", io.ErrShortBuffer)
		}
		reftype := expr.Data[0]
		if reftype != RefTypeFuncref && reftype != RefTypeExternref {
			return fmt.Errorf("invalid type for ref.null: 0x%x", reftype)
		}
		actualType = reftype
	case OpcodeRefFunc:
		index, _, err := leb128.LoadUint32(expr.Data)
		if err != nil {
			return fmt.Errorf("read i32: %w", err)
		} else if index >= numFuncs {
			return fmt.Errorf("ref.func index out of range [%d] with length %d", index, numFuncs-1)
		}
		actualType = ValueTypeFuncref
	case OpcodeVecV128Const:
		if len(expr.Data) != 16 {
			return fmt.Errorf("%s needs 16 bytes but was %d bytes", OpcodeVecV128ConstName, len(expr.Data))
		}
		actualType = ValueTypeV128
	default:
		return fmt.Errorf("invalid opcode for const expression: 0x%x", expr.Opcode)
	}

	if actualType != expectedType {
		return fmt.Errorf("const expression type mismatch expected %s but got %s",
			ValueTypeName(expectedType), ValueTypeName(actualType))
	}
	return nil
}

func (m *Module) validateDataCountSection() (err error) {
	if m.DataCountSection != nil && int(*m.DataCountSection) != len(m.DataSection) {
		err = fmt.Errorf("data count section (%d) doesn't match the length of data section (%d)",
			*m.DataCountSection, len(m.DataSection))
	}
	return
}

func (m *ModuleInstance) buildGlobals(module *Module, funcRefResolver func(funcIndex Index) Reference) {
	importedGlobals := m.Globals[:module.ImportGlobalCount]

	me := m.Engine
	engineOwnGlobal := me.OwnsGlobals()
	for i := Index(0); i < Index(len(module.GlobalSection)); i++ {
		gs := &module.GlobalSection[i]
		g := &GlobalInstance{}
		if engineOwnGlobal {
			g.Me = me
			g.Index = i + module.ImportGlobalCount
		}
		m.Globals[i+module.ImportGlobalCount] = g
		g.Type = gs.Type
		g.initialize(importedGlobals, &gs.Init, funcRefResolver)
	}
}

func paramNames(localNames IndirectNameMap, funcIdx uint32, paramLen int) []string {
	for i := range localNames {
		nm := &localNames[i]
		// Only build parameter names if we have one for each.
		if nm.Index != funcIdx || len(nm.NameMap) < paramLen {
			continue
		}

		ret := make([]string, paramLen)
		for j := range nm.NameMap {
			p := &nm.NameMap[j]
			if int(p.Index) < paramLen {
				ret[p.Index] = p.Name
			}
		}
		return ret
	}
	return nil
}

func (m *ModuleInstance) buildMemory(module *Module, allocator experimental.MemoryAllocator) {
	memSec := module.MemorySection
	if memSec != nil {
		m.MemoryInstance = NewMemoryInstance(memSec, allocator, m.Engine)
		m.MemoryInstance.definition = &module.MemoryDefinitionSection[0]
	}
}

// Index is the offset in an index, not necessarily an absolute position in a Module section. This is because
// indexs are often preceded by a corresponding type in the Module.ImportSection.
//
// For example, the function index starts with any ExternTypeFunc in the Module.ImportSection followed by
// the Module.FunctionSection
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-index
type Index = uint32

// FunctionType is a possibly empty function signature.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#function-types%E2%91%A0
type FunctionType struct {
	// Params are the possibly empty sequence of value types accepted by a function with this signature.
	Params []ValueType

	// Results are the possibly empty sequence of value types returned by a function with this signature.
	//
	// Note: In WebAssembly 1.0 (20191205), there can be at most one result.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#result-types%E2%91%A0
	Results []ValueType

	// string is cached as it is used both for String and key
	string string

	// ParamNumInUint64 is the number of uint64 values requires to represent the Wasm param type.
	ParamNumInUint64 int

	// ResultsNumInUint64 is the number of uint64 values requires to represent the Wasm result type.
	ResultNumInUint64 int
}

func (f *FunctionType) CacheNumInUint64() {
	if f.ParamNumInUint64 == 0 {
		for _, tp := range f.Params {
			f.ParamNumInUint64++
			if tp == ValueTypeV128 {
				f.ParamNumInUint64++
			}
		}
	}

	if f.ResultNumInUint64 == 0 {
		for _, tp := range f.Results {
			f.ResultNumInUint64++
			if tp == ValueTypeV128 {
				f.ResultNumInUint64++
			}
		}
	}
}

// EqualsSignature returns true if the function type has the same parameters and results.
func (f *FunctionType) EqualsSignature(params []ValueType, results []ValueType) bool {
	return bytes.Equal(f.Params, params) && bytes.Equal(f.Results, results)
}

// key gets or generates the key for Store.typeIDs. e.g. "i32_v" for one i32 parameter and no (void) result.
func (f *FunctionType) key() string {
	if f.string != "" {
		return f.string
	}
	var ret string
	for _, b := range f.Params {
		ret += ValueTypeName(b)
	}
	if len(f.Params) == 0 {
		ret += "v_"
	} else {
		ret += "_"
	}
	for _, b := range f.Results {
		ret += ValueTypeName(b)
	}
	if len(f.Results) == 0 {
		ret += "v"
	}
	f.string = ret
	return ret
}

// String implements fmt.Stringer.
func (f *FunctionType) String() string {
	return f.key()
}

// Import is the binary representation of an import indicated by Type
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-import
type Import struct {
	Type ExternType
	// Module is the possibly empty primary namespace of this import
	Module string
	// Module is the possibly empty secondary namespace of this import
	Name string
	// DescFunc is the index in Module.TypeSection when Type equals ExternTypeFunc
	DescFunc Index
	// DescTable is the inlined Table when Type equals ExternTypeTable
	DescTable Table
	// DescMem is the inlined Memory when Type equals ExternTypeMemory
	DescMem *Memory
	// DescGlobal is the inlined GlobalType when Type equals ExternTypeGlobal
	DescGlobal GlobalType
	// IndexPerType has the index of this import per ExternType.
	IndexPerType Index
}

// Memory describes the limits of pages (64KB) in a memory.
type Memory struct {
	Min, Cap, Max uint32
	// IsMaxEncoded true if the Max is encoded in the original binary.
	IsMaxEncoded bool
	// IsShared true if the memory is shared for access from multiple agents.
	IsShared bool
}

// Validate ensures values assigned to Min, Cap and Max are within valid thresholds.
func (m *Memory) Validate(memoryLimitPages uint32) error {
	min, capacity, max := m.Min, m.Cap, m.Max

	if max > memoryLimitPages {
		return fmt.Errorf("max %d pages (%s) over limit of %d pages (%s)",
			max, PagesToUnitOfBytes(max), memoryLimitPages, PagesToUnitOfBytes(memoryLimitPages))
	} else if min > memoryLimitPages {
		return fmt.Errorf("min %d pages (%s) over limit of %d pages (%s)",
			min, PagesToUnitOfBytes(min), memoryLimitPages, PagesToUnitOfBytes(memoryLimitPages))
	} else if min > max {
		return fmt.Errorf("min %d pages (%s) > max %d pages (%s)",
			min, PagesToUnitOfBytes(min), max, PagesToUnitOfBytes(max))
	} else if capacity < min {
		return fmt.Errorf("capacity %d pages (%s) less than minimum %d pages (%s)",
			capacity, PagesToUnitOfBytes(capacity), min, PagesToUnitOfBytes(min))
	} else if capacity > memoryLimitPages {
		return fmt.Errorf("capacity %d pages (%s) over limit of %d pages (%s)",
			capacity, PagesToUnitOfBytes(capacity), memoryLimitPages, PagesToUnitOfBytes(memoryLimitPages))
	}
	return nil
}

type GlobalType struct {
	ValType ValueType
	Mutable bool
}

type Global struct {
	Type GlobalType
	Init ConstantExpression
}

type ConstantExpression struct {
	Opcode Opcode
	Data   []byte
}

// Export is the binary representation of an export indicated by Type
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-export
type Export struct {
	Type ExternType

	// Name is what the host refers to this definition as.
	Name string

	// Index is the index of the definition to export, the index is by Type
	// e.g. If ExternTypeFunc, this is a position in the function index.
	Index Index
}

// Code is an entry in the Module.CodeSection containing the locals and body of the function.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-code
type Code struct {
	// LocalTypes are any function-scoped variables in insertion order.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-local
	LocalTypes []ValueType

	// Body is a sequence of expressions ending in OpcodeEnd
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-expr
	Body []byte

	// GoFunc is non-nil when IsHostFunction and defined in go, either
	// api.GoFunction or api.GoModuleFunction. When present, LocalTypes and Body must
	// be nil.
	//
	// Note: This has no serialization format, so is not encodable.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#host-functions%E2%91%A2
	GoFunc interface{}

	// BodyOffsetInCodeSection is the offset of the beginning of the body in the code section.
	// This is used for DWARF based stack trace where a program counter represents an offset in code section.
	BodyOffsetInCodeSection uint64
}

type DataSegment struct {
	OffsetExpression ConstantExpression
	Init             []byte
	Passive          bool
}

// IsPassive returns true if this data segment is "passive" in the sense that memory offset and
// index is determined at runtime and used by OpcodeMemoryInitName instruction in the bulk memory
// operations proposal.
//
// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/appendix/changes.html#bulk-memory-and-table-instructions
func (d *DataSegment) IsPassive() bool {
	return d.Passive
}

// NameSection represent the known custom name subsections defined in the WebAssembly Binary Format
//
// Note: This can be nil if no names were decoded for any reason including configuration.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#name-section%E2%91%A0
type NameSection struct {
	// ModuleName is the symbolic identifier for a module. e.g. math
	//
	// Note: This can be empty for any reason including configuration.
	ModuleName string

	// FunctionNames is an association of a function index to its symbolic identifier. e.g. add
	//
	// * the key (idx) is in the function index, where module defined functions are preceded by imported ones.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#functions%E2%91%A7
	//
	// For example, assuming the below text format is the second import, you would expect FunctionNames[1] = "mul"
	//	(import "Math" "Mul" (func $mul (param $x f32) (param $y f32) (result f32)))
	//
	// Note: FunctionNames are only used for debugging. At runtime, functions are called based on raw numeric index.
	// Note: This can be nil for any reason including configuration.
	FunctionNames NameMap

	// LocalNames contains symbolic names for function parameters or locals that have one.
	//
	// Note: In the Text Format, function local names can inherit parameter
	// names from their type. Here are some examples:
	//  * (module (import (func (param $x i32) (param i32))) (func (type 0))) = [{0, {x,0}}]
	//  * (module (import (func (param i32) (param $y i32))) (func (type 0) (local $z i32))) = [0, [{y,1},{z,2}]]
	//  * (module (func (param $x i32) (local $y i32) (local $z i32))) = [{x,0},{y,1},{z,2}]
	//
	// Note: LocalNames are only used for debugging. At runtime, locals are called based on raw numeric index.
	// Note: This can be nil for any reason including configuration.
	LocalNames IndirectNameMap

	// ResultNames is a wazero-specific mechanism to store result names.
	ResultNames IndirectNameMap
}

// CustomSection contains the name and raw data of a custom section.
type CustomSection struct {
	Name string
	Data []byte
}

// NameMap associates an index with any associated names.
//
// Note: Often the index bridges multiple sections. For example, the function index starts with any
// ExternTypeFunc in the Module.ImportSection followed by the Module.FunctionSection
//
// Note: NameMap is unique by NameAssoc.Index, but NameAssoc.Name needn't be unique.
// Note: When encoding in the Binary format, this must be ordered by NameAssoc.Index
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-namemap
type NameMap []NameAssoc

type NameAssoc struct {
	Index Index
	Name  string
}

// IndirectNameMap associates an index with an association of names.
//
// Note: IndirectNameMap is unique by NameMapAssoc.Index, but NameMapAssoc.NameMap needn't be unique.
// Note: When encoding in the Binary format, this must be ordered by NameMapAssoc.Index
// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-indirectnamemap
type IndirectNameMap []NameMapAssoc

type NameMapAssoc struct {
	Index   Index
	NameMap NameMap
}

// AllDeclarations returns all declarations for functions, globals, memories and tables in a module including imported ones.
func (m *Module) AllDeclarations() (functions []Index, globals []GlobalType, memory *Memory, tables []Table, err error) {
	for i := range m.ImportSection {
		imp := &m.ImportSection[i]
		switch imp.Type {
		case ExternTypeFunc:
			functions = append(functions, imp.DescFunc)
		case ExternTypeGlobal:
			globals = append(globals, imp.DescGlobal)
		case ExternTypeMemory:
			memory = imp.DescMem
		case ExternTypeTable:
			tables = append(tables, imp.DescTable)
		}
	}

	functions = append(functions, m.FunctionSection...)
	for i := range m.GlobalSection {
		g := &m.GlobalSection[i]
		globals = append(globals, g.Type)
	}
	if m.MemorySection != nil {
		if memory != nil { // shouldn't be possible due to Validate
			err = errors.New("at most one table allowed in module")
			return
		}
		memory = m.MemorySection
	}
	if m.TableSection != nil {
		tables = append(tables, m.TableSection...)
	}
	return
}

// SectionID identifies the sections of a Module in the WebAssembly 1.0 (20191205) Binary Format.
//
// Note: these are defined in the wasm package, instead of the binary package, as a key per section is needed regardless
// of format, and deferring to the binary type avoids confusion.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#sections%E2%91%A0
type SectionID = byte

const (
	// SectionIDCustom includes the standard defined NameSection and possibly others not defined in the standard.
	SectionIDCustom SectionID = iota // don't add anything not in https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#sections%E2%91%A0
	SectionIDType
	SectionIDImport
	SectionIDFunction
	SectionIDTable
	SectionIDMemory
	SectionIDGlobal
	SectionIDExport
	SectionIDStart
	SectionIDElement
	SectionIDCode
	SectionIDData

	// SectionIDDataCount may exist in WebAssembly 2.0 or WebAssembly 1.0 with CoreFeatureBulkMemoryOperations enabled.
	//
	// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#data-count-section
	// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/appendix/changes.html#bulk-memory-and-table-instructions
	SectionIDDataCount
)

// SectionIDName returns the canonical name of a module section.
// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#sections%E2%91%A0
func SectionIDName(sectionID SectionID) string {
	switch sectionID {
	case SectionIDCustom:
		return "custom"
	case SectionIDType:
		return "type"
	case SectionIDImport:
		return "import"
	case SectionIDFunction:
		return "function"
	case SectionIDTable:
		return "table"
	case SectionIDMemory:
		return "memory"
	case SectionIDGlobal:
		return "global"
	case SectionIDExport:
		return "export"
	case SectionIDStart:
		return "start"
	case SectionIDElement:
		return "element"
	case SectionIDCode:
		return "code"
	case SectionIDData:
		return "data"
	case SectionIDDataCount:
		return "data_count"
	}
	return "unknown"
}

// ValueType is an alias of api.ValueType defined to simplify imports.
type ValueType = api.ValueType

const (
	ValueTypeI32 = api.ValueTypeI32
	ValueTypeI64 = api.ValueTypeI64
	ValueTypeF32 = api.ValueTypeF32
	ValueTypeF64 = api.ValueTypeF64
	// TODO: ValueTypeV128 is not exposed in the api pkg yet.
	ValueTypeV128 ValueType = 0x7b
	// TODO: ValueTypeFuncref is not exposed in the api pkg yet.
	ValueTypeFuncref   ValueType = 0x70
	ValueTypeExternref           = api.ValueTypeExternref
)

// ValueTypeName is an alias of api.ValueTypeName defined to simplify imports.
func ValueTypeName(t ValueType) string {
	if t == ValueTypeFuncref {
		return "funcref"
	} else if t == ValueTypeV128 {
		return "v128"
	}
	return api.ValueTypeName(t)
}

func isReferenceValueType(vt ValueType) bool {
	return vt == ValueTypeExternref || vt == ValueTypeFuncref
}

// ExternType is an alias of api.ExternType defined to simplify imports.
type ExternType = api.ExternType

const (
	ExternTypeFunc       = api.ExternTypeFunc
	ExternTypeFuncName   = api.ExternTypeFuncName
	ExternTypeTable      = api.ExternTypeTable
	ExternTypeTableName  = api.ExternTypeTableName
	ExternTypeMemory     = api.ExternTypeMemory
	ExternTypeMemoryName = api.ExternTypeMemoryName
	ExternTypeGlobal     = api.ExternTypeGlobal
	ExternTypeGlobalName = api.ExternTypeGlobalName
)

// ExternTypeName is an alias of api.ExternTypeName defined to simplify imports.
func ExternTypeName(t ValueType) string {
	return api.ExternTypeName(t)
}
