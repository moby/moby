package wasm

import (
	"fmt"
	"math"
	"sync"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Table describes the limits of elements and its type in a table.
type Table struct {
	Min  uint32
	Max  *uint32
	Type RefType
}

// RefType is either RefTypeFuncref or RefTypeExternref as of WebAssembly core 2.0.
type RefType = byte

const (
	// RefTypeFuncref represents a reference to a function.
	RefTypeFuncref = ValueTypeFuncref
	// RefTypeExternref represents a reference to a host object, which is not currently supported in wazero.
	RefTypeExternref = ValueTypeExternref
)

func RefTypeName(t RefType) (ret string) {
	switch t {
	case RefTypeFuncref:
		ret = "funcref"
	case RefTypeExternref:
		ret = "externref"
	default:
		ret = fmt.Sprintf("unknown(0x%x)", t)
	}
	return
}

// ElementMode represents a mode of element segment which is either active, passive or declarative.
//
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/syntax/modules.html#element-segments
type ElementMode = byte

const (
	// ElementModeActive is the mode which requires the runtime to initialize table with the contents in .Init field combined with OffsetExpr.
	ElementModeActive ElementMode = iota
	// ElementModePassive is the mode which doesn't require the runtime to initialize table, and only used with OpcodeTableInitName.
	ElementModePassive
	// ElementModeDeclarative is introduced in reference-types proposal which can be used to declare function indexes used by OpcodeRefFunc.
	ElementModeDeclarative
)

// ElementSegment are initialization instructions for a TableInstance
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-elem
type ElementSegment struct {
	// OffsetExpr returns the table element offset to apply to Init indices.
	// Note: This can be validated prior to instantiation unless it includes OpcodeGlobalGet (an imported global).
	OffsetExpr ConstantExpression

	// TableIndex is the table's index to which this element segment is applied.
	// Note: This is used if and only if the Mode is active.
	TableIndex Index

	// Followings are set/used regardless of the Mode.

	// Init indices are (nullable) table elements where each index is the function index by which the module initialize the table.
	Init []Index

	// Type holds the type of this element segment, which is the RefType in WebAssembly 2.0.
	Type RefType

	// Mode is the mode of this element segment.
	Mode ElementMode
}

const (
	// ElementInitNullReference represents the null reference in ElementSegment's Init.
	// In Wasm spec, an init item represents either Function's Index or null reference,
	// and in wazero, we limit the maximum number of functions available in a module to
	// MaximumFunctionIndex. Therefore, it is safe to use 1 << 31 to represent the null
	// reference in Element segments.
	ElementInitNullReference Index = 1 << 31
	// elementInitImportedGlobalReferenceType represents an init item which is resolved via an imported global constexpr.
	// The actual function reference stored at Global is only known at instantiation-time, so we set this flag
	// to items of ElementSegment.Init at binary decoding, and unwrap this flag at instantiation to resolve the value.
	//
	// This might collide the init element resolved via ref.func instruction which is resolved with the func index at decoding,
	// but in practice, that is not allowed in wazero thanks to our limit MaximumFunctionIndex. Thus, it is safe to set this flag
	// in init element to indicate as such.
	elementInitImportedGlobalReferenceType Index = 1 << 30
)

// unwrapElementInitGlobalReference takes an item of the init vector of an ElementSegment,
// and returns the Global index if it is supposed to get generated from a global.
// ok is true if the given init item is as such.
func unwrapElementInitGlobalReference(init Index) (_ Index, ok bool) {
	if init&elementInitImportedGlobalReferenceType == elementInitImportedGlobalReferenceType {
		return init &^ elementInitImportedGlobalReferenceType, true
	}
	return init, false
}

// WrapGlobalIndexAsElementInit wraps the given index as an init item which is resolved via an imported global value.
// See the comments on elementInitImportedGlobalReferenceType for more details.
func WrapGlobalIndexAsElementInit(init Index) Index {
	return init | elementInitImportedGlobalReferenceType
}

// IsActive returns true if the element segment is "active" mode which requires the runtime to initialize table
// with the contents in .Init field.
func (e *ElementSegment) IsActive() bool {
	return e.Mode == ElementModeActive
}

// TableInstance represents a table of (RefTypeFuncref) elements in a module.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#table-instances%E2%91%A0
type TableInstance struct {
	// References holds references whose type is either RefTypeFuncref or RefTypeExternref (unsupported).
	//
	// Currently, only function references are supported.
	References []Reference

	// Min is the minimum (function) elements in this table and cannot grow to accommodate ElementSegment.
	Min uint32

	// Max if present is the maximum (function) elements in this table, or nil if unbounded.
	Max *uint32

	// Type is either RefTypeFuncref or RefTypeExternRef.
	Type RefType

	// The following is only used when the table is exported.

	// involvingModuleInstances is a set of module instances which are involved in the table instance.
	// This is critical for safety purpose because once a table is imported, it can hold any reference to
	// any function in the owner and importing module instances. Therefore, these module instance,
	// transitively the compiled modules, must be alive as long as the table instance is alive.
	involvingModuleInstances []*ModuleInstance
	// involvingModuleInstancesMutex is a mutex to protect involvingModuleInstances.
	involvingModuleInstancesMutex sync.RWMutex
}

// ElementInstance represents an element instance in a module.
//
// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/runtime.html#element-instances
type ElementInstance = []Reference

// Reference is the runtime representation of RefType which is either RefTypeFuncref or RefTypeExternref.
type Reference = uintptr

// validateTable ensures any ElementSegment is valid. This caches results via Module.validatedActiveElementSegments.
// Note: limitsType are validated by decoders, so not re-validated here.
func (m *Module) validateTable(enabledFeatures api.CoreFeatures, tables []Table, maximumTableIndex uint32) error {
	if len(tables) > int(maximumTableIndex) {
		return fmt.Errorf("too many tables in a module: %d given with limit %d", len(tables), maximumTableIndex)
	}

	importedTableCount := m.ImportTableCount

	// Create bounds checks as these can err prior to instantiation
	funcCount := m.ImportFunctionCount + m.SectionElementCount(SectionIDFunction)
	globalsCount := m.ImportGlobalCount + m.SectionElementCount(SectionIDGlobal)

	// Now, we have to figure out which table elements can be resolved before instantiation and also fail early if there
	// are any imported globals that are known to be invalid by their declarations.
	for i := range m.ElementSection {
		elem := &m.ElementSection[i]
		idx := Index(i)
		initCount := uint32(len(elem.Init))

		// Any offset applied is to the element, not the function index: validate here if the funcidx is sound.
		for ei, init := range elem.Init {
			if init == ElementInitNullReference {
				continue
			}
			index, ok := unwrapElementInitGlobalReference(init)
			if ok {
				if index >= globalsCount {
					return fmt.Errorf("%s[%d].init[%d] global index %d out of range", SectionIDName(SectionIDElement), idx, ei, index)
				}
			} else {
				if elem.Type == RefTypeExternref {
					return fmt.Errorf("%s[%d].init[%d] must be ref.null but was %d", SectionIDName(SectionIDElement), idx, ei, init)
				}
				if index >= funcCount {
					return fmt.Errorf("%s[%d].init[%d] func index %d out of range", SectionIDName(SectionIDElement), idx, ei, index)
				}
			}
		}

		if elem.IsActive() {
			if len(tables) <= int(elem.TableIndex) {
				return fmt.Errorf("unknown table %d as active element target", elem.TableIndex)
			}

			t := tables[elem.TableIndex]
			if t.Type != elem.Type {
				return fmt.Errorf("element type mismatch: table has %s but element has %s",
					RefTypeName(t.Type), RefTypeName(elem.Type),
				)
			}

			// global.get needs to be discovered during initialization
			oc := elem.OffsetExpr.Opcode
			if oc == OpcodeGlobalGet {
				globalIdx, _, err := leb128.LoadUint32(elem.OffsetExpr.Data)
				if err != nil {
					return fmt.Errorf("%s[%d] couldn't read global.get parameter: %w", SectionIDName(SectionIDElement), idx, err)
				} else if err = m.verifyImportGlobalI32(SectionIDElement, idx, globalIdx); err != nil {
					return err
				}
			} else if oc == OpcodeI32Const {
				// Per https://github.com/WebAssembly/spec/blob/wg-1.0/test/core/elem.wast#L117 we must pass if imported
				// table has set its min=0. Per https://github.com/WebAssembly/spec/blob/wg-1.0/test/core/elem.wast#L142, we
				// have to do fail if module-defined min=0.
				if !enabledFeatures.IsEnabled(api.CoreFeatureReferenceTypes) && elem.TableIndex >= importedTableCount {
					// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
					o, _, err := leb128.LoadInt32(elem.OffsetExpr.Data)
					if err != nil {
						return fmt.Errorf("%s[%d] couldn't read i32.const parameter: %w", SectionIDName(SectionIDElement), idx, err)
					}
					offset := Index(o)
					if err = checkSegmentBounds(t.Min, uint64(initCount)+uint64(offset), idx); err != nil {
						return err
					}
				}
			} else {
				return fmt.Errorf("%s[%d] has an invalid const expression: %s", SectionIDName(SectionIDElement), idx, InstructionName(oc))
			}
		}
	}
	return nil
}

// buildTable returns TableInstances if the module defines or imports a table.
//   - importedTables: returned as `tables` unmodified.
//   - importedGlobals: include all instantiated, imported globals.
//
// If the result `init` is non-nil, it is the `tableInit` parameter of Engine.NewModuleEngine.
//
// Note: An error is only possible when an ElementSegment.OffsetExpr is out of range of the TableInstance.Min.
func (m *ModuleInstance) buildTables(module *Module, skipBoundCheck bool) (err error) {
	idx := module.ImportTableCount
	for i := range module.TableSection {
		tsec := &module.TableSection[i]
		// The module defining the table is the one that sets its Min/Max etc.
		m.Tables[idx] = &TableInstance{
			References: make([]Reference, tsec.Min), Min: tsec.Min, Max: tsec.Max,
			Type: tsec.Type,
		}
		idx++
	}

	if !skipBoundCheck {
		for elemI := range module.ElementSection { // Do not loop over the value since elementSegments is a slice of value.
			elem := &module.ElementSection[elemI]
			table := m.Tables[elem.TableIndex]
			var offset uint32
			if elem.OffsetExpr.Opcode == OpcodeGlobalGet {
				// Ignore error as it's already validated.
				globalIdx, _, _ := leb128.LoadUint32(elem.OffsetExpr.Data)
				global := m.Globals[globalIdx]
				offset = uint32(global.Val)
			} else { // i32.const
				// Ignore error as it's already validated.
				o, _, _ := leb128.LoadInt32(elem.OffsetExpr.Data)
				offset = uint32(o)
			}

			// Check to see if we are out-of-bounds
			initCount := uint64(len(elem.Init))
			if err = checkSegmentBounds(table.Min, uint64(offset)+initCount, Index(elemI)); err != nil {
				return
			}
		}
	}
	return
}

// checkSegmentBounds fails if the capacity needed for an ElementSegment.Init is larger than limitsType.Min
//
// WebAssembly 1.0 (20191205) doesn't forbid growing to accommodate element segments, and spectests are inconsistent.
// For example, the spectests enforce elements within Table limitsType.Min, but ignore Import.DescTable min. What this
// means is we have to delay offset checks on imported tables until we link to them.
// e.g. https://github.com/WebAssembly/spec/blob/wg-1.0/test/core/elem.wast#L117 wants pass on min=0 for import
// e.g. https://github.com/WebAssembly/spec/blob/wg-1.0/test/core/elem.wast#L142 wants fail on min=0 module-defined
func checkSegmentBounds(min uint32, requireMin uint64, idx Index) error { // uint64 in case offset was set to -1
	if requireMin > uint64(min) {
		return fmt.Errorf("%s[%d].init exceeds min table size", SectionIDName(SectionIDElement), idx)
	}
	return nil
}

func (m *Module) verifyImportGlobalI32(sectionID SectionID, sectionIdx Index, idx uint32) error {
	ig := uint32(math.MaxUint32) // +1 == 0
	for i := range m.ImportSection {
		imp := &m.ImportSection[i]
		if imp.Type == ExternTypeGlobal {
			ig++
			if ig == idx {
				if imp.DescGlobal.ValType != ValueTypeI32 {
					return fmt.Errorf("%s[%d] (global.get %d): import[%d].global.ValType != i32", SectionIDName(sectionID), sectionIdx, idx, i)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("%s[%d] (global.get %d): out of range of imported globals", SectionIDName(sectionID), sectionIdx, idx)
}

// Grow appends the `initialRef` by `delta` times into the References slice.
// Returns -1 if the operation is not valid, otherwise the old length of the table.
//
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/instructions.html#xref-syntax-instructions-syntax-instr-table-mathsf-table-grow-x
func (t *TableInstance) Grow(delta uint32, initialRef Reference) (currentLen uint32) {
	currentLen = uint32(len(t.References))
	if delta == 0 {
		return
	}

	if newLen := int64(currentLen) + int64(delta); // adding as 64bit ints to avoid overflow.
	newLen >= math.MaxUint32 || (t.Max != nil && newLen > int64(*t.Max)) {
		return 0xffffffff // = -1 in signed 32-bit integer.
	}
	t.References = append(t.References, make([]uintptr, delta)...)

	// Uses the copy trick for faster filling the new region with the initial value.
	// https://gist.github.com/taylorza/df2f89d5f9ab3ffd06865062a4cf015d
	newRegion := t.References[currentLen:]
	newRegion[0] = initialRef
	for i := 1; i < len(newRegion); i *= 2 {
		copy(newRegion[i:], newRegion[:i])
	}
	return
}
